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

	ctx         context.Context
	cancel      context.CancelFunc
	stopOnce    sync.Once
	cycleActive atomic.Bool
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
	s.monitor.Infof("Планировщик запущен (интервал: %g ч)", s.cfg.Schedule.IntervalHours)
	go s.runLoop()
}

// Stop останавливает планировщик (идемпотентно)
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		s.monitor.Infof("Остановка планировщика...")
		s.cancel()
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

func (s *Scheduler) runLoop() {
	interval := time.Duration(s.cfg.Schedule.IntervalHours * float64(time.Hour))
	if interval <= 0 {
		interval = 6 * time.Hour
		s.monitor.Warnf("Некорректный интервал расписания — использую 6 часов")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Первый запуск сразу после старта
	s.runBackupCycle()

	for {
		select {
		case <-ticker.C:
			s.runBackupCycle()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Scheduler) runBackupCycle() {
	if !s.cycleActive.CompareAndSwap(false, true) {
		s.monitor.Warnf("Цикл уже выполняется — пропускаю повторный запуск")
		return
	}
	defer s.cycleActive.Store(false)

	s.monitor.Infof("Начало цикла бэкапов")
	defer s.monitor.Infof("Цикл бэкапов завершен")

	apps, err := s.client.GetApplications(s.ctx)
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
		s.dl.DownloadAll(s.ctx, s.client, candidates, s.cfg.Storage.Path)
	}
	s.cln.Cleanup(s.cfg.Storage.Path)
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
