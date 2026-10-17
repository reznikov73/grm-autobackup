package ui

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"grm-autobackup/internal/api"
	"grm-autobackup/internal/autostart"
	"grm-autobackup/internal/config"
	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
)

// pickerApp — строка списка выбора приложений
type pickerApp struct {
	ID         string
	Name       string
	Status     string
	CustomerID string
}

func statusRu(status string) string {
	switch status {
	case "running":
		return "работает"
	case "stopped":
		return "остановлено"
	case "deleting":
		return "удаляется"
	default:
		return status
	}
}

// OpenSettingsWindow открывает окно настроек.
func OpenSettingsWindow(cfg *config.Config, cfgPath string, s *scheduler.Scheduler, m *monitor.Monitor) {
	openWindow(m, func() (func() int, error) {
		var dlg *walk.MainWindow
		var tabs *walk.TabWidget
		var urlEdit, tokenEdit, pathEdit, startTimeEdit *walk.LineEdit
		var limitEdit, intervalEdit *walk.NumberEdit
		var modeCombo, levelCombo *walk.ComboBox
		var stoppedCheck, autostartCheck *walk.CheckBox
		var pickSearch *walk.LineEdit
		var pickStatus *walk.Label
		var pickHost *walk.ScrollView

		levels := []string{"debug", "info", "warn", "error"}
		levelIdx := indexOf(levels, strings.ToLower(cfg.Logging.Level))
		modes := []string{"Каждые N часов", "Ежедневно в указанное время"}
		modeIdx := 0
		if cfg.Schedule.Mode == "daily" {
			modeIdx = 1
		}

		var (
			apps      []pickerApp
			custNames = map[string]string{}
			checked   = map[string]bool{}
			listBuilt bool
		)

		var rebuildPicker func()

		initChecks := func() {
			if listBuilt {
				return
			}
			listBuilt = true

			inclusions := make(map[string]bool, len(cfg.Inclusions.ApplicationIDs))
			for _, id := range cfg.Inclusions.ApplicationIDs {
				inclusions[id] = true
			}
			excludedIDs := make(map[string]bool, len(cfg.Exclusions.ApplicationIDs))
			for _, id := range cfg.Exclusions.ApplicationIDs {
				excludedIDs[id] = true
			}
			excludedNames := make(map[string]bool, len(cfg.Exclusions.ApplicationNames))
			for _, n := range cfg.Exclusions.ApplicationNames {
				excludedNames[n] = true
			}

			for _, app := range apps {
				on := false
				if len(inclusions) > 0 {
					on = inclusions[app.ID]
				} else {
					on = !excludedIDs[app.ID] && !excludedNames[app.Name]
					if app.Status != "running" && !cfg.IncludeStopped {
						on = false
					}
				}
				checked[app.ID] = on
			}
		}

		rebuildPicker = func() {
			if pickHost == nil || pickSearch == nil || !listBuilt {
				return
			}
			for pickHost.Children().Len() > 0 {
				pickHost.Children().At(0).Dispose()
			}

			filter := strings.ToLower(strings.TrimSpace(pickSearch.Text()))
			byCust := map[string][]pickerApp{}
			for _, app := range apps {
				if filter != "" {
					text := strings.ToLower(custNames[app.CustomerID] + " " + app.Name)
					if !strings.Contains(text, filter) {
						continue
					}
				}
				byCust[app.CustomerID] = append(byCust[app.CustomerID], app)
			}

			var custIDs []string
			for id := range byCust {
				custIDs = append(custIDs, id)
			}
			sort.Slice(custIDs, func(i, j int) bool { return custNames[custIDs[i]] < custNames[custIDs[j]] })

			hdrFont := uiFontBold(10)
			countFont, _ := walk.NewFont("Segoe UI", 8, 0)
			cbFont := uiFont()

			shownApps := 0
			for _, cid := range custIDs {
				head, err := walk.NewComposite(pickHost)
				if err == nil {
					_ = head.SetLayout(walk.NewHBoxLayout())
					nameLbl, errL := walk.NewLabel(head)
					if errL == nil {
						_ = nameLbl.SetText(custNames[cid])
						if hdrFont != nil {
							nameLbl.SetFont(hdrFont)
						}
					}
					cntLbl, errC := walk.NewLabel(head)
					if errC == nil {
						_ = cntLbl.SetText(fmt.Sprintf("· приложений: %d", len(byCust[cid])))
						if countFont != nil {
							cntLbl.SetFont(countFont)
						}
					}
				}

				for _, app := range byCust[cid] {
					cb, err := walk.NewCheckBox(pickHost)
					if err != nil {
						continue
					}
					_ = cb.SetText("   " + app.Name + "   [" + statusRu(app.Status) + "]")
					if cbFont != nil {
						cb.SetFont(cbFont)
					}
					cb.SetChecked(checked[app.ID])
					id := app.ID
					cb.CheckedChanged().Attach(func() {
						checked[id] = cb.Checked()
					})
					shownApps++
				}
			}
			if shownApps == 0 {
				if lbl, err := walk.NewLabel(pickHost); err == nil {
					_ = lbl.SetText("Ничего не найдено")
				}
			}
			_ = pickStatus.SetText(fmt.Sprintf("Показано: %d приложений у %d клиентов (всего %d)", shownApps, len(custIDs), len(apps)))
		}

		loadPicker := func() {
			_ = pickStatus.SetText("Загрузка списка приложений…")
			go func() {
				c := api.New(cfg.API.URL, cfg.API.JWTToken, cfg.API.TimeoutSec)
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				list, err := c.GetApplications(ctx)
				if err != nil {
					dlg.Synchronize(func() {
						_ = pickStatus.SetText("Ошибка загрузки: " + err.Error())
					})
					return
				}

				names := map[string]string{}
				for _, app := range list {
					if _, ok := names[app.CustomerID]; ok {
						continue
					}
					if cust, err := c.GetCustomer(ctx, app.CustomerID); err == nil && cust.Name != "" {
						names[app.CustomerID] = cust.Name
					} else {
						names[app.CustomerID] = "Клиент " + shortID8(app.CustomerID)
					}
				}

				dlg.Synchronize(func() {
					custNames = names
					apps = apps[:0]
					for _, app := range list {
						apps = append(apps, pickerApp{
							ID:         app.ID,
							Name:       app.Name,
							Status:     app.Status,
							CustomerID: app.CustomerID,
						})
					}
					sort.Slice(apps, func(i, j int) bool {
						ci, cj := custNames[apps[i].CustomerID], custNames[apps[j].CustomerID]
						if ci != cj {
							return ci < cj
						}
						return apps[i].Name < apps[j].Name
					})
					initChecks()
					rebuildPicker()
					_ = pickStatus.SetText(fmt.Sprintf("Загружено: приложений %d, клиентов %d", len(apps), len(custNames)))
				})
			}()
		}

		setAll := func(on bool) {
			if !listBuilt {
				return
			}
			filter := ""
			if pickSearch != nil {
				filter = strings.ToLower(strings.TrimSpace(pickSearch.Text()))
			}
			for _, app := range apps {
				if filter != "" {
					text := strings.ToLower(custNames[app.CustomerID] + " " + app.Name)
					if !strings.Contains(text, filter) {
						continue
					}
				}
				checked[app.ID] = on
			}
			rebuildPicker()
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
			if _, err := time.Parse("15:04", startTime); err != nil {
				walk.MsgBox(dlg, "Настройки", "Время в расписании должно быть в формате ЧЧ:ММ.", walk.MsgBoxIconWarning)
				return
			}
			if limit < 1 {
				limit = 1
			}
			if interval < 1 {
				interval = 6
			}

			if !listBuilt {
				walk.MsgBox(dlg, "Настройки", "Список приложений ещё не загружен. Дождитесь загрузки на вкладке «Выбор приложений».", walk.MsgBoxIconWarning)
				return
			}
			var ids []string
			for id, on := range checked {
				if on {
					ids = append(ids, id)
				}
			}
			if len(ids) == 0 {
				walk.MsgBox(dlg, "Настройки", "Отметьте хотя бы одно приложение на вкладке «Выбор приложений».", walk.MsgBoxIconWarning)
				return
			}
			sort.Strings(ids)

			cfg.Inclusions.ApplicationIDs = ids
			// Новый интерфейс работает через выбор приложений; старые исключения сбрасываем.
			cfg.Exclusions.ApplicationIDs = []string{}
			cfg.Exclusions.ApplicationNames = []string{}
			cfg.API.URL = url
			cfg.API.JWTToken = token
			cfg.Storage.Path = dir
			cfg.Storage.BackupLimit = limit
			cfg.Schedule.Mode = mode
			cfg.Schedule.IntervalHours = interval
			cfg.Schedule.StartTime = startTime
			cfg.IncludeStopped = stoppedCheck.Checked()
			cfg.Autostart = autostartCheck.Checked()
			if i := levelCombo.CurrentIndex(); i >= 0 && i < len(levels) {
				cfg.Logging.Level = levels[i]
			}

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
			walk.MsgBox(dlg, "Настройки", fmt.Sprintf("Сохранено.\nПриложений выбрано: %d", len(ids)), walk.MsgBoxIconInformation)
		}

		err := MainWindow{
			AssignTo: &dlg,
			Title:    "Настройки — 1С:ГРМ AutoBackup",
			Size:     Size{Width: 780, Height: 660},
			MinSize:  Size{Width: 660, Height: 540},
			Layout:   VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
			Children: []Widget{
				TabWidget{
					AssignTo: &tabs,
					Pages: []TabPage{
						{
							Title:  "Подключение",
							Layout: VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
							Children: []Widget{
								GroupBox{
									Title:  "Параметры API",
									Layout: Grid{Columns: 2},
									Children: []Widget{
										Label{Text: "URL API:"},
										LineEdit{AssignTo: &urlEdit, Text: cfg.API.URL},
										Label{Text: "JWT токен:"},
										LineEdit{AssignTo: &tokenEdit, Text: cfg.API.JWTToken},
									},
								},
								Label{Text: "Токен выдаётся в личном кабинете 1С:ГРМ (Partner API v2)."},
								VSpacer{},
							},
						},
						{
							Title:  "Расписание",
							Layout: VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
							Children: []Widget{
								GroupBox{
									Title:  "Когда скачивать бэкапы",
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
								VSpacer{},
							},
						},
						{
							Title:  "Выбор приложений",
							Layout: VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
							Children: []Widget{
								Composite{
									Layout: HBox{Spacing: 8},
									Children: []Widget{
										LineEdit{AssignTo: &pickSearch, CueBanner: "Поиск по клиенту или конфигурации…"},
										PushButton{Text: "Обновить из API", OnClicked: func() { loadPicker() }},
										PushButton{Text: "Все", OnClicked: func() { setAll(true) }},
										PushButton{Text: "Снять все", OnClicked: func() { setAll(false) }},
									},
								},
								ScrollView{
									AssignTo: &pickHost,
									Layout:   VBox{Margins: Margins{Left: 10, Top: 8, Right: 10, Bottom: 8}, Spacing: 3, Alignment: AlignHNearVNear},
								},
								Label{AssignTo: &pickStatus, Text: "Список ещё не загружен — нажмите «Обновить из API»."},
							},
						},
						{
							Title:  "Прочее",
							Layout: VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
							Children: []Widget{
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
									Title:  "Логирование",
									Layout: Grid{Columns: 2},
									Children: []Widget{
										Label{Text: "Уровень лога:"},
										ComboBox{AssignTo: &levelCombo, Model: levels, CurrentIndex: levelIdx},
									},
								},
								CheckBox{AssignTo: &autostartCheck, Text: "Автозапуск при входе в Windows", Checked: cfg.Autostart},
								VSpacer{},
							},
						},
					},
				},
				Composite{
					Layout: HBox{Spacing: 8},
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

		centerForm(dlg, 780, 660)
		uiF := uiFont()
		applyFont(dlg, uiF)
		if tabs != nil && uiF != nil {
			for i := 0; i < tabs.Pages().Len(); i++ {
				if c, ok := interface{}(tabs.Pages().At(i)).(walk.Container); ok {
					applyFontTo(c, uiF)
				}
			}
		}

		if pickSearch != nil {
			pickSearch.TextChanged().Attach(func() { rebuildPicker() })
		}
		loadPicker()

		// Тестовый хук: открыть окно настроек на заданной вкладке (GRM_TEST_TAB=0..3)
		if v := os.Getenv("GRM_TEST_TAB"); v != "" {
			if idx, err := strconv.Atoi(v); err == nil && idx >= 0 {
				_ = tabs.SetCurrentIndex(idx)
			}
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

func shortID8(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
