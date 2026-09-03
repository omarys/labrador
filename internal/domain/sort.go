package domain

// SortOrder defines the ordering criteria applied when browsing a Provider's catalog.
type SortOrder string

const (
	SortPopular      SortOrder = "popular"
	SortRecent       SortOrder = "recent"
	SortAlphabetical SortOrder = "alphabetical"
	SortRating       SortOrder = "rating"
)
