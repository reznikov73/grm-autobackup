//go:build windows

package winutil

import (
	"syscall"
	"unsafe"
)

var (
	modUser32       = syscall.NewLazyDLL("user32.dll")
	modKernel       = syscall.NewLazyDLL("kernel32.dll")
	procMsgBox      = modUser32.NewProc("MessageBoxW")
	procCreateMutex = modKernel.NewProc("CreateMutexW")
)

const mutexName = `Local\GrmAutoBackup_SingleInstance`

// EnsureSingleInstance сообщает, является ли это первым экземпляром приложения.
// Вторая копия показывает сообщение и завершается.
func EnsureSingleInstance(title, text string) bool {
	name, _ := syscall.UTF16PtrFromString(mutexName)
	h, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		// Не удалось создать мьютекс — не блокируем запуск.
		return true
	}
	if err == syscall.ERROR_ALREADY_EXISTS {
		t, _ := syscall.UTF16PtrFromString(text)
		c, _ := syscall.UTF16PtrFromString(title)
		procMsgBox.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), 0x40)
		return false
	}
	return true
}
