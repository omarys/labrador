# Labrador Go Architecture & Scraping Resources

## Knowledge

- [Go Standard Library: `net/http` & `net/http/httptest`](https://pkg.go.dev/net/http/httptest)
  Primary reference for HTTP clients, transports, round-trippers, and mock test servers in Go. Use for: HTTP mocking and network seams.
- [Go Concurrency Patterns: Pipelines and cancellation (Go Blog)](https://go.dev/blog/pipelines)
  Foundational article on channels, worker pools, and context cancellation. Use for: Downloader worker pools and Queue scheduling.
- [ComicRack ComicInfo.xml Schema Specification](https://anansi-project.github.io/docs/comicinfo/documentation)
  Standard XML metadata specification for `.cbz` comic archives. Use for: Generating `ComicInfo.xml` metadata in archives.
- [Charm CLI: Bubble Tea (Elm architecture for Go)](https://github.com/charmbracelet/bubbletea)
  Official framework for terminal user interfaces in Go. Use for: Labrador's interactive TUI mode.
- [PuerkitoBio/goquery (HTML manipulation for Go)](https://github.com/PuerkitoBio/goquery)
  jQuery-like CSS selector engine in Go. Use for: HTML-based webcomic scraping.

## Wisdom (Communities)

- [Gophers Slack (#networking, #concurrency)](https://gophers.slack.com)
  High-signal community for idiomatic Go architecture discussions.
- [r/golang](https://reddit.com/r/golang)
  Active subreddit for Go design patterns, testing, and ecosystem questions.
