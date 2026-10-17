package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// pickerAppJs — приложение для HTML-списка выбора
type pickerAppJs struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// pickerCustomer — клиент с приложениями для HTML-списка выбора
type pickerCustomer struct {
	Name string        `json:"name"`
	Apps []pickerAppJs `json:"apps"`
}

// pickerData — данные страницы выбора приложений
type pickerData struct {
	Customers []pickerCustomer `json:"customers"`
	Initial   []string         `json:"initial"`
}

// pickerHTMLTemplate — HTML-страница выбора приложений (IE11-совместимый CSS/JS).
// __DATA__ заменяется на JSON с данными.
const pickerHTMLTemplate = `<!DOCTYPE html>
<html>
<head>
<meta http-equiv="X-UA-Compatible" content="IE=edge">
<meta charset="utf-8">
<title>Выбор приложений</title>
<style>
html,body{height:100%;margin:0;padding:0;}
body{font-family:"Segoe UI",Tahoma,sans-serif;background:#eef1f6;color:#2c3646;font-size:13px;overflow:hidden;}
#top{position:absolute;top:0;left:0;right:0;height:53px;background:#ffffff;border-bottom:1px solid #e4e9f2;}
#search{position:absolute;left:14px;top:12px;width:320px;padding:6px 10px;font-size:13px;border:1px solid #cfd8e6;border-radius:6px;outline:none;font-family:inherit;}
#search:focus{border-color:#2e6fe8;}
#actions{position:absolute;right:14px;top:12px;}
.btn{height:30px;padding:0 13px;margin-left:8px;font-size:12.5px;border:1px solid #cfd8e6;border-radius:6px;background:#ffffff;color:#2c3646;cursor:pointer;font-family:inherit;}
.btn:hover{background:#f2f6fc;}
.btn.primary{background:#2e6fe8;border-color:#2e6fe8;color:#ffffff;}
.btn.primary:hover{background:#2560cf;}
#list{position:absolute;top:54px;bottom:36px;left:0;right:0;overflow-y:auto;}
.card{background:#ffffff;border:1px solid #e4e9f2;border-radius:10px;margin:10px 14px;padding:10px 14px 8px 14px;box-shadow:0 1px 2px rgba(21,42,69,0.06);}
.head{font-weight:600;color:#25476a;font-size:13.5px;margin-bottom:4px;}
.head .cnt{color:#8b95a6;font-weight:400;font-size:12px;margin-left:6px;}
.row{display:block;padding:6px 8px;border-radius:6px;cursor:pointer;color:#2c3646;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;}
.row:hover{background:#f4f8fd;}
.row input{vertical-align:middle;margin:0 8px 0 0;}
.nm{vertical-align:middle;}
.pill{display:inline-block;margin-left:8px;padding:1px 8px;border-radius:9px;font-size:11px;vertical-align:middle;}
.pill.run{background:#e6f6ec;color:#1e7f3c;}
.pill.stop{background:#fff3e0;color:#b26a00;}
.empty{color:#8b95a6;padding:24px;text-align:center;font-size:13px;}
#status{position:absolute;left:0;right:0;bottom:0;height:36px;line-height:36px;padding-left:16px;color:#8b95a6;font-size:12px;background:#ffffff;border-top:1px solid #e4e9f2;}
</style>
</head>
<body>
<div id="top">
  <input id="search" type="text" placeholder="Поиск по клиенту или конфигурации…" onkeyup="applyFilter()" />
  <div id="actions">
    <button class="btn primary" onclick="cmdRefresh()">Обновить из API</button>
    <button class="btn" onclick="checkAll(true)">Все</button>
    <button class="btn" onclick="checkAll(false)">Снять все</button>
  </div>
</div>
<div id="list"></div>
<div id="status">Загрузка…</div>
<script>
var DATA = __DATA__;
var state = {};
var customers = DATA.customers || [];

function esc(s) {
  return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function build() {
  var i, a, c, app, cust, html = '', visibleTotal = 0;
  for (i = 0; i < DATA.initial.length; i++) { state[DATA.initial[i]] = true; }
  for (c = 0; c < customers.length; c++) {
    cust = customers[c];
    html += '<div class="card" data-name="' + esc((cust.name + ' ' + appsText(cust.apps)).toLowerCase()) + '">';
    html += '<div class="head">' + esc(cust.name) + '<span class="cnt">· приложений: ' + cust.apps.length + '</span></div>';
    for (a = 0; a < cust.apps.length; a++) {
      app = cust.apps[a];
      if (!state.hasOwnProperty(app.id)) { state[app.id] = false; }
      visibleTotal++;
      var pill = app.status === 'running' ? '<span class="pill run">работает</span>' : '<span class="pill stop">остановлено</span>';
      var chk = state[app.id] ? ' checked="checked"' : '';
      html += '<label class="row" data-name="' + esc((app.name + ' ' + app.status).toLowerCase()) + '">'
           + '<input type="checkbox" data-id="' + esc(app.id) + '"' + chk + ' onchange="onToggle(this)" />'
           + '<span class="nm">' + esc(app.name) + '</span>' + pill + '</label>';
    }
    html += '</div>';
  }
  document.getElementById('list').innerHTML = html || '<div class="empty">Список пуст — нажмите «Обновить из API»</div>';
  document.getElementById('status').innerText = 'Всего приложений: ' + visibleTotal + ' · клиентов: ' + customers.length;
  publish();
  updateCounter();
}

function appsText(apps) {
  var t = '', i;
  for (i = 0; i < apps.length; i++) { t += apps[i].name + ' '; }
  return t;
}

function onToggle(cb) {
  state[cb.getAttribute('data-id')] = cb.checked;
  publish();
  updateCounter();
}

function checkAll(on) {
  var boxes = document.getElementsByTagName('input'), i, cb;
  for (i = 0; i < boxes.length; i++) {
    cb = boxes[i];
    if (cb.type !== 'checkbox') { continue; }
    if (cb.parentNode.style.display === 'none') { continue; }
    cb.checked = on;
    state[cb.getAttribute('data-id')] = on;
  }
  publish();
  updateCounter();
}

function applyFilter() {
  var q = document.getElementById('search').value.toLowerCase();
  var cards = document.getElementById('list').childNodes, i, r, rows, card, visible, show;
  for (i = 0; i < cards.length; i++) {
    card = cards[i];
    if (!card.getElementsByTagName) { continue; }
    rows = card.getElementsByTagName('label');
    visible = 0;
    for (r = 0; r < rows.length; r++) {
      show = q === '' || (rows[r].getAttribute('data-name') || '').indexOf(q) >= 0 || (card.getAttribute('data-name') || '').indexOf(q) >= 0;
      rows[r].style.display = show ? 'block' : 'none';
      if (show) { visible++; }
    }
    card.style.display = (visible > 0 || q === '') ? 'block' : 'none';
  }
}

function updateCounter() {
  var n = 0, total = 0, k;
  for (k in state) {
    if (state.hasOwnProperty(k)) { total++; if (state[k]) { n++; } }
  }
  document.getElementById('status').innerText = 'Отмечено: ' + n + ' из ' + total;
}

function publish() {
  var ids = [], k;
  for (k in state) { if (state.hasOwnProperty(k) && state[k]) { ids.push(k); } }
  document.title = 'G|' + ids.join('|');
}

function cmdRefresh() {
  document.title = 'CMD|refresh|' + (new Date().getTime());
}

build();
</script>
</body>
</html>`

