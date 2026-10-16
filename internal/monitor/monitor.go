package monitor

import (
	"sync"
	"time"
)

// ActionType тип действия
type ActionStatus string

const (
	StatusIdle       ActionStatus = "idle"
	StatusDownloading ActionStatus = "downloading"
	StatusCleaning   ActionStatus = "cleaning"
	StatusError      ActionStatus = "error"
)

// Action текущее действие
type Action struct {
	ID          string        `json:"id"`
	Type        ActionStatus  `json:"type"`
	Description string        `json:"description"`
	Progress    float64       `json:"progress"`
	StartedAt   time.Time     `json:"started_at"`
	FinishedAt  time.Time     `json:"finished_at"`
	Error       string        `json:"error,omitempty"`
}

// Monitor мониторит действия приложения
type Monitor struct {
	mu       sync.RWMutex
	current  *Action
	history  []*Action
	logs     []string
	maxLogs  int
}

// New создает новый монитор
func New() *Monitor {
	return &Monitor{
		history: make([]*Action, 0),
		logs:    make([]string, 0),
		maxLogs: 1000,
	}
}

// SetCurrent устанавливает текущее действие
func (m *Monitor) SetCurrent(action *Action) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = action
}

// GetCurrent возвращает текущее действие
func (m *Monitor) GetCurrent() *Action {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.current == nil {
		return &Action{Type: StatusIdle}
	}
	return m.current
}

// AddLog добавляет запись в лог
func (m *Monitor) AddLog(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := timestamp + " " + msg
	m.logs = append(m.logs, entry)
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}
}

// GetLogs возвращает последние N записей лога
func (m *Monitor) GetLogs(n int) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if n > len(m.logs) {
		n = len(m.logs)
	}
	if n == 0 {
		return m.logs
	}
	return m.logs[len(m.logs)-n:]
}

// AddToHistory добавляет действие в историю
func (m *Monitor) AddToHistory(action *Action) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = append(m.history, action)
	if len(m.history) > 50 {
		m.history = m.history[len(m.history)-50:]
	}
}

// GetHistory возвращает историю действий
func (m *Monitor) GetHistory() []*Action {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Action, len(m.history))
	copy(result, m.history)
	return result
}
