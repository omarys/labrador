package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/omarys/labrador/internal/domain"
	"github.com/omarys/labrador/internal/downloader"
	"github.com/omarys/labrador/internal/provider"
	"github.com/omarys/labrador/internal/tui"
)

// App encapsulates dependencies for CLI execution.
type App struct {
	Registry   *provider.Registry
	Downloader *downloader.Downloader
	Stdout     io.Writer
	Stderr     io.Writer
}

// New creates an App instance with standard IO streams.
func New(reg *provider.Registry, dl *downloader.Downloader) *App {
	return &App{
		Registry:   reg,
		Downloader: dl,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
}

// Run executes the application based on arguments.
func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return a.printUsage()
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "search":
		return a.runSearch(ctx, subArgs)
	case "fetch":
		return a.runFetch(ctx, subArgs)
	case "browse":
		return a.runBrowse(ctx, subArgs)
	case "providers":
		return a.runProviders(ctx, subArgs)
	case "resume", "queue":
		return a.runResume(ctx, subArgs)
	case "select":
		return a.runSelect(ctx, subArgs)
	case "help", "-h", "--help":
		return a.printUsage()
	default:
		return fmt.Errorf("unknown command: %s (run 'labrador help' for usage)", subcommand)
	}
}

func (a *App) printUsage() error {
	usage := `Labrador - Webcomic Scraper & CBZ Archiver

Usage:
  labrador <command> [options]

Commands:
  search <query> [--provider <id>] [--json]
      Search for series by title across providers.

  fetch --url <series_or_chapter_url> --chapter <num> [--output-dir <path>] [--json]
      Download chapter pages and package into a .cbz archive.

  browse [--provider <id>] [--tag <id>] [--sort popular|recent] [--page <n>] [--json]
      Browse a provider's catalog by genre or sort order.

  providers [--json]
      List all available providers and their capabilities.
  `
	_, _ = fmt.Fprint(a.Stdout, usage)
	return nil
}

func (a *App) runSearch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	providerID := fs.String("provider", "", "Optional provider ID filter")
	jsonOutput := fs.Bool("json", false, "Output results as JSON")

	// Separate flags from positional search terms so flags work anywhere in the command
	var flagArgs, positionalArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if (arg == "--provider" || arg == "-provider") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	query := strings.TrimSpace(strings.Join(positionalArgs, " "))
	if query == "" {
		return fmt.Errorf("search query required")
	}

	var providersToSearch []provider.Provider
	if *providerID != "" {
		p, ok := a.Registry.Get(*providerID)
		if !ok {
			return fmt.Errorf("provider %s not found", *providerID)
		}
		providersToSearch = append(providersToSearch, p)
	} else {
		providersToSearch = a.Registry.All()
	}

	type searchResultItem struct {
		ProviderID   string `json:"provider_id"`
		ProviderName string `json:"provider_name"`
		domain.Series
	}

	var allResults []searchResultItem
	for _, p := range providersToSearch {
		if !p.Capabilities().CanSearch {
			continue
		}
		results, err := p.Search(ctx, query)
		if err != nil {
			_, _ = fmt.Fprintf(a.Stderr, "Warning: search on provider %s failed: %v\n", p.Name(), err)
			continue
		}
		for _, s := range results {
			allResults = append(allResults, searchResultItem{
				ProviderID:   p.ID(),
				ProviderName: p.Name(),
				Series:       s,
			})
		}
	}

	if *jsonOutput {
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(allResults)
	}

	if len(allResults) == 0 {
		_, _ = fmt.Fprintf(a.Stdout, "No series found for query: %s\n", query)
		return nil
	}

	for _, item := range allResults {
		_, _ = fmt.Fprintf(a.Stdout, "[%s] %s (%s)\n    URL: %s\n", item.ProviderName, item.Title, item.ID, item.URL)
	}

	return nil
}

