package ui

import (
	"runtime"
	"sync"

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
