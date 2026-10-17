//go:build !windows

package winutil

// EnsureSingleInstance — заглушка для не-Windows платформ
func EnsureSingleInstance(title, text string) bool { return true }
