package scheduler

import (
	"context"
	"grm-autobackup/internal/api"
	"grm-autobackup/internal/config"
	"grm-autobackup/internal/cleaner"
	"grm-autobackup/internal/downloader"
	"grm-autobackup/internal/monitor"
	"time"
)

// Scheduler планирует и запускает задачи
type Scheduler struct {
	cfg       *config.Config
	client    *api.Client
	dl        *downloader.Downloader
	cln       *cleaner.Cleaner
	monitor   *monitor.Monitor
	ctx       context.Context
	cancel    context.CancelFunc
	ticker    *time.Ticker
	stopChan  chan struct{}
}

// New создает новый планировщик
func New(cfg *config.Config, m *monitor.Monitor) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Scheduler{
		cfg:     cfg,
		client:  api.New(cfg.API.URL, cfg.API.JWTToken, cfg.API.TimeoutSec),
		dl:      downloader.New(m),
		cln:     cleaner.New(cfg.Storage.BackupLimit, cfg.Storage.Path, m),
		monitor: m,
		ctx:     ctx,
		cancel:  cancel,
		stopChan: make(chan struct{}),
	}
}

// Start запускает планировщик
func (s *Scheduler) Start() {
	if !s.cfg.Schedule.Enabled {
		s.monitor.AddLog("Планировщик отключен в настройках")
		return
	}

	s.monitor.AddLog("Планировщик запущен")
	
	// Запуск цикла
	go s.runLoop()
}

// Stop останавливает планировщик
func (s *Scheduler) Stop() {
	s.monitor.AddLog("Остановка планировщика...")
	s.cancel()
	close(s.stopChan)
	s.monitor.AddLog("Планировщик остановлен")
}

// RunNow запускает задачу вручную
func (s *Scheduler) RunNow() {
	s.monitor.AddLog("Ручной запуск...")
	s.runBackupCycle()
}

func (s *Scheduler) runLoop() {
	interval := time.Duration(s.cfg.Schedule.IntervalHours) * time.Hour
	
	s.ticker = time.NewTicker(interval)
	defer s.ticker.Stop()

	// Первый запуск сразу
	s.runBackupCycle()

	for {
		select {
		case <-s.ticker.C:
			s.runBackupCycle()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Scheduler) runBackupCycle() {
	s.monitor.AddLog("Начало цикла бэкапов...")
	
	// Получаем список приложений
	apps, err := s.client.GetApplications(s.ctx)
	if err != nil {
		s.monitor.AddLog("Ошибка получения приложений: " + err.Error())
		return
	}

	s.monitor.AddLog("Найдено приложений: " + string(rune(apps[0].Status[0])))
	
	// Фильтруем running приложения
	var runningApps []api.Application
	for _, app := range apps {
		if app.Status == "running" {
			runningApps = append(runningApps, app)
		}
	}

	s.monitor.AddLog("Running приложений: " + string(rune(len(runningApps))))
	
	// Скачиваем бэкапы
	for _, app := range runningApps {
		s.dl.DownloadLatestBackup(s.ctx, s.client, app, s.cfg.Storage.Path)
	}

	// Очищаем старые бэкапы
	s.cln.Cleanup(s.cfg.Storage.Path)
	
	s.monitor.AddLog("Цикл бэкапов завершен")
}
