package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"crunchyroll-downloader/internal/api"
	"crunchyroll-downloader/internal/config"
)

func TestProcessURLRejectsInvalidContentIDLength(t *testing.T) {
	output := captureMainStderr(t, func() {
		processURL(context.Background(), nil, "https://www.crunchyroll.com/watch/short/title", "", nil, nil)
	})

	if !strings.Contains(output, "Invalid URL format") {
		t.Fatalf("processURL() output = %q, want invalid URL format message", output)
	}
}

func TestProcessURLRejectsUnsupportedContentType(t *testing.T) {
	output := captureMainStderr(t, func() {
		processURL(context.Background(), nil, "https://www.crunchyroll.com/browse/G123456789/title", "", nil, nil)
	})

	if !strings.Contains(output, "Invalid URL (must be /watch/ or /series/)") {
		t.Fatalf("processURL() output = %q, want unsupported content type message", output)
	}
}

func TestResolveEtpRtPrefersFlag(t *testing.T) {
	flags := map[string]bool{"etp-rt": true}
	result := resolveEtpRt(flags, "my-cookie", nil)
	if result != "my-cookie" {
		t.Fatalf("resolveEtpRt() = %q, want 'my-cookie'", result)
	}
}

func TestResolveEtpRtPrefersFlagOverEnv(t *testing.T) {
	t.Setenv("CRUNCHYROLL_ETP_RT", "env-cookie")
	flags := map[string]bool{"etp-rt": true}
	result := resolveEtpRt(flags, "flag-cookie", nil)
	if result != "flag-cookie" {
		t.Fatalf("resolveEtpRt() = %q, want 'flag-cookie'", result)
	}
}

func TestResolveEtpRtFallsBackToEnv(t *testing.T) {
	t.Setenv("CRUNCHYROLL_ETP_RT", "env-cookie")
	flags := map[string]bool{} // etp-rt not explicitly set
	result := resolveEtpRt(flags, "", nil)
	if result != "env-cookie" {
		t.Fatalf("resolveEtpRt() = %q, want 'env-cookie'", result)
	}
}

func TestResolveEtpRtFallsBackToConfig(t *testing.T) {
	configVal := "config-cookie"
	flags := map[string]bool{} // etp-rt not explicitly set
	result := resolveEtpRt(flags, "", &configVal)
	if result != "config-cookie" {
		t.Fatalf("resolveEtpRt() = %q, want 'config-cookie'", result)
	}
}

func TestResolveEtpRtReturnsEmptyWhenUnset(t *testing.T) {
	flags := map[string]bool{}
	result := resolveEtpRt(flags, "", nil)
	if result != "" {
		t.Fatalf("resolveEtpRt() = %q, want ''", result)
	}
}

func TestValidateURLValidWatchPath(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/watch/GGGGGGGGG")
	if err != nil {
		t.Fatalf("validateURL() error = %v, want nil", err)
	}
}

func TestValidateURLValidSeriesPath(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/series/GGGGGGGGGGGG")
	if err != nil {
		t.Fatalf("validateURL() error = %v, want nil", err)
	}
}

func TestValidateURLTooShort(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/watch/short")
	if err == nil {
		t.Fatal("validateURL() error = nil, want content ID length error")
	}
	if !strings.Contains(err.Error(), "content ID length") {
		t.Fatalf("validateURL() error = %q, want content ID length error", err)
	}
}

func TestValidateURLTooLong(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/watch/GGGGGGGGGGGGGGG")
	if err == nil {
		t.Fatal("validateURL() error = nil, want content ID length error")
	}
	if !strings.Contains(err.Error(), "content ID length") {
		t.Fatalf("validateURL() error = %q, want content ID length error", err)
	}
}

func TestValidateURLWrongContentType(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/browse/GGGGGGGGG")
	if err == nil {
		t.Fatal("validateURL() error = nil, want watch/series error")
	}
	if !strings.Contains(err.Error(), "must be /watch/ or /series/") {
		t.Fatalf("validateURL() error = %q, want watch/series error", err)
	}
}

func TestValidateURLTrailingSlash(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/watch/GGGGGGGGG/")
	if err != nil {
		t.Fatalf("validateURL() with trailing slash error = %v, want nil", err)
	}
}

