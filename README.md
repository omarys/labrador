# Labrador — Comic & Light-Novel Scraper

**Labrador** is a robust, high-performance scraper and chapter packager written in Go. It is designed to discover, extract metadata for, and download chapters from webcomic and light-novel sources, archiving each chapter into a clean file: ordered comic pages as a `.cbz`, prose as a Markdown (`.md`) document.

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
│ • Builds .cbz & .md files   │ │ • Zero-layout-shift cache   │
└──────────────┬──────────────┘ └──────────────▲──────────────┘
               │                               │
               └─────────── Reads ─────────────┘
                        (.cbz / .md)
```

- **[Labrador](file:///home/omary/Dev/labrador)**: Discovers series, extracts chapter metadata, safely downloads page assets or scrapes prose, and packages chapters into standard `.cbz` archives or Markdown files.
- **[Dewey](file:///home/omary/Dev/dewey)**: Terminal user interface (TUI) library manager orchestrating comic libraries, tracking updates, and driving downloads via Labrador.
- **[Continuum](file:///home/omary/Dev/continuum)**: Modern GTK4 / Libadwaita reader optimized for seamless vertical continuous scrolling of `.cbz` archives.

---

## Core Capabilities & Architecture

- **Domain Model**: Strongly typed representations of `Series`, `Chapter`, and `Page` domain entities, treating all remote provider input as untrusted. A declared `MediaType` distinguishes page-image chapters from prose.
- **Two Media Kinds**: Media kind is declared by the Provider, never inferred. Image chapters are an ordered list of Pages archived as `.cbz`; prose chapters are a single body archived as Markdown, with the chapter heading, inline emphasis, links and line breaks preserved, literal Markdown syntax escaped, and hidden anti-piracy notices dropped.
- **Hardened HTTP Client**:
  - **SSRF & Address Policy Enforcement**: Restricts and validates resolved IP addresses to prevent local network probing and internal address leakage.
  - **Custom Safe Resolver**: Pre-resolves and validates DNS responses before dialing.
  - **Granular Timeout Controls**: Dedicated deadlines for metadata queries, page downloads, TLS handshakes, response headers, and idle connection reuse.
  - **Redirect Limiting**: Strict redirect caps and validation on every hop.
- **Chapter Packaging**: Bundles comic pages in correct sequence with zero data corruption and clean metadata; prose chapters become a Markdown document with a heading and normalised paragraphs.
- **Modular Provider System**: Pluggable scraping interfaces for different comic and light-novel sources. Each Provider declares its capabilities — search, browse, tags, supported sort orders, and media type — so the CLI can validate requests instead of silently approximating them.

---

## Providers

| Provider | ID | Media | Browse | Tags | Sort orders |
|---|---|---|---|---|---|
| MangaDex | `mangadex` | image | yes | yes | popular, recent |
| MangaKatana | `mangakatana` | image | yes | yes | popular, recent |
| MangaPill | `mangapill` | image | yes | yes | popular, recent |
| Manganato | `manganato` | image | yes | yes | popular, recent |
| Manganelo | `manganelo` | image | yes | yes | popular, recent |
| NovelPing | `novelping` | text | yes | yes | popular, recent |
| ReadComicsOnline | `readcomicsonline` | image | no | no | — |
| Webtoons | `webtoons` | image | yes | yes | popular, recent, rating |
| Weeb Central | `weebcentral` | image | yes | no | popular, recent |

The table mirrors what each Provider declares at runtime — run `labrador providers` for the authoritative list. `browse --sort` and `--tag` validate against these declarations: an unsupported sort order is rejected rather than silently mapped to the default.

A text Provider archives each chapter as a Markdown `.md` document instead of a `.cbz`; `labrador fetch` picks the extension from the Provider's declared media type.

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
