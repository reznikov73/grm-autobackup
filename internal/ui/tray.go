package ui

import (
	"fmt"
	"time"

	"fyne.io/systray"

	"grm-autobackup/internal/config"
	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
	"grm-autobackup/resources/icons"
)

// RunTray запускает трей-приложение
func RunTray(cfg *config.Config, cfgPath string, s *scheduler.Scheduler, m *monitor.Monitor) error {
	m.Infof("Запуск трей-приложения")

	systray.Run(func() {
		m.Infof("Трей инициализирован")
		systray.SetIcon(icons.Gray)
		systray.SetTitle("1С:ГРМ AutoBackup")

		// Меню
		menRefresh := systray.AddMenuItem("🔄 Обновить сейчас", "Запустить бэкап вручную")
		menSettings := systray.AddMenuItem("⚙ Настройки", "Открыть настройки")
		menMonitor := systray.AddMenuItem("📊 Монитор действий", "Показать текущие операции")
		menLog := systray.AddMenuItem("📝 Лог", "Открыть журнал")
		systray.AddSeparator()
		menStatus := systray.AddMenuItem("Статус: работает", "Статус работы")
		systray.AddSeparator()
		menQuit := systray.AddMenuItem("❌ Выход", "Закрыть приложение")

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
					OpenSettingsWindow(cfg, cfgPath, s, m)
				case <-menMonitor.ClickedCh:
					m.Infof("Меню: монитор действий")
					OpenMonitorWindow(s, m)
				case <-menLog.ClickedCh:
					m.Infof("Меню: лог")
					OpenLogWindow(m)
				case <-menQuit.ClickedCh:
					m.Infof("Меню: выход")
					s.Stop()
					systray.SetIcon(icons.Gray)
					systray.Quit()
					return
				}
			}
		}()

		// Периодическое обновление статуса в трее
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			lastIcon := "gray"
			lastTitle := ""
			for range ticker.C {
				action := m.GetCurrent()
				icon := "green"
				title := "Статус: готов"
				switch action.Type {
				case monitor.StatusDownloading:
					icon = "yellow"
					title = fmt.Sprintf("Статус: скачивание %d%%", int(action.Progress))
				case monitor.StatusCleaning:
					icon = "yellow"
					title = "Статус: очистка"
				case monitor.StatusError:
					icon = "red"
					title = "Статус: ошибка"
				}

				if title != lastTitle {
					menStatus.SetTitle(title)
					systray.SetTitle(title)
					lastTitle = title
				}
				if icon != lastIcon {
					switch icon {
					case "green":
						systray.SetIcon(icons.Green)
					case "yellow":
						systray.SetIcon(icons.Yellow)
					case "red":
						systray.SetIcon(icons.Red)
					default:
						systray.SetIcon(icons.Gray)
					}
					lastIcon = icon
				}
			}
		}()

	}, func() {
		m.Infof("Трей завершен")
	})

	return nil
}
