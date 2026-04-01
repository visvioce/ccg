package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/musistudio/ccg/internal/config"
)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
	LevelFatal Level = "fatal"
)

type LogEntry struct {
	Timestamp string         `json:"timestamp"`
	Level     Level          `json:"level"`
	Message   string         `json:"message"`
	Source    string         `json:"source,omitempty"`
	ReqID     string         `json:"req_id,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

type Logger struct {
	mu         sync.RWMutex
	file       *os.File
	logDir     string
	minLevel   Level
	entries    []LogEntry
	maxEntries int
}

var (
	defaultLogger *Logger
	once          sync.Once
)

func GetLogger() *Logger {
	once.Do(func() {
		defaultLogger = NewLogger()
	})
	return defaultLogger
}

func NewLogger() *Logger {
	logDir := filepath.Join(config.GetConfigDir(), "logs")
	os.MkdirAll(logDir, 0755)

	logger := &Logger{
		logDir:     logDir,
		minLevel:   LevelInfo,
		maxEntries: 10000,
	}

	return logger
}

func (l *Logger) log(level Level, message string, source string, reqID string, data map[string]any) {
	if level < l.minLevel {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Message:   message,
		Source:    source,
		ReqID:     reqID,
		Data:      data,
	}

	l.mu.Lock()
	l.entries = append(l.entries, entry)

	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[len(l.entries)-l.maxEntries:]
	}
	l.mu.Unlock()

	if l.file != nil {
		fmt.Fprintf(l.file, "[%s] %s: %s\n", entry.Timestamp, level, message)
	}

	switch level {
	case LevelDebug, LevelInfo:
		fmt.Printf("[%s] %s: %s\n", entry.Timestamp, level, message)
	case LevelWarn, LevelError:
		fmt.Printf("[%s] %s: %s\n", entry.Timestamp, level, message)
	}
}

func (l *Logger) Debug(message string, data ...map[string]any) {
	l.log(LevelDebug, message, "", "", mergeData(data...))
}

func (l *Logger) Info(message string, data ...map[string]any) {
	l.log(LevelInfo, message, "", "", mergeData(data...))
}

func (l *Logger) Warn(message string, data ...map[string]any) {
	l.log(LevelWarn, message, "", "", mergeData(data...))
}

func (l *Logger) Error(message string, data ...map[string]any) {
	l.log(LevelError, message, "", "", mergeData(data...))
}

func (l *Logger) Fatal(message string, data ...map[string]any) {
	l.log(LevelFatal, message, "", "", mergeData(data...))
	os.Exit(1)
}

func mergeData(data ...map[string]any) map[string]any {
	result := make(map[string]any)
	for _, d := range data {
		for k, v := range d {
			result[k] = v
		}
	}
	return result
}

func (l *Logger) GetEntries(level Level, limit int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var entries []LogEntry
	for i := len(l.entries) - 1; i >= 0 && len(entries) < limit; i-- {
		if level == "" || l.entries[i].Level == level {
			entries = append(entries, l.entries[i])
		}
	}
	return entries
}

func (l *Logger) GetLogFiles() []map[string]any {
	entries, err := os.ReadDir(l.logDir)
	if err != nil {
		return []map[string]any{}
	}

	var files []map[string]any
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		files = append(files, map[string]any{
			"name":         e.Name(),
			"path":         filepath.Join(l.logDir, e.Name()),
			"size":         info.Size(),
			"lastModified": info.ModTime().Format(time.RFC3339),
		})
	}
	return files
}

func (l *Logger) GetLogContent(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	result := make([]string, 0)
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

func (l *Logger) ClearLogs() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]LogEntry, 0)
	return nil
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func Debug(message string, data ...map[string]any) { GetLogger().Debug(message, data...) }
func Info(message string, data ...map[string]any)  { GetLogger().Info(message, data...) }
func Warn(message string, data ...map[string]any)  { GetLogger().Warn(message, data...) }
func Error(message string, data ...map[string]any) { GetLogger().Error(message, data...) }
func Fatal(message string, data ...map[string]any) { GetLogger().Fatal(message, data...) }