func TestValidateURLWithQueryParams(t *testing.T) {
	err := validateURL("https://www.crunchyroll.com/watch/GGGGGGGGG?foo=bar")
	if err != nil {
		t.Fatalf("validateURL() with query params error = %v, want nil", err)
	}
}

func TestOutputDirMissingDirErrors(t *testing.T) {
	errMsg := validateOutputDir("/nonexistent/path/that/definitely/does/not/exist")
	if errMsg == "" {
		t.Fatal("validateOutputDir() returned empty, want error message for nonexistent path")
	}
	if !strings.Contains(errMsg, "does not exist") {
		t.Fatalf("validateOutputDir() = %q, want 'does not exist' message", errMsg)
	}
}

func TestOutputDirEmptyIsValid(t *testing.T) {
	errMsg := validateOutputDir("")
	if errMsg != "" {
		t.Fatalf("validateOutputDir('') = %q, want empty string", errMsg)
	}
}

func TestOutputDirFileIsNotValid(t *testing.T) {
	f := t.TempDir() + "/notadir"
	if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	errMsg := validateOutputDir(f)
	if errMsg == "" {
		t.Fatal("validateOutputDir(file) returned empty, want error for file path")
	}
	if !strings.Contains(errMsg, "not a directory") {
		t.Fatalf("validateOutputDir(file) = %q, want 'not a directory' message", errMsg)
	}
}

func TestOutputDirValidDirPasses(t *testing.T) {
	dir := t.TempDir()
	errMsg := validateOutputDir(dir)
	if errMsg != "" {
		t.Fatalf("validateOutputDir(%q) = %q, want empty", dir, errMsg)
	}
}

func TestValidateAllURLsReportsAll(t *testing.T) {
	urls := []string{
		"https://www.crunchyroll.com/watch/GGGGGGGGG",       // valid
		"https://www.crunchyroll.com/series/GGGGGGGGGGGG",   // valid
		"https://www.crunchyroll.com/watch/short",           // too short
		"https://www.crunchyroll.com/browse/GGGGGGGGG",      // wrong type
		"https://www.crunchyroll.com/watch/GGGGGGGGGGGGGGG", // too long
	}
	invalid := validateAllURLs(urls)
	if len(invalid) != 3 {
		t.Fatalf("validateAllURLs() returned %d invalid URLs, want 3", len(invalid))
	}
}

func TestProcessURLWatchPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/v1/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"test-token"}`)
	})
	mux.HandleFunc("/content/v2/cms/objects/G123456789", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, _ := os.ReadFile("internal/api/testdata/api/episode-info-response.json")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/playback/v3/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, _ := os.ReadFile("internal/api/testdata/api/episode-playback-response.json")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/license/v1/license/widevine", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, _ := os.ReadFile("internal/api/testdata/api/license-response.json")
		_, _ = w.Write(data)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := api.NewTestClient(nil, server.URL, "test-token")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	output := captureMainStderr(t, func() {
		processURL(ctx, client, "https://www.crunchyroll.com/watch/G123456789", "", nil, nil)
	})

	if !strings.Contains(output, "context canceled") {
		t.Fatalf("processURL() stderr = %q, want context canceled error", output)
	}
}

func captureMainStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe(): %v", err)
	}
	os.Stdout = writePipe

	fn()

	if err := writePipe.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	os.Stdout = original

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, readPipe); err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	return buf.String()
}

func TestResolveLangs(t *testing.T) {
	// resolveLangs implements the D-01/D-02 precedence: explicit CLI flag
	// > config array > default. The first element of the returned slice is
	// the protected primary track, so order must be preserved exactly.
	tests := []struct {
		name          string
		explicitFlags map[string]bool
		flagName      string
		flagVal       string
		configVal     []string
		defaultVal    []string
		want          []string
	}{
		{
			name:          "audio explicit flag wins over config",
			explicitFlags: map[string]bool{"audio-lang": true},
			flagName:      "audio-lang",
			flagVal:       "pt-BR,en-US",
			configVal:     []string{"ja-JP"},
			defaultVal:    []string{"ja-JP"},
			want:          []string{"pt-BR", "en-US"},
		},
		{
			name:          "audio non-explicit uses config array in order, en-US primary",
			explicitFlags: map[string]bool{},
			flagName:      "audio-lang",
			flagVal:       "ja-JP",
			configVal:     []string{"en-US", "ja-JP"},
			defaultVal:    []string{"ja-JP"},
			want:          []string{"en-US", "ja-JP"},
		},
		{
			name:          "subs non-explicit uses config array in order, pt-BR primary",
			explicitFlags: map[string]bool{},
			flagName:      "subs-lang",
			flagVal:       "en-US",
			configVal:     []string{"pt-BR", "en-US"},
			defaultVal:    []string{"en-US"},
			want:          []string{"pt-BR", "en-US"},
		},
		{
			name:          "subs explicit flag wins over config",
			explicitFlags: map[string]bool{"subs-lang": true},
			flagName:      "subs-lang",
			flagVal:       "es-419,en-US",
			configVal:     []string{"pt-BR", "en-US"},
			defaultVal:    []string{"en-US"},
			want:          []string{"es-419", "en-US"},
		},
		{
			name:          "audio nil config falls back to default",
			explicitFlags: map[string]bool{},
			flagName:      "audio-lang",
			flagVal:       "ja-JP",
			configVal:     nil,
			defaultVal:    []string{"ja-JP"},
			want:          []string{"ja-JP"},
		},
		{
			name:          "subs nil config falls back to default",
			explicitFlags: map[string]bool{},
			flagName:      "subs-lang",
			flagVal:       "en-US",
			configVal:     nil,
			defaultVal:    []string{"en-US"},
			want:          []string{"en-US"},
		},
		{
			name:          "audio explicit empty CLI value yields nil slice (ERR-03 hard-error path)",
			explicitFlags: map[string]bool{"audio-lang": true},
			flagName:      "audio-lang",
			flagVal:       "",
			configVal:     []string{"ja-JP"},
			defaultVal:    []string{"ja-JP"},
			want:          nil,
		},
		{
			name:          "audio non-explicit with explicit empty config array yields empty slice (ERR-03 hard-error path)",
			explicitFlags: map[string]bool{},
			flagName:      "audio-lang",
			flagVal:       "ja-JP",
			configVal:     []string{},
			defaultVal:    []string{"ja-JP"},
			want:          []string{},
		},
		{
			name:          "subs non-explicit with explicit empty config array yields empty slice",
			explicitFlags: map[string]bool{},
			flagName:      "subs-lang",
			flagVal:       "en-US",
			configVal:     []string{},
			defaultVal:    []string{"en-US"},
			want:          []string{},
		},
		{
			name:          "audio non-explicit preserves multi-element config order with three locales",
			explicitFlags: map[string]bool{},
			flagName:      "audio-lang",
			flagVal:       "ja-JP",
			configVal:     []string{"en-US", "ja-JP", "pt-BR"},
			defaultVal:    []string{"ja-JP"},
			want:          []string{"en-US", "ja-JP", "pt-BR"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveLangs(tt.explicitFlags, tt.flagName, tt.flagVal, tt.configVal, tt.defaultVal)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("resolveLangs(%q) = %v, want %v", tt.flagName, got, tt.want)
			}
		})
	}
}

func TestParseLangs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "single locale", input: "ja-JP", want: []string{"ja-JP"}},
		{name: "multiple locales", input: "ja-JP,en-US", want: []string{"ja-JP", "en-US"}},
		{name: "whitespace around locales", input: " ja-JP , en-US ", want: []string{"ja-JP", "en-US"}},
		{name: "empty string", input: "", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLangs(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseLangs(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveString(t *testing.T) {
	tests := []struct {
		name          string
		explicitFlags map[string]bool
		flagName      string
		flagVal       string
		envName       string
		envVal        *string
		configVal     *string
		defaultVal    string
		want          string
	}{
		{
			name:          "log-level explicit flag wins",
			explicitFlags: map[string]bool{"log-level": true},
			flagName:      "log-level",
			flagVal:       "warn",
			envName:       "CRUNCHYROLL_LOG_LEVEL",
			envVal:        stringPtr("debug"),
			configVal:     stringPtr("info"),
			defaultVal:    "info",
			want:          "warn",
		},
		{
			name:          "log-level env over config",
			explicitFlags: map[string]bool{},
			flagName:      "log-level",
			flagVal:       "",
			envName:       "CRUNCHYROLL_LOG_LEVEL",
			envVal:        stringPtr("debug"),
			configVal:     stringPtr("info"),
			defaultVal:    "info",
			want:          "debug",
		},
		{
			name:          "log-level config over default",
			explicitFlags: map[string]bool{},
			flagName:      "log-level",
			flagVal:       "",
			envName:       "CRUNCHYROLL_LOG_LEVEL",
			configVal:     stringPtr("error"),
			defaultVal:    "info",
			want:          "error",
		},
		{
			name:          "log-level default last",
			explicitFlags: map[string]bool{},
			flagName:      "log-level",
			flagVal:       "",
			envName:       "CRUNCHYROLL_LOG_LEVEL",
			configVal:     nil,
			defaultVal:    "info",
			want:          "info",
		},
		{
			name:          "log-file explicit flag wins",
			explicitFlags: map[string]bool{"log-file": true},
			flagName:      "log-file",
			flagVal:       "/tmp/flag.log",
			envName:       "CRUNCHYROLL_LOG_FILE",
			envVal:        stringPtr("/tmp/env.log"),
			configVal:     stringPtr("/tmp/config.log"),
			defaultVal:    "./logs/animeheaven.log",
			want:          "/tmp/flag.log",
		},
		{
			name:          "log-file env over config",
			explicitFlags: map[string]bool{},
			flagName:      "log-file",
			flagVal:       "",
			envName:       "CRUNCHYROLL_LOG_FILE",
			envVal:        stringPtr("/tmp/env.log"),
			configVal:     stringPtr("/tmp/config.log"),
			defaultVal:    "./logs/animeheaven.log",
			want:          "/tmp/env.log",
		},
		{
			name:          "log-file config over default",
			explicitFlags: map[string]bool{},
			flagName:      "log-file",
			flagVal:       "",
			envName:       "CRUNCHYROLL_LOG_FILE",
			configVal:     stringPtr("/tmp/config.log"),
			defaultVal:    "./logs/animeheaven.log",
			want:          "/tmp/config.log",
		},
		{
			name:          "log-file default last",
			explicitFlags: map[string]bool{},
			flagName:      "log-file",
			flagVal:       "",
			envName:       "CRUNCHYROLL_LOG_FILE",
			configVal:     nil,
			defaultVal:    "./logs/animeheaven.log",
			want:          "./logs/animeheaven.log",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origEnv, hadEnv := os.LookupEnv(tt.envName)
			t.Cleanup(func() {
				if hadEnv {
					os.Setenv(tt.envName, origEnv)
				} else {
					os.Unsetenv(tt.envName)
				}
			})
			if tt.envVal != nil {
				os.Setenv(tt.envName, *tt.envVal)
			} else {
				os.Unsetenv(tt.envName)
			}
			got := resolveString(tt.explicitFlags, tt.flagName, tt.flagVal, tt.envName, tt.configVal, tt.defaultVal)
			if got != tt.want {
				t.Fatalf("resolveString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}

func TestIsAllNilConfig(t *testing.T) {
	t.Run("all nil returns true", func(t *testing.T) {
		if !isAllNilConfig(&config.Config{}) {
			t.Fatal("isAllNilConfig(&config.Config{}) = false, want true")
		}
	})

	fields := map[string]func(*config.Config){
		"AudioLang":      func(c *config.Config) { c.AudioLang = []string{"x"} },
		"SubsLang":       func(c *config.Config) { c.SubsLang = []string{"x"} },
		"VideoQuality":   func(c *config.Config) { s := "x"; c.VideoQuality = &s },
		"AudioQuality":   func(c *config.Config) { s := "x"; c.AudioQuality = &s },
		"Workers":        func(c *config.Config) { i := 1; c.Workers = &i },
		"OutputDir":      func(c *config.Config) { s := "x"; c.OutputDir = &s },
		"EtpRt":          func(c *config.Config) { s := "x"; c.EtpRt = &s },
		"WidevineDevice": func(c *config.Config) { s := "x"; c.WidevineDevice = &s },
	}
	for name, setter := range fields {
		t.Run(name+" non-nil returns false", func(t *testing.T) {
			cfg := &config.Config{}
			setter(cfg)
			if isAllNilConfig(cfg) {
				t.Fatalf("isAllNilConfig with %s set = true, want false", name)
			}
		})
	}
}

func captureMainStderr(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe(): %v", err)
	}
	os.Stderr = writePipe

	fn()

	if err := writePipe.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}
	os.Stderr = original

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, readPipe); err != nil {
		t.Fatalf("read stderr pipe: %v", err)
	}
	return buf.String()
}
