//go:build windows

package winutil

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")

	procCreateMutexW        = modKernel32.NewProc("CreateMutexW")
	procCreateEventW        = modKernel32.NewProc("CreateEventW")
	procOpenEventW          = modKernel32.NewProc("OpenEventW")
	procSetEvent            = modKernel32.NewProc("SetEvent")
	procCloseHandle         = modKernel32.NewProc("CloseHandle")
	procWaitForSingleObject = modKernel32.NewProc("WaitForSingleObject")
)

const (
	mutexName    = `Local\GrmAutoBackup_SingleInstance`
	activateName = `Local\GrmAutoBackup_Activate`

	eventModifyState = 0x0002
	waitObject0      = 0
)

// EnsureSingleInstance сообщает, является ли это первым экземпляром приложения.
func EnsureSingleInstance() bool {
	name, _ := syscall.UTF16PtrFromString(mutexName)
	h, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		// Не удалось создать мьютекс — не блокируем запуск.
		return true
	}
	if err == syscall.ERROR_ALREADY_EXISTS {
		return false
	}
	return true
}

// SignalExistingInstance будит уже запущенную копию, чтобы она открыла настройки.
func SignalExistingInstance() {
	name, _ := syscall.UTF16PtrFromString(activateName)
	// Небольшие повторы покрывают случай, когда первая копия ещё стартует.
	for i := 0; i < 15; i++ {
		h, _, _ := procOpenEventW.Call(eventModifyState, 0, uintptr(unsafe.Pointer(name)))
		if h != 0 {
			procSetEvent.Call(h)
			procCloseHandle.Call(h)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// OnActivateRequested вызывает cb при каждом повторном запуске приложения.
func OnActivateRequested(cb func()) {
	name, _ := syscall.UTF16PtrFromString(activateName)
	// Авто-сбрасываемое событие: сигнал не накапливается.
	h, _, _ := procCreateEventW.Call(0, 0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return
	}
	go func() {
		for {
			r, _, _ := procWaitForSingleObject.Call(h, 0xFFFFFFFF) // INFINITE
			if uint32(r) != waitObject0 {
				return
			}
			cb()
			// Склеиваем частые повторные запуски.
			time.Sleep(300 * time.Millisecond)
		}
	}()
}
