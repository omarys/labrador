package httpclient

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"testing"
)

type dialerFunc func(
	context.Context,
	string,
	string,
) (net.Conn, error)

func (dial dialerFunc) DialContext(
	ctx context.Context,
	network string,
	address string,
) (net.Conn, error) {
	return dial(ctx, network, address)
}

func TestPublicDialerDoesNotDialPrivateResolution(t *testing.T) {
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
	dialer := publicDialer{
		resolver: stubResolver{
			addresses: []netip.Addr{netip.MustParseAddr("10.0.0.1")},
		},
		dialer: underlying,
	}

	_, err := dialer.DialContext(
		context.Background(),
		"tcp",
		"example.com:443",
	)
	if err == nil {
		t.Fatal("DialContext() accepted a private DNS result")
	}
	if dialCalled {
		t.Fatal("DialContext() called the underlying dialer")
	}
}

func TestPublicDialerDialsValidatedIPAddress(t *testing.T) {
	t.Parallel()

	dialErr := errors.New("stop after recording dial")
	var gotNetwork string
	var gotAddress string

	underlying := dialerFunc(func(
		_ context.Context,
		network string,
		address string,
	) (net.Conn, error) {
		gotNetwork = network
		gotAddress = address

		return nil, dialErr
	})

	dialer := publicDialer{
		resolver: stubResolver{
			addresses: []netip.Addr{
				netip.MustParseAddr("1.1.1.1"),
			},
		},
		dialer: underlying,
	}

	_, err := dialer.DialContext(
		context.Background(),
		"tcp",
		"example.com:443",
	)
	if !errors.Is(err, dialErr) {
		t.Fatalf("DialContext() error = %v, want wrapped %v", err, dialErr)
	}
	if gotNetwork != "tcp" {
		t.Errorf("underlying network = %q, want %q", gotNetwork, "tcp")
	}
	if gotAddress != "1.1.1.1:443" {
		t.Errorf(
			"underlying address = %q, want %q",
			gotAddress,
			"1.1.1.1:443",
		)
	}
}

func TestPublicDialerTriesEachValidatedAddress(t *testing.T) {
	t.Parallel()

	dialErr := errors.New("dial failed")
	var attempts []string

	underlying := dialerFunc(func(
		_ context.Context,
		_ string,
		address string,
	) (net.Conn, error) {
		attempts = append(attempts, address)

		return nil, dialErr
	})

	dialer := publicDialer{
		resolver: stubResolver{
			addresses: []netip.Addr{
				netip.MustParseAddr("1.1.1.1"),
				netip.MustParseAddr("2606:4700:4700::1111"),
			},
		},
		dialer: underlying,
	}

	_, err := dialer.DialContext(
		context.Background(),
		"tcp",
		"example.com:443",
	)
	if !errors.Is(err, dialErr) {
		t.Fatalf("DialContext() error = %v, want wrapped %v", err, dialErr)
	}

	wantAttempts := []string{
		"1.1.1.1:443",
		"[2606:4700:4700::1111]:443",
	}
	if !slices.Equal(attempts, wantAttempts) {
		t.Errorf("dial attempts = %v, want %v", attempts, wantAttempts)
	}
}

func TestPublicDialerStopsFallbackWhenContextIsCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	attempts := 0
	underlying := dialerFunc(func(
		_ context.Context,
		_ string,
		_ string,
	) (net.Conn, error) {
		attempts++
		cancel()

		return nil, errors.New("dial interrupted")
	})

	dialer := publicDialer{
		resolver: stubResolver{
			addresses: []netip.Addr{
				netip.MustParseAddr("1.1.1.1"),
				netip.MustParseAddr("2606:4700:4700::1111"),
			},
		},
		dialer: underlying,
	}

	_, err := dialer.DialContext(
		ctx,
		"tcp",
		"example.com:443",
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DialContext() error = %v, want context.Canceled", err)
	}
	if attempts != 1 {
		t.Errorf("dial attempts = %d, want 1", attempts)
	}
}

func TestPublicDialerStopsAfterSuccessfulConnection(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	attempts := 0
	underlying := dialerFunc(func(
		_ context.Context,
		_ string,
		_ string,
	) (net.Conn, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("first address failed")
		}

		return clientConn, nil
	})

	dialer := publicDialer{
		resolver: stubResolver{
			addresses: []netip.Addr{
				netip.MustParseAddr("1.1.1.1"),
				netip.MustParseAddr("8.8.8.8"),
				netip.MustParseAddr("9.9.9.9"),
			},
		},
		dialer: underlying,
	}

	got, err := dialer.DialContext(
		context.Background(),
		"tcp",
		"example.com:443",
	)
	if err != nil {
		t.Fatalf("DialContext() returned an unexpected error: %v", err)
	}
	if got != clientConn {
		t.Error("DialContext() returned the wrong connection")
	}
	if attempts != 2 {
		t.Errorf("dial attempts = %d, want 2", attempts)
	}
}

func TestPublicDialerRejectsAddressWithoutPort(t *testing.T) {
	t.Parallel()

	dialer := publicDialer{}

	_, err := dialer.DialContext(
		context.Background(),
		"tcp",
		"example.com",
	)
	if err == nil {
		t.Fatal("DialContext() accepted an address without a port")
	}
}

func TestPublicDialerPassesContextToUnderlyingDialer(t *testing.T) {
	t.Parallel()

	type contextKey struct{}
	key := contextKey{}
	ctx := context.WithValue(context.Background(), key, "marker")

	dialErr := errors.New("dial stopped")
	underlying := dialerFunc(func(
		gotCtx context.Context,
		_ string,
		_ string,
	) (net.Conn, error) {
		if gotCtx.Value(key) != "marker" {
			t.Error("underlying dialer did not receive the caller's context")
		}

		return nil, dialErr
	})

	dialer := publicDialer{
		resolver: stubResolver{
			addresses: []netip.Addr{
				netip.MustParseAddr("1.1.1.1"),
			},
		},
		dialer: underlying,
	}

	_, err := dialer.DialContext(ctx, "tcp", "example.com:443")
	if !errors.Is(err, dialErr) {
		t.Fatalf("DialContext() error = %v, want wrapped %v", err, dialErr)
	}
}
