package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"grm-autobackup/internal/autostart"
	"grm-autobackup/internal/config"
	"grm-autobackup/internal/monitor"
	"grm-autobackup/internal/scheduler"
	"grm-autobackup/internal/ui"
	"grm-autobackup/internal/webui"
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

	// Автозапуск: приводим запись в реестре в соответствие с настройкой.
	if err := autostart.Sync(cfg.Autostart, exePath); err != nil {
		m.Warnf("Автозапуск: %v", err)
	}

	s := scheduler.New(cfg, m)

	// Локальный сервер HTML-интерфейса (127.0.0.1, случайный порт и ключ).
	webSrv, webErr := webui.Start(cfg, cfgPath, s, m)
	if webErr != nil {
		m.Errorf("Не удалось запустить сервер интерфейса: %v", webErr)
	}
	defer func() {
		if webSrv != nil {
			webSrv.Stop()
		}
	}()

	// Тестовый хук для автотестов: GRM_TEST_WINDOW=settings|monitor|log
	if w := os.Getenv("GRM_TEST_WINDOW"); w != "" {
		go func() {
			time.Sleep(3 * time.Second)
			switch w {
			case "settings":
				ui.OpenSettingsWindow(cfg, cfgPath, s, m, webSrv)
			case "monitor":
				ui.OpenMonitorWindow(s, m, webSrv)
			case "log":
				ui.OpenLogWindow(m, webSrv)
			}
		}()
	}

	if err := ui.RunTray(cfg, cfgPath, s, m, webSrv); err != nil {
		m.Errorf("Ошибка запуска UI: %v", err)
	}

	m.Infof("Завершение работы")
	m.Close()
}
