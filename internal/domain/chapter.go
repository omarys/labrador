package domain

// Chapter describes a remotely hosted chapter discovered by a provider.
//
// All provider-supplied identifiers, labels, URLs, and headers are untrusted
// and must be validated before they reach the network or filesystem.
type Chapter struct {
	ID            string              `json:"id"`
	SeriesID      string              `json:"series_id"`
	Title         string              `json:"title"`
	URL           string              `json:"url"`
	OriginalLabel string              `json:"original_label"`
	Number        *float64            `json:"number,omitempty"`
	Index         int                 `json:"index"`
	Headers       map[string][]string `json:"headers,omitempty"`
}
