package ui

import (
	"strings"
	"time"

	"github.com/lxn/win"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"grm-autobackup/internal/monitor"
)

// OpenLogWindow открывает окно журнала
func OpenLogWindow(m *monitor.Monitor) {
	openWindow(m, func() (func() int, error) {
		var dlg *walk.MainWindow
		var levelCombo *walk.ComboBox
		var searchEdit *walk.LineEdit
		var autoScroll *walk.CheckBox
		var te *walk.TextEdit

		levels := []string{"Все", "DEBUG", "INFO", "WARN", "ERROR"}

		refresh := func() {
			if dlg == nil || dlg.Handle() == 0 {
				return
			}
			level := ""
			if i := levelCombo.CurrentIndex(); i > 0 && i < len(levels) {
				level = levels[i]
			}
			search := strings.ToLower(strings.TrimSpace(searchEdit.Text()))

			lines := m.GetLogs(0)
			filtered := make([]string, 0, len(lines))
			for _, l := range lines {
				if level != "" && !strings.Contains(l, "["+level+"]") {
					continue
				}
				if search != "" && !strings.Contains(strings.ToLower(l), search) {
					continue
				}
				filtered = append(filtered, l)
			}

			text := strings.Join(filtered, "\r\n")
			_ = te.SetText(text)
			if autoScroll.Checked() {
				n := len(text)
				te.SetTextSelection(n, n)
				win.SendMessage(te.Handle(), win.EM_SCROLLCARET, 0, 0)
			}
		}

		err := MainWindow{
			AssignTo: &dlg,
			Title:    "Лог — 1С:ГРМ AutoBackup",
			Size:     Size{Width: 900, Height: 560},
			MinSize:  Size{Width: 700, Height: 420},
			Layout:   VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
			Children: []Widget{
				Composite{
					Layout: HBox{Spacing: 8},
					Children: []Widget{
						Label{Text: "Уровень:"},
						ComboBox{AssignTo: &levelCombo, Model: levels, CurrentIndex: 0},
						Label{Text: "Поиск:"},
						LineEdit{AssignTo: &searchEdit, CueBanner: "фильтр по тексту…"},
						CheckBox{AssignTo: &autoScroll, Text: "Автоскролл", Checked: true},
					},
				},
				TextEdit{
					AssignTo: &te,
					ReadOnly: true,
					VScroll:  true,
					HScroll:  true,
				},
				Composite{
					Layout: HBox{Spacing: 8},
					Children: []Widget{
						PushButton{Text: "Обновить", OnClicked: refresh},
						PushButton{Text: "Очистить", OnClicked: func() {
							m.ClearLogs()
							refresh()
						}},
						HSpacer{},
						PushButton{Text: "Закрыть", OnClicked: func() { dlg.Close() }},
					},
				},
			},
		}.Create()
		if err != nil {
			return nil, err
		}

		centerForm(dlg, 900, 560)
		applyFont(dlg, uiFont())
		// В текстовом поле — моноширинный шрифт (после общего применения шрифтов)
		if f, err := walk.NewFont("Consolas", 9, 0); err == nil {
			te.SetFont(f)
		}

		levelCombo.CurrentIndexChanged().Attach(func() { refresh() })
		searchEdit.TextChanged().Attach(func() { refresh() })
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