func (a *App) runFetch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	seriesFlag := fs.String("series", "", "Series title (for automated provider search/resolution)")
	urlFlag := fs.String("url", "", "Series or Chapter URL")
	chapterFlag := fs.String("chapter", "", "Chapter number or title (e.g. 105 or 105.5)")
	outputDir := fs.String("output-dir", "", "Target library directory")
	outputFile := fs.String("output-file", "", "Target .cbz file path:")
	jsonOutput := fs.Bool("json", false, "Output result as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var prov provider.Provider
	var series domain.Series
	var chapters []domain.Chapter

	if *urlFlag != "" {
		p, ok := a.Registry.FindByURL(*urlFlag)
		if !ok {
			return fmt.Errorf("no registered provider matches URL: %s", *urlFlag)
		}
		prov = p
		series = domain.Series{URL: *urlFlag}
		if *seriesFlag != "" {
			series.Title = *seriesFlag
		}

		chaps, err := prov.GetChapters(ctx, series)
		if err != nil {
			return fmt.Errorf("fetching chapters: %w", err)
		}
		chapters = chaps
	} else if *seriesFlag != "" {
		// Resolve series across registered providers (Dewey headless flow)
		_, _ = fmt.Fprintf(a.Stderr, "Searching providers for series: %s\n", *seriesFlag)

		// Prioritize preferred providers first
		preferredOrder := []string{"weebcentral", "mangakatana", "mangadistrict", "private_gallery"}
		allProviders := a.Registry.List()
		var sortedProviders []provider.Provider
		for _, id := range preferredOrder {
			if p, ok := a.Registry.Get(id); ok && p.Capabilities().CanSearch {
				sortedProviders = append(sortedProviders, p)
			}
		}
		for _, p := range allProviders {
			if !slices.Contains(preferredOrder, p.ID()) && p.Capabilities().CanSearch {
				sortedProviders = append(sortedProviders, p)
			}
		}

		targetNum, hasNum := 0.0, false
		if *chapterFlag != "" {
			if n, err := strconv.ParseFloat(*chapterFlag, 64); err == nil {
				targetNum = n
				hasNum = true
			}
		}

		for _, p := range sortedProviders {
			res, err := p.Search(ctx, *seriesFlag)
			if err != nil || len(res) == 0 {
				continue
			}

			// Find best match in provider's search results
			for _, s := range res {
				chaps, err := p.GetChapters(ctx, s)
				if err != nil || len(chaps) == 0 {
					continue
				}

				// If a specific chapter was requested, make sure this provider actually has it!
				if *chapterFlag != "" {
					hasTarget := false
					for _, ch := range chaps {
						if matchChapter(ch, *chapterFlag, targetNum, hasNum) {
							hasTarget = true
							break
						}
					}
					if !hasTarget {
						continue // Provider is missing requested chapter; try next
					}
				}

				prov = p
				series = s
				chapters = chaps
				_, _ = fmt.Fprintf(a.Stderr, "Resolved series '%s' on provider %s\n", s.Title, p.Name())
				break
			}
			if prov != nil {
				break
			}
		}
		if prov == nil {
			return fmt.Errorf("could not resolve series '%s' on any registered provider", *seriesFlag)
		}
	} else {
		return fmt.Errorf("either --url or --series is required")
	}

	if len(chapters) == 0 {
		return fmt.Errorf("no chapters found for series %s", series.Title)
	}

	// Find the requested chapter
	var targetChapter *domain.Chapter
	if *chapterFlag != "" {
		targetNum, parseErr := strconv.ParseFloat(*chapterFlag, 64)
		hasNum := parseErr == nil
		for _, ch := range chapters {
			if matchChapter(ch, *chapterFlag, targetNum, hasNum) {
				targetChapter = &ch
				break
			}
		}
	} else {
		// Default to first chapter if none specified
		targetChapter = &chapters[0]
	}

	if targetChapter == nil {
		return fmt.Errorf("chapter %s not found in series", *chapterFlag)
	}

	_, _ = fmt.Fprintf(a.Stderr, "Downloading %s...\n", targetChapter.Title)

	// Execute download
	res, err := a.Downloader.DownloadChapter(ctx, prov, series, *targetChapter, downloader.DownloadOptions{
		OutputDir:  *outputDir,
		OutputFile: *outputFile,
	})
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// If invoked by Dewey (--series present) or --json requested, emit clean JSON on stdout
	if *jsonOutput || *seriesFlag != "" {
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	_, _ = fmt.Fprintf(a.Stdout, "Saved chapter to: %s (%d pages)\nURL: %s\n", res.FilePath, res.PageCount, res.FetchURL)
	return nil
}

func (a *App) runBrowse(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("browse", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	providerID := fs.String("provider", "", "Provider ID to browse (required)")
	tagID := fs.String("tag", "", "Optional tag ID")
	sortStr := fs.String("sort", "popular", "Sort order: popular | recent")
	page := fs.Int("page", 1, "Page number")
	jsonOutput := fs.Bool("json", false, "Output results as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *providerID == "" {
		return fmt.Errorf("--provider is required for browse")
	}

	prov, ok := a.Registry.Get(*providerID)
	if !ok {
		return fmt.Errorf("provider %s not found", *providerID)
	}

	var tag *domain.Tag
	if *tagID != "" {
		tag = &domain.Tag{ID: *tagID}
	}

	sortOrder := domain.SortPopular
	if *sortStr == "recent" {
		sortOrder = domain.SortRecent
	}

	results, err := prov.Browse(ctx, provider.BrowseOptions{
		Tag:  tag,
		Sort: sortOrder,
		Page: *page,
	})
	if err != nil {
		return fmt.Errorf("browsing provider: %w", err)
	}

	if *jsonOutput {
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	for _, s := range results {
		_, _ = fmt.Fprintf(a.Stdout, "[%s] %s\n    URL: %s\n", prov.Name(), s.Title, s.URL)
	}

	return nil
}

func (a *App) runProviders(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("providers", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	jsonOutput := fs.Bool("json", false, "Output results as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	type providerSummary struct {
		ID           string                `json:"id"`
		Name         string                `json:"name"`
		Capabilities provider.Capabilities `json:"capabilities"`
	}

	var summaries []providerSummary
	for _, p := range a.Registry.All() {
		summaries = append(summaries, providerSummary{
			ID:           p.ID(),
			Name:         p.Name(),
			Capabilities: p.Capabilities(),
		})
	}

	if *jsonOutput {
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summaries)
	}

	for _, s := range summaries {
		_, _ = fmt.Fprintf(a.Stdout, "• %s (%s) - Search: %t, Browse: %t\n", s.Name, s.ID, s.Capabilities.CanSearch, s.Capabilities.CanBrowse)
	}

	return nil
}

func (a *App) runResume(ctx context.Context, _ []string) error {
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			cacheDir = "/tmp/labrador"
		} else {
			cacheDir = filepath.Join(home, ".cache", "labrador")
		}
	} else {
		cacheDir = filepath.Join(cacheDir, "labrador")
	}
	cacheFile := filepath.Join(cacheDir, "queue.json")

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			_, _ = fmt.Fprintln(a.Stdout, "No pending downloads in queue.")
			return nil
		}
		return fmt.Errorf("reading queue cache: %w", err)
	}

	type persistedItem struct {
		ID           string         `json:"id"`
		ProviderID   string         `json:"provider_id"`
		ProviderName string         `json:"provider_name"`
		Series       domain.Series  `json:"series"`
		Chapter      domain.Chapter `json:"chapter"`
	}

	var items []persistedItem
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("parsing queue cache: %w", err)
	}

	if len(items) == 0 {
		_, _ = fmt.Fprintln(a.Stdout, "No pending downloads in queue.")
		_ = os.Remove(cacheFile)
		return nil
	}

	_, _ = fmt.Fprintf(a.Stdout, "Resuming %d download(s) from queue...\n\n", len(items))

	var remaining []persistedItem
	for i, item := range items {
		prov, ok := a.Registry.Get(item.ProviderID)
		if !ok {
			_, _ = fmt.Fprintf(a.Stderr, "[%d/%d] Provider %s not found for %s, skipping\n", i+1, len(items), item.ProviderID, item.Chapter.Title)
			remaining = append(remaining, item)
			continue
		}

		_, _ = fmt.Fprintf(a.Stdout, "[%d/%d] Downloading %s - %s (%s)... ", i+1, len(items), item.Series.Title, item.Chapter.Title, prov.Name())

		res, err := a.Downloader.DownloadChapter(ctx, prov, item.Series, item.Chapter, downloader.DownloadOptions{})
		if err != nil {
			_, _ = fmt.Fprintf(a.Stdout, "FAILED: %v\n", err)
			remaining = append(remaining, item)
		} else {
			_, _ = fmt.Fprintf(a.Stdout, "DONE! (%d pages -> %s)\n", res.PageCount, res.FilePath)
		}

		// Update cache file after each item
		if len(remaining) > 0 {
			updatedData, _ := json.MarshalIndent(remaining, "", "  ")
			_ = os.WriteFile(cacheFile, updatedData, 0644)
		} else {
			_ = os.Remove(cacheFile)
		}
	}

	if len(remaining) == 0 {
		_, _ = fmt.Fprintln(a.Stdout, "\nAll queued downloads completed successfully!")
		_ = os.Remove(cacheFile)
	} else {
		_, _ = fmt.Fprintf(a.Stdout, "\nCompleted with %d failed item(s) remaining in queue.\n", len(remaining))
	}
	return nil
}

