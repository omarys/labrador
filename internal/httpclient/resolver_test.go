package httpclient

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"
)

type stubResolver struct {
	addresses []netip.Addr
	err       error
}

type resolverFunc func(
	context.Context,
	string,
	string,
) ([]netip.Addr, error)

func (resolve resolverFunc) LookupNetIP(
	ctx context.Context,
	network string,
	host string,
) ([]netip.Addr, error) {
	return resolve(ctx, network, host)
}

func (resolver stubResolver) LookupNetIP(
	_ context.Context,
	_ string,
	_ string,
) ([]netip.Addr, error) {
	return resolver.addresses, resolver.err
}

func TestResolvePublicIPsRejectPrivateAddress(t *testing.T) {
	t.Parallel()

	resolver := stubResolver{
		addresses: []netip.Addr{
			netip.MustParseAddr("10.0.0.1"),
		},
	}

	_, err := resolvePublicIPs(
		context.Background(),
		resolver,
		"example.com",
	)
	if err == nil {
		t.Fatal("resolvePublicIPs() accepted a private address")
	}
}

func TestResolvePublicIPsReturnsValidatedAddresses(t *testing.T) {
	t.Parallel()

	want := []netip.Addr{
		netip.MustParseAddr("1.1.1.1"),
		netip.MustParseAddr("2606:4700:4700::1111"),
	}
	resolver := stubResolver{addresses: want}

	got, err := resolvePublicIPs(
		context.Background(),
		resolver,
		"example.com",
	)
	if err != nil {
		t.Fatalf("resolvePublicIPs() returned an unexpected error: %v", err)
	}

	if !slices.Equal(got, want) {
		t.Errorf("resolvePublicIPs() = %v, want %v", got, want)
	}
}

func TestResolvePublicIPsPreservesResolverError(t *testing.T) {
	t.Parallel()

	resolverErr := errors.New("resolver failed")
	resolver := stubResolver{err: resolverErr}

	_, err := resolvePublicIPs(
		context.Background(),
		resolver,
		"example.com",
	)
	if !errors.Is(err, resolverErr) {
		t.Fatalf("resolvePublicIPs() error = %v, want wrapped %v",
			err,
			resolverErr,
		)
	}
}

func TestResolvePublicIPsPassesLookupArguments(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resolver := resolverFunc(func(
		gotCtx context.Context,
		network string,
		host string,
	) ([]netip.Addr, error) {
		if !errors.Is(gotCtx.Err(), context.Canceled) {
			t.Errorf("resolver context error = %v, want context.Canceled", gotCtx.Err())
		}
		if network != "ip" {
			t.Errorf("resolver network = %q, want %q", network, "ip")
		}
		if host != "example.com" {
			t.Errorf("resolver host = %q, want %q", host, "example.com")
		}

		return nil, gotCtx.Err()
	})

	_, err := resolvePublicIPs(ctx, resolver, "example.com")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("resolvePublicIPs() error = %v, want context.Canceled", err)
	}
}
