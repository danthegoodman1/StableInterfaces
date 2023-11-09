package stableinterfaces

import (
	"context"
	"log"
	"log/slog"
	"os"
)

type (
	DefaultLogger struct{}
)

func (dl *DefaultLogger) logLevel(level slog.Level, msg string, err error) {
	if err != nil {
		slog.Log(context.Background(), level, msg, "err", err)
		return
	}
	log.Println(level, msg)
}

func (dl *DefaultLogger) Debug(msg string) {
	dl.logLevel(slog.LevelDebug, msg, nil)
}

func (dl *DefaultLogger) Info(msg string) {
	dl.logLevel(slog.LevelInfo, msg, nil)
}

func (dl *DefaultLogger) Warn(msg string) {
	dl.logLevel(slog.LevelWarn, msg, nil)
}

func (dl *DefaultLogger) Error(msg string, err error) {
	dl.logLevel(slog.LevelError, msg, nil)
}

func (dl *DefaultLogger) Fatal(msg string, err error) {
	slog.Log(context.Background(), slog.LevelError, msg, "FATAL", "t", "err", err)
	os.Exit(1)
}