func (a *App) runSelect(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("select", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	queryFlag := fs.String("query", "", "Series title to search across all providers")
	outputFile := fs.String("output-file", "", "Optional file path to write chosen series URL")
	jsonOutput := fs.Bool("json", false, "Output result as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	query := *queryFlag
	if query == "" && len(fs.Args()) > 0 {
		query = strings.Join(fs.Args(), " ")
	}

	chosen, err := tui.RunSelect(ctx, a.Registry, a.Downloader, query)
	if err != nil {
		return fmt.Errorf("select failed: %w", err)
	}

	if chosen == nil || chosen.URL == "" {
		return nil // User cancelled
	}

	if *outputFile != "" {
		if *jsonOutput {
			data, _ := json.Marshal(chosen)
			_ = os.WriteFile(*outputFile, data, 0644)
		} else {
			_ = os.WriteFile(*outputFile, []byte(chosen.URL), 0644)
		}
	}

	if *jsonOutput {
		enc := json.NewEncoder(a.Stdout)
		return enc.Encode(chosen)
	}

	_, _ = fmt.Fprintln(a.Stdout, chosen.URL)
	return nil
}

var chapterNumberRe = regexp.MustCompile(`(?i)(?:chapter|ch\.?|ep\.?|episode)?\s*([0-9]+(?:\.[0-9]+)?)`)

func matchChapter(ch domain.Chapter, targetStr string, targetNum float64, hasNum bool) bool {
	if hasNum && ch.Number != nil && math.Abs(*ch.Number-targetNum) < 0.001 {
		return true
	}
	if hasNum {
		matches := chapterNumberRe.FindStringSubmatch(ch.Title)
		if len(matches) > 1 {
			if n, err := strconv.ParseFloat(matches[1], 64); err == nil && math.Abs(n-targetNum) < 0.001 {
				return true
			}
		}
	}
	if strings.EqualFold(ch.Title, targetStr) {
		return true
	}
	targetTrimmed := strings.TrimSuffix(targetStr, ".0")
	return strings.EqualFold(ch.Title, targetTrimmed)
}
