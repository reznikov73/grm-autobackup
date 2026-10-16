package config

import (
	"encoding/json"
	"os"
)

// Config хранит всю конфигурацию приложения
type Config struct {
	API         APIConfig         `json:"api"`
	Schedule    ScheduleConfig    `json:"schedule"`
	Storage     StorageConfig     `json:"storage"`
	Exclusions  ExclusionsConfig  `json:"exclusions"`
	Logging     LoggingConfig     `json:"logging"`
}

// APIConfig настройки API
type APIConfig struct {
	URL           string `json:"url"`
	JWTToken      string `json:"jwt_token"`
	TimeoutSec    int    `json:"timeout_seconds"`
}

// ScheduleConfig настройки расписания
type ScheduleConfig struct {
	Enabled       bool    `json:"enabled"`
	IntervalHours float64 `json:"interval_hours"`
	StartTime     string  `json:"start_time"`
}

// StorageConfig настройки хранилища
type StorageConfig struct {
	Path          string `json:"path"`
	BackupLimit   int    `json:"backup_limit"`
	MaxDiskUsageMB int   `json:"max_disk_usage_mb"`
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

// Load загружает конфигурацию из файла
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Значения по умолчанию
	if cfg.API.TimeoutSec == 0 {
		cfg.API.TimeoutSec = 300
	}
	if cfg.Storage.BackupLimit == 0 {
		cfg.Storage.BackupLimit = 7
	}
	if cfg.Storage.MaxDiskUsageMB == 0 {
		cfg.Storage.MaxDiskUsageMB = 10000
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}

	return &cfg, nil
}

// Save сохраняет конфигурацию в файл
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
