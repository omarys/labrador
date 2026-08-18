package httpclient

import (
	"context"
	"fmt"
	"net/netip"
)

type ipResolver interface {
	LookupNetIP(
		ctx context.Context,
		network string,
		host string,
	) ([]netip.Addr, error)
}

func resolvePublicIPs(
	ctx context.Context,
	resolver ipResolver,
	hostname string,
) ([]netip.Addr, error) {
	addresses, err := resolver.LookupNetIP(ctx, "ip", hostname)
	if err != nil {
		return nil, fmt.Errorf("resolve hostname: %w", err)
	}

	if err := validateResolvedIPs(addresses); err != nil {
		return nil, fmt.Errorf("validate resolved IP addresses: %w", err)
	}

	return addresses, nil
}
