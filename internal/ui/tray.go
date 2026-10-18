package ui

import (
	"fmt"
	"sync/atomic"
	"time"

	"fyne.io/systray"

	"grm-autobackup/internal/config"
	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
	"grm-autobackup/internal/webui"
	"grm-autobackup/resources/icons"
)

// demoUntil — время окончания демонстрации значка (unix nano), 0 — не идёт.
var demoUntil int64

// RunTray запускает трей-приложение
func RunTray(cfg *config.Config, cfgPath string, s *scheduler.Scheduler, m *monitor.Monitor, srv *webui.Server) error {
	m.Infof("Запуск трей-приложения")

	systray.Run(func() {
		m.Infof("Трей инициализирован")
		systray.SetIcon(icons.Idle())
		systray.SetTitle("1С:ГРМ AutoBackup")

		// Меню
		menRefresh := systray.AddMenuItem("Обновить сейчас", "Запустить бэкап вручную")
		menSettings := systray.AddMenuItem("Настройки", "Открыть настройки")
		menMonitor := systray.AddMenuItem("Монитор действий", "Показать текущие операции")
		menLog := systray.AddMenuItem("Лог", "Открыть журнал")
		menDemo := systray.AddMenuItem("Проверка значка (демо)", "Показать, как меняется значок при скачивании")
		systray.AddSeparator()
		menStatus := systray.AddMenuItem("Статус: работает", "Статус работы")
		systray.AddSeparator()
		menQuit := systray.AddMenuItem("Выход", "Закрыть приложение")

		// Запуск планировщика
		s.Start()

		// Обработчики меню
		go func() {
			for {
				select {
				case <-menRefresh.ClickedCh:
					m.Infof("Меню: ручной запуск")
					s.RunNow()
				case <-menSettings.ClickedCh:
					m.Infof("Меню: настройки")
					OpenSettingsWindow(cfg, cfgPath, s, m, srv)
				case <-menMonitor.ClickedCh:
					m.Infof("Меню: монитор действий")
					OpenMonitorWindow(s, m, srv)
				case <-menLog.ClickedCh:
					m.Infof("Меню: лог")
					OpenLogWindow(m, srv)
				case <-menDemo.ClickedCh:
					m.Infof("Меню: демонстрация значка")
					atomic.StoreInt64(&demoUntil, time.Now().Add(8*time.Second).UnixNano())
				case <-menQuit.ClickedCh:
					m.Infof("Меню: выход")
					s.Stop()
					systray.SetIcon(icons.Paused())
					systray.Quit()
					return
				}
			}
		}()

		// Периодическое обновление статуса и иконки в трее
		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			lastIconKey := ""
			lastTitle := ""
			for range ticker.C {
				action := m.GetCurrent()

				// Демонстрация: 0–6 с скачивание, затем очистка и ошибка.
				if du := atomic.LoadInt64(&demoUntil); du > time.Now().UnixNano() {
					action = demoAction(du)
				}

				iconKey := "idle"
				iconBytes := icons.Idle()
				title := "Статус: готов"

				switch action.Type {
				case monitor.StatusDownloading:
					frame := (int(action.Progress) / 5) * 5
					iconKey = fmt.Sprintf("dl-%d", frame)
					iconBytes = icons.Downloading(float64(frame))
					title = fmt.Sprintf("Скачивание: %d%%", int(action.Progress))
					if action.SpeedBps > 0 {
						title += " · " + formatBytes(int64(action.SpeedBps)) + "/с"
					}
				case monitor.StatusCleaning:
					iconKey = "cleaning"
					iconBytes = icons.Cleaning()
					title = "Статус: очистка"
				case monitor.StatusError:
					iconKey = "error"
					iconBytes = icons.Error()
					title = "Статус: ошибка"
				}

				if iconKey != lastIconKey && len(iconBytes) > 0 {
					systray.SetIcon(iconBytes)
					lastIconKey = iconKey
				}
				if title != lastTitle {
					menStatus.SetTitle(title)
					systray.SetTitle(title)
					lastTitle = title
				}
			}
		}()

	}, func() {
		m.Infof("Трей завершен")
	})

	return nil
}

// demoAction возвращает фиктивное состояние для демонстрации значка.
func demoAction(until int64) *monitor.Action {
	total := 8 * time.Second
	elapsed := total - time.Until(time.Unix(0, until))
	var a monitor.Action
	switch {
	case elapsed < 6*time.Second:
		a.Type = monitor.StatusDownloading
		a.Progress = float64(elapsed) / float64(6*time.Second) * 100
		a.SpeedBps = 5.2 * 1024 * 1024
	case elapsed < 6700*time.Millisecond:
		a.Type = monitor.StatusCleaning
	case elapsed < 7400*time.Millisecond:
		a.Type = monitor.StatusError
	}
	return &a
}
