package domain

// MediaType distinguishes sources whose chapters are page images (manga,
// webtoons, comics) from sources whose chapters are prose (light novels).
//
// The zero value is MediaImage so providers that do not declare a media type
// keep the image pipeline.
type MediaType string

const (
	// MediaImage chapters are ordered page images archived into a .cbz.
	MediaImage MediaType = "image"
	// MediaText chapters are prose archived into a Markdown .md file.
	MediaText MediaType = "text"
)
