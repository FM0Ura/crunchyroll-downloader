package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSeasonsUsesClientRequestPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/content/v2/cms/series/G123456789/seasons" {
			t.Fatalf("path = %q, want seasons path", r.URL.Path)
		}
		if got := r.URL.Query().Get("preferred_audio_language"); got != "ja-JP" {
			t.Fatalf("preferred_audio_language = %q, want ja-JP", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer initial-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"season-1","season_number":1}]}`)
	}))
	defer server.Close()

	seasons, err := newTestClient(server.URL).GetSeasons(context.Background(), "G123456789", "ja-JP", "en-US")
	if err != nil {
		t.Fatalf("GetSeasons() error = %v", err)
	}
	if len(seasons) != 1 || seasons[0].ID != "season-1" {
		t.Fatalf("seasons = %#v, want season-1", seasons)
	}
}

// TestGetSeriesInfo asserts the NEW series-level CMS call (D-07) fires against
// the confirmed /content/v2/cms/series/{id} endpoint, decodes a rich
// SeriesInfo (id/title/description/genres/studio), and propagates the bearer
// token. Cloned verbatim from the GetSeasons test pattern, swapping the path
// assertion + JSON body to the series shape.
func TestGetSeriesInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/content/v2/cms/series/GSERIES123" {
			t.Fatalf("path = %q, want series path", r.URL.Path)
		}
		if got := r.URL.Query().Get("preferred_audio_language"); got != "ja-JP" {
			t.Fatalf("preferred_audio_language = %q, want ja-JP", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer initial-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"GSERIES123","title":"Test Series","description":"A test series plot.","genres":["Action","Comedy"],"studio":"Test Studio"}]}`)
	}))
	defer server.Close()

	info, err := newTestClient(server.URL).GetSeriesInfo(context.Background(), "GSERIES123", "", "")
	if err != nil {
		t.Fatalf("GetSeriesInfo() error = %v", err)
	}
	if info == nil {
		t.Fatal("GetSeriesInfo() info = nil, want non-nil SeriesInfo")
	}
	if info.ID != "GSERIES123" {
		t.Fatalf("ID = %q, want GSERIES123", info.ID)
	}
	if info.Title != "Test Series" {
		t.Fatalf("Title = %q, want Test Series", info.Title)
	}
	if info.Description != "A test series plot." {
		t.Fatalf("Description = %q, want plot", info.Description)
	}
	if len(info.Genres) != 2 || info.Genres[0] != "Action" || info.Genres[1] != "Comedy" {
		t.Fatalf("Genres = %#v, want [Action Comedy]", info.Genres)
	}
	if info.Studio != "Test Studio" {
		t.Fatalf("Studio = %q, want Test Studio", info.Studio)
	}
}

// TestGetSeasonEpisodesDecodesSeriesID asserts the enriched SeasonEpisode
// SeriesID field (D-07 Option A) populates from the GetSeasonEpisodes
// response — the source of the series-id for the season-path GetSeriesInfo
// call (Task 3). Protects the Option-A wiring.
func TestGetSeasonEpisodesDecodesSeriesID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/content/v2/cms/seasons/season-1/episodes" {
			t.Fatalf("path = %q, want season episodes path", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"ep-1","series_title":"Test Series","series_id":"GSERIES456","season_number":1,"episode_number":1,"audio_locale":"ja-JP","title":"First"}]}`)
	}))
	defer server.Close()

	episodes, err := newTestClient(server.URL).GetSeasonEpisodes(context.Background(), "season-1", "", "")
	if err != nil {
		t.Fatalf("GetSeasonEpisodes() error = %v", err)
	}
	if len(episodes) != 1 {
		t.Fatalf("episodes len = %d, want 1", len(episodes))
	}
	if episodes[0].SeriesID != "GSERIES456" {
		t.Fatalf("SeasonEpisode.SeriesID = %q, want GSERIES456", episodes[0].SeriesID)
	}
	if episodes[0].Title != "First" {
		t.Fatalf("Title = %q, want First", episodes[0].Title)
	}
}
