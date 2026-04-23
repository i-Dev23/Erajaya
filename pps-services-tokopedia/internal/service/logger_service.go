package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Logger is an interface for logging, allowing for easy mocking and extension.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
	Warn(msg string, args ...any)
	Debug(msg string, args ...any)
}

// loggerImpl is a concrete implementation of Logger.
type loggerImpl struct {
	mu              sync.Mutex
	logger          *log.Logger
	currentDay      string
	logFile         *os.File
	logDir          string
	telegramService TelegramService // Optional Telegram service for error alerts
}

var (
	once     sync.Once
	instance Logger
)

// getLogDir returns the log directory from PATH_LOG env or default to "./logs"
func getLogDir() string {
	if dir := os.Getenv("PATH_LOG"); dir != "" {
		log.Printf("[DEBUG] Using PATH_LOG from environment: %s", dir)
		return dir
	}
	log.Printf("[DEBUG] PATH_LOG not set, using default: ./logs")
	return "./logs"
}

// openLogFile opens (or creates) the log file for the current day.
func openLogFile(logDir, day string) (*os.File, error) {
	log.Printf("[DEBUG] openLogFile: logDir=%s, day=%s", logDir, day)

	// Check if directory exists before creating
	fileInfo, err := os.Stat(logDir)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("[DEBUG] os.Stat failed for logDir: %v", err)
		return nil, fmt.Errorf("failed to stat logDir: %w", err)
	}
	if err == nil && !fileInfo.IsDir() {
		log.Printf("[DEBUG] logDir exists but is not a directory: %s", logDir)
		return nil, fmt.Errorf("logDir exists but is not a directory: %s", logDir)
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("[DEBUG] Failed to create directory %s: %v", logDir, err)
		return nil, fmt.Errorf("failed to create logDir: %w", err)
	}
	log.Printf("[DEBUG] Directory created/exists: %s", logDir)

	filename := filepath.Join(logDir, fmt.Sprintf("%s.log", day))
	log.Printf("[DEBUG] Opening/creating log file: %s", filename)

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("[DEBUG] Failed to open log file %s: %v", filename, err)
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	log.Printf("[DEBUG] Successfully opened log file: %s", filename)
	return f, nil
}

// NewLogger returns a singleton Logger instance.
func NewLogger() Logger {
	once.Do(func() {
		log.Printf("[DEBUG] Initializing Logger...")

		// Pilih output logger berdasarkan env LOG_TO_FILE (default: Y)
		// Default menggunakan file logging kecuali LOG_TO_FILE diset ke N/NO/FALSE
		envVal := strings.ToUpper(strings.TrimSpace(os.Getenv("LOG_TO_FILE")))
		useFile := true
		if envVal == "N" || envVal == "NO" || envVal == "FALSE" {
			useFile = false
		}
		log.Printf("[DEBUG] LOG_TO_FILE env value: %s, useFile: %v", envVal, useFile)

		currentDay := time.Now().Format("2006-01-02")
		log.Printf("[DEBUG] Current day: %s", currentDay)

		var baseLogger *log.Logger
		var logFile *os.File
		var logDir string

		if useFile {
			// gunakan PATH_LOG atau default ./logs
			logDir = getLogDir()
			log.Printf("[DEBUG] Attempting to open log file in directory: %s", logDir)

			f, err := openLogFile(logDir, currentDay)
			if err != nil {
				log.Printf("[DEBUG] Failed to open log file in %s: %v", logDir, err)

				// coba fallback ke ./logs jika PATH_LOG gagal (mis-match OS path atau permission)
				fallbackDir := "./logs"
				if logDir != fallbackDir {
					log.Printf("[DEBUG] Trying fallback directory: %s", fallbackDir)
					if f2, err2 := openLogFile(fallbackDir, currentDay); err2 == nil {
						baseLogger = log.New(f2, "", log.LstdFlags|log.Lshortfile)
						logFile = f2
						logDir = fallbackDir
						log.Printf("[DEBUG] Successfully opened log file in fallback directory: %s", fallbackDir)
						log.Printf("Failed to open PATH_LOG (%s): %v, using fallback %s", logDir, err, fallbackDir)
					} else {
						// final fallback ke stdout
						log.Printf("[DEBUG] Fallback directory also failed: %v, using stdout", err2)
						baseLogger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
						log.Printf("Failed to open log file: %v; fallback also failed: %v; using stdout", err, err2)
					}
				} else {
					// final fallback ke stdout
					log.Printf("[DEBUG] Already using ./logs, fallback to stdout: %v", err)
					baseLogger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
					log.Printf("Failed to open log file, fallback to stdout: %v", err)
				}
			} else {
				log.Printf("[DEBUG] Successfully opened log file: %s", logDir)
				baseLogger = log.New(f, "", log.LstdFlags|log.Lshortfile)
				logFile = f
			}
		} else {
			// default pakai stdout
			log.Printf("[DEBUG] LOG_TO_FILE is disabled, using stdout")
			baseLogger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
			logDir = ""
		}

		instance = &loggerImpl{
			logger:          baseLogger,
			currentDay:      currentDay,
			logFile:         logFile,
			logDir:          logDir,
			telegramService: nil,
		}
		log.Printf("[DEBUG] Logger initialization complete. Using logDir: %s, useFile: %v", logDir, useFile)
	})
	return instance
}

