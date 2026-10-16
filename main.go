package main

import (
	"grm-autobackup/internal/config"
	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
	"grm-autobackup/internal/ui"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Загрузка конфигурации
	cfg, err := config.Load("data/config.json")
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// Инициализация монитора
	m := monitor.New()

	// Инициализация планировщика
	s := scheduler.New(cfg, m)

	// Запуск UI (трей)
	if err := ui.RunTray(cfg, s, m); err != nil {
		log.Fatalf("Ошибка запуска UI: %v", err)
	}

	// Ожидание сигналов завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Завершение работы...")
}
