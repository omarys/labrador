// Package domain defines the scraper's transport-neutral core models.
package domain

// Page describes a remotely hosted page image discovered by a provider.
//
// Index is zero-based. URL, Headers, and ExtensionHint are untrusted provider
// input and must be validated before they reach the network or filesystem.
type Page struct {
	URL           string              `json:"url"`
	Index         int                 `json:"index"`
	Headers       map[string][]string `json:"headers,omitempty"`
	ExtensionHint string              `json:"extension_hint,omitempty"`
}
