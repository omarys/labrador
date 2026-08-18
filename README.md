# Labrador — Webcomic Scraper & CBZ Archiver

**Labrador** is a robust, high-performance webcomic scraper and `.cbz` archive generator written in Go. It is designed to discover, extract metadata for, and download chapters from webcomic sources, bundling them into clean comic book archive (`.cbz`) files.

Labrador forms the data retrieval engine in a unified local comic and manhwa management ecosystem alongside **[Continuum](file:///home/omary/Dev/continuum)** and **[Dewey](file:///home/omary/Dev/dewey)**.

---

## The Ecosystem

```
┌─────────────────────────────────────────────────────────────┐
│                           Dewey                             │
│             (TUI Manga & Manhwa Library Manager)            │
└──────────────┬───────────────────────────────┬──────────────┘
               │ Orchestrates                  │ Launches
               ▼                               ▼
┌─────────────────────────────┐ ┌─────────────────────────────┐
│          Labrador           │ │          Continuum          │
│   (Go Scraper & Packager)   │ │    (GTK4 Manhwa Reader)     │
│                             │ │                             │
│ • Fetches metadata & pages  │ │ • Vertical continuous scroll│
│ • Builds .cbz archives      │ │ • Zero-layout-shift cache   │
└──────────────┬──────────────┘ └──────────────▲──────────────┘
               │                               │
               └─────────── Reads ─────────────┘
                          (.cbz)
```

- **[Labrador](file:///home/omary/Dev/labrador)**: Discovers series, extracts chapter metadata, safely downloads page assets, and packages chapters into standard `.cbz` archives.
- **[Dewey](file:///home/omary/Dev/dewey)**: Terminal user interface (TUI) library manager orchestrating comic libraries, tracking updates, and driving downloads via Labrador.
- **[Continuum](file:///home/omary/Dev/continuum)**: Modern GTK4 / Libadwaita reader optimized for seamless vertical continuous scrolling of `.cbz` archives.

---

## Core Capabilities & Architecture

- **Domain Model**: Strongly typed representations of `Series`, `Chapter`, and `Page` domain entities, treating all remote provider input as untrusted.
- **Hardened HTTP Client**:
  - **SSRF & Address Policy Enforcement**: Restricts and validates resolved IP addresses to prevent local network probing and internal address leakage.
  - **Custom Safe Resolver**: Pre-resolves and validates DNS responses before dialing.
  - **Granular Timeout Controls**: Dedicated deadlines for metadata queries, page downloads, TLS handshakes, response headers, and idle connection reuse.
  - **Redirect Limiting**: Strict redirect caps and validation on every hop.
- **CBZ Packaging**: Bundles comic pages in correct sequence with zero data corruption and clean metadata.
- **Modular Provider System**: Pluggable scraping interfaces for targeting different webcomic sources and platforms.

---

## Development Setup

Environment tools are managed via [`mise`](https://mise.jdx.dev/) and pre-commit hooks via [`pre-commit`](https://pre-commit.com/).

### Prerequisites

- Go `1.26+`
- `mise` (recommended for automatic tool version management)

### Setup

```bash
# Install tools defined in mise.toml (Go, golangci-lint, pre-commit)
mise install

# Install git pre-commit hooks
pre-commit install
```

### Running Tests & Linting

```bash
# Run unit tests
go test ./...

# Run tests with race detector and coverage
go test -race -cover ./...

# Run linter
golangci-lint run
```

---

## License

MIT
