package cliui

import (
	"fmt"
	"io"
	"strings"
)

type HelpCommand struct {
	Name        string
	Description string
}

var topLevelCommands = []HelpCommand{
	{Name: "version", Description: "Print the glade version."},
	{Name: "doctor", Description: "Print environment and project configuration status."},
	{Name: "parse", Description: "Parse Apex source files."},
	{Name: "inspect", Description: "Inspect indexed project symbols and performance risks."},
	{Name: "schema", Description: "Load local Salesforce metadata schema."},
	{Name: "check", Description: "Run semantic checks over a project."},
	{Name: "exec", Description: "Execute anonymous Apex."},
	{Name: "debug", Description: "Parse, profile, explain, and synthesize from Salesforce debug logs."},
	{Name: "editor", Description: "Install and check editor integrations."},
	{Name: "dap", Description: "Run the Debug Adapter Protocol server over stdio."},
	{Name: "test", Description: "Discover and run supported Apex tests."},
	{Name: "dev", Description: "Run the human-focused local development cockpit."},
	{Name: "report", Description: "List, show, export, and clean saved run reports."},
	{Name: "lsp", Description: "Run the Language Server Protocol server over stdio."},
	{Name: "profile", Description: "Analyze glade trace output."},
	{Name: "package", Description: "Build managed package artifacts."},
	{Name: "server", Description: "Start the local Salesforce-compatible API baseline."},
	{Name: "playground", Description: "Start the local Apex playground web UI."},
	{Name: "db", Description: "Seed, reset, export, and inspect a persistent local database."},
	{Name: "help", Description: "Print this help text."},
}

func WriteHelp(w io.Writer) error {
	t := NewTheme(w)
	if _, err := fmt.Fprintln(w, t.Bold("glade")+" — local Apex runtime"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, t.Bold("Usage")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "  glade <command> [flags]"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, t.Bold("Commands")); err != nil {
		return err
	}
	maxName := 0
	for _, cmd := range topLevelCommands {
		if len(cmd.Name) > maxName {
			maxName = len(cmd.Name)
		}
	}
	for _, cmd := range topLevelCommands {
		name := t.Cyan(cmd.Name)
		if !t.Color {
			name = cmd.Name
		}
		desc := t.Dim(cmd.Description)
		if !t.Color {
			desc = cmd.Description
		}
		if _, err := fmt.Fprintf(w, "  %-*s  %s\n", maxName, name, desc); err != nil {
			return err
		}
	}
	return nil
}

func WriteTestHelp(w io.Writer) error {
	body := strings.TrimSpace(`
Run local Apex tests.

Usage:
  glade test [--project <root>] [flags]
  glade test serve [--project <root>] [serve flags]
  glade test clear-cache [--project <root>]

Persistent test server:
  glade test serve keeps the project runtime warm across CLI invocations.
  It writes .glade/test/serve.sock and serve.pid under the project root.
  Later glade test runs auto-connect when that socket is reachable.
  Use --no-serve to force a local build, or --connect to require the server.

Serve flags:
  --project <root>          Project root. Defaults to current directory.
  --socket <path>           Override the unix socket path.
  --no-watch                Do not watch project files for changes.
  --no-warm                 Skip the initial runtime warm on startup.

Common flags:
  --project <root>          Project root. Defaults to current directory.
  --filter <pattern>        Run matching test classes or methods.
  --connect                 Require a running test server (see serve).
  --no-serve                Do not auto-connect to a running test server.
  --no-cache                Skip the on-disk startup cache for this run.
  --daemon                  Keep index warm in-process for --watch loops.
  --json                    Write JSON test results.
  --junit <path>            Write JUnit XML results.
  --progress                Print line progress to stderr, even when not attached to a terminal.
  --progress-json           Print NDJSON progress events to stderr.
  --no-progress, --quiet    Disable terminal progress.
  --debug                   Run one DAP snapshot session after tests.
  --watch                   Watch source files and emit NDJSON events.
  --watch-once              Run one watch cycle and exit.
  --changed-since <ref>     Select tests affected since a git ref.
  --debounce <dur>          Watch debounce interval (default 500ms).
  --watch-backend <mode>    Watch backend: auto, native, or poll.
  --parallel-methods        Run test methods in parallel (default).
  --no-parallel-methods     Force serial method execution within a class.
  --parallelism <n>         Worker count (default: GOMAXPROCS).
  --test-timeout <dur>      Per-test timeout (default 5m, e.g. 30s, 2m).
  --gc-aggressive           Run with GOGC=50 for memory-constrained hosts.
  --limit-mode <mode>       Use strict or permissive governor limits.

Startup cache (.glade/test/startup.gob):
  Written after a cold harness build; loaded when file/config/package fingerprints
  still match. Does not store test results. A stale cache can hide new code —
  use clear-cache after git pull or Glade upgrades, and --no-cache to debug.
  See docs/TEST_STARTUP_CACHE.md for freshness rules and recovery.

Examples:
  glade test serve --project .
  glade test clear-cache --project .
  glade test --project . --filter AccountServiceTest
  glade test --project . --no-cache --filter AccountServiceTest
  glade test --project . --connect --filter AccountServiceTest
  glade test --project . --daemon --watch
  glade test --project . --changed-since origin/main --json
`)
	_, err := fmt.Fprintln(w, body)
	return err
}
