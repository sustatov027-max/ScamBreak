package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/sustatov027-max/ScamBreak/pkg/config"
)

// Config задаёт параметры создания логгера.
type Config struct {
	Level     string
	Format    string
	AddSource bool
}

// ParseLevel преобразует строку в уровень логирования slog.Level.
func ParseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// New создаёт новый экземпляр *slog.Logger на основе Config с выводом в os.Stdout.
func New(cfg Config) *slog.Logger {
	return NewWithWriter(os.Stdout, cfg)
}

// NewFromLogConfig создаёт логгер из конфигурации config.LogConfig.
func NewFromLogConfig(cfg config.LogConfig) *slog.Logger {
	return New(Config{
		Level:     cfg.Level,
		Format:    cfg.Format,
		AddSource: cfg.AddSource,
	})
}

// NewWithWriter создаёт логгер с возможностью переопределения потока вывода (например, для тестов).
func NewWithWriter(w io.Writer, cfg Config) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     ParseLevel(cfg.Level),
		AddSource: cfg.AddSource,
	}

	var handler slog.Handler
	if strings.ToLower(strings.TrimSpace(cfg.Format)) == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	return slog.New(handler)
}

// Init инициализирует глобальный логгер по умолчанию (slog.SetDefault) и возвращает его.
func Init(cfg config.LogConfig) *slog.Logger {
	l := NewFromLogConfig(cfg)
	slog.SetDefault(l)
	return l
}

// Err возвращает атрибут ошибки для структурированного логирования.
func Err(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}
	return slog.String("error", err.Error())
}

// With добавляет общие атрибуты к текущему логгеру по умолчанию.
func With(args ...any) *slog.Logger {
	return slog.Default().With(args...)
}

// Debug логирует сообщение уровня DEBUG в логгер по умолчанию.
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// Info логирует сообщение уровня INFO в логгер по умолчанию.
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Warn логирует сообщение уровня WARN в логгер по умолчанию.
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error логирует сообщение уровня ERROR в логгер по умолчанию.
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}
