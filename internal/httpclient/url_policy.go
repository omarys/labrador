package httpclient

import (
	"errors"
	"fmt"
	"net"
	"net/url"
)

// URLPolicy validates provider-supplied request URLs.
type URLPolicy struct {
	AllowHTTP bool
}

// Validate verifies that rawURL is an absolute request URL allowed by the policy.
func (policy URLPolicy) Validate(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse request URL: %w", err)
	}

	schemeAllowed := parsedURL.Scheme == "https" || (policy.AllowHTTP && parsedURL.Scheme == "http")
	if !schemeAllowed {
		return fmt.Errorf("url scheme %q is not allowed", parsedURL.Scheme)
	}

	if parsedURL.User != nil {
		return errors.New("url must not contain embedded credentials")
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return errors.New("url must have a hostname")
	}

	if net.ParseIP(hostname) != nil {
		return errors.New("url hostname must not be an ip address")
	}

	expectedPort := "443"
	if parsedURL.Scheme == "http" {
		expectedPort = "80"
	}

	port := parsedURL.Port()
	if port != "" && port != expectedPort {
		return fmt.Errorf(
			"url port %q is not allowed for %s",
			port,
			parsedURL.Scheme,
		)
	}

	if parsedURL.Fragment != "" {
		return errors.New("url must not contain a fragment")
	}

	return nil
}
