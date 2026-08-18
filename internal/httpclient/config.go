package httpclient

import (
	"errors"
	"fmt"
	"time"
)

const (
	defaultMetadataTimeout       = 30 * time.Second
	defaultDownloadTimeout       = 2 * time.Minute
	defaultDialTimeout           = 10 * time.Second
	defaultTLSHandshakeTimeout   = 10 * time.Second
	defaultResponseHeaderTimeout = 15 * time.Second
	defaultIdleConnTimeout       = 90 * time.Second
	defaultMaxRedirects          = 10
)

// Config controls HTTP deadlines and redirect limits.
type Config struct {
	MetadataTimeout       time.Duration
	DownloadTimeout       time.Duration
	DialTimeout           time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	IdleConnTimeout       time.Duration
	MaxRedirects          int
}

func (config Config) withDefaults() Config {
	if config.MetadataTimeout == 0 {
		config.MetadataTimeout = defaultMetadataTimeout
	}
	if config.DownloadTimeout == 0 {
		config.DownloadTimeout = defaultDownloadTimeout
	}
	if config.DialTimeout == 0 {
		config.DialTimeout = defaultDialTimeout
	}
	if config.TLSHandshakeTimeout == 0 {
		config.TLSHandshakeTimeout = defaultTLSHandshakeTimeout
	}
	if config.ResponseHeaderTimeout == 0 {
		config.ResponseHeaderTimeout = defaultResponseHeaderTimeout
	}
	if config.IdleConnTimeout == 0 {
		config.IdleConnTimeout = defaultIdleConnTimeout
	}
	if config.MaxRedirects == 0 {
		config.MaxRedirects = defaultMaxRedirects
	}
	return config
}

func (config Config) validate() error {
	durations := []struct {
		name  string
		value time.Duration
	}{
		{name: "metadata timeout", value: config.MetadataTimeout},
		{name: "download timeout", value: config.DownloadTimeout},
		{name: "dial timeout", value: config.DialTimeout},
		{name: "tls handshake timeout", value: config.TLSHandshakeTimeout},
		{name: "response header timeout", value: config.ResponseHeaderTimeout},
		{name: "idle connection timeout", value: config.IdleConnTimeout},
	}

	for _, duration := range durations {
		if duration.value < 0 {
			return fmt.Errorf("%s must not be negative", duration.name)
		}
	}

	if config.MaxRedirects < 0 {
		return errors.New("maximum redirects must not be negative")
	}

	if config.MaxRedirects > defaultMaxRedirects {
		return fmt.Errorf("maximum redirects must not exceed %d", defaultMaxRedirects)
	}

	return nil
}
