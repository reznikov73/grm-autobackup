package ui

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
	"grm-autobackup/internal/webui"
)

// OpenMonitorWindow открывает окно монитора действий (HTML-интерфейс в WebView).
func OpenMonitorWindow(s *scheduler.Scheduler, m *monitor.Monitor, srv *webui.Server) {
	if srv == nil {
		m.Errorf("Монитор недоступен: сервер интерфейса не запущен")
		return
	}
	openWindow(m, func() (func() int, error) {
		var dlg *walk.MainWindow
		var wv *walk.WebView

		err := MainWindow{
			AssignTo: &dlg,
			Title:    "Монитор действий — 1С:ГРМ AutoBackup",
			Size:     Size{Width: 860, Height: 560},
			MinSize:  Size{Width: 640, Height: 420},
			Layout:   VBox{Margins: Margins{Left: 0, Top: 0, Right: 0, Bottom: 0}, Spacing: 0},
			Children: []Widget{
				WebView{AssignTo: &wv, StretchFactor: 1, ShortcutsEnabled: true, NativeContextMenuEnabled: false},
			},
		}.Create()
		if err != nil {
			return nil, err
		}

		centerForm(dlg, 860, 560)
		if err := wv.SetURL(srv.URL("monitor")); err != nil {
			return nil, err
		}

		srv.RegisterCloser("monitor", func() {
			dlg.Synchronize(func() { _ = dlg.Close() })
		})

		return func() int {
			ret := dlg.Run()
			srv.UnregisterCloser("monitor")
			return ret
		}, nil
	})
}
