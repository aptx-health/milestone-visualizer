package gh

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	client, _, err := NewClientWithCache(ctx, nil)
	return client, err
}

// ResponseCacheEntry is a persisted conditional-request cache record. It holds
// everything needed to replay a prior 200 response when GitHub answers a
// conditional request with 304 Not Modified: the validator (ETag), the body,
// and the pagination/parse headers go-github relies on.
type ResponseCacheEntry struct {
	ETag        string `json:"etag"`
	Body        []byte `json:"body"` // JSON-encoded as base64
	ContentType string `json:"content_type,omitempty"`
	Link        string `json:"link,omitempty"` // RFC 5988 Link header for pagination
}

// ConditionalCache stores per-URL response entries and the most recent
// rate-limit metadata observed by a client. It is safe for concurrent use.
type ConditionalCache struct {
	mu                 sync.Mutex
	entries            map[string]ResponseCacheEntry
	rateLimitRemaining int
	rateLimitReset     time.Time
	rateLimitLimit     int
	rateLimitUsed      int
}

var rateHeaders = []string{
	"X-RateLimit-Remaining",
	"X-RateLimit-Reset",
	"X-RateLimit-Limit",
	"X-RateLimit-Used",
}

// NewClientWithCache returns an authenticated GitHub client backed by a
// response cache. For any GET whose URL is present in the cache the client
// sends If-None-Match; a 304 is replayed transparently from the cached body
// (headers and all) so callers — including go-github's pagination — never see
// the 304. Fresh 200 responses are recorded back into the cache.
func NewClientWithCache(ctx context.Context, entries map[string]ResponseCacheEntry) (*github.Client, *ConditionalCache, error) {
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
	cache := NewConditionalCache(entries)
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})
	httpClient := oauth2.NewClient(ctx, src)
	httpClient.Transport = &conditionalTransport{
		base:  httpClient.Transport,
		cache: cache,
	}
	return github.NewClient(httpClient), cache, nil
}

func NewConditionalCache(entries map[string]ResponseCacheEntry) *ConditionalCache {
	cp := map[string]ResponseCacheEntry{}
	for k, v := range entries {
		if strings.TrimSpace(v.ETag) == "" || len(v.Body) == 0 {
			continue
		}
		cp[k] = cloneEntry(v)
	}
	return &ConditionalCache{
		entries:            cp,
		rateLimitRemaining: -1,
	}
}

// Entries returns a deep copy of the cached response entries, suitable for
// persisting alongside the snapshot.
func (c *ConditionalCache) Entries() map[string]ResponseCacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := map[string]ResponseCacheEntry{}
	for k, v := range c.entries {
		cp[k] = cloneEntry(v)
	}
	return cp
}

func (c *ConditionalCache) RateLimit() (remaining int, reset time.Time, limit int, used int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rateLimitRemaining, c.rateLimitReset, c.rateLimitLimit, c.rateLimitUsed
}

func (c *ConditionalCache) entry(key string) (ResponseCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return ResponseCacheEntry{}, false
	}
	return cloneEntry(e), true
}

func (c *ConditionalCache) putEntry(key string, e ResponseCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cloneEntry(e)
}

func (c *ConditionalCache) deleteEntry(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

func (c *ConditionalCache) recordRate(resp *http.Response) {
	c.mu.Lock()
	defer c.mu.Unlock()
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

func cloneEntry(e ResponseCacheEntry) ResponseCacheEntry {
	e.Body = append([]byte(nil), e.Body...)
	return e
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
	key := ""
	if req.Method == http.MethodGet {
		key = cacheKey(req)
	}

	var stored ResponseCacheEntry
	conditional := false
	if key != "" {
		if e, ok := t.cache.entry(key); ok {
			stored = e
			conditional = true
			req = req.Clone(req.Context())
			req.Header.Set("If-None-Match", e.ETag)
		}
	}

	resp, err := base.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	// Record rate metadata from the real response — a 304 still carries a
	// fresh (and cost-free) budget snapshot.
	t.cache.recordRate(resp)

	if key == "" {
		return resp, nil
	}
	if conditional && resp.StatusCode == http.StatusNotModified {
		return replayResponse(req, resp, stored), nil
	}
	if resp.StatusCode == http.StatusOK {
		return t.storeResponse(key, resp)
	}
	return resp, nil
}

// replayResponse rebuilds a 200 response from a cached entry, carrying the live
// rate-limit headers from the 304 so callers see the current budget. Only cached
// entries (which always have an ETag and body) reach here.
func replayResponse(req *http.Request, orig *http.Response, e ResponseCacheEntry) *http.Response {
	if orig.Body != nil {
		_, _ = io.Copy(io.Discard, orig.Body)
		_ = orig.Body.Close()
	}
	header := make(http.Header)
	if e.ContentType != "" {
		header.Set("Content-Type", e.ContentType)
	}
	if e.Link != "" {
		header.Set("Link", e.Link)
	}
	if e.ETag != "" {
		header.Set("ETag", e.ETag)
	}
	for _, h := range rateHeaders {
		if v := orig.Header.Get(h); v != "" {
			header.Set(h, v)
		}
	}
	body := append([]byte(nil), e.Body...)
	return &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         orig.Proto,
		ProtoMajor:    orig.ProtoMajor,
		ProtoMinor:    orig.ProtoMinor,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
}

// storeResponse buffers the body so it can be cached and still consumed by the
// caller, then records the entry (only when an ETag is present to validate it).
func (t *conditionalTransport) storeResponse(key string, resp *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if etag := strings.TrimSpace(resp.Header.Get("ETag")); etag != "" {
		t.cache.putEntry(key, ResponseCacheEntry{
			ETag:        etag,
			Body:        body,
			ContentType: resp.Header.Get("Content-Type"),
			Link:        resp.Header.Get("Link"),
		})
	} else {
		t.cache.deleteEntry(key)
	}
	return resp, nil
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
