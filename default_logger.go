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
		slog.LogAttrs(context.Background(), level, msg, slog.String("err", err.Error()))
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

func (dl *DefaultLogger) Warn(msg string, err error) {
	dl.logLevel(slog.LevelWarn, msg, err)
}

func (dl *DefaultLogger) Error(msg string, err error) {
	dl.logLevel(slog.LevelError, msg, nil)
}

func (dl *DefaultLogger) Fatal(msg string, err error) {
	slog.LogAttrs(context.Background(), slog.LevelError, msg, slog.Bool("FATAL", true), slog.String("err", err.Error()))
	os.Exit(1)
}