// NewLoggerWithTelegram creates a logger with Telegram error alerting
func NewLoggerWithTelegram(telegramService TelegramService) Logger {
	logger := NewLogger()
	if impl, ok := logger.(*loggerImpl); ok {
		impl.telegramService = telegramService
	}
	return logger
}

// rotateIfNeeded checks if the day has changed and rotates the log file if necessary.
func (l *loggerImpl) rotateIfNeeded() {
	// Skip rotation if logDir is empty (stdout mode)
	if l.logDir == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	day := time.Now().Format("2006-01-02")
	if day != l.currentDay {
		// Close old file if open
		if l.logFile != nil {
			l.logFile.Close()
		}
		logFile, err := openLogFile(l.logDir, day)
		if err != nil {
			// fallback to stdout if file can't be opened
			l.logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
			l.logFile = nil
			l.currentDay = day
			log.Printf("Failed to rotate log file: %v, fallback to stdout", err)
			return
		}
		l.logger = log.New(logFile, "", log.LstdFlags|log.Lshortfile)
		l.logFile = logFile
		l.currentDay = day
	}
}

// formatLogMessage formats a log message with key-value pairs
func formatLogMessage(msg string, args ...any) string {
	if len(args) == 0 {
		return msg
	}

	// Build key-value pairs string
	var pairs []string
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key := fmt.Sprintf("%v", args[i])
			value := fmt.Sprintf("%v", args[i+1])
			pairs = append(pairs, fmt.Sprintf("%s=%v", key, value))
		} else {
			// Odd number of args, just append the last one
			pairs = append(pairs, fmt.Sprintf("%v", args[i]))
		}
	}

	if len(pairs) > 0 {
		return fmt.Sprintf("%s | %s", msg, strings.Join(pairs, ", "))
	}
	return msg
}

func (l *loggerImpl) Info(msg string, args ...any) {
	l.rotateIfNeeded()
	formattedMsg := formatLogMessage(msg, args...)
	l.logger.Print("[INFO] " + formattedMsg)
}

func (l *loggerImpl) Error(msg string, args ...any) {
	l.rotateIfNeeded()
	formattedMsg := formatLogMessage(msg, args...)
	l.logger.Print("[ERROR] " + formattedMsg)

	// Send Telegram alert asynchronously (non-blocking)
	if l.telegramService != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Build metadata from args
			metadata := make(map[string]interface{})
			for i := 0; i < len(args)-1; i += 2 {
				if key, ok := args[i].(string); ok && i+1 < len(args) {
					metadata[key] = args[i+1]
				}
			}

			// Send error alert (ignore errors to prevent logging loops)
			_ = l.telegramService.SendErrorAlert(ctx, msg, metadata)
		}()
	}
}

func (l *loggerImpl) Warn(msg string, args ...any) {
	l.rotateIfNeeded()
	formattedMsg := formatLogMessage(msg, args...)
	l.logger.Print("[WARN] " + formattedMsg)
}

func (l *loggerImpl) Debug(msg string, args ...any) {
	l.rotateIfNeeded()
	formattedMsg := formatLogMessage(msg, args...)
	l.logger.Print("[DEBUG] " + formattedMsg)
}

// CleanupOldLogFiles deletes log files older than retentionDays in PATH_LOG (or ./logs)
// Returns the number of files deleted and any error encountered during scanning/removal.
func CleanupOldLogFiles(retentionDays int) (int, error) {
	dir := getLogDir()
	if strings.TrimSpace(dir) == "" {
		dir = "./logs"
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to read log directory %s: %w", dir, err)
	}

	cutoffDate := time.Now().AddDate(0, 0, -retentionDays).Truncate(24 * time.Hour)
	deleted := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Expect filenames like YYYY-MM-DD.log
		if !strings.HasSuffix(name, ".log") || len(name) < len("2006-01-02.log") {
			continue
		}

		datePart := strings.TrimSuffix(name, ".log")
		// Some implementations may include additional suffixes; take first 10 chars for date
		if len(datePart) >= 10 {
			datePart = datePart[:10]
		} else {
			continue
		}

		fileDate, parseErr := time.Parse("2006-01-02", datePart)
		if parseErr != nil {
			// Skip non-conforming files
			continue
		}

		if fileDate.Before(cutoffDate) {
			fullPath := filepath.Join(dir, name)
			if rmErr := os.Remove(fullPath); rmErr == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}
