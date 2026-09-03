# Labrador

Webcomic discovery, scraping engine, and `.cbz` archive generator in Go.

## Language

### Core Domain

**Provider**:
A registered implementation capable of searching, browsing, and extracting content from a specific webcomic website or API.
_Avoid_: Scraper, Source, Extractor, Plugin

**Capabilities**:
The declarative feature manifest exposed by a Provider specifying supported search modes, sort orders, and tag/filter support.
_Avoid_: Features, Flags, Options

**Series**:
A distinct webcomic or manhwa work hosted by a Provider, containing metadata and a list of Chapters.
_Avoid_: Manga, Comic, Title, Book

**Chapter**:
A single numbered installment or release of a Series, composed of an ordered list of Pages.
_Avoid_: Issue, Episode, Volume, Release

**Page**:
A single image asset representing one page of a Chapter, fetched from a remote Provider URL.
_Avoid_: Image, Scan, Leaf

**Tag**:
A genre, theme, or demographic category defined by a Provider for filtering Series.
_Avoid_: Genre, Category, Keyword, Label

**SortOrder**:
The ordering criteria applied when browsing a Provider's catalog (e.g. Popular, Recent).
_Avoid_: Sort, Order, Ranking

**Archive**:
A zipped `.cbz` file containing the ordered Page images and a standardized `ComicInfo.xml` metadata descriptor for a single Chapter.
_Avoid_: Zip, Bundle, Comic file, Package

**Queue**:
The download scheduler that serializes downloads per Provider and Series for politeness while enabling concurrent downloads across distinct Providers.
_Avoid_: Pool, Scheduler, Dispatcher, Worker

### Ecosystem & External

**Library**:
A directory managed by Dewey containing Series folders populated with Chapter Archives.
_Avoid_: Collection, Vault, Catalog, Store

**Dewey**:
The external TUI library manager and orchestrator that invokes Labrador to search, fetch, and update Series and Chapters.
_Avoid_: Manager, Daemon
