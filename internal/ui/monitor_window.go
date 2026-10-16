package ui

import (
	"fmt"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
)

// OpenMonitorWindow открывает окно монитора действий
func OpenMonitorWindow(s *scheduler.Scheduler, m *monitor.Monitor) {
	openWindow(m, func() (func() int, error) {
		var dlg *walk.MainWindow
		var statusLabel, nextLabel *walk.Label
		var progress *walk.ProgressBar
		var historyList *walk.ListBox

		refresh := func() {
			if dlg == nil || dlg.Handle() == 0 {
				return
			}

			action := m.GetCurrent()
			status := "Ожидание"
			switch action.Type {
			case monitor.StatusDownloading:
				status = fmt.Sprintf("Скачивание: %s — %d%%", action.Description, int(action.Progress))
			case monitor.StatusCleaning:
				status = "Очистка старых бэкапов…"
			case monitor.StatusError:
				status = "Ошибка: " + action.Error
			}
			statusLabel.SetText(status)

			pv := int(action.Progress)
			if pv < 0 {
				pv = 0
			}
			if pv > 100 {
				pv = 100
			}
			progress.SetValue(pv)

			next := s.NextRun()
			if next.IsZero() {
				nextLabel.SetText("Следующий запуск: —")
			} else {
				nextLabel.SetText("Следующий запуск: " + next.Format("02.01.2006 15:04"))
			}

			hist := m.GetHistory()
			lines := make([]string, 0, len(hist))
			for i := len(hist) - 1; i >= 0; i-- {
				a := hist[i]
				lines = append(lines, fmt.Sprintf("%s — %s", a.FinishedAt.Format("02.01 15:04:05"), a.Description))
			}
			_ = historyList.SetModel(lines)
		}

		err := MainWindow{
			AssignTo: &dlg,
			Title:    "Монитор действий — 1С:ГРМ AutoBackup",
			MinSize:  Size{Width: 640, Height: 440},
			Layout:   VBox{},
			Children: []Widget{
				Label{AssignTo: &statusLabel, Text: "Ожидание"},
				ProgressBar{AssignTo: &progress, MinValue: 0, MaxValue: 100},
				Label{AssignTo: &nextLabel, Text: "Следующий запуск: —"},
				Label{Text: "Последние операции:"},
				ListBox{AssignTo: &historyList},
				Composite{
					Layout: HBox{},
					Children: []Widget{
						PushButton{Text: "Обновить сейчас", OnClicked: func() { s.RunNow() }},
						PushButton{Text: "Обновить", OnClicked: refresh},
						HSpacer{},
						PushButton{Text: "Закрыть", OnClicked: func() { dlg.Close() }},
					},
				},
			},
		}.Create()
		if err != nil {
			return nil, err
		}

		refresh()

		stopCh := make(chan struct{})
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					dlg.Synchronize(refresh)
				}
			}
		}()

		return func() int {
			ret := dlg.Run()
			close(stopCh)
			return ret
		}, nil
	})
}
