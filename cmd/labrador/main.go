package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/omarys/labrador/internal/cli"
	"github.com/omarys/labrador/internal/downloader"
	"github.com/omarys/labrador/internal/httpclient"
	"github.com/omarys/labrador/internal/provider"
	"github.com/omarys/labrador/internal/providers"
	"github.com/omarys/labrador/internal/tui"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 1. Initialize stealth HTTP client with anti-scraping countermeasures & cookie jar
	httpClient := httpclient.NewStealthClient(30 * time.Second)

	// 2. Initialize registry and register all providers
	reg := provider.NewRegistry()
	providers.RegisterAll(reg, httpClient)

	// 3. Initialize downloader
	dl := downloader.New(httpClient)

	// 3. If invoked with no arguments in an interactive terminal, launch TUI!
	if len(os.Args) <= 1 {
		if err := tui.Run(ctx, reg, dl); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 4. Otherwise, execute CLI command
	app := cli.New(reg, dl)
	if err := app.Run(ctx, os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
