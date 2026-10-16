package downloader

import (
	"context"
	"grm-autobackup/internal/api"
	"grm-autobackup/internal/monitor"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Downloader скачивает бэкапы
type Downloader struct {
	monitor *monitor.Monitor
}

// New создает новый downloader
func New(m *monitor.Monitor) *Downloader {
	return &Downloader{monitor: m}
}

// DownloadLatestBackup скачивает последний бэкап приложения
func (d *Downloader) DownloadLatestBackup(ctx context.Context, client *api.Client, app api.Application, basePath string) {
	appDir := filepath.Join(basePath, app.CustomerID, app.ID)
	os.MkdirAll(appDir, 0755)

	// Получаем бэкапы
	backups, err := client.GetBackups(ctx, app.ID)
	if err != nil {
		d.monitor.AddLog("Ошибка получения бэкапов для " + app.Name + ": " + err.Error())
		return
	}

	// Фильтруем готовые бэкапы
	var readyBackups []api.Backup
	for _, b := range backups {
		if b.Status == "ready" {
			readyBackups = append(readyBackups, b)
		}
	}

	if len(readyBackups) == 0 {
		d.monitor.AddLog("Нет готовых бэкапов для " + app.Name)
		return
	}

	// Сортируем по дате (последний первый)
	sort.Slice(readyBackups, func(i, j int) bool {
		return readyBackups[i].Date > readyBackups[j].Date
	})

	latest := readyBackups[0]
	
	// Формируем имя файла
	filename := "backup_" + latest.Date[:10] + ".zip"
	savePath := filepath.Join(appDir, filename)

	// Проверяем, существует ли уже
	if _, err := os.Stat(savePath); err == nil {
		d.monitor.AddLog("Бэкап уже существует: " + filename)
		return
	}

	// Устанавливаем текущее действие
	action := &monitor.Action{
		ID:          app.ID,
		Type:        monitor.StatusDownloading,
		Description: "Скачивание: " + app.Name,
		StartedAt:   time.Now(),
	}
	d.monitor.SetCurrent(action)

	d.monitor.AddLog("Скачивание бэкапа: " + app.Name + " (" + string(rune(int(latest.Size/1024))) + " MB)")

	// Скачиваем
	err = client.DownloadBackup(ctx, latest.Link, savePath, func(written, total int64) {
		progress := float64(written) / float64(total) * 100
		action.Progress = progress
		d.monitor.SetCurrent(action)
	})

	if err != nil {
		action.Type = monitor.StatusError
		action.Error = err.Error()
		d.monitor.AddLog("Ошибка скачивания: " + err.Error())
	} else {
		action.Type = monitor.StatusIdle
		action.FinishedAt = time.Now()
		d.monitor.AddToHistory(action)
		d.monitor.AddLog("Бэкап сохранен: " + filename)
	}
}
