package cleaner

import (
	"grm-autobackup/internal/monitor"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Cleaner удаляет старые бэкапы
type Cleaner struct {
	limit     int
	basePath  string
	monitor   *monitor.Monitor
}

// New создает новый cleaner
func New(limit int, basePath string, m *monitor.Monitor) *Cleaner {
	return &Cleaner{
		limit:   limit,
		basePath: basePath,
		monitor: m,
	}
}

// Cleanup удаляет старые бэкапы
func (c *Cleaner) Cleanup(basePath string) {
	// Получаем все папки приложений
	apps, err := os.ReadDir(basePath)
	if err != nil {
		c.monitor.AddLog("Ошибка чтения папки бэкапов: " + err.Error())
		return
	}

	for _, app := range apps {
		if !app.IsDir() {
			continue
		}

		appDir := filepath.Join(basePath, app.Name())
		c.cleanAppDir(appDir)
	}
}

func (c *Cleaner) cleanAppDir(appDir string) {
	// Получаем все бэкапы
	files, err := os.ReadDir(appDir)
	if err != nil {
		return
	}

	var backupFiles []os.DirEntry
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".zip") {
			backupFiles = append(backupFiles, f)
		}
	}

	if len(backupFiles) <= c.limit {
		return
	}

	// Сортируем по времени изменения (новые первые)
	sort.Slice(backupFiles, func(i, j int) bool {
		infoI, _ := backupFiles[i].Info()
		infoJ, _ := backupFiles[j].Info()
		return infoI.ModTime().After(infoJ.ModTime())
	})

	// Удаляем старые
	toDelete := backupFiles[c.limit:]
	for _, f := range toDelete {
		path := filepath.Join(appDir, f.Name())
		os.Remove(path)
		c.monitor.AddLog("Удален старый бэкап: " + f.Name())
	}
}
