package domain

// Series describes a remotely hosted series discovered by a provider.
//
// All provider-supplied identifiers, labels, URLs, and headers are untrusted
// and must be validated before they reach the network or filesystem.
type Series struct {
	ID      string              `json:"id"`
	Title   string              `json:"title"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers,omitempty"`
}
