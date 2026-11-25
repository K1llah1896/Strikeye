// Package gcpd lists and downloads CS2 match demos from Valve's Game Coordinator
// Public Data (GCPD) page.
//
// The page lives at https://steamcommunity.com/my/gcpd/730?tab=<tab> and is only
// reachable with a logged-in Steam web session, so the caller supplies the
// Cookie header. Demo files themselves are plain, unauthenticated
// https://replayN.valve.net/730/<match>_<outcome>.dem.bz2 URLs, which the watcher
// already knows how to ingest.
package gcpd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the Steam Community root.
const DefaultBaseURL = "https://steamcommunity.com"

// DefaultUserAgent mirrors a desktop browser: Valve serves a different page to
// unknown clients.
const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// Known tabs. Valve has renamed these over time; unknown tabs still work as long
// as the page renders demo links.
var Tabs = []string{
	"matchhistorypremier",
	"matchhistorycompetitive",
	"matchhistorycompetitivepermap",
	"matchhistoryscrimmage",
	"matchhistorywingman",
}

// Match is one downloadable demo entry.
type Match struct {
	MatchID  uint64    `json:"match_id"`
	Outcome  uint64    `json:"outcome_id"`
	URL      string    `json:"url"`
	Date     time.Time `json:"date"`
	Type     string    `json:"type"`
	Filename string    `json:"filename"`
}

// Client reads the GCPD page.
type Client struct {
	HTTP      *http.Client
	BaseURL   string
	Cookie    string
	UserAgent string
	// PageSize caps how many match pages are followed via continue_token.
	MaxPages int
}

// NewClient returns a client that authenticates with a Cookie header value.
func NewClient(cookie string) *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		BaseURL:   DefaultBaseURL,
		Cookie:    cookie,
		UserAgent: DefaultUserAgent,
		MaxPages:  4,
	}
}

var (
	// Valve serves demos from replayN.valve.net, but the host is not part of the
	// contract (proxies and mirrors exist), so only the /730/<match>_<outcome>
	// path shape is enforced.
	demoURLRe = regexp.MustCompile(`https?://[^\s"'<>]+/730/(\d+)_(\d+)\.dem\.bz2`)
	dateRe    = regexp.MustCompile(`(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} GMT)`)
	contToken = regexp.MustCompile(`g_sGcContinueToken\s*=\s*'(\d+)'`)
	sessionRe = regexp.MustCompile(`g_sessionID\s*=\s*"([0-9a-f]{24})"`)
	_         = 0
)

// ParsePage extracts the demo entries of one GCPD page.
//
// Each scoreboard cell is a nested table, so cells are not split on </td>;
// instead every demo URL is located and the date is read from a window around
// it. Cells without a demo link (Valve expires old demos) are skipped.
func ParsePage(html, matchType string) []Match {
	const window = 2400
	var out []Match
	seen := map[uint64]bool{}
	for _, loc := range demoURLRe.FindAllStringSubmatchIndex(html, -1) {
		u := demoURLRe.FindStringSubmatch(html[loc[0]:loc[1]])
		if len(u) != 3 {
			continue
		}
		matchID, err1 := strconv.ParseUint(u[1], 10, 64)
		outcomeID, err2 := strconv.ParseUint(u[2], 10, 64)
		if err1 != nil || err2 != nil || seen[matchID] {
			continue
		}
		seen[matchID] = true
		start := loc[1]
		end := start + window
		if end > len(html) {
			end = len(html)
		}
		var when time.Time
		if d := dateRe.FindStringSubmatch(html[start:end]); len(d) == 2 {
			if parsed, err := time.Parse("2006-01-02 15:04:05 MST", d[1]); err == nil {
				when = parsed
			}
		}
		out = append(out, Match{
			MatchID:  matchID,
			Outcome:  outcomeID,
			URL:      u[0],
			Date:     when,
			Type:     matchType,
			Filename: fmt.Sprintf("%d_%d.dem.bz2", matchID, outcomeID),
		})
	}
	return out
}

