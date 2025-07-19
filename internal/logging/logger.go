package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/USSTM/cv-backend/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logger *slog.Logger

func Init(cfg *config.LoggingConfig) error {
	if err := os.MkdirAll(filepath.Dir(cfg.Filename), 0755); err != nil {
		return err
	}

	roller := &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	var writer io.Writer = roller
	if cfg.Format != "json" {
		writer = io.MultiWriter(os.Stdout, roller)
	}

	level := parseLevel(cfg.Level)
	
	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(writer, &slog.HandlerOptions{Level: level})
	}

	logger = slog.New(handler)
	slog.SetDefault(logger)

	return nil
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Debug(msg string, args ...any) {
	if logger != nil {
		logger.Debug(msg, args...)
	}
}

func Info(msg string, args ...any) {
	if logger != nil {
		logger.Info(msg, args...)
	}
}

func Warn(msg string, args ...any) {
	if logger != nil {
		logger.Warn(msg, args...)
	}
}

func Error(msg string, args ...any) {
	if logger != nil {
		logger.Error(msg, args...)
	}
}

func With(args ...any) *slog.Logger {
	if logger != nil {
		return logger.With(args...)
	}
	return slog.Default().With(args...)
}