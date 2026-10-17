// Package webui — локальный HTTP-сервер для HTML-интерфейса (127.0.0.1, случайный порт и ключ).
package webui

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"grm-autobackup/internal/api"
	"grm-autobackup/internal/autostart"
	"grm-autobackup/internal/config"
	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
)

//go:embed assets/*
var assetsFS embed.FS

// Server обслуживает HTML-страницы интерфейса и JSON-API для них.
type Server struct {
	ln      net.Listener
	httpSrv *http.Server
	key     string

	cfg     *config.Config
	cfgPath string
	sched   *scheduler.Scheduler
	mon     *monitor.Monitor

	mu            sync.Mutex
	closers       map[string]func()
	folderPickers map[string]func() (string, error)
}

// Start запускает сервер на 127.0.0.1 со случайным портом и ключом доступа.
func Start(cfg *config.Config, cfgPath string, s *scheduler.Scheduler, m *monitor.Monitor) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	rawKey := make([]byte, 16)
	if _, err := rand.Read(rawKey); err != nil {
		ln.Close()
		return nil, err
	}

	srv := &Server{
		ln:            ln,
		key:           hex.EncodeToString(rawKey),
		cfg:           cfg,
		cfgPath:       cfgPath,
		sched:         s,
		mon:           m,
		closers:       make(map[string]func()),
		folderPickers: make(map[string]func() (string, error)),
	}

	mux := http.NewServeMux()
	assetsSub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		ln.Close()
		return nil, err
	}
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsSub))))
	mux.HandleFunc("/settings", srv.servePage("settings.html"))
	mux.HandleFunc("/monitor", srv.servePage("monitor.html"))
	mux.HandleFunc("/log", srv.servePage("log.html"))
	mux.HandleFunc("/api/settings", srv.auth(srv.handleSettings))
	mux.HandleFunc("/api/apps", srv.auth(srv.handleApps))
	mux.HandleFunc("/api/save", srv.auth(srv.handleSave))
	mux.HandleFunc("/api/state", srv.auth(srv.handleState))
	mux.HandleFunc("/api/logs", srv.auth(srv.handleLogs))
	mux.HandleFunc("/api/logs/clear", srv.auth(srv.handleLogsClear))
	mux.HandleFunc("/api/refresh", srv.auth(srv.handleRefresh))
	mux.HandleFunc("/api/close", srv.auth(srv.handleClose))
	mux.HandleFunc("/api/pickfolder", srv.auth(srv.handlePickFolder))

	nocache := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		mux.ServeHTTP(w, r)
	})
	srv.httpSrv = &http.Server{Handler: nocache}
	go func() {
		_ = srv.httpSrv.Serve(ln)
	}()
	return srv, nil
}

// URL возвращает адрес страницы интерфейса.
func (s *Server) URL(page string) string {
	port := 0
	if addr, ok := s.ln.Addr().(*net.TCPAddr); ok {
		port = addr.Port
	}
	return fmt.Sprintf("http://127.0.0.1:%d/%s?k=%s", port, page, s.key)
}

// RegisterCloser регистрирует обработчик закрытия окна страницы.
func (s *Server) RegisterCloser(page string, fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closers[page] = fn
}

// UnregisterCloser удаляет обработчик закрытия окна страницы.
func (s *Server) UnregisterCloser(page string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.closers, page)
}

// RegisterFolderPicker регистрирует диалог выбора папки для страницы.
func (s *Server) RegisterFolderPicker(page string, fn func() (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.folderPickers[page] = fn
}

// UnregisterFolderPicker удаляет диалог выбора папки.
func (s *Server) UnregisterFolderPicker(page string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.folderPickers, page)
}

// Stop останавливает сервер.
func (s *Server) Stop() {
	if s.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.httpSrv.Shutdown(ctx)
	}
}

