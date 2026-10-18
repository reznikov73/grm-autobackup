//go:build !windows

package winutil

// EnsureSingleInstance — заглушка для не-Windows платформ
func EnsureSingleInstance() bool { return true }

// SignalExistingInstance — заглушка для не-Windows платформ
func SignalExistingInstance() {}

// OnActivateRequested — заглушка для не-Windows платформ
func OnActivateRequested(cb func()) {}
