package api

type Episode struct {
	ManifestURL string               `json:"url"`
	Subtitles   map[string]*Subtitle `json:"subtitles"`
	Token       string               `json:"token"`
	Error       any                  `json:"error"`
}

type Subtitle struct {
	Language string `json:"language"`
	URL      string `json:"url"`
}

type EpisodeMetadataResponse struct {
	Data []EpisodeInfo `json:"data"`
}

type EpisodeInfo struct {
	EpisodeMetadata EpisodeMetadata `json:"episode_metadata"`
	Title           string          `json:"title"`
}

// CMS /content/v2/cms/objects/{id} enrichment fields (D-06): the real response
// carries additional fields beyond the pre-Phase-7 minimal decode. The names below
// mirror the Crunchyroll CMS v2 snake_case convention established by the existing
// `series_title` tag: `series_id`, `description`, `slug`, `duration_ms`. The
// `series_id` field is the single-episode-path source for client.GetSeriesInfo
// (Option A — NO new network call just to obtain the id).
type EpisodeMetadata struct {
	AudioLocale        string        `json:"audio_locale"`
	EpisodeNumber      int           `json:"episode_number"`
	SeasonNumber       int           `json:"season_number"`
	SeriesTitle        string        `json:"series_title"`
	SeriesID           string        `json:"series_id"`
	PosterURL          string        `json:"poster_url"`
	BackdropURL        string        `json:"backdrop_url"`
	Description        string        `json:"description"`
	Slug               string        `json:"slug"`
	DurationMs         int           `json:"duration_ms"`
	AvailabilityStarts string        `json:"availability_starts"`
	Versions           []*DubVersion `json:"versions"`
}

type DubVersion struct {
	AudioLocale string `json:"audio_locale"`
	GUID        string `json:"guid"`
}

type SeasonEpisodes struct {
	Data []SeasonEpisode `json:"data"`
}

// SeasonEpisode carries one episode object from the
// /content/v2/cms/seasons/{id}/episodes response. The SeriesID field (D-07
// Option A) is the season-path source for client.GetSeriesInfo — decoded from
// the EXISTING GetSeasonEpisodes response, so NO new network call is required
// just to obtain the series id.
type SeasonEpisode struct {
	ID                 string        `json:"id"`
	Versions           []*DubVersion `json:"versions"`
	SeasonNumber       int           `json:"season_number"`
	EpisodeNumber      int           `json:"episode_number"`
	SeriesTitle        string        `json:"series_title"`
	SeriesID           string        `json:"series_id"`
	AudioLocale        string        `json:"audio_locale"`
	Title              string        `json:"title"`
	AvailabilityStarts string        `json:"availability_starts"`
}

type Seasons struct {
	Data []Season `json:"data"`
}

type Season struct {
	ID           string `json:"id"`
	SeasonNumber int    `json:"season_number"`
}

// SeriesInfoResponse is the top-level wrapper for the series-level CMS
// /content/v2/cms/series/{id} response (D-07). Modeled on the Seasons/Season
// sibling pair: a `data` array carrying the series object.
type SeriesInfoResponse struct {
	Data []SeriesInfo `json:"data"`
}

// SeriesInfo carries the series-level fields a rich tvshow.nfo needs
// (<plot>/<genre>/<studio>). The `id` is the Crunchyroll series content id
// and doubles as the <uniqueid type="crunchyroll"> chardata for tvshow.nfo.
type SeriesInfo struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	PosterURL   string   `json:"poster_url"`
	BackdropURL string   `json:"backdrop_url"`
	Description string   `json:"description"`
	Genres      []string `json:"genres"`
	Studio      string   `json:"studio"`
}

type CrunchyrollTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type CrunchyrollWidevineLicenseResponse struct {
	License string `json:"license"`
}
