package domain

// Tag describes a genre, theme, or demographic category used to filter Series within a Provider.
//
// All provider-supplied identifiers and labels are untrusted input.
type Tag struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
