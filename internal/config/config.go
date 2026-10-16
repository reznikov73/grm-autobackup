package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

// Config хранит всю конфигурацию приложения
type Config struct {
	API            APIConfig        `json:"api"`
	Schedule       ScheduleConfig   `json:"schedule"`
	Storage        StorageConfig    `json:"storage"`
	Exclusions     ExclusionsConfig `json:"exclusions"`
	Logging        LoggingConfig    `json:"logging"`
	IncludeStopped bool             `json:"include_stopped"`
	Autostart      bool             `json:"autostart"`
}

// APIConfig настройки API
type APIConfig struct {
	URL        string `json:"url"`
	JWTToken   string `json:"jwt_token"`
	TimeoutSec int    `json:"timeout_seconds"`
}

// ScheduleConfig настройки расписания
type ScheduleConfig struct {
	Enabled       bool    `json:"enabled"`
	Mode          string  `json:"mode"` // "interval" (каждые N часов) или "daily" (ежедневно в StartTime)
	IntervalHours float64 `json:"interval_hours"`
	StartTime     string  `json:"start_time"` // ЧЧ:ММ для режима daily
}

// StorageConfig настройки хранилища
type StorageConfig struct {
	Path           string `json:"path"`
	BackupLimit    int    `json:"backup_limit"`
	MaxDiskUsageMB int    `json:"max_disk_usage_mb"`
}

// ExclusionsConfig исключения
type ExclusionsConfig struct {
	ApplicationIDs   []string `json:"application_ids"`
	ApplicationNames []string `json:"application_names"`
}

// LoggingConfig настройки логирования
type LoggingConfig struct {
	Level      string `json:"level"`
	File       string `json:"file"`
	MaxSizeMB  int    `json:"max_size_mb"`
	MaxBackups int    `json:"max_backups"`
}

// Default возвращает конфигурацию со значениями по умолчанию
func Default() *Config {
	cfg := &Config{}
	cfg.API.URL = "https://service-api.1capp.com/partner-api/v2"
	cfg.API.TimeoutSec = 300
	cfg.Schedule.Enabled = true
	cfg.Schedule.Mode = "interval"
	cfg.Schedule.IntervalHours = 6
	cfg.Schedule.StartTime = "00:00"
	cfg.Storage.Path = "E:\\Backups\\1cGRM"
	cfg.Storage.BackupLimit = 7
	cfg.Storage.MaxDiskUsageMB = 10000
	cfg.Exclusions.ApplicationIDs = []string{}
	cfg.Exclusions.ApplicationNames = []string{}
	cfg.Logging.Level = "info"
	cfg.Logging.File = "logs/autobackup.log"
	cfg.Logging.MaxSizeMB = 50
	cfg.Logging.MaxBackups = 5
	return cfg
}

// applyDefaults дополняет пустые значения значениями по умолчанию
func (c *Config) applyDefaults() {
	if c.API.URL == "" {
		c.API.URL = "https://service-api.1capp.com/partner-api/v2"
	}
	if c.API.TimeoutSec <= 0 {
		c.API.TimeoutSec = 300
	}
	if c.Schedule.Mode != "daily" {
		c.Schedule.Mode = "interval"
	}
	if c.Schedule.IntervalHours <= 0 {
		c.Schedule.IntervalHours = 6
	}
	if c.Storage.BackupLimit == 0 {
		c.Storage.BackupLimit = 7
	}
	if c.Storage.MaxDiskUsageMB == 0 {
		c.Storage.MaxDiskUsageMB = 10000
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.File == "" {
		c.Logging.File = "logs/autobackup.log"
	}
	if c.Logging.MaxSizeMB == 0 {
		c.Logging.MaxSizeMB = 50
	}
	if c.Logging.MaxBackups == 0 {
		c.Logging.MaxBackups = 5
	}
}

// Load загружает конфигурацию из файла
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Устойчивость к BOM в начале файла
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	return &cfg, nil
}

// Save сохраняет конфигурацию в файл
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0644)
}
