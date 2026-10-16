package cleaner

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"grm-autobackup/internal/monitor"
)

// Cleaner удаляет старые бэкапы
type Cleaner struct {
	limit   int
	monitor *monitor.Monitor
}

// New создает новый cleaner
func New(limit int, m *monitor.Monitor) *Cleaner {
	if limit <= 0 {
		limit = 7
	}
	return &Cleaner{limit: limit, monitor: m}
}

// Cleanup удаляет старые бэкапы в структуре basePath/<клиент>/<приложение>/*.zip
func (c *Cleaner) Cleanup(basePath string) {
	clients, err := os.ReadDir(basePath)
	if err != nil {
		c.monitor.Warnf("Очистка: не удалось прочитать папку %s: %v", basePath, err)
		return
	}

	removed := 0
	for _, client := range clients {
		if !client.IsDir() {
			continue
		}
		clientDir := filepath.Join(basePath, client.Name())
		apps, err := os.ReadDir(clientDir)
		if err != nil {
			continue
		}
		for _, app := range apps {
			if !app.IsDir() {
				continue
			}
			removed += c.cleanAppDir(filepath.Join(clientDir, app.Name()))
		}
	}
	if removed > 0 {
		c.monitor.Infof("Очистка: удалено файлов %d", removed)
	}
}

func (c *Cleaner) cleanAppDir(appDir string) int {
	files, err := os.ReadDir(appDir)
	if err != nil {
		return 0
	}

	type entry struct {
		path string
		mod  time.Time
	}
	var backups []entry
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(f.Name()), ".zip") {
			continue
		}
		info, err := f.Info()
		if err != nil {
			continue
		}
		backups = append(backups, entry{filepath.Join(appDir, f.Name()), info.ModTime()})
	}

	if len(backups) <= c.limit {
		return 0
	}

	// Сортируем по времени изменения (новые первые)
	sort.Slice(backups, func(i, j int) bool { return backups[i].mod.After(backups[j].mod) })

	removed := 0
	for _, b := range backups[c.limit:] {
		if err := os.Remove(b.path); err != nil {
			c.monitor.Warnf("Не удалось удалить %s: %v", b.path, err)
			continue
		}
		c.monitor.Infof("Удален старый бэкап: %s", b.path)
		removed++
	}
	return removed
}
