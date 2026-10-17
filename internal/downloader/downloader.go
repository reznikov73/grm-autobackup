package downloader

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"grm-autobackup/internal/api"
	"grm-autobackup/internal/monitor"
)

// Downloader скачивает бэкапы
type Downloader struct {
	monitor *monitor.Monitor

	mu        sync.Mutex
	nameCache map[string]string // customerID -> отображаемое имя клиента
}

// New создает новый downloader
func New(m *monitor.Monitor) *Downloader {
	return &Downloader{
		monitor:   m,
		nameCache: make(map[string]string),
	}
}

// DownloadAll скачивает последние бэкапы для переданных приложений
func (d *Downloader) DownloadAll(ctx context.Context, client *api.Client, apps []api.Application, basePath string) {
	if len(apps) == 0 {
		d.monitor.Infof("Нечего скачивать: список приложений пуст")
		return
	}

	custFolders := d.customerFolders(ctx, client, apps)
	appFoldersMap := appFolders(apps)

	for _, app := range apps {
		appDir := filepath.Join(basePath, custFolders[app.CustomerID], appFoldersMap[app.ID])
		d.downloadLatest(ctx, client, app, appDir)
	}
}

// customerFolders определяет имена папок клиентов (с санитайзом и учетом коллизий)
func (d *Downloader) customerFolders(ctx context.Context, client *api.Client, apps []api.Application) map[string]string {
	names := make(map[string]string)
	var ids []string
	for _, app := range apps {
		if _, ok := names[app.CustomerID]; ok {
			continue
		}

		d.mu.Lock()
		cached, ok := d.nameCache[app.CustomerID]
		d.mu.Unlock()
		if ok {
			names[app.CustomerID] = cached
			ids = append(ids, app.CustomerID)
			continue
		}

		name := ""
		if cust, err := client.GetCustomer(ctx, app.CustomerID); err == nil && cust.Name != "" {
			name = cust.Name
		} else {
			if err != nil {
				d.monitor.Warnf("Не удалось получить имя клиента %s: %v", shortID(app.CustomerID), err)
			}
			name = "Клиент " + shortID(app.CustomerID)
		}
		d.mu.Lock()
		d.nameCache[app.CustomerID] = name
		d.mu.Unlock()
		names[app.CustomerID] = name
		ids = append(ids, app.CustomerID)
	}

	sort.Strings(ids)
	return uniqueFolders(ids, names)
}

// appFolders определяет имена папок приложений (коллизии — внутри одного клиента)
func appFolders(apps []api.Application) map[string]string {
	byCustomer := make(map[string][]string)
	names := make(map[string]string, len(apps))
	for _, app := range apps {
		byCustomer[app.CustomerID] = append(byCustomer[app.CustomerID], app.ID)
		names[app.ID] = app.Name
	}

	result := make(map[string]string, len(apps))
	for _, ids := range byCustomer {
		sort.Strings(ids)
		for id, folder := range uniqueFolders(ids, names) {
			result[id] = folder
		}
	}
	return result
}

func (d *Downloader) downloadLatest(ctx context.Context, client *api.Client, app api.Application, appDir string) {
	backups, err := client.GetBackups(ctx, app.ID)
	if err != nil {
		d.monitor.Errorf("Ошибка получения бэкапов для %s: %v", app.Name, err)
		return
	}

	var ready []api.Backup
	for _, b := range backups {
		if b.Status == "ready" {
			ready = append(ready, b)
		}
	}
	if len(ready) == 0 {
		d.monitor.Infof("Нет готовых бэкапов: %s", app.Name)
		return
	}

	sort.Slice(ready, func(i, j int) bool { return ready[i].Date > ready[j].Date })
	latest := ready[0]

	baseName := backupBaseName(latest)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		d.monitor.Errorf("Не удалось создать папку %s: %v", appDir, err)
		return
	}
	basePath := filepath.Join(appDir, baseName)

	// Бэкап уже скачан (в любом формате: .zip/.bz2/...)?
	if existing := findExistingBackup(basePath); existing != "" {
		d.monitor.Infof("Бэкап уже существует: %s", existing)
		return
	}

	// Проверка свободного места
	if free, err := diskFreeBytes(appDir); err == nil {
		need := estimateNeedBytes(latest.Size)
		if free < uint64(need) {
			d.monitor.Warnf("Недостаточно места для %s.*: свободно %s, нужно ~%s — пропуск",
				baseName, humanBytes(int64(free)), humanBytes(need))
			return
		}
	} else {
		d.monitor.Warnf("Не удалось определить свободное место для %s: %v", appDir, err)
	}

	action := &monitor.Action{
		ID:          app.ID,
		Type:        monitor.StatusDownloading,
		Description: app.Name,
		StartedAt:   time.Now(),
	}
	d.monitor.SetCurrent(action)

	partPath := basePath + ".part"
	var resumeFrom int64
	if st, err := os.Stat(partPath); err == nil {
		resumeFrom = st.Size()
	}
	if resumeFrom > 0 {
		d.monitor.Infof("Докачка: %s (уже %s)", baseName, humanBytes(resumeFrom))
	} else {
		d.monitor.Infof("Скачивание: %s (в API: %.1f МБ)", baseName, latest.Size)
	}

	var (
		lastPublish time.Time
		lastTime    time.Time
		lastWritten int64
		speed       float64
	)
	finalPath, err := client.DownloadBackup(ctx, latest.Link, basePath, func(written, total int64) {
		now := time.Now()
		if lastTime.IsZero() {
			lastTime = now
			lastWritten = written
		} else if dt := now.Sub(lastTime).Seconds(); dt >= 0.5 {
			inst := float64(written-lastWritten) / dt
			if speed == 0 {
				speed = inst
			} else {
				speed = 0.7*speed + 0.3*inst
			}
			lastTime = now
			lastWritten = written
		}
		if now.Sub(lastPublish) < 250*time.Millisecond {
			return
		}
		lastPublish = now
		percent := 0.0
		if total > 0 {
			percent = float64(written) / float64(total) * 100
		}
		d.monitor.UpdateProgress(app.ID, percent, speed, written, total)
	})
	if err != nil {
		d.monitor.Errorf("Ошибка скачивания %s: %v", baseName, err)
		d.monitor.SetCurrent(&monitor.Action{
			ID:          app.ID,
			Type:        monitor.StatusError,
			Description: "Ошибка: " + app.Name,
			Error:       err.Error(),
			StartedAt:   action.StartedAt,
			FinishedAt:  time.Now(),
		})
		return
	}

	d.monitor.AddToHistory(&monitor.Action{
		ID:          app.ID,
		Type:        monitor.StatusIdle,
		Description: "Скачано: " + app.Name,
		Progress:    100,
		StartedAt:   action.StartedAt,
		FinishedAt:  time.Now(),
	})
	d.monitor.SetCurrent(&monitor.Action{Type: monitor.StatusIdle})

	if st, err := os.Stat(finalPath); err == nil {
		d.monitor.Infof("Бэкап сохранен: %s (%s)", finalPath, humanBytes(st.Size()))
	} else {
		d.monitor.Infof("Бэкап сохранен: %s", finalPath)
	}
}
