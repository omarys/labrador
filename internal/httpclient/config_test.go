package httpclient

import (
	"testing"
	"time"
)

func TestConfigWithDefaults(t *testing.T) {
	t.Parallel()

	got := (Config{}).withDefaults()
	want := Config{
		MetadataTimeout:       30 * time.Second,
		DownloadTimeout:       2 * time.Minute,
		DialTimeout:           10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxRedirects:          10,
	}

	if got != want {
		t.Errorf("Config{}.withDefaults() = %+v, want %+v", got, want)
	}
}

func TestConfigValidateRejectsNegativeValues(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name:   "metadata timeout",
			config: Config{MetadataTimeout: -time.Second},
		},
		{
			name:   "download timeout",
			config: Config{DownloadTimeout: -time.Second},
		},
		{
			name:   "dial timeout",
			config: Config{DialTimeout: -time.Second},
		},
		{
			name:   "tls handshake timeout",
			config: Config{TLSHandshakeTimeout: -time.Second},
		},
		{
			name:   "response header timeout",
			config: Config{ResponseHeaderTimeout: -time.Second},
		},
		{
			name:   "idle connection timeout",
			config: Config{IdleConnTimeout: -time.Second},
		},
		{
			name:   "maximum redirects",
			config: Config{MaxRedirects: -1},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if err := test.config.validate(); err == nil {
				t.Fatal("Config.validate() accepted a negative value")
			}
		})
	}
}

func TestConfigValidateRejectsTooManyRedirects(t *testing.T) {
	t.Parallel()

	config := Config{
		MaxRedirects: defaultMaxRedirects + 1,
	}

	if err := config.validate(); err == nil {
		t.Fatal("Config.validate() accepted more than ten redirects")
	}
}

func TestConfigWithDefaultsPreservesOverrides(t *testing.T) {
	t.Parallel()

	config := Config{
		MetadataTimeout:       11 * time.Second,
		DownloadTimeout:       12 * time.Second,
		DialTimeout:           13 * time.Second,
		TLSHandshakeTimeout:   14 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout:       16 * time.Second,
		MaxRedirects:          3,
	}

	got := config.withDefaults()
	if got != config {
		t.Errorf("Config.withDefaults() = %+v, want unchanged %+v", got, config)
	}
}
