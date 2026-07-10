package gh

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v72/github"
	"golang.org/x/oauth2"
)

// NewClient returns an authenticated GitHub client.
// Auth precedence: GITHUB_TOKEN env, then `gh auth token`.
func NewClient(ctx context.Context) (*github.Client, error) {
	client, _, err := NewClientWithETags(ctx, nil)
	return client, err
}

// ConditionalCache tracks ETags and rate-limit metadata observed by a client.
type ConditionalCache struct {
	mu                 sync.Mutex
	etags              map[string]string
	rateLimitRemaining int
	rateLimitReset     time.Time
	rateLimitLimit     int
	rateLimitUsed      int
}

// NewClientWithETags returns an authenticated GitHub client that sends
// If-None-Match for URLs present in etags and records fresh ETags from GitHub.
func NewClientWithETags(ctx context.Context, etags map[string]string) (*github.Client, *ConditionalCache, error) {
	tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if tok == "" {
		out, err := exec.Command("gh", "auth", "token").Output()
		if err != nil {
			return nil, nil, fmt.Errorf("no GITHUB_TOKEN and `gh auth token` failed: %w", err)
		}
		tok = strings.TrimSpace(string(out))
	}
	if tok == "" {
		return nil, nil, fmt.Errorf("could not resolve GitHub token")
	}
	cache := NewConditionalCache(etags)
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})
	httpClient := oauth2.NewClient(ctx, src)
	httpClient.Transport = &conditionalTransport{
		base:  httpClient.Transport,
		cache: cache,
	}
	return github.NewClient(httpClient), cache, nil
}

func NewConditionalCache(etags map[string]string) *ConditionalCache {
	cp := map[string]string{}
	for k, v := range etags {
		if strings.TrimSpace(v) != "" {
			cp[k] = v
		}
	}
	return &ConditionalCache{
		etags:              cp,
		rateLimitRemaining: -1,
	}
}

func (c *ConditionalCache) ETags() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := map[string]string{}
	for k, v := range c.etags {
		cp[k] = v
	}
	return cp
}

func (c *ConditionalCache) RateLimit() (remaining int, reset time.Time, limit int, used int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rateLimitRemaining, c.rateLimitReset, c.rateLimitLimit, c.rateLimitUsed
}

type conditionalTransport struct {
	base  http.RoundTripper
	cache *ConditionalCache
}

func (t *conditionalTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	key := cacheKey(req)
	if req.Method == http.MethodGet && key != "" {
		if etag := t.cache.etag(key); etag != "" {
			req = req.Clone(req.Context())
			req.Header.Set("If-None-Match", etag)
		}
	}
	resp, err := base.RoundTrip(req)
	if resp != nil && key != "" {
		t.cache.record(key, resp)
	}
	return resp, err
}

func (c *ConditionalCache) etag(key string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.etags[key]
}

func (c *ConditionalCache) record(key string, resp *http.Response) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if etag := strings.TrimSpace(resp.Header.Get("ETag")); etag != "" {
		c.etags[key] = etag
	}
	if remaining, ok := parseIntHeader(resp.Header.Get("X-RateLimit-Remaining")); ok {
		c.rateLimitRemaining = remaining
	}
	if reset, ok := parseIntHeader(resp.Header.Get("X-RateLimit-Reset")); ok {
		c.rateLimitReset = time.Unix(int64(reset), 0).UTC()
	}
	if limit, ok := parseIntHeader(resp.Header.Get("X-RateLimit-Limit")); ok {
		c.rateLimitLimit = limit
	}
	if used, ok := parseIntHeader(resp.Header.Get("X-RateLimit-Used")); ok {
		c.rateLimitUsed = used
	}
}

func cacheKey(req *http.Request) string {
	if req == nil || req.URL == nil || req.URL.Path == "" {
		return ""
	}
	query := req.URL.Query()
	query.Del("access_token")
	rawQuery := query.Encode()
	return req.Method + " " + req.URL.EscapedPath() + "?" + rawQuery
}

func parseIntHeader(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

// ParseOwnerRepo splits "owner/repo" into its parts.
func ParseOwnerRepo(s string) (string, string, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected <owner>/<repo>, got %q", s)
	}
	return parts[0], parts[1], nil
}
