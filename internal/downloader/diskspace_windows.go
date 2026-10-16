//go:build windows

package downloader

import (
	"golang.org/x/sys/windows"
)

// diskFreeBytes возвращает свободное место на диске, содержащем путь
func diskFreeBytes(path string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var freeAvailable, total, freeTotal uint64
	if err := windows.GetDiskFreeSpaceEx(p, &freeAvailable, &total, &freeTotal); err != nil {
		return 0, err
	}
	return freeAvailable, nil
}
