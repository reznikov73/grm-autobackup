// Package icons содержит иконки трея и приложения.
package icons

import (
	"embed"
	"fmt"
)

//go:embed *.ico
var fs embed.FS

func mustRead(name string) []byte {
	data, err := fs.ReadFile(name)
	if err != nil {
		return nil
	}
	return data
}

// App — иконка приложения (встраивается в exe через rsrc.syso)
func App() []byte { return mustRead("app.ico") }

// Idle — состояние «готов»
func Idle() []byte { return mustRead("idle.ico") }

// Cleaning — состояние «очистка»
func Cleaning() []byte { return mustRead("cleaning.ico") }

// Error — состояние «ошибка»
func Error() []byte { return mustRead("error.ico") }

// Paused — состояние «остановлено»
func Paused() []byte { return mustRead("paused.ico") }

// Downloading — кадр иконки «скачивание» для прогресса 0..100 (шаг 10%)
func Downloading(percent float64) []byte {
	p := int(percent)
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	p = (p / 10) * 10
	return mustRead(fmt.Sprintf("downloading_%03d.ico", p))
}
