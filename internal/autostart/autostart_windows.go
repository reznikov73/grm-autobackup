//go:build windows

package autostart

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName  = "GrmAutoBackup"
)

// IsEnabled сообщает, включена ли наша запись автозапуска
func IsEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()

	v, _, err := k.GetStringValue(valueName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(v) != "", nil
}

// Sync включает или выключает автозапуск для указанного exe.
// При выключении удаляется только наша запись, и только если она указывает
// на этот же exe.
func Sync(enabled bool, exePath string) error {
	if enabled {
		k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		return k.SetStringValue(valueName, `"`+exePath+`"`)
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	v, _, err := k.GetStringValue(valueName)
	if err == registry.ErrNotExist {
		return nil
	}
	if err != nil {
		return err
	}
	if !sameExe(v, exePath) {
		return fmt.Errorf("значение %s указывает на другой файл (%s) — не трогаю", valueName, v)
	}
	return k.DeleteValue(valueName)
}

func sameExe(value, exePath string) bool {
	v := strings.TrimSpace(value)
	v = strings.Trim(v, `"`)
	return strings.EqualFold(v, exePath)
}
