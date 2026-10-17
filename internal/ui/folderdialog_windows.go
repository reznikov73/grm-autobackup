//go:build windows

package ui

import (
	"syscall"
	"unsafe"
)

// browseInfoW — структура BROWSEINFOW для SHBrowseForFolderW
type browseInfoW struct {
	owner       syscall.Handle
	root        uintptr
	displayName *uint16
	title       *uint16
	flags       uint32
	callback    uintptr
	lParam      uintptr
	imageIndex  int32
}

const (
	bifReturnOnlyFSDirs = 0x00000001
	bifEditBox          = 0x00000010
	bifNewDialogStyle   = 0x00000040
	coinitApartment     = 0x2
)

var (
	modShell32               = syscall.NewLazyDLL("shell32.dll")
	modOle32                 = syscall.NewLazyDLL("ole32.dll")
	procSHBrowseForFolder    = modShell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListW = modShell32.NewProc("SHGetPathFromIDListW")
	procCoTaskMemFree        = modOle32.NewProc("CoTaskMemFree")
	procCoInitializeEx       = modOle32.NewProc("CoInitializeEx")
)

// pickFolder показывает системный диалог выбора папки; ok=false при отмене.
func pickFolder(owner uintptr, title string) (path string, ok bool) {
	// COM может быть уже инициализирован в STA — результат не важен.
	procCoInitializeEx.Call(0, coinitApartment)

	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return "", false
	}
	display := make([]uint16, syscall.MAX_PATH)

	bi := browseInfoW{
		owner:       syscall.Handle(owner),
		displayName: &display[0],
		title:       titlePtr,
		flags:       bifReturnOnlyFSDirs | bifEditBox | bifNewDialogStyle,
	}

	ret, _, _ := procSHBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
	if ret == 0 {
		return "", false
	}
	defer procCoTaskMemFree.Call(ret)

	out := make([]uint16, syscall.MAX_PATH)
	r, _, _ := procSHGetPathFromIDListW.Call(ret, uintptr(unsafe.Pointer(&out[0])))
	if r == 0 {
		return "", false
	}
	return syscall.UTF16ToString(out), true
}
