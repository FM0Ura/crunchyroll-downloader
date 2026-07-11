package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var ErrArtworkNotFound = errors.New("artwork not found (404)")

// allowedArtworkHostSuffixes lists Crunchyroll CDN host families accepted for
// artwork fetches. Add new Crunchyroll-owned CDN suffixes here when observed.
var allowedArtworkHostSuffixes = []string{".crunchyroll.com", ".vmdcdn.com", ".akamaized.net"}

func allowedArtworkURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" {
		return false
	}

	host := parsed.Hostname()
	if host == "" {
		host = parsed.Host
		if splitHost, _, err := net.SplitHostPort(parsed.Host); err == nil {
			host = splitHost
		}
	}
	host = strings.ToLower(host)
	for _, suffix := range allowedArtworkHostSuffixes {
		normalized := strings.ToLower(suffix)
		if strings.HasPrefix(normalized, ".") {
			if strings.HasSuffix(host, normalized) || host == strings.TrimPrefix(normalized, ".") {
				return true
			}
			continue
		}
		if host == normalized || strings.HasSuffix(host, "."+normalized) {
			return true
		}
	}
	return false
}

func (c *Client) FetchArtwork(ctx context.Context, artworkURL, destPath string) error {
	if !allowedArtworkURL(artworkURL) {
		return ErrArtworkNotFound
	}

	req, err := c.newRequest(ctx, http.MethodGet, artworkURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrArtworkNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("artwork fetch: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("artwork read: %w", err)
	}
	if err := os.WriteFile(destPath, body, 0644); err != nil {
		return fmt.Errorf("artwork write %s: %w", destPath, err)
	}
	return nil
}
