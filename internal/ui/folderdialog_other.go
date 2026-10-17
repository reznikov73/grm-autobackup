//go:build !windows

package ui

// pickFolder — заглушка для не-Windows платформ
func pickFolder(owner uintptr, title string) (string, bool) {
	return "", false
}
