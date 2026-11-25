// Package avatars fetches Steam profile avatars through the public community
// XML endpoint (no API key required) and caches them on disk.
package avatars

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// DefaultProfileURL is the public XML profile endpoint. Tests override it.
const DefaultProfileURL = "https://steamcommunity.com/profiles/%s/?xml=1"

// MaxProfileBytes caps the XML payload; avatars are fetched separately.
const MaxProfileBytes = 512 << 10

// MaxImageBytes caps the avatar download.
const MaxImageBytes = 2 << 20

// DefaultMaxAge is how long a cached avatar is reused before refetching.
const DefaultMaxAge = 30 * 24 * time.Hour

// ErrNotFound is returned when the profile is private or missing.
var ErrNotFound = errors.New("steam profile not found or private")

// ErrInvalidSteamID is returned for a malformed SteamID64.
var ErrInvalidSteamID = errors.New("invalid steamid64")

var (
	steamIDPattern = regexp.MustCompile(`^[0-9]{17}$`)
	avatarPattern  = regexp.MustCompile(`(?s)<avatarFull><!\[CDATA\[(.*?)\]\]></avatarFull>`)
)

// CacheDir returns the avatar cache directory for a DataRoot.
func CacheDir(dataRoot string) string {
	return filepath.Join(dataRoot, "cache", "radar_cards", "portraits")
}

// Path returns the cached avatar path and whether it exists.
func Path(dir, steamID string) (string, bool) {
	for _, ext := range []string{".png", ".jpg", ".jpeg"} {
		p := filepath.Join(dir, steamID+ext)
		if info, err := os.Stat(p); err == nil && info.Size() > 0 {
			return p, true
		}
	}
	return "", false
}

// ValidSteamID64 reports whether s looks like a SteamID64.
func ValidSteamID64(s string) bool { return steamIDPattern.MatchString(strings.TrimSpace(s)) }

// Client fetches avatars with an injectable base URL for tests.
type Client struct {
	HTTP       *http.Client
	ProfileURL string
}

// NewClient returns a client with sane timeouts.
func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 20 * time.Second}, ProfileURL: DefaultProfileURL}
}

// Fetch downloads the avatar for steamID into dir, reusing a cache entry newer
// than maxAge. It returns the path written.
func (c *Client) Fetch(ctx context.Context, steamID, dir string, maxAge time.Duration) (string, error) {
	steamID = strings.TrimSpace(steamID)
	if !ValidSteamID64(steamID) {
		return "", ErrInvalidSteamID
	}
	if existing, ok := Path(dir, steamID); ok && maxAge > 0 {
		if info, err := os.Stat(existing); err == nil && time.Since(info.ModTime()) <= maxAge {
			return existing, nil
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create avatar dir: %w", err)
	}
	template := c.ProfileURL
	if template == "" {
		template = DefaultProfileURL
	}
	avatarURL, err := c.profileAvatar(ctx, fmt.Sprintf(template, steamID))
	if err != nil {
		return "", err
	}
	body, err := c.download(ctx, avatarURL, MaxImageBytes)
	if err != nil {
		return "", fmt.Errorf("download avatar: %w", err)
	}
	ext := ".png"
	switch {
	case strings.HasSuffix(strings.ToLower(avatarURL), ".jpg"), strings.HasSuffix(strings.ToLower(avatarURL), ".jpeg"):
		ext = ".jpg"
	}
	dest := filepath.Join(dir, steamID+ext)
	tmp := dest + ".part"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return "", fmt.Errorf("write avatar: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("commit avatar: %w", err)
	}
	return dest, nil
}

// profileAvatar reads avatarFull from the public XML profile.
func (c *Client) profileAvatar(ctx context.Context, profileURL string) (string, error) {
	body, err := c.download(ctx, profileURL, MaxProfileBytes)
	if err != nil {
		return "", fmt.Errorf("fetch profile: %w", err)
	}
	m := avatarPattern.FindSubmatch(body)
	if len(m) < 2 {
		return "", ErrNotFound
	}
	url := strings.TrimSpace(string(m[1]))
	if url == "" {
		return "", ErrNotFound
	}
	return url, nil
}

func (c *Client) download(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Strikeye avatar fetch)")
	client := c.HTTP
	if client == nil {
		// A zero Client must still work: callers building a struct literal with
		// only ProfileURL used to panic here.
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return body, nil
}