func (s *Server) auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		k := r.URL.Query().Get("k")
		if k == "" {
			k = r.Header.Get("X-Key")
		}
		if k != s.key {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

func (s *Server) servePage(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := assetsFS.ReadFile("assets/" + name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, msg string) {
	writeJSON(w, map[string]interface{}{"ok": false, "error": msg})
}

// --- /api/settings ---

type settingsResp struct {
	API struct {
		URL string `json:"url"`
		JWT string `json:"jwt"`
	} `json:"api"`
	Schedule struct {
		Enabled  bool    `json:"enabled"`
		Mode     string  `json:"mode"`
		Interval float64 `json:"interval"`
		Start    string  `json:"start"`
	} `json:"schedule"`
	Storage struct {
		Path  string `json:"path"`
		Limit int    `json:"limit"`
	} `json:"storage"`
	Level          string `json:"level"`
	IncludeStopped bool   `json:"includeStopped"`
	Autostart      bool   `json:"autostart"`
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	var resp settingsResp
	resp.API.URL = s.cfg.API.URL
	resp.API.JWT = s.cfg.API.JWTToken
	resp.Schedule.Enabled = s.cfg.Schedule.Enabled
	resp.Schedule.Mode = s.cfg.Schedule.Mode
	resp.Schedule.Interval = s.cfg.Schedule.IntervalHours
	resp.Schedule.Start = s.cfg.Schedule.StartTime
	resp.Storage.Path = s.cfg.Storage.Path
	resp.Storage.Limit = s.cfg.Storage.BackupLimit
	resp.Level = s.cfg.Logging.Level
	resp.IncludeStopped = s.cfg.IncludeStopped
	resp.Autostart = s.cfg.Autostart
	writeJSON(w, resp)
}

// --- /api/apps ---

type appItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type customerItem struct {
	Name string    `json:"name"`
	Apps []appItem `json:"apps"`
}

type appsResp struct {
	Customers []customerItem `json:"customers"`
	Initial   []string       `json:"initial"`
}

func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	client := api.New(s.cfg.API.URL, s.cfg.API.JWTToken, s.cfg.API.TimeoutSec)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	list, err := client.GetApplications(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	names := map[string]string{}
	for _, app := range list {
		if _, ok := names[app.CustomerID]; ok {
			continue
		}
		if cust, err := client.GetCustomer(ctx, app.CustomerID); err == nil && cust.Name != "" {
			names[app.CustomerID] = cust.Name
		} else {
			names[app.CustomerID] = "Клиент " + shortID(app.CustomerID)
		}
	}

	byCust := map[string]*customerItem{}
	var order []string
	for _, app := range list {
		cu := byCust[app.CustomerID]
		if cu == nil {
			cu = &customerItem{Name: names[app.CustomerID]}
			byCust[app.CustomerID] = cu
			order = append(order, app.CustomerID)
		}
		cu.Apps = append(cu.Apps, appItem{ID: app.ID, Name: app.Name, Status: app.Status})
	}
	sort.Slice(order, func(i, j int) bool { return byCust[order[i]].Name < byCust[order[j]].Name })

	customers := make([]customerItem, 0, len(order))
	for _, cid := range order {
		cu := byCust[cid]
		sort.Slice(cu.Apps, func(i, j int) bool { return cu.Apps[i].Name < cu.Apps[j].Name })
		customers = append(customers, *cu)
	}

	writeJSON(w, appsResp{Customers: customers, Initial: s.computeInitial(list)})
}

func (s *Server) computeInitial(list []api.Application) []string {
	inclusions := make(map[string]bool, len(s.cfg.Inclusions.ApplicationIDs))
	for _, id := range s.cfg.Inclusions.ApplicationIDs {
		inclusions[id] = true
	}
	excludedIDs := make(map[string]bool, len(s.cfg.Exclusions.ApplicationIDs))
	for _, id := range s.cfg.Exclusions.ApplicationIDs {
		excludedIDs[id] = true
	}
	excludedNames := make(map[string]bool, len(s.cfg.Exclusions.ApplicationNames))
	for _, n := range s.cfg.Exclusions.ApplicationNames {
		excludedNames[n] = true
	}

	initial := []string{}
	for _, app := range list {
		on := false
		if len(inclusions) > 0 {
			on = inclusions[app.ID]
		} else {
			on = !excludedIDs[app.ID] && !excludedNames[app.Name]
			if app.Status != "running" && !s.cfg.IncludeStopped {
				on = false
			}
		}
		if on {
			initial = append(initial, app.ID)
		}
	}
	return initial
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// --- /api/save ---

type saveReq struct {
	URL            string   `json:"url"`
	JWT            string   `json:"jwt"`
	Path           string   `json:"path"`
	Limit          float64  `json:"limit"`
	Mode           string   `json:"mode"`
	Interval       float64  `json:"interval"`
	Start          string   `json:"start"`
	IncludeStopped bool     `json:"includeStopped"`
	Autostart      bool     `json:"autostart"`
	Level          string   `json:"level"`
	Checked        []string `json:"checked"`
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req saveReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, "некорректный запрос: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Path) == "" {
		writeErr(w, "укажите папку сохранения бэкапов")
		return
	}
	if _, err := time.Parse("15:04", strings.TrimSpace(req.Start)); err != nil {
		writeErr(w, "время в расписании должно быть в формате ЧЧ:ММ")
		return
	}
	if len(req.Checked) == 0 {
		writeErr(w, "отметьте хотя бы одно приложение")
		return
	}
	limit := int(req.Limit)
	if limit < 1 {
		limit = 1
	}
	interval := req.Interval
	if interval < 1 {
		interval = 6
	}
	mode := req.Mode
	if mode != "daily" {
		mode = "interval"
	}

	ids := append([]string{}, req.Checked...)
	sort.Strings(ids)

	s.cfg.Inclusions.ApplicationIDs = ids
	s.cfg.Exclusions.ApplicationIDs = []string{}
	s.cfg.Exclusions.ApplicationNames = []string{}
	s.cfg.API.URL = strings.TrimSpace(req.URL)
	s.cfg.API.JWTToken = strings.TrimSpace(req.JWT)
	s.cfg.Storage.Path = strings.TrimSpace(req.Path)
	s.cfg.Storage.BackupLimit = limit
	s.cfg.Schedule.Mode = mode
	s.cfg.Schedule.IntervalHours = interval
	s.cfg.Schedule.StartTime = strings.TrimSpace(req.Start)
	s.cfg.IncludeStopped = req.IncludeStopped
	s.cfg.Autostart = req.Autostart
	if req.Level != "" {
		s.cfg.Logging.Level = req.Level
	}

	if err := s.cfg.Save(s.cfgPath); err != nil {
		writeErr(w, "не удалось сохранить: "+err.Error())
		return
	}

	s.mon.SetLevel(s.cfg.Logging.Level)
	s.sched.Reload()

	autostartWarn := ""
	if exe, err := exePath(); err == nil {
		if err := autostart.Sync(s.cfg.Autostart, exe); err != nil {
			s.mon.Warnf("Автозапуск: %v", err)
			autostartWarn = err.Error()
		}
	}

	s.mon.Infof("Настройки сохранены: %s (приложений: %d)", s.cfgPath, len(ids))
	writeJSON(w, map[string]interface{}{"ok": true, "autostartWarning": autostartWarn, "checkedCount": len(ids)})
}

// --- /api/state ---

type histItem struct {
	Time   string `json:"time"`
	Desc   string `json:"desc"`
	Result string `json:"result"`
}

type stateResp struct {
	Type        string     `json:"type"`
	Description string     `json:"description"`
	Progress    float64    `json:"progress"`
	Speed       float64    `json:"speed"`
	Written     int64      `json:"written"`
	Total       int64      `json:"total"`
	Error       string     `json:"error"`
	NextRun     string     `json:"nextRun"`
	History     []histItem `json:"history"`
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	action := s.mon.GetCurrent()
	var resp stateResp
	resp.Type = string(action.Type)
	resp.Description = action.Description
	resp.Progress = action.Progress
	resp.Speed = action.SpeedBps
	resp.Written = action.BytesWritten
	resp.Total = action.BytesTotal
	resp.Error = action.Error

	if next := s.sched.NextRun(); !next.IsZero() {
		resp.NextRun = next.Format("02.01.2006 15:04")
	}

	hist := s.mon.GetHistory()
	resp.History = make([]histItem, 0, len(hist))
	for i := len(hist) - 1; i >= 0; i-- {
		a := hist[i]
		result := "ok"
		if a.Type == monitor.StatusError {
			result = "ошибка"
		}
		resp.History = append(resp.History, histItem{
			Time:   a.FinishedAt.Format("02.01 15:04:05"),
			Desc:   a.Description,
			Result: result,
		})
	}
	writeJSON(w, resp)
}

// --- /api/logs ---

type logsResp struct {
	Lines []string `json:"lines"`
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	level := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("level")))
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	lines := s.mon.GetLogs(0)
	filtered := make([]string, 0, len(lines))
	for _, l := range lines {
		if level != "" && level != "ALL" && !strings.Contains(l, "["+level+"]") {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(l), q) {
			continue
		}
		filtered = append(filtered, l)
	}
	if len(filtered) > 3000 {
		filtered = filtered[len(filtered)-3000:]
	}
	writeJSON(w, logsResp{Lines: filtered})
}

func (s *Server) handleLogsClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mon.ClearLogs()
	writeJSON(w, map[string]interface{}{"ok": true})
}

// --- /api/refresh и /api/close ---

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.sched.RunNow()
	writeJSON(w, map[string]interface{}{"ok": true})
}

func (s *Server) handleClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Page string `json:"page"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req)

	s.mu.Lock()
	fn := s.closers[req.Page]
	s.mu.Unlock()
	if fn != nil {
		go fn()
	}
	writeJSON(w, map[string]interface{}{"ok": true})
}

func (s *Server) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Page string `json:"page"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req)

	s.mu.Lock()
	fn := s.folderPickers[req.Page]
	s.mu.Unlock()
	if fn == nil {
		writeErr(w, "выбор папки недоступен")
		return
	}
	path, err := fn()
	if err != nil {
		writeErr(w, err.Error())
		return
	}
	if path == "" {
		writeJSON(w, map[string]interface{}{"ok": false, "canceled": true})
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "path": path})
}

// exePath возвращает путь к исполняемому файлу.
func exePath() (string, error) { return os.Executable() }