// OpenSettingsWindow открывает окно настроек.
func OpenSettingsWindow(cfg *config.Config, cfgPath string, s *scheduler.Scheduler, m *monitor.Monitor) {
	openWindow(m, func() (func() int, error) {
		var dlg *walk.MainWindow
		var tabs *walk.TabWidget
		var urlEdit, tokenEdit, pathEdit, startTimeEdit *walk.LineEdit
		var limitEdit, intervalEdit *walk.NumberEdit
		var modeCombo, levelCombo *walk.ComboBox
		var stoppedCheck, autostartCheck *walk.CheckBox
		var wv *walk.WebView
		var pickStatus *walk.Label

		levels := []string{"debug", "info", "warn", "error"}
		levelIdx := indexOf(levels, strings.ToLower(cfg.Logging.Level))
		modes := []string{"Каждые N часов", "Ежедневно в указанное время"}
		modeIdx := 0
		if cfg.Schedule.Mode == "daily" {
			modeIdx = 1
		}

		checked := map[string]bool{}
		listBuilt := false

		computeInitial := func(list []api.Application) []string {
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

			initial := []string{}
			for _, app := range list {
				on := false
				if len(inclusions) > 0 {
					on = inclusions[app.ID]
				} else {
					on = !excludedIDs[app.ID] && !excludedNames[app.Name]
					if app.Status != "running" && !cfg.IncludeStopped {
						on = false
					}
				}
				if on {
					initial = append(initial, app.ID)
				}
			}
			return initial
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

				// Группировка по клиентам
				byCust := map[string]*pickerCustomer{}
				var order []string
				for _, app := range list {
					cu := byCust[app.CustomerID]
					if cu == nil {
						cu = &pickerCustomer{Name: names[app.CustomerID]}
						byCust[app.CustomerID] = cu
						order = append(order, app.CustomerID)
					}
					cu.Apps = append(cu.Apps, pickerAppJs{ID: app.ID, Name: app.Name, Status: app.Status})
				}
				sort.Slice(order, func(i, j int) bool { return byCust[order[i]].Name < byCust[order[j]].Name })
				customers := make([]pickerCustomer, 0, len(order))
				for _, cid := range order {
					cu := byCust[cid]
					sort.Slice(cu.Apps, func(i, j int) bool { return cu.Apps[i].Name < cu.Apps[j].Name })
					customers = append(customers, *cu)
				}

				initial := computeInitial(list)

				dlg.Synchronize(func() {
					data := pickerData{Customers: customers, Initial: initial}
					blob, err := json.Marshal(data)
					if err != nil {
						_ = pickStatus.SetText("Ошибка подготовки данных: " + err.Error())
						return
					}
					html := strings.Replace(pickerHTMLTemplate, "__DATA__", string(blob), 1)
					htmlPath := filepath.Join(os.TempDir(), "grm-picker.html")
					if err := os.WriteFile(htmlPath, []byte(html), 0644); err != nil {
						_ = pickStatus.SetText("Не удалось записать страницу: " + err.Error())
						return
					}
					url := "file:///" + filepath.ToSlash(htmlPath) + "?v=" + strconv.FormatInt(time.Now().UnixNano(), 10)
					if wv != nil {
						if err := wv.SetURL(url); err != nil {
							_ = pickStatus.SetText("Не удалось открыть страницу: " + err.Error())
							return
						}
					}
					_ = pickStatus.SetText(fmt.Sprintf("Загружено: приложений %d, клиентов %d", len(list), len(customers)))
				})
			}()
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
			Size:     Size{Width: 800, Height: 680},
			MinSize:  Size{Width: 680, Height: 560},
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
							Layout: VBox{Margins: Margins{Left: 0, Top: 0, Right: 0, Bottom: 0}, Spacing: 0},
							Children: []Widget{
								WebView{AssignTo: &wv, StretchFactor: 1, ShortcutsEnabled: true, NativeContextMenuEnabled: false},
								Label{AssignTo: &pickStatus, Text: "Загрузка списка приложений…"},
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

		centerForm(dlg, 800, 680)
		uiF := uiFont()
		applyFont(dlg, uiF)
		if tabs != nil && uiF != nil {
			for i := 0; i < tabs.Pages().Len(); i++ {
				if c, ok := interface{}(tabs.Pages().At(i)).(walk.Container); ok {
					applyFontTo(c, uiF)
				}
			}
		}

		// Мост HTML → Go: состояние выбора приходит через заголовок документа
		if wv != nil {
			wv.DocumentTitleChanged().Attach(func() {
				title := wv.DocumentTitle()
				switch {
				case strings.HasPrefix(title, "G|"):
					newChecked := map[string]bool{}
					for _, id := range strings.Split(title[2:], "|") {
						if id != "" {
							newChecked[id] = true
						}
					}
					checked = newChecked
					listBuilt = true
					m.Infof("Выбор приложений: отмечено %d", len(newChecked))
				case strings.HasPrefix(title, "CMD|refresh"):
					loadPicker()
				}
			})
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
