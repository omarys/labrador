package httpclient

import (
	"context"
	"fmt"
	"net"
)

type contextDialer interface {
	DialContext(
		ctx context.Context,
		network string,
		address string,
	) (net.Conn, error)
}

type publicDialer struct {
	resolver ipResolver
	dialer   contextDialer
}

func (dialer publicDialer) DialContext(
	ctx context.Context,
	network string,
	address string,
) (net.Conn, error) {
	hostname, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("split dial address: %w", err)
	}

	addresses, err := resolvePublicIPs(ctx, dialer.resolver, hostname)
	if err != nil {
		return nil, fmt.Errorf("resolve dial hostname: %w", err)
	}

	var lastErr error

	for _, resolvedAddress := range addresses {
		target := net.JoinHostPort(resolvedAddress.String(), port)

		connection, err := dialer.dialer.DialContext(ctx, network, target)
		if err == nil {
			return connection, nil
		}

		lastErr = err

		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("dial validated address: %w", ctxErr)
		}
	}

	return nil, fmt.Errorf("dial validated addresses: %w", lastErr)
}
