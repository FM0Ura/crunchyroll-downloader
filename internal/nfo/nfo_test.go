package nfo

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"crunchyroll-downloader/internal/api"
)

// TestMarshalEpisodeEscaping covers Pitfall 6: encoding/xml must entity-escape
// `&`, `<`, `>`, `"`, `'` in title/plot and preserve CJK + emoji as UTF-8.
// table-driven sub-tests so each escape case is independently attributable.
func TestMarshalEpisodeEscaping(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		plot      string
		contentID string
		// wantContains: substrings that MUST be present in the marshaled output.
		wantContains []string
		// wantNotContains: substrings that MUST NOT be present (unescaped forms).
		wantNotContains []string
	}{
		{
			name:       "ampersand escaped",
			title:      "Tom & Jerry",
			plot:       "Cat & mouse",
			contentID:  "EP-AMP",
			wantContains:  []string{"&amp;"},
			wantNotContains: []string{"<title>Tom & Jerry", "<plot>Cat & mouse"},
		},
		{
			name:       "less-than greater-than escaped",
			title:      "<Title>Tag</Title>",
			plot:       "a < b > c",
			contentID:  "EP-LT",
			wantContains:  []string{"&lt;Title&gt;Tag&lt;/Title&gt;"},
			wantNotContains: []string{"<title><Title>Tag</Title>"},
		},
		{
			name:       "quotes escaped",
			title:       `"Quoted"`,
			plot:        `say "hello"`,
			contentID:  "EP-QUOTE",
			wantContains:  []string{`&quot;`},
		},
		{
			name:       "CJK multiplication sign preserved",
			title:      "Hidamari Sketch \u00d7 Honeycomb",
			plot:       "episode plot",
			contentID:  "EP-CJK",
			wantContains:  []string{"Hidamari Sketch \u00d7 Honeycomb"},
		},
		{
			name:       "emoji preserved as UTF-8",
			title:       "Emoji Test",
			plot:       "flags \U0001F38C",
			contentID:  "EP-EMOJI",
			wantContains:  []string{"\U0001F38C"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &api.EpisodeInfo{
				EpisodeMetadata: api.EpisodeMetadata{
					SeriesTitle:  "Test Series",
					SeasonNumber: 1,
					EpisodeNumber: 1,
					Description:  tt.plot,
				},
				Title: tt.title,
			}
			out, err := marshalEpisode(info, tt.contentID)
			if err != nil {
				t.Fatalf("marshalEpisode() error = %v", err)
			}
			if len(out) == 0 {
				t.Fatal("marshalEpisode() output empty, want marshaled XML")
			}
			s := string(out)
			for _, want := range tt.wantContains {
				if !strings.Contains(s, want) {
					t.Errorf("output missing %q\noutput:\n%s", want, s)
				}
			}
			for _, bad := range tt.wantNotContains {
				if strings.Contains(s, bad) {
					t.Errorf("output unexpectedly contains unescaped %q\noutput:\n%s", bad, s)
				}
			}
			// well-formedness: re-parse with xml.Unmarshal into a struct.
			var reparse episodedetails
			if err := xml.Unmarshal(out, &reparse); err != nil {
				t.Fatalf("output not well-formed XML: %v\noutput:\n%s", err, s)
			}
			if reparse.UniqueID.Value != tt.contentID {
				t.Errorf("uniqueid chardata = %q, want %q", reparse.UniqueID.Value, tt.contentID)
			}
			if reparse.Season != 1 {
				t.Errorf("season = %d, want 1 (SeasonNumber not EpisodeNumber)", reparse.Season)
			}
		})
	}
}

// TestMarshalEpisodeUniqueID asserts <uniqueid type="crunchyroll"> is present
// with the content id as chardata (D-08).
func TestMarshalEpisodeUniqueID(t *testing.T) {
	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:  "Test Series",
			SeasonNumber: 2,
			EpisodeNumber: 7,
		},
		Title: "Test Episode",
	}
	out, err := marshalEpisode(info, "GEP123456")
	if err != nil {
		t.Fatalf("marshalEpisode() error = %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `<uniqueid type="crunchyroll"`) {
		t.Errorf("output missing <uniqueid type=\"crunchyroll\">\noutput:\n%s", s)
	}
	if !strings.Contains(s, "GEP123456") {
		t.Errorf("output missing content id GEP123456 as chardata\noutput:\n%s", s)
	}
}

// TestMarshalEpisodeEmptyPlotOmitted asserts an empty plot yields no empty
// <plot></plot> element (omitempty on the xml tag).
func TestMarshalEpisodeEmptyPlotOmitted(t *testing.T) {
	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:  "Test Series",
			SeasonNumber: 1,
			EpisodeNumber: 1,
		},
		Title: "Test Episode",
	}
	out, err := marshalEpisode(info, "EP-EMPTY")
	if err != nil {
		t.Fatalf("marshalEpisode() error = %v", err)
	}
	if strings.Contains(string(out), "<plot>") {
		t.Errorf("output contains <plot> element; empty plot should be omitted\noutput:\n%s", out)
	}
}

