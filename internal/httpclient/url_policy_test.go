package httpclient

import "testing"

func TestURLPolicyValidateAcceptsHTTPS(t *testing.T) {
	t.Parallel()

	policy := URLPolicy{}

	err := policy.Validate("https://example.com/chapter/1")
	if err != nil {
		t.Fatalf("Validate() returned an unexpected error: %v", err)
	}
}

func TestURLPolicyValidateRejectsMissingHostname(t *testing.T) {
	t.Parallel()

	policy := URLPolicy{}

	err := policy.Validate("https:///chapter/1")
	if err == nil {
		t.Fatal("Validate() accepted a URL without a hostname")
	}
}

func TestURLPolicyValidateRejectsPortWithoutHostname(t *testing.T) {
	t.Parallel()

	policy := URLPolicy{}

	err := policy.Validate("https://:8080/chapter/1")
	if err == nil {
		t.Fatal("Validate() accepted a URL with a port but no hostname")
	}
}

func TestURLPolicyValidateRejectsEmbeddedCredentials(t *testing.T) {
	t.Parallel()

	policy := URLPolicy{}

	err := policy.Validate("https://user:password@example.com/chapter/1")
	if err == nil {
		t.Fatal("Validate() accepted a URL with embedded credentials")
	}
}

func TestURLPolicyValidateRejectsHTTPByDefault(t *testing.T) {
	t.Parallel()

	policy := URLPolicy{}

	err := policy.Validate("http://example.com/chapter/1")
	if err == nil {
		t.Fatal("Validate() accepted HTTP without explicit permission")
	}
}

func TestURLPolicyValidateAcceptsHTTPWhenAllowed(t *testing.T) {
	t.Parallel()

	policy := URLPolicy{AllowHTTP: true}

	err := policy.Validate("http://example.com/chapter/1")
	if err != nil {
		t.Fatalf("Validate() rejected explicitly allowed HTTP: %v", err)
	}
}

func TestURLPolicyValidateRejectsNonstandardHTTPSPort(t *testing.T) {
	t.Parallel()

	policy := URLPolicy{}

	err := policy.Validate("https://example.com:8443/chapter/1")
	if err == nil {
		t.Fatalf("Validate() accepted a nonstandard HTTPS port")
	}
}

func TestURLPolicyValidateRejectsNonstandardHTTPPort(t *testing.T) {
	t.Parallel()

	policy := URLPolicy{AllowHTTP: true}

	err := policy.Validate("http://example.com:8080/chapter/1")
	if err == nil {
		t.Fatalf("Validate() accepted a nonstandard HTTP port")
	}
}

func TestURLPolicyValidateRejectsIPLiteral(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
	}{
		{
			name:   "IPv4",
			rawURL: "https://1.1.1.1/chapter/1",
		},
		{
			name:   "IPv6",
			rawURL: "https://[2001:db8::1]/chapter/1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			policy := URLPolicy{}

			err := policy.Validate(test.rawURL)
			if err == nil {
				t.Fatalf("Validate() accepted IP-literal URL %q", test.rawURL)
			}
		})
	}
}

func TestURLPolicyValidateAcceptsQueryString(t *testing.T) {
	t.Parallel()

	policy := URLPolicy{}

	err := policy.Validate("https://example.com/chapter/1?quality=high")
	if err != nil {
		t.Fatalf("Validate() rejected a URL containing a query string: %v", err)
	}
}

func TestURLPolicyValidateRejectsFragment(t *testing.T) {
	t.Parallel()

	policy := URLPolicy{}

	err := policy.Validate("https://example.com/chapter/1#page-2")
	if err == nil {
		t.Fatal("Validate() accepted a URL containing a fragment")
	}
}
