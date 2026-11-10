# Text Media: Prose Chapters Alongside Page Images

## Context & Decision

Every Provider until now delivered Chapters as ordered Page images, archived as `.cbz`. Light-novel sources (e.g. NovelPing) deliver prose instead: one HTTP response containing the chapter body, no page list at all.

Forcing prose through the image pipeline would mean inventing a single fake Page whose "image" is HTML, then shipping a `.cbz` of markup — a lying archive that Dewey and any reader would mis-handle. Instead, media kind is declared, not inferred:

1. **Declared media kind** — `Capabilities.Media` carries a `domain.MediaType` (`MediaImage`, the zero value, or `MediaText`). Empty means image, so every pre-existing Provider keeps working unchanged.
2. **A separate capability interface** — a Provider declaring `MediaText` must also implement `provider.TextProvider` (`GetChapterText`). `GetPages` returns `ErrTextOnly` rather than a fabricated page: a text Provider forced into image mode fails loudly instead of writing a `.cbz` of HTML.
3. **One fetch, one file** — the Downloader branches on `Media` *before* page discovery and writes `.md` via `archive.WriteText`. Chapter listings, existence checks, and skip-on-rerun logic recognise `.md` alongside `.cbz`/`.zip`.
4. **Prose is archived as Markdown, not plain text** — the body arrives as HTML and is converted by `internal/htmlmd` (`Fragment`/`Paragraph`/`Inline`): one paragraph per `<p>`, `<em>`/`<strong>` as emphasis, links as inline references, `<br>` as a hard line break, and literal Markdown syntax in the prose escaped so it renders unchanged. Non-prose elements (page chrome, form controls, embedded media) are dropped, as is any element hidden from readers — the `hidden` attribute, `aria-hidden="true"`, inline `display:none`/`visibility:hidden`/`font-size:0`, and utility classes such as `sr-only`/`d-none`. Hiding is how scraped novel sites embed "read this at \<site\>" notices *inside* chapter paragraphs, so hidden content is never prose. `archive.WriteText` prepends `# <chapter title>` as the document heading, so the file is a complete Markdown document and the Provider returns only the body.
5. **Sort vocabulary stays Provider-local** — genre and tag listings on a scraped site rarely share the global feed's ordering. NovelPing's genre pages take their own `?sort=` values (`ALL_TIME`, `LASTEST`) while its global feeds use `/sort/popular` and `/sort/updates`; the Provider maps one `domain.SortOrder` onto whichever vocabulary the endpoint it is calling actually speaks.

## Consequences

- Page-oriented concepts (Page, page count, CBZ) are image-media concerns; `DownloadResult.PageCount` is 1 for a prose Chapter because there is exactly one body, not one "page".
- The `.cbz`-only assumptions in the library layer had to learn a second extension. Any future media kind (audio, EPUB) extends `domain.MediaType` and the same two branch points rather than adding a parallel pipeline.
- Markdown is a lossy-but-honest target: a site's CSS, ruby annotations and illustration figures do not survive, and `<br>` is preserved as a CommonMark hard break rather than a blank line. Prose that contains Markdown syntax is escaped, so plain-text readers see stray backslashes rather than emphasised words — the trade accepted for files that render correctly.
- The conversion lives in `internal/htmlmd`, not in a Provider, because every text Provider scrapes HTML and needs the same rules. `archive.WriteText` stays format-agnostic: it normalises whitespace and adds the heading, nothing more.
- A Provider's `SupportedSorts` is now part of the CLI contract: `browse --sort` validates against it and rejects unsupported orders instead of silently collapsing to the default.
