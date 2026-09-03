# Provider Architecture, Queue Politeness, and Dewey Integration Contract

## Context & Decision

To replace `mangal` while learning TDD in Go and integrating with the Dewey library manager:
1. **Provider Engine**: Providers are pure Go structs implementing a core interface with declarative `Capabilities` manifests (search modes, sort orders, filter tags) rather than dynamic scripting runtimes. Private providers are isolated in a gitignored package (`internal/providers/private/`) to prevent leaking custom/private sources to public repositories while preserving native Go type safety and unit testing.
2. **Politeness Queue**: Downloads are serialized per provider and per series (allowing multi-chapter series queues to download in chapter sequence) while allowing parallel downloads across distinct providers.
3. **Headless JSON Contract**: Subcommands (`search`, `fetch`, `browse`) support `--json` emitting strictly valid JSON on stdout for Dewey, reserving stderr for diagnostics and launching an interactive Bubble Tea TUI on interactive TTY invocations.
