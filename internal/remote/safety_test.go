package remote

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type oversizedTransport struct{}

func (oversizedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", (32<<20)+1))), Header: make(http.Header)}, nil
}
func TestHTTPRejectsTruncatedResponse(t *testing.T) {
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://fixture", nil)
	b, _, e := readLimited(context.Background(), &http.Client{Transport: oversizedTransport{}}, req)
	if e == nil || b != nil {
		t.Fatal("oversized response silently truncated to 32 MiB")
	}
}
