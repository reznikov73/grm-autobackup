package ui

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"

	"github.com/lxn/walk"
	"github.com/lxn/win"

	"grm-autobackup/internal/monitor"
)

// Ограничение: одно окно каждого типа одновременно (разные окна — можно).
var (
	windowMu  sync.Mutex
	openPages = map[string]bool{}
)

// windowTitles — заголовки окон по страницам (для поиска окна при активации).
var windowTitles = map[string]string{
	"settings": "Настройки — 1С:ГРМ AutoBackup",
	"monitor":  "Монитор действий — 1С:ГРМ AutoBackup",
	"log":      "Лог — 1С:ГРМ AutoBackup",
}

func acquireWindow(page string) bool {
	windowMu.Lock()
	defer windowMu.Unlock()
	if openPages[page] {
		return false
	}
	openPages[page] = true
	return true
}

func releaseWindow(page string) {
	windowMu.Lock()
	openPages[page] = false
	delete(openPages, page)
	windowMu.Unlock()
}

// activateWindow поднимает уже открытое окно страницы на передний план.
func activateWindow(page string) bool {
	title := windowTitles[page]
	if title == "" {
		return false
	}
	p, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return false
	}
	h := win.FindWindow(nil, p)
	if h == 0 {
		return false
	}
	win.ShowWindow(h, win.SW_RESTORE)
	win.SetForegroundWindow(h)
	return true
}

// openWindow создаёт окно walk в отдельной горутине.
// Каждое окно walk должно создаваться на OS-потоке, привязанном к горутине
// (см. требования walk), поэтому горутина жёстко привязывается к потоку.
func openWindow(page string, m *monitor.Monitor, build func() (run func() int, err error)) bool {
	if !acquireWindow(page) {
		if activateWindow(page) {
			m.Infof("Окно «%s» уже открыто — выведено на передний план", page)
		} else {
			m.Infof("Окно «%s» уже открыто", page)
		}
		return false
	}
	go func() {
		defer releaseWindow(page)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		run, err := build()
		if err != nil {
			m.Errorf("Не удалось создать окно: %v", err)
			return
		}
		run()
	}()
	return true
}

// centerForm задаёт окну компактный размер и располагает его по центру экрана.
func centerForm(form *walk.MainWindow, width, height int) {
	sw := int(win.GetSystemMetrics(win.SM_CXSCREEN))
	sh := int(win.GetSystemMetrics(win.SM_CYSCREEN))
	if sw <= 0 || sh <= 0 {
		return
	}
	if width > sw-40 {
		width = sw - 40
	}
	if height > sh-80 {
		height = sh - 80
	}
	x := (sw - width) / 2
	y := (sh - height) / 3
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	_ = form.SetBounds(walk.Rectangle{X: x, Y: y, Width: width, Height: height})
}

// uiFont возвращает базовый шрифт интерфейса (Segoe UI 9pt)
func uiFont() *walk.Font {
	f, err := walk.NewFont("Segoe UI", 9, 0)
	if err != nil {
		return nil
	}
	return f
}

// uiFontBold возвращает полужирный шрифт интерфейса
func uiFontBold(pointSize int) *walk.Font {
	f, err := walk.NewFont("Segoe UI", pointSize, walk.FontBold)
	if err != nil {
		return nil
	}
	return f
}

// applyFontTo рекурсивно применяет шрифт ко всем дочерним виджетам контейнера
func applyFontTo(c walk.Container, font *walk.Font) {
	if c == nil || font == nil {
		return
	}
	children := c.Children()
	for i := 0; i < children.Len(); i++ {
		w := children.At(i)
		if wf, ok := w.(interface{ SetFont(*walk.Font) }); ok {
			wf.SetFont(font)
		}
		if cc, ok := w.(walk.Container); ok {
			applyFontTo(cc, font)
		}
	}
}

// applyFont применяет шрифт к окну и всем вложенным виджетам
func applyFont(form *walk.MainWindow, font *walk.Font) {
	if form == nil || font == nil {
		return
	}
	form.SetFont(font)
	applyFontTo(form, font)
}

// formatBytes форматирует размер в человекочитаемый вид
func formatBytes(n int64) string {
	f := float64(n)
	units := []string{"Б", "КБ", "МБ", "ГБ", "ТБ"}
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

// formatSpeed форматирует скорость в виде "5.1 МБ/с"
func formatSpeed(bps float64) string {
	return formatBytes(int64(bps)) + "/с"
}

// formatETA форматирует оставшееся время
func formatETA(seconds float64) string {
	if seconds < 0 || seconds > 30*24*3600 {
		return "—"
	}
	s := int(seconds)
	h := s / 3600
	mnt := (s % 3600) / 60
	sec := s % 60
	if h > 0 {
		return fmt.Sprintf("%d ч %02d мин", h, mnt)
	}
	if mnt > 0 {
		return fmt.Sprintf("%d мин %02d с", mnt, sec)
	}
	return fmt.Sprintf("%d с", sec)
}
