package ui

import (
	"os"
	"strings"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"grm-autobackup/internal/autostart"
	"grm-autobackup/internal/config"
	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
)

// OpenSettingsWindow открывает окно настроек.
// Сохранение пишет config.json и применяет изменения через scheduler.Reload().
func OpenSettingsWindow(cfg *config.Config, cfgPath string, s *scheduler.Scheduler, m *monitor.Monitor) {
	openWindow(m, func() (func() int, error) {
		var dlg *walk.MainWindow
		var urlEdit, tokenEdit, pathEdit, startTimeEdit *walk.LineEdit
		var limitEdit, intervalEdit *walk.NumberEdit
		var modeCombo, levelCombo *walk.ComboBox
		var stoppedCheck, autostartCheck *walk.CheckBox
		var idsEdit, namesEdit *walk.TextEdit

		levels := []string{"debug", "info", "warn", "error"}
		levelIdx := indexOf(levels, strings.ToLower(cfg.Logging.Level))
		modes := []string{"Каждые N часов", "Ежедневно в указанное время"}
		modeIdx := 0
		if cfg.Schedule.Mode == "daily" {
			modeIdx = 1
		}

		save := func() {
			url := strings.TrimSpace(urlEdit.Text())
			token := strings.TrimSpace(tokenEdit.Text())
			dir := strings.TrimSpace(pathEdit.Text())
			limit := int(limitEdit.Value())
			interval := intervalEdit.Value()
			startTime := strings.TrimSpace(startTimeEdit.Text())
			mode := "interval"
			if modeCombo.CurrentIndex() == 1 {
				mode = "daily"
			}

			if dir == "" {
				walk.MsgBox(dlg, "Настройки", "Укажите папку сохранения бэкапов.", walk.MsgBoxIconWarning)
				return
			}
			if limit < 1 {
				limit = 1
			}
			if interval < 1 {
				interval = 6
			}
			if _, err := time.Parse("15:04", startTime); err != nil {
				walk.MsgBox(dlg, "Настройки", "Время в расписании должно быть в формате ЧЧ:ММ.", walk.MsgBoxIconWarning)
				return
			}

			cfg.API.URL = url
			cfg.API.JWTToken = token
			cfg.Storage.Path = dir
			cfg.Storage.BackupLimit = limit
			cfg.Schedule.Mode = mode
			cfg.Schedule.IntervalHours = interval
			cfg.Schedule.StartTime = startTime
			cfg.IncludeStopped = stoppedCheck.Checked()
			cfg.Exclusions.ApplicationIDs = splitLines(idsEdit.Text())
			cfg.Exclusions.ApplicationNames = splitLines(namesEdit.Text())
			if i := levelCombo.CurrentIndex(); i >= 0 && i < len(levels) {
				cfg.Logging.Level = levels[i]
			}
			cfg.Autostart = autostartCheck.Checked()

			if err := cfg.Save(cfgPath); err != nil {
				walk.MsgBox(dlg, "Настройки", "Не удалось сохранить: "+err.Error(), walk.MsgBoxIconError)
				return
			}

			m.SetLevel(cfg.Logging.Level)
			s.Reload()

			if exe, err := os.Executable(); err == nil {
				if err := autostart.Sync(cfg.Autostart, exe); err != nil {
					m.Warnf("Автозапуск: %v", err)
					walk.MsgBox(dlg, "Автозапуск", "Не удалось изменить автозапуск: "+err.Error(), walk.MsgBoxIconWarning)
				}
			}

			m.Infof("Настройки сохранены: %s", cfgPath)
			walk.MsgBox(dlg, "Настройки", "Сохранено.", walk.MsgBoxIconInformation)
		}

		err := MainWindow{
			AssignTo: &dlg,
			Title:    "Настройки — 1С:ГРМ AutoBackup",
			MinSize:  Size{Width: 560, Height: 640},
			Layout:   VBox{},
			Children: []Widget{
				GroupBox{
					Title:  "Подключение",
					Layout: Grid{Columns: 2},
					Children: []Widget{
						Label{Text: "URL API:"},
						LineEdit{AssignTo: &urlEdit, Text: cfg.API.URL},
						Label{Text: "JWT токен:"},
						LineEdit{AssignTo: &tokenEdit, Text: cfg.API.JWTToken},
					},
				},
				GroupBox{
					Title:  "Хранилище",
					Layout: Grid{Columns: 2},
					Children: []Widget{
						Label{Text: "Папка сохранения:"},
						LineEdit{AssignTo: &pathEdit, Text: cfg.Storage.Path},
						Label{Text: "Лимит бэкапов на приложение:"},
						NumberEdit{AssignTo: &limitEdit, MinValue: 1, MaxValue: 1000, Value: float64(cfg.Storage.BackupLimit)},
					},
				},
				GroupBox{
					Title:  "Расписание",
					Layout: Grid{Columns: 2},
					Children: []Widget{
						Label{Text: "Режим:"},
						ComboBox{AssignTo: &modeCombo, Model: modes, CurrentIndex: modeIdx},
						Label{Text: "Интервал, часов:"},
						NumberEdit{AssignTo: &intervalEdit, MinValue: 1, MaxValue: 720, Value: cfg.Schedule.IntervalHours},
						Label{Text: "Время (ЧЧ:ММ):"},
						LineEdit{AssignTo: &startTimeEdit, Text: cfg.Schedule.StartTime},
					},
				},
				CheckBox{AssignTo: &stoppedCheck, Text: "Обрабатывать и остановленные приложения", Checked: cfg.IncludeStopped},
				CheckBox{AssignTo: &autostartCheck, Text: "Автозапуск при входе в Windows", Checked: cfg.Autostart},
				GroupBox{
					Title:  "Исключения",
					Layout: VBox{},
					Children: []Widget{
						Label{Text: "По ID приложений (по одному на строку):"},
						TextEdit{AssignTo: &idsEdit, Text: strings.Join(cfg.Exclusions.ApplicationIDs, "\r\n"), VScroll: true, MinSize: Size{Height: 70}},
						Label{Text: "По именам приложений (по одному на строку):"},
						TextEdit{AssignTo: &namesEdit, Text: strings.Join(cfg.Exclusions.ApplicationNames, "\r\n"), VScroll: true, MinSize: Size{Height: 70}},
					},
				},
				GroupBox{
					Title:  "Прочее",
					Layout: Grid{Columns: 2},
					Children: []Widget{
						Label{Text: "Уровень лога:"},
						ComboBox{AssignTo: &levelCombo, Model: levels, CurrentIndex: levelIdx},
					},
				},
				Composite{
					Layout: HBox{},
					Children: []Widget{
						HSpacer{},
						PushButton{Text: "Сохранить", OnClicked: save},
						PushButton{Text: "Закрыть", OnClicked: func() { dlg.Close() }},
					},
				},
			},
		}.Create()
		if err != nil {
			return nil, err
		}
		return func() int { return dlg.Run() }, nil
	})
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return 0
}

func splitLines(s string) []string {
	out := []string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
