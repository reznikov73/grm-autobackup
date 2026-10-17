package ui

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"grm-autobackup/internal/config"
	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
	"grm-autobackup/internal/webui"
)

// OpenSettingsWindow открывает окно настроек (HTML-интерфейс в WebView).
func OpenSettingsWindow(cfg *config.Config, cfgPath string, s *scheduler.Scheduler, m *monitor.Monitor, srv *webui.Server) {
	if srv == nil {
		m.Errorf("Настройки недоступны: сервер интерфейса не запущен")
		return
	}
	openWindow(m, func() (func() int, error) {
		var dlg *walk.MainWindow
		var wv *walk.WebView

		err := MainWindow{
			AssignTo: &dlg,
			Title:    "Настройки — 1С:ГРМ AutoBackup",
			Size:     Size{Width: 940, Height: 720},
			MinSize:  Size{Width: 760, Height: 560},
			Layout:   VBox{Margins: Margins{Left: 0, Top: 0, Right: 0, Bottom: 0}, Spacing: 0},
			Children: []Widget{
				WebView{AssignTo: &wv, StretchFactor: 1, ShortcutsEnabled: true, NativeContextMenuEnabled: false},
			},
		}.Create()
		if err != nil {
			return nil, err
		}

		centerForm(dlg, 940, 720)
		if err := wv.SetURL(srv.URL("settings")); err != nil {
			return nil, err
		}

		srv.RegisterCloser("settings", func() {
			dlg.Synchronize(func() { _ = dlg.Close() })
		})

		return func() int {
			ret := dlg.Run()
			srv.UnregisterCloser("settings")
			return ret
		}, nil
	})
}
