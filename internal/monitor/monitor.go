package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ActionStatus тип статуса действия
type ActionStatus string

const (
	StatusIdle        ActionStatus = "idle"
	StatusDownloading ActionStatus = "downloading"
	StatusCleaning    ActionStatus = "cleaning"
	StatusError       ActionStatus = "error"
)

// Action текущее действие
type Action struct {
	ID           string       `json:"id"`
	Type         ActionStatus `json:"type"`
	Description  string       `json:"description"`
	Customer     string       `json:"customer,omitempty"`
	Progress     float64      `json:"progress"`
	SpeedBps     float64      `json:"speed_bps,omitempty"`
	BytesWritten int64        `json:"bytes_written,omitempty"`
	BytesTotal   int64        `json:"bytes_total,omitempty"`
	StartedAt    time.Time    `json:"started_at"`
	FinishedAt   time.Time    `json:"finished_at"`
	Error        string       `json:"error,omitempty"`
}

// PlanItem — пункт плана цикла (очередь скачивания)
type PlanItem struct {
	ID       string `json:"id"`
	Customer string `json:"customer"`
	App      string `json:"app"`
	Status   string `json:"status"` // pending | downloading | skipped | done | error
}

// Уровни логирования
const (
	levelDebug = iota
	levelInfo
	levelWarn
	levelError
)

// Monitor мониторит действия приложения и ведет лог (память + файл)
type Monitor struct {
	mu      sync.RWMutex
	current *Action
	history []*Action
	logs    []string
	plan    []PlanItem
	maxLogs int

	logFile    *os.File
	logPath    string
	maxSize    int64
	maxBackups int
	level      int
}

// New создает новый монитор
func New() *Monitor {
	return &Monitor{
		history: make([]*Action, 0),
		logs:    make([]string, 0),
		maxLogs: 1000,
		level:   levelInfo,
	}
}

// SetupFile подключает файловый лог с ротацией
func (m *Monitor) SetupFile(path string, maxSizeMB, maxBackups int, level string) error {
	if maxSizeMB <= 0 {
		maxSizeMB = 50
	}
	if maxBackups <= 0 {
		maxBackups = 5
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.logFile != nil {
		m.logFile.Close()
	}
	m.logFile = f
	m.logPath = path
	m.maxSize = int64(maxSizeMB) * 1024 * 1024
	m.maxBackups = maxBackups
	m.level = parseLevel(level)
	return nil
}

func parseLevel(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return levelDebug
	case "warn", "warning":
		return levelWarn
	case "error":
		return levelError
	default:
		return levelInfo
	}
}

func levelName(l int) string {
	switch l {
	case levelDebug:
		return "DEBUG"
	case levelWarn:
		return "WARN"
	case levelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// logf пишет строку в память и файл (с учетом уровня)
func (m *Monitor) logf(level int, format string, args ...interface{}) {
	if level < m.level {
		return
	}
	msg := fmt.Sprintf(format, args...)
	entry := time.Now().Format("2006-01-02 15:04:05") + " [" + levelName(level) + "] " + msg

	m.mu.Lock()
	defer m.mu.Unlock()

	m.logs = append(m.logs, entry)
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}

	if m.logFile == nil {
		return
	}
	m.rotateIfNeededLocked()
	if _, err := m.logFile.WriteString(entry + "\n"); err != nil {
		fmt.Fprintf(os.Stderr, "log write error: %v\n", err)
	}
}

// rotateIfNeededLocked выполняет ротацию файла лога при превышении размера
func (m *Monitor) rotateIfNeededLocked() {
	if m.logFile == nil || m.maxSize <= 0 || m.maxBackups <= 0 {
		return
	}
	st, err := m.logFile.Stat()
	if err != nil || st.Size() < m.maxSize {
		return
	}
	m.logFile.Close()
	base := m.logPath
	_ = os.Remove(fmt.Sprintf("%s.%d", base, m.maxBackups))
	for i := m.maxBackups - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", base, i), fmt.Sprintf("%s.%d", base, i+1))
	}
	_ = os.Rename(base, base+".1")
	f, err := os.OpenFile(base, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		m.logFile = nil
		return
	}
	m.logFile = f
}

// Debugf пишет отладочную запись
func (m *Monitor) Debugf(format string, args ...interface{}) { m.logf(levelDebug, format, args...) }

// Infof пишет информационную запись
func (m *Monitor) Infof(format string, args ...interface{}) { m.logf(levelInfo, format, args...) }

// Warnf пишет предупреждение
func (m *Monitor) Warnf(format string, args ...interface{}) { m.logf(levelWarn, format, args...) }

// Errorf пишет ошибку
func (m *Monitor) Errorf(format string, args ...interface{}) { m.logf(levelError, format, args...) }

// AddLog добавляет запись в лог (совместимость, уровень INFO)
func (m *Monitor) AddLog(msg string) { m.logf(levelInfo, "%s", msg) }

// SetCurrent устанавливает текущее действие (сохраняется копия)
func (m *Monitor) SetCurrent(action *Action) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if action == nil {
		m.current = nil
		return
	}
	c := *action
	m.current = &c
}

// UpdateProgress обновляет прогресс и скорость текущего действия
func (m *Monitor) UpdateProgress(id string, percent, speedBps float64, written, total int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current != nil && m.current.ID == id {
		m.current.Progress = percent
		m.current.SpeedBps = speedBps
		m.current.BytesWritten = written
		m.current.BytesTotal = total
	}
}

// GetCurrent возвращает копию текущего действия
func (m *Monitor) GetCurrent() *Action {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.current == nil {
		return &Action{Type: StatusIdle}
	}
	c := *m.current
	return &c
}

// AddToHistory добавляет действие в историю (сохраняется копия)
func (m *Monitor) AddToHistory(action *Action) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := *action
	m.history = append(m.history, &c)
	if len(m.history) > 50 {
		m.history = m.history[len(m.history)-50:]
	}
}

// GetHistory возвращает копии истории действий
func (m *Monitor) GetHistory() []*Action {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Action, len(m.history))
	for i, a := range m.history {
		c := *a
		result[i] = &c
	}
	return result
}

// GetLogs возвращает последние N записей лога (n <= 0 — все)
func (m *Monitor) GetLogs(n int) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if n <= 0 || n > len(m.logs) {
		n = len(m.logs)
	}
	out := make([]string, n)
	copy(out, m.logs[len(m.logs)-n:])
	return out
}

// SetPlan задаёт план текущего цикла
func (m *Monitor) SetPlan(items []PlanItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plan = append([]PlanItem{}, items...)
}

// UpdatePlan меняет статус пункта плана
func (m *Monitor) UpdatePlan(id, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.plan {
		if m.plan[i].ID == id {
			m.plan[i].Status = status
			return
		}
	}
}

// GetPlan возвращает копию плана цикла
func (m *Monitor) GetPlan() []PlanItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]PlanItem, len(m.plan))
	copy(out, m.plan)
	return out
}

// ClearLogs очищает лог в памяти (файл не затрагивается)
func (m *Monitor) ClearLogs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = m.logs[:0]
}

// SetLevel меняет порог логирования
func (m *Monitor) SetLevel(level string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.level = parseLevel(level)
}

// Close закрывает файловый лог
func (m *Monitor) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.logFile != nil {
		m.logFile.Close()
		m.logFile = nil
	}
}
