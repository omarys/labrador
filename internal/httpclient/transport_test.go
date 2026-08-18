package httpclient

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestNewTransportAppliesSecureDefaults(t *testing.T) {
	t.Parallel()

	underlying := dialerFunc(func(
		_ context.Context,
		_ string,
		_ string,
	) (net.Conn, error) {
		return nil, errors.New("unexpected dial")
	})

	transport, err := newTransport(
		Config{},
		stubResolver{},
		underlying,
	)
	if err != nil {
		t.Fatalf("newTransport() returned an unexpected error: %v", err)
	}

	if transport.TLSHandshakeTimeout != defaultTLSHandshakeTimeout {
		t.Errorf("TLS handshake timeout = %v, want %v", transport.TLSHandshakeTimeout, defaultTLSHandshakeTimeout)
	}
	if transport.ResponseHeaderTimeout != defaultResponseHeaderTimeout {
		t.Errorf("response header timeout = %v, want %v", transport.ResponseHeaderTimeout, defaultResponseHeaderTimeout)
	}
	if transport.IdleConnTimeout != defaultIdleConnTimeout {
		t.Errorf("idle connection timeout = %v, want %v", transport.IdleConnTimeout, defaultIdleConnTimeout)
	}
	if transport.Proxy != nil {
		t.Error("transport unexpectedly permits environment proxies")
	}
	if transport.DialContext == nil {
		t.Error("transport has no secure dial function")
	}
	if !transport.ForceAttemptHTTP2 {
		t.Error("transport does not enable HTTP/2")
	}
}

func TestNewTransportRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	underlying := dialerFunc(func(
		_ context.Context,
		_ string,
		_ string,
	) (net.Conn, error) {
		return nil, errors.New("unexpected dial")
	})

	transport, err := newTransport(
		Config{DialTimeout: -time.Second},
		stubResolver{},
		underlying,
	)
	if err == nil {
		t.Error("newTransport() accepted negative dial timeout")
	}
	if transport != nil {
		t.Error("newTransport() returned a transport for invalid configuration")
	}
}

func TestNewTransportAppliesCustomTimeouts(t *testing.T) {
	t.Parallel()

	underlying := dialerFunc(func(
		_ context.Context,
		_ string,
		_ string,
	) (net.Conn, error) {
		return nil, errors.New("unexpected dial")
	})

	config := Config{
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 4 * time.Second,
		IdleConnTimeout:       5 * time.Second,
	}

	transport, err := newTransport(
		config,
		stubResolver{},
		underlying,
	)
	if err != nil {
		t.Fatalf("newTransport() returned an unexpected error: %v", err)
	}

	if transport.TLSHandshakeTimeout != config.TLSHandshakeTimeout {
		t.Errorf(
			"TLS handshake timeout = %v, want %v",
			transport.TLSHandshakeTimeout,
			config.TLSHandshakeTimeout,
		)
	}
	if transport.ResponseHeaderTimeout != config.ResponseHeaderTimeout {
		t.Errorf(
			"response header timeout = %v, want %v",
			transport.ResponseHeaderTimeout,
			config.ResponseHeaderTimeout,
		)
	}
	if transport.IdleConnTimeout != config.IdleConnTimeout {
		t.Errorf(
			"idle connection timeout = %v, want %v",
			transport.IdleConnTimeout,
			config.IdleConnTimeout,
		)
	}
}

func TestNewTransportUsesPublicDialer(t *testing.T) {
	t.Parallel()

	dialCalled := false

	underlying := dialerFunc(func(
		_ context.Context,
		_ string,
		_ string,
	) (net.Conn, error) {
		dialCalled = true
		return nil, errors.New("unexpected dial")
	})

	resolver := stubResolver{
		addresses: []netip.Addr{
			netip.MustParseAddr("10.0.0.1"),
		},
	}

	transport, err := newTransport(
		Config{},
		resolver,
		underlying,
	)
	if err != nil {
		t.Fatalf("newTransport() returned an unexpected error: %v", err)
	}

	_, err = transport.DialContext(
		context.Background(),
		"tcp",
		"example.com:443",
	)
	if err == nil {
		t.Error("transport accepted a private DNS result")
	}
	if dialCalled {
		t.Error("transport called the underlying dialer for a private DNS result")
	}
}
