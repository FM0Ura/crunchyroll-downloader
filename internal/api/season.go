package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (c *Client) GetSeasonEpisodes(ctx context.Context, contentId, audioLocale, subLocale string) ([]SeasonEpisode, error) {
	if audioLocale == "" {
		audioLocale = "ja-JP"
	}
	if subLocale == "" {
		subLocale = "en-US"
	}

	req, err := c.newRequest(ctx, http.MethodGet,
		c.url(fmt.Sprintf("/content/v2/cms/seasons/%s/episodes?preferred_audio_language=%s&locale=%s",
			contentId, audioLocale, subLocale)), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var episodes SeasonEpisodes
	if err := json.Unmarshal(body, &episodes); err != nil {
		return nil, err
	}

	return episodes.Data, nil
}

func (c *Client) GetSeasons(ctx context.Context, contentId, audioLocale, subLocale string) ([]Season, error) {
	if audioLocale == "" {
		audioLocale = "ja-JP"
	}
	if subLocale == "" {
		subLocale = "en-US"
	}

	req, err := c.newRequest(ctx, http.MethodGet,
		c.url(fmt.Sprintf("/content/v2/cms/series/%s/seasons?force_locale=&preferred_audio_language=%s&locale=%s",
			contentId, audioLocale, subLocale)), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var seasons Seasons
	if err := json.Unmarshal(body, &seasons); err != nil {
		return nil, err
	}

	return seasons.Data, nil
}

// GetSeriesInfo fetches the series-level CMS object for the given series id
// (D-07 LOCKED). The confirmed endpoint path mirrors the GetSeasons prefix
// (`/content/v2/cms/series/%s/seasons`): `/content/v2/cms/series/%s` returns
// the series object carrying title/plot/genre/studio. The decoded body is
// the NEW SeriesInfoResponse (types.go). Preserves the 401-refresh retry
// inherited from c.Do (client.go). audioLocale/subLocale default to ja-JP/en-US
// for consistency with GetSeasons/GetSeasonEpisodes.
func (c *Client) GetSeriesInfo(ctx context.Context, seriesId, audioLocale, subLocale string) (*SeriesInfo, error) {
	if audioLocale == "" {
		audioLocale = "ja-JP"
	}
	if subLocale == "" {
		subLocale = "en-US"
	}

	req, err := c.newRequest(ctx, http.MethodGet,
		c.url(fmt.Sprintf("/content/v2/cms/series/%s?preferred_audio_language=%s&locale=%s",
			seriesId, audioLocale, subLocale)), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var series SeriesInfoResponse
	if err := json.Unmarshal(body, &series); err != nil {
		return nil, err
	}

	if len(series.Data) == 0 {
		return nil, fmt.Errorf("no series info found for id: %s", seriesId)
	}

	return &series.Data[0], nil
}
