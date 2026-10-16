//go:build !windows

package downloader

// diskFreeBytes — заглушка для не-Windows платформ: проверка места не выполняется
func diskFreeBytes(path string) (uint64, error) {
	return ^uint64(0), nil
}
