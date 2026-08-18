package httpclient

import (
	"errors"
	"net/netip"
)

var nonPublicPrefixes = [...]netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),   // Shared address space.
	netip.MustParsePrefix("192.0.2.0/24"),    // TEST-NET-1.
	netip.MustParsePrefix("198.51.100.0/24"), // TEST-NET-2.
	netip.MustParsePrefix("203.0.113.0/24"),  // TEST-NET-3.
	netip.MustParsePrefix("198.18.0.0/15"),   // Benchmarking.
	netip.MustParsePrefix("2001:db8::/32"),   // IPv6 documentation.
}

func isPublicIP(address netip.Addr) bool {
	address = address.Unmap()

	if !address.IsValid() ||
		!address.IsGlobalUnicast() ||
		address.IsPrivate() {
		return false
	}

	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}

	return true
}

func validateResolvedIPs(addresses []netip.Addr) error {
	if len(addresses) == 0 {
		return errors.New("hostname resolved to no IP addresses")
	}

	for _, address := range addresses {
		if !isPublicIP(address) {
			return errors.New("hostname resolved to a non-public IP address")
		}
	}
	return nil
}
