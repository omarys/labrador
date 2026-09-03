package archive

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/omarys/labrador/internal/domain"
)

// PageData contains the raw bytes and image extension for a downloaded page.
type PageData struct {
	Index     int
	Extension string // e.g. ".jpg", ".png", ".webp"
	Data      []byte
}

func WriteCBZ(targetPath string, series domain.Series, chapter domain.Chapter, pages []PageData) error {
	if len(pages) == 0 {
		// WriteCBZ writes a complete .cbz archive atomically to the target destination.
		return fmt.Errorf("cannot create empty cbz archive")
	}

	// Ensure destination directory exists
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	// Write to temporary file first for atomicity
	tmpPath := targetPath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}

	// Clean up tmp file if error occurs before rename
	success := false
	defer func() {
		_ = tmpFile.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	zipWriter := zip.NewWriter(tmpFile)

	// 1. Write ComicInfo.xml
	comicInfoData, err := GenerateComicInfo(series, chapter, len(pages))
	if err != nil {
		return fmt.Errorf("generating comicinfo: %w", err)
	}

	xmlEntry, err := zipWriter.Create("ComicInfo.xml")
	if err != nil {
		return fmt.Errorf("creating ComicInfo.xml entry: %w", err)
	}
	if _, err := xmlEntry.Write(comicInfoData); err != nil {
		return fmt.Errorf("writing ComicInfo.xml: %w", err)
	}

	// 2. Write each page image with 3-digit zero-padding (e.g. 000.jpg, 001.png)
	for _, page := range pages {
		ext := page.Extension
		if ext == "" {
			ext = ".jpg" // Default to .jpg if extension is missing
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}

		entryName := fmt.Sprintf("%03d%s", page.Index, ext)
		header := &zip.FileHeader{
			Name:   entryName,
			Method: zip.Store,
		}
		entryWriter, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("creating entry for %s: %w", entryName, err)
		}

		if _, err := entryWriter.Write(page.Data); err != nil {
			return fmt.Errorf("writing page %d: %w", page.Index, err)
		}
	}

	// Flush and close the zip stream
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("closing zip writer: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temporary file: %w", err)
	}

	// Atomic rename to final path
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("moving temp archive to final path: %w", err)
	}

	success = true
	return nil
}
