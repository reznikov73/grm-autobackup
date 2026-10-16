//go:build !windows

package autostart

// IsEnabled — заглушка для не-Windows платформ
func IsEnabled() (bool, error) { return false, nil }

// Sync — заглушка для не-Windows платформ
func Sync(enabled bool, exePath string) error { return nil }
