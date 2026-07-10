package diag

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want slog.Level
	}{
		{name: "debug", in: "debug", want: slog.LevelDebug},
		{name: "info", in: "info", want: slog.LevelInfo},
		{name: "warn", in: "warn", want: slog.LevelWarn},
		{name: "error", in: "error", want: slog.LevelError},
		{name: "case insensitive", in: "DEBUG", want: slog.LevelDebug},
		{name: "unknown defaults info", in: "xyz", want: slog.LevelInfo},
		{name: "empty defaults info", in: "", want: slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseLevel(tt.in); got != tt.want {
				t.Fatalf("ParseLevel(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestRedactKeys(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{key: "token", want: true},
		{key: "device_id", want: false},
		{key: "authorization", want: true},
		{key: "url", want: false},
		{key: "etp_rt", want: true},
		{key: "client_id", want: true},
		{key: "private_key", want: true},
		{key: "cookie", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			_, got := redactKeys[tt.key]
			if got != tt.want {
				t.Fatalf("redactKeys[%q] presence = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestInitWiresSubsystemLoggers(t *testing.T) {
	restoreDiagGlobals(t)
	isolateLogEnv(t)

	logFile := filepath.Join(t.TempDir(), "test.log")
	Init(slog.LevelDebug, logFile)

	if Global == nil {
		t.Fatal("Global = nil, want logger")
	}
	for name, logger := range map[string]*slog.Logger{
		"download": DownloadLogger,
		"drm":      DrmLogger,
		"media":    MediaLogger,
		"mux":      MuxLogger,
		"api":      ApiLogger,
	} {
		if logger == nil {
			t.Fatalf("%s logger = nil, want logger", name)
		}
	}
}

func TestRedactionEndToEnd(t *testing.T) {
	restoreDiagGlobals(t)
	isolateLogEnv(t)

	logFile := filepath.Join(t.TempDir(), "test.log")
	Init(slog.LevelDebug, logFile)

	Global.With("token", "supersecret").Info("x")
	Global.With("device_id", "uuid-123").Info("y")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", logFile, err)
	}
	content := string(data)
	if !strings.Contains(content, "[REDACTED]") {
		t.Fatalf("log content = %q, want [REDACTED]", content)
	}
	if !strings.Contains(content, "device_id=uuid-123") {
		t.Fatalf("log content = %q, want unredacted device_id", content)
	}
	if strings.Contains(content, "supersecret") {
		t.Fatalf("log content leaked secret: %q", content)
	}
}

func restoreDiagGlobals(t *testing.T) {
	t.Helper()
	originalGlobal := Global
	originalDownload := DownloadLogger
	originalDrm := DrmLogger
	originalMedia := MediaLogger
	originalMux := MuxLogger
	originalAPI := ApiLogger

	t.Cleanup(func() {
		Global = originalGlobal
		DownloadLogger = originalDownload
		DrmLogger = originalDrm
		MediaLogger = originalMedia
		MuxLogger = originalMux
		ApiLogger = originalAPI
	})
}

func isolateLogEnv(t *testing.T) {
	t.Helper()
	origLevel, hadLevel := os.LookupEnv("CRUNCHYROLL_LOG_LEVEL")
	origFile, hadFile := os.LookupEnv("CRUNCHYROLL_LOG_FILE")

	t.Cleanup(func() {
		if hadLevel {
			os.Setenv("CRUNCHYROLL_LOG_LEVEL", origLevel)
		} else {
			os.Unsetenv("CRUNCHYROLL_LOG_LEVEL")
		}
		if hadFile {
			os.Setenv("CRUNCHYROLL_LOG_FILE", origFile)
		} else {
			os.Unsetenv("CRUNCHYROLL_LOG_FILE")
		}
	})

	os.Unsetenv("CRUNCHYROLL_LOG_LEVEL")
	os.Unsetenv("CRUNCHYROLL_LOG_FILE")
}
