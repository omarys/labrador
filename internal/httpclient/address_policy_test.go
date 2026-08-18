package httpclient

import (
	"net/netip"
	"testing"
)

func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{
			name:    "Public IPv4",
			address: "1.1.1.1",
			want:    true,
		},
		{
			name:    "Private IPv4",
			address: "10.0.0.1",
			want:    false,
		},
		{
			name:    "CGNAT IPv4",
			address: "100.64.0.1",
			want:    false,
		},
		{
			name:    "Loopback IPv4",
			address: "127.0.0.1",
			want:    false,
		},
		{
			name:    "Link-local IPv4",
			address: "169.254.1.1",
			want:    false,
		},
		{
			name:    "Documentation IPv4 TEST-NET-1",
			address: "192.0.2.1",
			want:    false,
		},
		{
			name:    "Documentation IPv4 TEST-NET-2",
			address: "198.51.100.1",
			want:    false,
		},
		{
			name:    "Documentation IPv4 TEST-NET-3",
			address: "203.0.113.1",
			want:    false,
		},
		{
			name:    "Benchmark IPv4",
			address: "198.18.0.1",
			want:    false,
		},
		{
			name:    "Public IPv6",
			address: "2606:4700:4700::1111",
			want:    true,
		},
		{
			name:    "Loopback IPv6",
			address: "::1",
			want:    false,
		},
		{
			name:    "Unique-local IPv6",
			address: "fc00::1",
			want:    false,
		},
		{
			name:    "Link-local IPv6",
			address: "fe80::1",
			want:    false,
		},
		{
			name:    "Documentation IPv6",
			address: "2001:db8::1",
			want:    false,
		},
		{
			name:    "Multicast IPv6",
			address: "ff02::1",
			want:    false,
		},
		{
			name:    "Mapped private IPv6",
			address: "::ffff:10.0.0.1",
			want:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			address := netip.MustParseAddr(test.address)
			got := isPublicIP(address)

			if got != test.want {
				t.Errorf("isPublicIP(%q) = %v, want %v", test.address, got, test.want)
			}
		})
	}
}

func TestValidateResolvedIPs(t *testing.T) {
	tests := []struct {
		name      string
		addresses []netip.Addr
		wantErr   bool
	}{
		{
			name: "All public",
			addresses: []netip.Addr{
				netip.MustParseAddr("1.1.1.1"),
				netip.MustParseAddr("2606:4700:4700::1111"),
			},
			wantErr: false,
		},
		{
			name: "Mixed public and private",
			addresses: []netip.Addr{
				netip.MustParseAddr("1.1.1.1"),
				netip.MustParseAddr("10.0.0.1"),
			},
			wantErr: true,
		},
		{
			name: "All private",
			addresses: []netip.Addr{
				netip.MustParseAddr("10.0.0.1"),
				netip.MustParseAddr("192.168.1.1"),
			},
			wantErr: true,
		},
		{
			name:      "Empty result",
			addresses: nil,
			wantErr:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateResolvedIPs(test.addresses)
			if (err != nil) != test.wantErr {
				t.Errorf("validateResolvedIPs() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
