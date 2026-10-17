package ui

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/lxn/walk"
	"github.com/lxn/win"

	"grm-autobackup/internal/monitor"
)

// Ограничение: одновременно разрешено только одно открытое окно walk.
var (
	windowMu   sync.Mutex
	windowOpen bool
)

func acquireWindow() bool {
	windowMu.Lock()
	defer windowMu.Unlock()
	if windowOpen {
		return false
	}
	windowOpen = true
	return true
}

func releaseWindow() {
	windowMu.Lock()
	windowOpen = false
	windowMu.Unlock()
}

// openWindow создаёт окно walk в отдельной горутине.
// Каждое окно walk должно создаваться на OS-потоке, привязанном к горутине
// (см. требования walk), поэтому горутина жёстко привязывается к потоку.
func openWindow(m *monitor.Monitor, build func() (run func() int, err error)) bool {
	if !acquireWindow() {
		m.Infof("Окно уже открыто — закройте текущее окно и повторите")
		return false
	}
	go func() {
		defer releaseWindow()
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
