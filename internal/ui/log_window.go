package ui

import (
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/webui"
)

// OpenLogWindow открывает окно журнала (HTML-интерфейс в WebView).
func OpenLogWindow(m *monitor.Monitor, srv *webui.Server) {
	if srv == nil {
		m.Errorf("Лог недоступен: сервер интерфейса не запущен")
		return
	}
	openWindow(m, func() (func() int, error) {
		var dlg *walk.MainWindow
		var wv *walk.WebView

		err := MainWindow{
			AssignTo: &dlg,
			Title:    "Лог — 1С:ГРМ AutoBackup",
			Size:     Size{Width: 960, Height: 600},
			MinSize:  Size{Width: 700, Height: 420},
			Layout:   VBox{Margins: Margins{Left: 0, Top: 0, Right: 0, Bottom: 0}, Spacing: 0},
			Children: []Widget{
				WebView{AssignTo: &wv, StretchFactor: 1, ShortcutsEnabled: true, NativeContextMenuEnabled: false},
			},
		}.Create()
		if err != nil {
			return nil, err
		}

		centerForm(dlg, 960, 600)
		if err := wv.SetURL(srv.URL("log")); err != nil {
			return nil, err
		}

		srv.RegisterCloser("log", func() {
			dlg.Synchronize(func() { _ = dlg.Close() })
		})

		return func() int {
			ret := dlg.Run()
			srv.UnregisterCloser("log")
			return ret
		}, nil
	})
}
