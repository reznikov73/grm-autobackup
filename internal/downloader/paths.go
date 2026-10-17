package downloader

import (
	"fmt"
	"os"
	"strings"
	"time"

	"grm-autobackup/internal/api"
)

// shortID возвращает первые 8 символов идентификатора
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// sanitizeName приводит имя к виду, допустимому в путях Windows
func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	replacer := strings.NewReplacer(
		"\"", "",
		"<", "_", ">", "_", ":", "_", "/", "_", "\\", "_", "|", "_", "?", "_", "*", "_",
	)
	name = replacer.Replace(name)

	var b strings.Builder
	lastUnderscore := false
	lastSpace := false
	for _, r := range name {
		switch {
		case r < 32 || r == '_':
			if !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
				lastSpace = false
			}
		case r == ' ':
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
				lastUnderscore = false
			}
		default:
			b.WriteRune(r)
			lastUnderscore = false
			lastSpace = false
		}
	}

	name = strings.Trim(b.String(), " ._")
	if runes := []rune(name); len(runes) > 80 {
		name = strings.Trim(string(runes[:80]), " ._")
	}
	return name
}

// uniqueFolders санитизирует имена и добавляет суффикс всем участникам совпадающих групп
func uniqueFolders(ids []string, names map[string]string) map[string]string {
	sanitized := make(map[string]string, len(ids))
	counts := make(map[string]int, len(ids))
	for _, id := range ids {
		s := sanitizeName(names[id])
		if s == "" {
			s = shortID(id)
		}
		sanitized[id] = s
		counts[strings.ToLower(s)]++
	}

	result := make(map[string]string, len(ids))
	for _, id := range ids {
		s := sanitized[id]
		if counts[strings.ToLower(s)] > 1 {
			s = fmt.Sprintf("%s (%s)", s, shortID(id))
		}
		result[id] = s
	}
	return result
}

// backupBaseName формирует базовое имя файла бэкапа без расширения:
// backup_2026-10-15_10-51-28 (расширение определяется по фактическому формату файла).
func backupBaseName(b api.Backup) string {
	if t, err := time.Parse(time.RFC3339, b.Date); err == nil {
		return "backup_" + t.UTC().Format("2006-01-02_15-04-05")
	}
	s := b.Date
	if len(s) >= 10 {
		s = s[:10]
	}
	s = strings.NewReplacer(":", "-", "/", "-", "\\", "-", " ", "_").Replace(s)
	if s == "" {
		s = shortID(b.ID)
	}
	return "backup_" + s
}

// knownBackupExts — возможные расширения скачанных бэкапов
var knownBackupExts = []string{".zip", ".bz2", ".gz", ".7z", ".rar"}

// findExistingBackup ищет уже скачанный бэкап с любым из известных расширений.
func findExistingBackup(basePath string) string {
	for _, ext := range knownBackupExts {
		p := basePath + ext
		if st, err := os.Stat(p); err == nil && st.Size() > 0 {
			return p
		}
	}
	return ""
}

// estimateNeedBytes оценивает требуемое место: размер из API (МБ, с запасом)
func estimateNeedBytes(sizeFromAPI float64) int64 {
	if sizeFromAPI < 500 {
		sizeFromAPI = 500
	}
	mb := int64(sizeFromAPI * 1024 * 1024)
	return mb + mb/2 + 500*1024*1024
}

// humanBytes форматирует размер в человекочитаемый вид
func humanBytes(n int64) string {
	f := float64(n)
	units := []string{"Б", "КБ", "МБ", "ГБ", "ТБ"}
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}
