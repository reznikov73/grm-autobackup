package ui

import (
	"fmt"
	"time"

	"fyne.io/systray"

	"grm-autobackup/internal/config"
	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
	"grm-autobackup/internal/webui"
	"grm-autobackup/resources/icons"
)

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
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			lastIconKey := ""
			lastTitle := ""
			for range ticker.C {
				action := m.GetCurrent()

				iconKey := "idle"
				iconBytes := icons.Idle()
				title := "Статус: готов"

				switch action.Type {
				case monitor.StatusDownloading:
					frame := (int(action.Progress) / 10) * 10
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
