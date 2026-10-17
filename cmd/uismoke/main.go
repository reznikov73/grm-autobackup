package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"grm-autobackup/internal/config"
	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
	"grm-autobackup/internal/ui"
	"grm-autobackup/internal/webui"
)

// uismoke — служебный помощник для проверки окон UI.
// Использование: uismoke settings|monitor|log [путь-к-config.json]
func main() {
	if len(os.Args) < 2 {
		fmt.Println("использование: uismoke settings|monitor|log [путь-к-config.json]")
		os.Exit(2)
	}
	which := os.Args[1]
	cfgPath := "data/config.json"
	if len(os.Args) >= 3 {
		cfgPath = os.Args[2]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Println("ошибка загрузки конфигурации:", err)
		os.Exit(1)
	}

	m := monitor.New()
	s := scheduler.New(cfg, m)

	srv, err := webui.Start(cfg, cfgPath, s, m)
	if err != nil {
		fmt.Println("ошибка запуска сервера интерфейса:", err)
		os.Exit(1)
	}
	defer srv.Stop()

	switch which {
	case "settings":
		ui.OpenSettingsWindow(cfg, cfgPath, s, m, srv)
	case "monitor":
		ui.OpenMonitorWindow(s, m, srv)
	case "log":
		ui.OpenLogWindow(m, srv)
	default:
		fmt.Println("неизвестное окно:", which)
		os.Exit(2)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
