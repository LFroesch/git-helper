package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logFile *os.File
	mu      sync.Mutex
	enabled = true
)

const maxLogSize = 5 * 1024 * 1024 // 5MB

func Init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot get home directory: %w", err)
	}

	logDir := filepath.Join(homeDir, ".config", "gitty")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("cannot create log directory: %w", err)
	}

	logPath := filepath.Join(logDir, "gitty.log")

	if info, err := os.Stat(logPath); err == nil {
		if info.Size() > maxLogSize {
			oldPath := logPath + ".old"
			os.Remove(oldPath)
			os.Rename(logPath, oldPath)
		}
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("cannot open log file: %w", err)
	}

	logFile = file
	return nil
}

func Close() {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

func Disable() {
	mu.Lock()
	defer mu.Unlock()
	enabled = false
}

func Enable() {
	mu.Lock()
	defer mu.Unlock()
	enabled = true
}

func Error(format string, args ...any) {
	log("ERROR", format, args...)
}

func Warn(format string, args ...any) {
	log("WARN", format, args...)
}

func Info(format string, args ...any) {
	log("INFO", format, args...)
}

func log(level string, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()

	if !enabled || logFile == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] %s: %s\n", timestamp, level, message)

	logFile.WriteString(logLine)
}
