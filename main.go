package main

import (
	"fmt"
	"os"
	"path/filepath"

	"grm-autobackup/internal/config"
	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
	"grm-autobackup/internal/ui"
)

func main() {
	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}
	exeDir := filepath.Dir(exePath)

	// Ищем конфигурацию: сначала рядом с exe, затем в текущей рабочей папке.
	exeCfg := filepath.Join(exeDir, "data", "config.json")
	cwdCfg := filepath.Join("data", "config.json")

	var cfg *config.Config
	var cfgPath string
	var loadErr error

	if _, statErr := os.Stat(exeCfg); statErr == nil {
		cfgPath = exeCfg
		cfg, loadErr = config.Load(exeCfg)
	} else if _, statErr := os.Stat(cwdCfg); statErr == nil {
		cfgPath = cwdCfg
		cfg, loadErr = config.Load(cwdCfg)
	} else {
		// Файла нет — создаём дефолтную конфигурацию рядом с exe.
		cfgPath = exeCfg
		cfg = config.Default()
		if saveErr := cfg.Save(exeCfg); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Не удалось создать конфигурацию %s: %v\n", exeCfg, saveErr)
		}
	}

	if loadErr != nil {
		fmt.Fprintf(os.Stderr, "Ошибка загрузки конфигурации %s: %v\n", cfgPath, loadErr)
		os.Exit(1)
	}

	// Базовая папка для относительных путей — корень проекта
	// (конфиг лежит в <корень>/data/config.json).
	baseDir := filepath.Dir(filepath.Dir(cfgPath))

	m := monitor.New()

	logPath := cfg.Logging.File
	if logPath == "" {
		logPath = filepath.Join("logs", "autobackup.log")
	}
	if !filepath.IsAbs(logPath) {
		logPath = filepath.Join(baseDir, logPath)
	}
	if err := m.SetupFile(logPath, cfg.Logging.MaxSizeMB, cfg.Logging.MaxBackups, cfg.Logging.Level); err != nil {
		fmt.Fprintf(os.Stderr, "Файловый лог недоступен (%s): %v\n", logPath, err)
	} else {
		m.Infof("=== Запуск grm-autobackup ===")
		m.Infof("Конфигурация: %s", cfgPath)
		m.Infof("Лог: %s", logPath)
	}
	if cfg.API.JWTToken == "" {
		m.Warnf("JWT токен не заполнен — заполните api.jwt_token в %s", cfgPath)
	}

	s := scheduler.New(cfg, m)

	if err := ui.RunTray(cfg, s, m); err != nil {
		m.Errorf("Ошибка запуска UI: %v", err)
	}

	m.Infof("Завершение работы")
	m.Close()
}
