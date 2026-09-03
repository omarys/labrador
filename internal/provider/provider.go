package provider

import (
	"context"

	"github.com/omarys/labrador/internal/domain"
)

// Capabilities declares what features and filters a Provider supports.
type Capabilities struct {
	CanSearch      bool               `json:"can_search"`
	CanBrowse      bool               `json:"can_browse"`
	SupportedSorts []domain.SortOrder `json:"supported_sorts,omitempty"`
	HasTags        bool               `json:"has_tags"`
}

// BrowseOptions defines parameters for browsing a Provider catalog.
type BrowseOptions struct {
	Tag  *domain.Tag      `json:"tag,omitempty"`
	Sort domain.SortOrder `json:"sort,omitempty"`
	Page int              `json:"page,omitempty"`
}

// Provider represents a webcomic source implementation.
type Provider interface {
	// ID returns the unique identifier for this provider (e.g. "mangadex").
	ID() string

	// Name returns the human-readable name of the provider (e.g. "MangaDex").
	Name() string

	// Capabilities returns the feature manifest for this provider.
	Capabilities() Capabilities

	// MatchesURL reports whether this provider handles the given webcomic URL.
	MatchesURL(rawURL string) bool

	// Search queries the provider for series matching the search string.
	Search(ctx context.Context, query string) ([]domain.Series, error)

	// Browse lists series from the provider according to the given filter and sort options.
	Browse(ctx context.Context, opts BrowseOptions) ([]domain.Series, error)

	// GetTags returns the list of available genres/tags for filtering.
	GetTags(ctx context.Context) ([]domain.Tag, error)

	// GetChapters returns the ordered list of chapters for a given series.
	GetChapters(ctx context.Context, series domain.Series) ([]domain.Chapter, error)

	// GetPages returns the ordered list of image pages for a given chapter.
	GetPages(ctx context.Context, chapter domain.Chapter) ([]domain.Page, error)
}
