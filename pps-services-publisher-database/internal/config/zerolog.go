package config

import (
	"io"
	"os"
	"regexp"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

// PIIMaskingWriter wraps an io.Writer to mask PII data before writing.
type PIIMaskingWriter struct {
	Writer   io.Writer
	patterns []*regexp.Regexp
}

func NewPIIMaskingWriter(writer io.Writer) *PIIMaskingWriter {
	return &PIIMaskingWriter{
		Writer: writer,
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`(\+62|62|0)8[1-9][0-9]{6,10}`),
			regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
			regexp.MustCompile(`\b[0-9]{16}\b`),
			regexp.MustCompile(`\b[0-9]{13,19}\b`),
		},
	}
}

func (w *PIIMaskingWriter) Write(p []byte) (n int, err error) {
	s := string(p)
	for _, pattern := range w.patterns {
		s = pattern.ReplaceAllString(s, "***MASKED***")
	}
	return w.Writer.Write([]byte(s))
}

// NewLogger creates a zerolog logger with PII masking and optional file rotation.
func NewLogger(config *viper.Viper) zerolog.Logger {
	levelMap := map[int]zerolog.Level{
		0: zerolog.PanicLevel,
		1: zerolog.FatalLevel,
		2: zerolog.ErrorLevel,
		3: zerolog.WarnLevel,
		4: zerolog.InfoLevel,
		5: zerolog.DebugLevel,
		6: zerolog.TraceLevel,
	}

	level, ok := levelMap[config.GetInt("log.level")]
	if !ok {
		level = zerolog.InfoLevel
	}

	zerolog.TimeFieldFormat = "2006-01-02 15:04:05.000"

	var baseWriter io.Writer
	output := config.GetString("log.output")
	switch output {
	case "file":
		baseWriter = newFileWriter(config)
	case "both":
		baseWriter = io.MultiWriter(os.Stdout, newFileWriter(config))
	default:
		baseWriter = os.Stdout
	}

	maskedWriter := NewPIIMaskingWriter(baseWriter)
	return zerolog.New(maskedWriter).With().Timestamp().Logger().Level(level)
}

func newFileWriter(config *viper.Viper) io.Writer {
	l := &lumberjack.Logger{
		Filename:   config.GetString("log.file.path"),
		MaxSize:    config.GetInt("log.file.max_size"),
		MaxBackups: config.GetInt("log.file.max_backups"),
		MaxAge:     config.GetInt("log.file.max_age"),
		Compress:   config.GetBool("log.file.compress"),
	}

	// Daily rotation via goroutine since lumberjack only rotates on size
	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			time.Sleep(next.Sub(now))
			_ = l.Rotate()
		}
	}()

	return l
}
