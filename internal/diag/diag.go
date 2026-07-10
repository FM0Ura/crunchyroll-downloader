package diag

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Global is the application-wide diagnostic logger.
// Set once in main() after flag parsing + config load — same pattern as
// internal/output. It defaults to a no-op logger until Init is called.
var Global *slog.Logger = slog.New(discardHandler{})

var (
	// Subsystem diagnostic loggers, populated by Init.
	DownloadLogger *slog.Logger // group "download"
	DrmLogger      *slog.Logger // group "drm"
	MediaLogger    *slog.Logger // group "media"
	MuxLogger      *slog.Logger // group "mux"
	ApiLogger      *slog.Logger // group "api"
)

// Init sets Global to a rotating text diagnostic logger.
func Init(level slog.Level, path string) {
	if path == "" {
		path = "./logs/animeheaven.log"
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: diagnostic log disabled: create log dir %s: %v\n", filepath.Dir(path), err)
		Global = slog.New(discardHandler{})
		initSubsystemLoggers()
		return
	}

	w := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    10,
		MaxBackups: 5,
		MaxAge:     0,
		Compress:   false,
	}
	base := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redactAttr,
	})
	h := handlerWrapper{inner: base}

	Global = slog.New(h)
	initSubsystemLoggers()
}

// ParseLevel converts a string log level to slog's level constants.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
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

func initSubsystemLoggers() {
	DownloadLogger = Global.WithGroup("download")
	DrmLogger = Global.WithGroup("drm")
	MediaLogger = Global.WithGroup("media")
	MuxLogger = Global.WithGroup("mux")
	ApiLogger = Global.WithGroup("api")
}

type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool { return false }

func (discardHandler) Handle(context.Context, slog.Record) error { return nil }

func (discardHandler) WithAttrs([]slog.Attr) slog.Handler { return discardHandler{} }

func (discardHandler) WithGroup(string) slog.Handler { return discardHandler{} }
