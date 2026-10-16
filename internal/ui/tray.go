package ui

import (
	"fyne.io/systray"
	"grm-autobackup/internal/config"
	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
	"log"
	"time"
)

// RunTray запускает трей-приложение
func RunTray(cfg *config.Config, s *scheduler.Scheduler, m *monitor.Monitor) error {
	log.Println("Запуск трей-приложения...")

	systray.Run(func() {
		log.Println("Трей инициализирован")
		
		// Меню
		menRefresh := systray.AddMenuItem("🔄 Обновить сейчас", "Запустить бэкап вручную")
		menSettings := systray.AddMenuItem("⚙ Настройки", "Открыть настройки")
		systray.AddSeparator()
		menStatus := systray.AddMenuItem("Статус: Готов", "Статус работы")
		systray.AddSeparator()
		menQuit := systray.AddMenuItem("❌ Выход", "Закрыть приложение")

		// Запуск планировщика
		s.Start()
		menStatus.SetTitle("Статус: Работает")

		// Обработчики
		go func() {
			for {
				select {
				case <-menRefresh.ClickedCh:
					log.Println("Ручной запуск...")
					s.RunNow()
				case <-menSettings.ClickedCh:
					log.Println("Открытие настроек...")
					// TODO: открыть окно настроек
				case <-menQuit.ClickedCh:
					log.Println("Выход...")
					s.Stop()
					systray.Quit()
					return
				}
			}
		}()

		// Периодическое обновление статуса
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				action := m.GetCurrent()
				if action.Type == "downloading" {
					menStatus.SetTitle("Статус: Скачивание... " + string(rune(int(action.Progress))) + "%")
				} else if action.Type == "error" {
					menStatus.SetTitle("Статус: Ошибка")
				} else {
					menStatus.SetTitle("Статус: Готов")
				}
			}
		}()

	}, func() {
		log.Println("Трей завершен")
	})

	return nil
}
