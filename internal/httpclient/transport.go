package httpclient

import (
	"fmt"
	"net/http"
)

func newTransport(
	config Config,
	resolver ipResolver,
	dialer contextDialer,
) (*http.Transport, error) {
	config = config.withDefaults()

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("validate HTTP transport config: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()

	secureDialer := publicDialer{
		resolver: resolver,
		dialer:   dialer,
	}

	transport.Proxy = nil
	transport.DialContext = secureDialer.DialContext
	transport.TLSHandshakeTimeout = config.TLSHandshakeTimeout
	transport.ResponseHeaderTimeout = config.ResponseHeaderTimeout
	transport.IdleConnTimeout = config.IdleConnTimeout
	transport.ForceAttemptHTTP2 = true

	return transport, nil
}
