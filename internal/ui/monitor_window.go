package ui

import (
	"fmt"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
)

// historyRow — строка таблицы истории операций
type historyRow struct {
	time  string
	desc  string
	state string
}

// historyModel — модель таблицы истории для walk.TableView
type historyModel struct {
	walk.TableModelBase
	rows []historyRow
}

// RowCount возвращает число строк
func (m *historyModel) RowCount() int { return len(m.rows) }

// Value возвращает значение ячейки
func (m *historyModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.rows) {
		return ""
	}
	r := m.rows[row]
	switch col {
	case 0:
		return r.time
	case 1:
		return r.desc
	case 2:
		return r.state
	}
	return ""
}

func stateRu(s monitor.ActionStatus) string {
	if s == monitor.StatusError {
		return "ошибка"
	}
	return "ok"
}

// OpenMonitorWindow открывает окно монитора действий
func OpenMonitorWindow(s *scheduler.Scheduler, m *monitor.Monitor) {
	openWindow(m, func() (func() int, error) {
		var dlg *walk.MainWindow
		var statusLabel, nextLabel *walk.Label
		var progress *walk.ProgressBar
		model := &historyModel{}

		refresh := func() {
			if dlg == nil || dlg.Handle() == 0 {
				return
			}

			action := m.GetCurrent()
			status := "Ожидание"
			switch action.Type {
			case monitor.StatusDownloading:
				line := fmt.Sprintf("%d%%", int(action.Progress))
				if action.SpeedBps > 0 {
					line += " · " + formatSpeed(action.SpeedBps)
				}
				if action.BytesTotal > 0 && action.SpeedBps > 0 && action.BytesWritten < action.BytesTotal {
					remain := float64(action.BytesTotal-action.BytesWritten) / action.SpeedBps
					line += " · осталось " + formatETA(remain)
				}
				status = "Скачивание: " + action.Description + " — " + line
			case monitor.StatusCleaning:
				status = "Очистка старых бэкапов…"
			case monitor.StatusError:
				status = "Ошибка: " + action.Error
			}
			_ = statusLabel.SetText(status)

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
				_ = nextLabel.SetText("Следующий запуск: —")
			} else {
				_ = nextLabel.SetText("Следующий запуск: " + next.Format("02.01.2006 15:04"))
			}

			hist := m.GetHistory()
			rows := make([]historyRow, 0, len(hist))
			for i := len(hist) - 1; i >= 0; i-- {
				a := hist[i]
				rows = append(rows, historyRow{
					time:  a.FinishedAt.Format("02.01 15:04:05"),
					desc:  a.Description,
					state: stateRu(a.Type),
				})
			}
			model.rows = rows
			model.PublishRowsReset()
		}

		err := MainWindow{
			AssignTo: &dlg,
			Title:    "Монитор действий — 1С:ГРМ AutoBackup",
			Size:     Size{Width: 780, Height: 500},
			MinSize:  Size{Width: 620, Height: 400},
			Layout:   VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
			Children: []Widget{
				Label{AssignTo: &statusLabel, Text: "Ожидание"},
				ProgressBar{AssignTo: &progress, MinValue: 0, MaxValue: 100},
				Label{AssignTo: &nextLabel, Text: "Следующий запуск: —"},
				Label{Text: "Последние операции:"},
				TableView{
					Model:               model,
					AlternatingRowBG:    true,
					ColumnsOrderable:    false,
					LastColumnStretched: true,
					Columns: []TableViewColumn{
						{Title: "Время", Width: 110},
						{Title: "Операция", Width: 480},
						{Title: "Результат", Width: 110},
					},
				},
				Composite{
					Layout: HBox{Spacing: 8},
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

		centerForm(dlg, 780, 500)
		applyFont(dlg, uiFont())
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
