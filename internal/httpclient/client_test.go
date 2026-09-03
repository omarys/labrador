package httpclient_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/omarys/labrador/internal/httpclient"
)

func TestStealthClient_AppliesHeadersAndCookies(t *testing.T) {
	var capturedUA, capturedSecCH, capturedReferer string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		capturedSecCH = r.Header.Get("Sec-Ch-Ua")
		capturedReferer = r.Header.Get("Referer")

		http.SetCookie(w, &http.Cookie{Name: "session", Value: "active123"})
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := httpclient.NewStealthClient(5 * time.Second)

	resp, err := client.Get(ts.URL + "/mangas")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if capturedUA != httpclient.DefaultUserAgent {
		t.Errorf("expected UA %s, got %s", httpclient.DefaultUserAgent, capturedUA)
	}
	if capturedSecCH == "" {
		t.Errorf("expected Sec-Ch-Ua header to be set")
	}
	if capturedReferer == "" {
		t.Errorf("expected Referer header to be set")
	}

	// Verify cookie jar retained cookie
	u, _ := resp.Request.URL.Parse(ts.URL)
	cookies := client.Jar.Cookies(u)
	if len(cookies) != 1 || cookies[0].Value != "active123" {
		t.Errorf("expected cookie to be preserved in cookie jar: %+v", cookies)
	}
}
