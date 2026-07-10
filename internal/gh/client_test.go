package gh

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestConditionalTransportStoresBodyAndRecordsRate(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/o/r/issues?access_token=secret&per_page=100", nil)
	if err != nil {
		t.Fatal(err)
	}
	key := "GET /repos/o/r/issues?per_page=100"
	cache := NewConditionalCache(nil)
	transport := &conditionalTransport{
		cache: cache,
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("If-None-Match"); got != "" {
				t.Fatalf("unexpected If-None-Match on cold cache: %q", got)
			}
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`[{"number":1}]`)),
			}
			resp.Header.Set("ETag", `"v1"`)
			resp.Header.Set("Content-Type", "application/json")
			resp.Header.Set("X-RateLimit-Remaining", "42")
			resp.Header.Set("X-RateLimit-Reset", "1700000000")
			resp.Header.Set("X-RateLimit-Limit", "5000")
			resp.Header.Set("X-RateLimit-Used", "4958")
			return resp, nil
		}),
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `[{"number":1}]` {
		t.Fatalf("caller body = %q, want the original payload", body)
	}
	entries := cache.Entries()
	e, ok := entries[key]
	if !ok || e.ETag != `"v1"` || string(e.Body) != `[{"number":1}]` {
		t.Fatalf("cached entry = %+v, want stored body+etag", e)
	}
	remaining, reset, limit, used := cache.RateLimit()
	if remaining != 42 || limit != 5000 || used != 4958 {
		t.Fatalf("rate metadata = remaining %d limit %d used %d", remaining, limit, used)
	}
	if want := time.Unix(1700000000, 0).UTC(); !reset.Equal(want) {
		t.Fatalf("reset = %s, want %s", reset, want)
	}
}

func TestConditionalTransportReplaysBodyOn304(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/o/r/issues?per_page=100", nil)
	if err != nil {
		t.Fatal(err)
	}
	key := "GET /repos/o/r/issues?per_page=100"
	cache := NewConditionalCache(map[string]ResponseCacheEntry{
		key: {
			ETag:        `"v1"`,
			Body:        []byte(`[{"number":1}]`),
			ContentType: "application/json",
			Link:        `<https://api.github.com/repos/o/r/issues?per_page=100&page=2>; rel="next"`,
		},
	})
	transport := &conditionalTransport{
		cache: cache,
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("If-None-Match"); got != `"v1"` {
				t.Fatalf("If-None-Match = %q, want stored ETag", got)
			}
			resp := &http.Response{
				StatusCode: http.StatusNotModified,
				Header:     make(http.Header),
				Body:       http.NoBody,
			}
			// 304 responses still report the (unspent) budget.
			resp.Header.Set("X-RateLimit-Remaining", "4999")
			return resp, nil
		}),
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("replayed status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `[{"number":1}]` {
		t.Fatalf("replayed body = %q, want cached payload", body)
	}
	if got := resp.Header.Get("Link"); !strings.Contains(got, "page=2") {
		t.Fatalf("replayed Link = %q, want cached pagination header", got)
	}
	if got := resp.Header.Get("X-RateLimit-Remaining"); got != "4999" {
		t.Fatalf("replayed rate header = %q, want live 304 value", got)
	}
	if remaining, _, _, _ := cache.RateLimit(); remaining != 4999 {
		t.Fatalf("recorded remaining = %d, want 4999 from the 304", remaining)
	}
}

func TestConditionalTransportSkipsConditionalWithoutEntry(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/o/r/issues", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Entry without a body must not be sent as a conditional request — a 304
	// could not be replayed.
	cache := NewConditionalCache(map[string]ResponseCacheEntry{
		"GET /repos/o/r/issues?": {ETag: `"v1"`},
	})
	transport := &conditionalTransport{
		cache: cache,
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("If-None-Match"); got != "" {
				t.Fatalf("If-None-Match = %q, want none for body-less entry", got)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody}, nil
		}),
	}
	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
}