// TestMarshalEpisodeHasXMLHeader asserts the xml.Header prepends the body
// (Jellyfin/Kodi both require the <?xml ...?> declaration).
func TestMarshalEpisodeHasXMLHeader(t *testing.T) {
	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:  "Test Series",
			SeasonNumber: 1,
			EpisodeNumber: 1,
		},
		Title: "Test Episode",
	}
	out, err := marshalEpisode(info, "EP-HDR")
	if err != nil {
		t.Fatalf("marshalEpisode() error = %v", err)
	}
	if !strings.HasPrefix(string(out), "<?xml") {
		t.Errorf("output missing xml.Header prefix\noutput:\n%s", out)
	}
}

// TestMarshalTVShowEscaping covers the same escape cases for tvshow.nfo +
// asserts <uniqueid type="crunchyroll"> carries the series content id.
func TestMarshalTVShowEscaping(t *testing.T) {
	info := &api.SeriesInfo{
		ID:          "GSERIES789",
		Title:       "Tom & Jerry: The Series <Reboot>",
		Description: "cats & dogs \"vs\" each other",
		Genres:      []string{"Action", "Comedy"},
		Studio:      "Test Studio",
	}
	out, err := marshalTVShow(info)
	if err != nil {
		t.Fatalf("marshalTVShow() error = %v", err)
	}
	if len(out) == 0 {
		t.Fatal("marshalTVShow() output empty, want marshaled XML")
	}
	s := string(out)
	// entity escaping
	if !strings.Contains(s, "&amp;") {
		t.Errorf("output missing escaped &amp;\noutput:\n%s", s)
	}
	if strings.Contains(s, "Tom & Jerry: The Series <Reboot>") {
		t.Errorf("output contains unescaped title\noutput:\n%s", s)
	}
	if !strings.Contains(s, "&lt;Reboot&gt;") {
		t.Errorf("output missing escaped &lt;Reboot&gt;\noutput:\n%s", s)
	}
	// uniqueid
	if !strings.Contains(s, `<uniqueid type="crunchyroll"`) {
		t.Errorf("output missing <uniqueid type=\"crunchyroll\">\noutput:\n%s", s)
	}
	if !strings.Contains(s, "GSERIES789") {
		t.Errorf("output missing series id GSERIES789 as chardata\noutput:\n%s", s)
	}
	// xml header
	if !strings.HasPrefix(s, "<?xml") {
		t.Errorf("output missing xml.Header prefix\noutput:\n%s", s)
	}
	// well-formedness
	var reparse tvshow
	if err := xml.Unmarshal(out, &reparse); err != nil {
		t.Fatalf("output not well-formed XML: %v\noutput:\n%s", err, s)
	}
	if reparse.UniqueID.Value != "GSERIES789" {
		t.Errorf("uniqueid chardata = %q, want GSERIES789", reparse.UniqueID.Value)
	}
}

// TestWriteEpisodeWritesToFile asserts WriteEpisode writes the marshaled XML
// to the given path (full disk round-trip once; escape cases tested via
// marshalEpisode directly).
func TestWriteEpisodeWritesToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "episode.nfo")
	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:  "Test Series",
			SeasonNumber: 1,
			EpisodeNumber: 1,
			Description: "plot text",
		},
		Title: "Test Episode",
	}
	if err := WriteEpisodeFn(nil, path, info, "EP-WRITE"); err != nil {
		t.Fatalf("WriteEpisode() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if !bytes.Contains(data, []byte("EP-WRITE")) {
		t.Errorf("written file missing content id EP-WRITE\ndata:\n%s", data)
	}
	if !strings.HasPrefix(string(data), "<?xml") {
		t.Errorf("written file missing xml.Header\ndata:\n%s", data)
	}
}

// TestWriteTVShowWritesToFile asserts WriteTVShow writes the marshaled XML
// to the given path.
func TestWriteTVShowWritesToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tvshow.nfo")
	info := &api.SeriesInfo{
		ID:          "GSERIES-WRITE",
		Title:       "Test Series",
		Description: "plot",
		Genres:      []string{"Drama"},
		Studio:      "Studio",
	}
	if err := WriteTVShowFn(nil, path, info); err != nil {
		t.Fatalf("WriteTVShow() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if !bytes.Contains(data, []byte("GSERIES-WRITE")) {
		t.Errorf("written file missing series id GSERIES-WRITE\ndata:\n%s", data)
	}
	if !strings.HasPrefix(string(data), "<?xml") {
		t.Errorf("written file missing xml.Header\ndata:\n%s", data)
	}
}