// List returns the matches of one tab, newest first.
func (c *Client) List(ctx context.Context, tab string) ([]Match, error) {
	if strings.TrimSpace(c.Cookie) == "" {
		return nil, fmt.Errorf("gcpd: steam cookie required")
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	maxPages := c.MaxPages
	if maxPages <= 0 {
		maxPages = 1
	}

	endpoint := base + "/my/gcpd/730?tab=" + tab
	matches, token, sessionID, err := c.fetchPage(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	for page := 1; page < maxPages && token != ""; page++ {
		next := fmt.Sprintf("%s/my/gcpd/730?tab=%s&continue_token=%s&sessionid=%s&ajax=1",
			base, tab, token, sessionID)
		more, nextToken, _, err := c.fetchPage(ctx, next)
		if err != nil {
			return nil, err
		}
		matches = append(matches, more...)
		if nextToken == token || len(more) == 0 {
			break
		}
		token = nextToken
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Date.After(matches[j].Date) })
	return matches, nil
}

// fetchPage returns the matches of one page plus the continuation token and
// session id discovered in it.
func (c *Client) fetchPage(ctx context.Context, endpoint string) ([]Match, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("Cookie", c.Cookie)
	ua := c.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.8")

	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, "", "", err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, "", "", fmt.Errorf("gcpd: steam rejected the session (status %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("gcpd: status %d", resp.StatusCode)
	}
	html := string(body)
	if strings.Contains(html, "g_steamID = false") {
		return nil, "", "", fmt.Errorf("gcpd: not signed in (cookie expired)")
	}
	var tok, sess string
	if m := contToken.FindStringSubmatch(html); len(m) == 2 {
		tok = m[1]
	}
	if m := sessionRe.FindStringSubmatch(html); len(m) == 2 {
		sess = m[1]
	}
	return ParsePage(html, tabOf(endpoint)), tok, sess, nil
}

func tabOf(endpoint string) string {
	if i := strings.Index(endpoint, "tab="); i >= 0 {
		rest := endpoint[i+4:]
		if j := strings.IndexAny(rest, "&"); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	return ""
}

// Download fetches one demo into dir and returns the written path.
func (c *Client) Download(ctx context.Context, m Match, dir string) (string, error) {
	if m.URL == "" {
		return "", fmt.Errorf("gcpd: empty demo url")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := m.Filename
	if name == "" {
		name = fmt.Sprintf("%d_%d.dem.bz2", m.MatchID, m.Outcome)
	}
	target := filepath.Join(dir, name)
	if st, err := os.Stat(target); err == nil && st.Size() > 0 {
		return target, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gcpd: download status %d", resp.StatusCode)
	}
	tmp := target + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	written, err := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}
	if written == 0 {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("gcpd: empty demo body")
	}
	if err := os.Rename(tmp, target); err != nil {
		return "", err
	}
	return target, nil
}

// ReadCookieFile loads a Cookie header value from a file. Both a bare header
// value and a Netscape cookies.txt export are accepted, which covers what users
// usually have at hand.
func ReadCookieFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "", fmt.Errorf("gcpd: empty cookie file")
	}
	if strings.HasPrefix(text, "# Netscape") || strings.Contains(text, "\t") {
		var parts []string
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimRight(line, "\r")
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Split(line, "\t")
			if len(fields) < 7 {
				continue
			}
			domain, name, value := fields[0], fields[5], fields[6]
			if !strings.Contains(domain, "steamcommunity.com") && !strings.Contains(domain, "steampowered.com") {
				continue
			}
			parts = append(parts, name+"="+value)
		}
		if len(parts) == 0 {
			return "", fmt.Errorf("gcpd: no steam cookies in file")
		}
		return strings.Join(parts, "; "), nil
	}
	return strings.TrimSpace(strings.TrimPrefix(text, "Cookie:")), nil
}
