package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func allowArtworkHostForTest(t *testing.T, suffix string) {
	t.Helper()
	original := append([]string(nil), allowedArtworkHostSuffixes...)
	allowedArtworkHostSuffixes = append(allowedArtworkHostSuffixes, suffix)
	t.Cleanup(func() {
		allowedArtworkHostSuffixes = original
	})
}

func TestFetchArtworkWritesFileOn200(t *testing.T) {
	body := []byte{0xff, 0xd8, 0xff, 0xdb}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer initial-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer server.Close()
	allowArtworkHostForTest(t, "127.0.0.1")

	client := NewTestClient(server.Client(), "", "initial-token")
	destPath := filepath.Join(t.TempDir(), "poster.jpg")

	if err := client.FetchArtwork(context.Background(), server.URL, destPath); err != nil {
		t.Fatalf("FetchArtwork() error = %v, want nil", err)
	}
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("file body = %v, want %v", got, body)
	}
}

func TestFetchArtworkReturnsSentinelOn404(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	allowArtworkHostForTest(t, "127.0.0.1")

	client := NewTestClient(server.Client(), "", "initial-token")
	destPath := filepath.Join(t.TempDir(), "poster.jpg")

	err := client.FetchArtwork(context.Background(), server.URL, destPath)
	if !errors.Is(err, ErrArtworkNotFound) {
		t.Fatalf("FetchArtwork() error = %v, want ErrArtworkNotFound", err)
	}
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Fatalf("destPath exists after 404; stat error = %v", statErr)
	}
}

func TestFetchArtworkReturnsErrorOn500(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	allowArtworkHostForTest(t, "127.0.0.1")

	client := NewTestClient(server.Client(), "", "initial-token")
	destPath := filepath.Join(t.TempDir(), "poster.jpg")

	err := client.FetchArtwork(context.Background(), server.URL, destPath)
	if err == nil {
		t.Fatal("FetchArtwork() error = nil, want status error")
	}
	if errors.Is(err, ErrArtworkNotFound) {
		t.Fatalf("FetchArtwork() error = %v, must not be ErrArtworkNotFound", err)
	}
}

func TestFetchArtworkRejectsUnknownHost(t *testing.T) {
	var requests int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "jpg")
	}))
	defer server.Close()

	client := NewTestClient(server.Client(), "", "initial-token")
	destPath := filepath.Join(t.TempDir(), "poster.jpg")

	err := client.FetchArtwork(context.Background(), server.URL, destPath)
	if !errors.Is(err, ErrArtworkNotFound) {
		t.Fatalf("FetchArtwork() error = %v, want ErrArtworkNotFound", err)
	}
	if requests != 0 {
		t.Fatalf("server received %d request(s), want 0", requests)
	}
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Fatalf("destPath exists after rejected host; stat error = %v", statErr)
	}

	err = client.FetchArtwork(context.Background(), "file:///etc/passwd", destPath)
	if !errors.Is(err, ErrArtworkNotFound) {
		t.Fatalf("FetchArtwork(file) error = %v, want ErrArtworkNotFound", err)
	}
	if requests != 0 {
		t.Fatalf("server received %d request(s) after file scheme, want 0", requests)
	}
}
