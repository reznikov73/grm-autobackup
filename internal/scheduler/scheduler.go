package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"grm-autobackup/internal/api"
	"grm-autobackup/internal/cleaner"
	"grm-autobackup/internal/config"
	"grm-autobackup/internal/downloader"
	"grm-autobackup/internal/monitor"
)

// Scheduler планирует и запускает задачи
type Scheduler struct {
	cfg     *config.Config
	client  *api.Client
	dl      *downloader.Downloader
	cln     *cleaner.Cleaner
	monitor *monitor.Monitor

	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once

	cycleActive atomic.Bool
	nextRunUnix atomic.Int64

	startMu    sync.Mutex
	started    bool
	loopCancel context.CancelFunc
}

// New создает новый планировщик
func New(cfg *config.Config, m *monitor.Monitor) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		cfg:     cfg,
		client:  api.New(cfg.API.URL, cfg.API.JWTToken, cfg.API.TimeoutSec),
		dl:      downloader.New(m),
		cln:     cleaner.New(cfg.Storage.BackupLimit, m),
		monitor: m,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start запускает планировщик
func (s *Scheduler) Start() {
	if !s.cfg.Schedule.Enabled {
		s.monitor.Infof("Планировщик отключен в настройках")
		return
	}
	s.startMu.Lock()
	defer s.startMu.Unlock()
	s.started = true
	s.startLoopLocked(true)
}

// Reload применяет изменённые настройки без перезапуска приложения
func (s *Scheduler) Reload() {
	s.startMu.Lock()
	defer s.startMu.Unlock()

	s.client = api.New(s.cfg.API.URL, s.cfg.API.JWTToken, s.cfg.API.TimeoutSec)
	s.cln = cleaner.New(s.cfg.Storage.BackupLimit, s.monitor)

	if s.started {
		s.startLoopLocked(false)
	}
	s.monitor.Infof("Настройки применены (клиент, лимит бэкапов, расписание)")
}

// Stop останавливает планировщик (идемпотентно)
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		s.monitor.Infof("Остановка планировщика...")
		s.cancel()
		s.startMu.Lock()
		if s.loopCancel != nil {
			s.loopCancel()
		}
		s.startMu.Unlock()
		s.monitor.Infof("Планировщик остановлен")
	})
}

// RunNow запускает задачу вручную
func (s *Scheduler) RunNow() {
	go func() {
		s.monitor.Infof("Ручной запуск...")
		s.runBackupCycle()
	}()
}

// NextRun возвращает время следующего цикла (нулевое значение, если неизвестно)
func (s *Scheduler) NextRun() time.Time {
	v := s.nextRunUnix.Load()
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(v, 0)
}

// startLoopLocked запускает (или перезапускает) цикл расписания. Вызывать под startMu.
func (s *Scheduler) startLoopLocked(immediate bool) {
	if s.loopCancel != nil {
		s.loopCancel()
	}
	loopCtx, cancel := context.WithCancel(context.Background())
	s.loopCancel = cancel
	go s.runLoop(loopCtx, immediate)
}

func (s *Scheduler) runLoop(ctx context.Context, immediate bool) {
	first := immediate
	for {
		var wait time.Duration
		if s.cfg.Schedule.Mode == "daily" {
			target, err := nextDailyTime(s.cfg.Schedule.StartTime, time.Now())
			if err != nil {
				s.monitor.Warnf("Некорректное время расписания %q (%v) — перехожу на интервал", s.cfg.Schedule.StartTime, err)
				wait = s.intervalDuration()
			} else {
				wait = time.Until(target)
			}
		} else if !first {
			wait = s.intervalDuration()
		}
		first = false

		if wait > 0 {
			s.setNextRun(time.Now().Add(wait))
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return
			}
			if ctx.Err() != nil {
				return
			}
		}

		s.setNextRun(time.Now())
		s.runBackupCycle()
		if ctx.Err() != nil {
			return
		}
	}
}

func (s *Scheduler) intervalDuration() time.Duration {
	h := s.cfg.Schedule.IntervalHours
	if h <= 0 {
		h = 6
	}
	return time.Duration(h * float64(time.Hour))
}

// nextDailyTime вычисляет ближайший момент ЧЧ:ММ в локальном времени
func nextDailyTime(hhmm string, now time.Time) (time.Time, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, err
	}
	target := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if !target.After(now) {
		target = target.Add(24 * time.Hour)
	}
	return target, nil
}

func (s *Scheduler) setNextRun(t time.Time) {
	s.nextRunUnix.Store(t.Unix())
}

func (s *Scheduler) runBackupCycle() {
	if !s.cycleActive.CompareAndSwap(false, true) {
		s.monitor.Warnf("Цикл уже выполняется — пропускаю повторный запуск")
		return
	}
	defer s.cycleActive.Store(false)

	s.monitor.Infof("Начало цикла бэкапов")
	defer s.monitor.Infof("Цикл бэкапов завершен")

	s.startMu.Lock()
	client := s.client
	cln := s.cln
	s.startMu.Unlock()

	apps, err := client.GetApplications(s.ctx)
	if err != nil {
		s.monitor.Errorf("Ошибка получения приложений: %v", err)
		return
	}
	if len(apps) == 0 {
		s.monitor.Warnf("Сервер вернул пустой список приложений")
		return
	}
	s.monitor.Infof("Найдено приложений: %d", len(apps))

	// Фильтр по статусу и исключениям
	var candidates []api.Application
	for _, app := range apps {
		statusOK := app.Status == "running" || (s.cfg.IncludeStopped && app.Status == "stopped")
		if !statusOK {
			continue
		}
		if s.isExcluded(app) {
			s.monitor.Infof("Исключено настройками: %s", app.Name)
			continue
		}
		candidates = append(candidates, app)
	}
	s.monitor.Infof("К обработке: %d приложений", len(candidates))

	if len(candidates) > 0 {
		s.dl.DownloadAll(s.ctx, client, candidates, s.cfg.Storage.Path)
	}
	cln.Cleanup(s.cfg.Storage.Path)
}

func (s *Scheduler) isExcluded(app api.Application) bool {
	for _, id := range s.cfg.Exclusions.ApplicationIDs {
		if id == app.ID {
			return true
		}
	}
	for _, name := range s.cfg.Exclusions.ApplicationNames {
		if name == app.Name {
			return true
		}
	}
	return false
}
