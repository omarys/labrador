package httpclient

import (
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

const (
	DefaultUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36"
)

// StealthTransport wraps an http.RoundTripper and injects authentic browser headers
// and referers on every outgoing request to defeat Cloudflare/WAF anti-bot 403 blocks.
type StealthTransport struct {
	Base http.RoundTripper
}

func (s *StealthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())

	// 1. Set Chrome Desktop User-Agent
	if r.Header.Get("User-Agent") == "" {
		r.Header.Set("User-Agent", DefaultUserAgent)
	}

	// 2. Set Realistic Accept Headers
	if r.Header.Get("Accept") == "" {
		if isImageURL(r.URL.Path) {
			r.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
		} else {
			r.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
		}
	}

	if r.Header.Get("Accept-Language") == "" {
		r.Header.Set("Accept-Language", "en-US,en;q=0.9")
	}

	// 3. Set Sec-CH-UA Client Hints (identifies as desktop Chromium)
	if r.Header.Get("Sec-Ch-Ua") == "" {
		r.Header.Set("Sec-Ch-Ua", `"Not(A:Brand";v="99", "Google Chrome";v="133", "Chromium";v="133"`)
	}
	if r.Header.Get("Sec-Ch-Ua-Mobile") == "" {
		r.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	}
	if r.Header.Get("Sec-Ch-Ua-Platform") == "" {
		r.Header.Set("Sec-Ch-Ua-Platform", `"Linux"`)
	}

	// 4. Set Sec-Fetch Metadata
	if r.Header.Get("Sec-Fetch-Dest") == "" {
		if isImageURL(r.URL.Path) {
			r.Header.Set("Sec-Fetch-Dest", "image")
			r.Header.Set("Sec-Fetch-Mode", "no-cors")
			r.Header.Set("Sec-Fetch-Site", "cross-site")
		} else {
			r.Header.Set("Sec-Fetch-Dest", "document")
			r.Header.Set("Sec-Fetch-Mode", "navigate")
			r.Header.Set("Sec-Fetch-Site", "none")
			r.Header.Set("Sec-Fetch-User", "?1")
		}
	}

	if r.Header.Get("Upgrade-Insecure-Requests") == "" {
		r.Header.Set("Upgrade-Insecure-Requests", "1")
	}

	// 5. Default Referer matching target domain (defeats hotlink / origin checks)
	if r.Header.Get("Referer") == "" && r.URL != nil && r.URL.Host != "" {
		r.Header.Set("Referer", "https://"+r.URL.Host+"/")
	}

	base := s.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(r)
}

func isImageURL(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".jpg") ||
		strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".png") ||
		strings.HasSuffix(lower, ".webp") ||
		strings.HasSuffix(lower, ".gif")
}

// NewStealthClient returns an HTTP client configured with cookie persistence and stealth headers.
func NewStealthClient(timeout time.Duration) *http.Client {
	jar, _ := cookiejar.New(nil)
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 120
	transport.MaxIdleConnsPerHost = 25
	transport.IdleConnTimeout = 90 * time.Second
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.TLSHandshakeTimeout = 10 * time.Second

	return &http.Client{
		Timeout: timeout,
		Jar:     jar,
		Transport: &StealthTransport{
			Base: transport,
		},
	}
}
