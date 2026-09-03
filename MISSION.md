# Mission: Build Labrador Webcomic Scraper & Engine in Go

## Why
Build Labrador from the ground up as a fast, maintainable, and robust Go webcomic scraper and `.cbz` archiver to replace Mangal and integrate headlessly with the Dewey library manager and Continuum reader, mastering idiomatic Go architecture, testing seams, polite concurrency queues, and terminal UI design.

## Success looks like
- Design and implement a pluggable pure Go `Provider` interface with declarative `Capabilities` and URL routing.
- Implement concrete public providers (MangaDex, MangaKatana, MangaNato, MangaNelo, MangaPill, ReadComicsOnline, WeebCentral) and an isolated private provider seam (`internal/providers/private/`).
- Build a polite `Queue` scheduler that serializes chapter downloads per series/provider while running multiple providers in parallel.
- Assemble `.cbz` archives with validated `ComicInfo.xml` metadata.
- Implement headless subcommands (`search`, `fetch`, `browse`) with structured JSON output for Dewey.
- Build an interactive Bubble Tea TUI with multi-provider search, single-provider tag browsing, and vim navigation.
- Write high-seam integration tests with HTTP fixtures (`httptest.Server`).

## Constraints
- Pure Go implementation (no external scripting runtimes or headless browser dependencies).
- Strict separation and gitignoring of private providers.
- Focus on clean architecture and high-seam testing over dogmatic slow unit mocking.

## Out of scope
- In-app GUI comic reading (delegated to Continuum).
- SQLite library state management and reading tracking (delegated to Dewey).
