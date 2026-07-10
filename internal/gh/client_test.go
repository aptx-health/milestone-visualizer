package gh

import (
	"net/http"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestConditionalTransportSendsIfNoneMatchAndRecordsMetadata(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/o/r/issues?access_token=secret&per_page=100", nil)
	if err != nil {
		t.Fatal(err)
	}
	key := "GET /repos/o/r/issues?per_page=100"
	cache := NewConditionalCache(map[string]string{key: `"old"`})
	transport := &conditionalTransport{
		cache: cache,
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("If-None-Match"); got != `"old"` {
				t.Fatalf("If-None-Match = %q, want old ETag", got)
			}
			resp := &http.Response{
				StatusCode: http.StatusNotModified,
				Header:     make(http.Header),
				Body:       http.NoBody,
			}
			resp.Header.Set("ETag", `"new"`)
			resp.Header.Set("X-RateLimit-Remaining", "42")
			resp.Header.Set("X-RateLimit-Reset", "1700000000")
			resp.Header.Set("X-RateLimit-Limit", "5000")
			resp.Header.Set("X-RateLimit-Used", "4958")
			return resp, nil
		}),
	}
	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	if got := cache.ETags()[key]; got != `"new"` {
		t.Fatalf("recorded ETag = %q, want new ETag", got)
	}
	remaining, reset, limit, used := cache.RateLimit()
	if remaining != 42 || limit != 5000 || used != 4958 {
		t.Fatalf("rate metadata = remaining %d limit %d used %d", remaining, limit, used)
	}
	if want := time.Unix(1700000000, 0).UTC(); !reset.Equal(want) {
		t.Fatalf("reset = %s, want %s", reset, want)
	}
}
