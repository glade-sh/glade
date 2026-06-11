# Glade Plugin Architecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `glade plugins list` and `glade plugins install` the user-facing path for maintenance extensions while keeping scanner fixtures, corpora, generated maintenance code, docs inventory, and advisory scanners outside the `glade` product repo.

**Architecture:** `glade` owns a small plugin manager, command router, and product runtime commands. Plugins are standalone executables installed under `~/.glade/plugins`, discovered through a JSON manifest, and run through argv/stdout/stderr like `kubectl` and Git external commands. Today’s `~/Dev/glade-tools` becomes the first first-party plugin binary, while its large fixtures, maintenance packages, docs inventory, project scans, and advisory performance scans stay out of the product repo.

**Tech Stack:** Go 1.26, `os/exec`, JSON manifests, SHA-256 checksums, tar.gz release assets, local filesystem plugin store, `httptest` for registry/install tests.

---

## Decision

Build the Salesforce-style user experience:

```bash
glade plugins list
glade plugins install @glade/compat
glade compat local-tests --project . --parallel auto --json
glade surface refresh --docs "$GLADE_SALESFORCE_DOCS_SOURCE" --out tmp/surface
```

Do not build a Go `plugin` shared-library system. Go shared libraries require compiler and ABI alignment that will make release support brittle. Use executable plugins instead.

Do not use a Git submodule as the product path. A submodule pins source; it does not solve user installation, versioning, checksums, command discovery, or CI restore. Use a `go.work` file for local multi-repo development and release archives for user installs.

Keep the current `glade-tools` repository as source for the first plugin during the migration. The binary identity changes first. The repository can be renamed after the protocol is stable.

## Boundary Rule

Use this rule before moving any package:

```text
If it runs local Salesforce behavior for users, it belongs in base Glade.
If it measures Glade, grades Glade, scans projects for missing support, mines docs, or generates maintenance reports, it belongs in a plugin.
```

Concrete assignments:

- Base Glade keeps parser, semantic checks, VM, test runner, SOQL, DML, storage, schema, metadata/resource runtime loaders, server, LSP/DAP, debug log parsing, measured profile analysis, and plugin management.
- Base Glade gets product-grade test selectors and sharding from `compat local-tests` where those features make `glade test` better.
- Base Glade gets a describe-catalog import command if it turns captured org describe JSON into a local schema or storage shape. Live org capture remains a plugin.
- Plugins own compatibility fixtures, corpus readiness, surface ledger, docs inventory, capability catalogs, post-parity scans, example scans, generated support reports, and advisory performance/security/AI scanners.
- `glade inspect performance` leaves base Glade. The first-party performance plugin owns that capability as `glade performance scan`.

## Current State

- `/Users/matt/Dev/glade` is the product repo.
- `/Users/matt/Dev/glade-tools` is a sibling Go module at `github.com/glade-sh/glade/tools`.
- `glade-tools` currently exposes one command runner in `internal/toolcli`.
- `glade-tools/internal/toolcli/compat_command.go` is 2462 lines and owns most command dispatch.
- `glade-tools/internal/toolcli/compat_surface_command.go` is 681 lines and owns surface subcommands.
- `/Users/matt/Dev/glade/internal/apexdocs` is only used by maintenance tooling and must move into the compat plugin.
- `/Users/matt/Dev/glade/internal/perfscan` backs `glade inspect performance`; it is advisory scanning and must move into a first-party performance plugin.
- `/Users/matt/Dev/glade/internal/orgdescribe` has no product command today. Keep only the describe-to-local-schema converter in base if exposed through `glade schema import describe`; put live capture and readiness scans in plugins.
- `glade-tools compat local-tests` has product-worthy test selectors and sharding. Fold the useful selectors into `glade test`; keep readiness scoring, top blocker grouping, corpus baselines, and compatibility JSON in the plugin.
- Existing Glade tests intentionally reject `glade compat` when no plugin exists.
- The work must preserve that behavior until a plugin is installed or linked.

## Command Contract

Core `glade` commands keep priority. A plugin cannot override these command roots:

```text
version, completion, doctor, parse, inspect, schema, check, exec, test,
dev, profile, package, server, playground, db, lsp, report, plugins, help
```

Plugin command roots can include:

```text
compat, surface, post-parity, examples, local-tests, dashboard, gaps, stdlib,
performance
```

The first implementation dispatches by first argument. If a plugin claims root `compat`, then `glade compat local-tests ...` invokes that plugin with the original args.

Do not let plugins claim `inspect`. Retire the base `glade inspect performance` command during the boundary correction packet and replace it with plugin-owned `glade performance scan`.

## Manifest Contract

Every plugin executable must support:

```bash
glade-plugin-compat manifest --json
```

It writes this shape:

```json
{
  "apiVersion": "glade.plugin.v1",
  "name": "compat",
  "version": "0.1.0",
  "summary": "Compatibility fixtures, surface ledgers, and maintenance scanners.",
  "commands": [
    {
      "path": ["compat"],
      "summary": "Run compatibility fixture and report commands."
    },
    {
      "path": ["surface"],
      "summary": "Refresh and inspect the Salesforce surface ledger."
    }
  ],
  "minimumGladeVersion": "0.1.0",
  "source": "github.com/glade-sh/glade/tools"
}
```

The host stores the manifest as installed state. On command dispatch it does not parse plugin-specific flags. It runs the plugin executable and streams stdout/stderr.

## Installed Store

Use this layout:

```text
~/.glade/
  plugins/
    installed.json
    compat/
      0.1.0/
        bin/
          glade-plugin-compat
        plugin.json
        checksums.txt
        data/
```

Use this installed state:

```json
{
  "version": 1,
  "plugins": [
    {
      "name": "compat",
      "version": "0.1.0",
      "executable": "/Users/matt/.glade/plugins/compat/0.1.0/bin/glade-plugin-compat",
      "manifest": "/Users/matt/.glade/plugins/compat/0.1.0/plugin.json",
      "source": "registry:compat",
      "linked": false
    }
  ]
}
```

Linked development plugins use the same shape with `"linked": true` and an executable path inside a local build directory.

## File Map

### `/Users/matt/Dev/glade`

- Create `internal/pluginhost/model.go`: manifest, command, installed plugin, registry entry, and lockfile types.
- Create `internal/pluginhost/store.go`: path resolution, installed state read/write, atomic file writes.
- Create `internal/pluginhost/manifest.go`: manifest loading from executables and JSON files.
- Create `internal/pluginhost/router.go`: command-root matching and core-command collision checks.
- Create `internal/pluginhost/runner.go`: `os/exec` runner with stdout/stderr streaming.
- Create `internal/pluginhost/install.go`: local archive install and registry-backed install.
- Create `internal/pluginhost/registry.go`: registry index fetch and verification.
- Create `internal/pluginhost/plugin_test.go`: unit tests for store, manifest, routing, install, and runner.
- Create `internal/gladecli/plugins_command.go`: `glade plugins` user commands.
- Modify `internal/gladecli/cli.go`: route `plugins`; fall through unknown command roots to `pluginhost`.
- Modify `internal/cliui/help.go`: add `plugins` as a product command.
- Modify `internal/gladecli/cli_test.go`: update `compat` tests to cover no-plugin and linked-plugin behavior.
- Modify `internal/repoguard/repo_standards_test.go`: keep scanner fixture directories out of product repo, but allow `internal/pluginhost`.
- Remove `internal/perfscan` after the performance plugin owns `glade performance scan`.
- Remove `internal/apexdocs` after the compat plugin owns docs inventory and surface-ledger docs snapshots.
- Modify `internal/gladecli/cli.go`: remove `inspect performance` from base dispatch once the performance plugin link path passes.
- Modify `internal/cliui/help.go`: remove `inspect performance` from base help and document the plugin migration.
- Modify `internal/gladecli/test_command.go`: add product-grade exact test selection and sharding from `compat local-tests`.
- Modify `internal/schema` and `internal/gladecli`: expose describe-catalog import if `internal/orgdescribe` stays in base.
- Create `docs/PLUGINS.md`: user and author documentation.
- Modify `README.md`, `docs/ARCHITECTURE.md`, `docs/COMPATIBILITY.md`, `docs/LOCAL_TESTING.md`: change `glade-tools` usage to plugin usage once link/install works.

### `/Users/matt/Dev/glade-tools`

- Create `cmd/glade-plugin-compat/main.go`: plugin binary entrypoint.
- Create `cmd/glade-plugin-performance/main.go`: first-party performance scanner plugin entrypoint.
- Modify `cmd/glade-tools/main.go`: keep wrapper for one migration release.
- Create `internal/toolcli/manifest.go`: manifest output.
- Modify `internal/toolcli/cli.go`: handle `manifest --json`; keep existing commands.
- Create `internal/toolcli/plugin_command_test.go`: manifest and command passthrough tests.
- Move `/Users/matt/Dev/glade/internal/apexdocs` to `/Users/matt/Dev/glade-tools/internal/apexdocs`.
- Move `/Users/matt/Dev/glade/internal/perfscan` to `/Users/matt/Dev/glade-tools/internal/perfscan`.
- Create `scripts/build-plugin-archives.sh`: builds darwin/linux archives with checksums.
- Create `plugin.json`: source manifest used in packaging.
- Modify `README.md`: document `glade plugins link` and `glade plugins install @glade/compat`.

## Task 1: Add Plugin Host Types In Glade

**Files:**
- Create: `/Users/matt/Dev/glade/internal/pluginhost/model.go`
- Create: `/Users/matt/Dev/glade/internal/pluginhost/model_test.go`

- [ ] **Step 1: Write manifest type tests**

Add `/Users/matt/Dev/glade/internal/pluginhost/model_test.go`:

```go
package pluginhost

import (
	"encoding/json"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	input := Manifest{
		APIVersion: "glade.plugin.v1",
		Name:       "compat",
		Version:    "0.1.0",
		Summary:    "Compatibility fixtures.",
		Commands: []CommandManifest{
			{Path: []string{"compat"}, Summary: "Run compatibility commands."},
			{Path: []string{"surface"}, Summary: "Run surface ledger commands."},
		},
		MinimumGladeVersion: "0.1.0",
		Source:              "github.com/glade-sh/glade/tools",
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var got Manifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "compat" || len(got.Commands) != 2 || got.Commands[1].Path[0] != "surface" {
		t.Fatalf("manifest round trip lost data: %#v", got)
	}
}

func TestManifestValidateRejectsBadRoot(t *testing.T) {
	manifest := Manifest{
		APIVersion: "glade.plugin.v1",
		Name:       "bad",
		Version:    "0.1.0",
		Commands:   []CommandManifest{{Path: []string{"plugins"}, Summary: "Override core command."}},
	}
	if err := manifest.Validate(); err == nil {
		t.Fatal("expected core command collision to fail")
	}
}
```

- [ ] **Step 2: Run the new tests and verify they fail**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost -run 'TestManifest' -count=1
```

Expected:

```text
package github.com/glade-sh/glade/internal/pluginhost is not in std
```

or:

```text
undefined: Manifest
```

- [ ] **Step 3: Add the manifest model**

Add `/Users/matt/Dev/glade/internal/pluginhost/model.go`:

```go
package pluginhost

import (
	"errors"
	"fmt"
	"strings"
)

const APIVersion = "glade.plugin.v1"

var coreCommandRoots = map[string]struct{}{
	"version": {}, "completion": {}, "doctor": {}, "parse": {}, "inspect": {},
	"schema": {}, "check": {}, "exec": {}, "test": {}, "dev": {}, "profile": {},
	"package": {}, "server": {}, "playground": {}, "db": {}, "lsp": {},
	"report": {}, "plugins": {}, "help": {},
}

type Manifest struct {
	APIVersion          string            `json:"apiVersion"`
	Name                string            `json:"name"`
	Version             string            `json:"version"`
	Summary             string            `json:"summary,omitempty"`
	Commands            []CommandManifest `json:"commands"`
	MinimumGladeVersion string            `json:"minimumGladeVersion,omitempty"`
	Source              string            `json:"source,omitempty"`
}

type CommandManifest struct {
	Path    []string `json:"path"`
	Summary string   `json:"summary,omitempty"`
}

type InstalledState struct {
	Version int               `json:"version"`
	Plugins []InstalledPlugin `json:"plugins"`
}

type InstalledPlugin struct {
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Executable string   `json:"executable"`
	Manifest   string   `json:"manifest"`
	Source     string   `json:"source,omitempty"`
	Linked     bool     `json:"linked"`
	Commands   []string `json:"commands"`
}

func (m Manifest) Validate() error {
	if m.APIVersion != APIVersion {
		return fmt.Errorf("unsupported plugin api version %q", m.APIVersion)
	}
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("plugin name is required")
	}
	if strings.TrimSpace(m.Version) == "" {
		return errors.New("plugin version is required")
	}
	if len(m.Commands) == 0 {
		return errors.New("plugin must declare at least one command")
	}
	for _, command := range m.Commands {
		if len(command.Path) == 0 || strings.TrimSpace(command.Path[0]) == "" {
			return errors.New("plugin command root is required")
		}
		root := command.Path[0]
		if _, exists := coreCommandRoots[root]; exists {
			return fmt.Errorf("plugin command %q conflicts with a core glade command", root)
		}
	}
	return nil
}

func (m Manifest) CommandRoots() []string {
	seen := map[string]struct{}{}
	var roots []string
	for _, command := range m.Commands {
		if len(command.Path) == 0 {
			continue
		}
		root := command.Path[0]
		if _, exists := seen[root]; exists {
			continue
		}
		seen[root] = struct{}{}
		roots = append(roots, root)
	}
	return roots
}
```

- [ ] **Step 4: Run tests and verify they pass**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost -run 'TestManifest' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/pluginhost
```

- [ ] **Step 5: Commit**

```bash
cd /Users/matt/Dev/glade
git add internal/pluginhost/model.go internal/pluginhost/model_test.go
git commit -m "feat: add glade plugin manifest model"
```

## Task 2: Add Installed Plugin Store

**Files:**
- Create: `/Users/matt/Dev/glade/internal/pluginhost/store.go`
- Create: `/Users/matt/Dev/glade/internal/pluginhost/store_test.go`

- [ ] **Step 1: Write store tests**

Add `/Users/matt/Dev/glade/internal/pluginhost/store_test.go`:

```go
package pluginhost

import (
	"path/filepath"
	"testing"
)

func TestStoreReadWriteInstalledState(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	state := InstalledState{
		Version: 1,
		Plugins: []InstalledPlugin{{
			Name:       "compat",
			Version:    "0.1.0",
			Executable: filepath.Join(root, "plugins", "compat", "0.1.0", "bin", "glade-plugin-compat"),
			Manifest:   filepath.Join(root, "plugins", "compat", "0.1.0", "plugin.json"),
			Source:     "link:/tmp/glade-tools",
			Linked:     true,
			Commands:   []string{"compat", "surface"},
		}},
	}
	if err := store.WriteInstalled(state); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Plugins) != 1 || got.Plugins[0].Name != "compat" || !got.Plugins[0].Linked {
		t.Fatalf("unexpected installed state: %#v", got)
	}
}

func TestStoreMissingInstalledStateIsEmpty(t *testing.T) {
	store := NewStore(t.TempDir())
	got, err := store.ReadInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || len(got.Plugins) != 0 {
		t.Fatalf("unexpected empty state: %#v", got)
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost -run 'TestStore' -count=1
```

Expected:

```text
undefined: NewStore
```

- [ ] **Step 3: Add the store**

Add `/Users/matt/Dev/glade/internal/pluginhost/store.go`:

```go
package pluginhost

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Store struct {
	root string
}

func DefaultRoot() string {
	if override := os.Getenv("GLADE_HOME"); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".glade"
	}
	return filepath.Join(home, ".glade")
}

func NewStore(root string) Store {
	return Store{root: root}
}

func (s Store) InstalledPath() string {
	return filepath.Join(s.root, "plugins", "installed.json")
}

func (s Store) ReadInstalled() (InstalledState, error) {
	path := s.InstalledPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return InstalledState{Version: 1}, nil
		}
		return InstalledState{}, err
	}
	var state InstalledState
	if err := json.Unmarshal(data, &state); err != nil {
		return InstalledState{}, err
	}
	if state.Version == 0 {
		state.Version = 1
	}
	return state, nil
}

func (s Store) WriteInstalled(state InstalledState) error {
	if state.Version == 0 {
		state.Version = 1
	}
	path := s.InstalledPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
```

- [ ] **Step 4: Run store tests**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost -run 'TestStore' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/pluginhost
```

- [ ] **Step 5: Commit**

```bash
cd /Users/matt/Dev/glade
git add internal/pluginhost/store.go internal/pluginhost/store_test.go
git commit -m "feat: add glade plugin install store"
```

## Task 3: Add Manifest Loading And Plugin Listing

**Files:**
- Create: `/Users/matt/Dev/glade/internal/pluginhost/manifest.go`
- Create: `/Users/matt/Dev/glade/internal/pluginhost/list.go`
- Create: `/Users/matt/Dev/glade/internal/pluginhost/manifest_test.go`
- Create: `/Users/matt/Dev/glade/internal/gladecli/plugins_command.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/cli.go`
- Modify: `/Users/matt/Dev/glade/internal/cliui/help.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`

- [ ] **Step 1: Add CLI tests for empty list**

Append to `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`:

```go
func TestPluginsListEmpty(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"plugins", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No plugins installed.") {
		t.Fatalf("unexpected stdout:\n%s", stdout.String())
	}
}
```

- [ ] **Step 2: Run the CLI test and verify it fails**

```bash
cd /Users/matt/Dev/glade
go test ./internal/gladecli -run TestPluginsListEmpty -count=1
```

Expected:

```text
unknown command "plugins"
```

- [ ] **Step 3: Add manifest loading helpers**

Add `/Users/matt/Dev/glade/internal/pluginhost/manifest.go`:

```go
package pluginhost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

func LoadManifestFromExecutable(ctx context.Context, executable string) (Manifest, error) {
	cmd := exec.CommandContext(ctx, executable, "manifest", "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Manifest{}, fmt.Errorf("read plugin manifest: %w: %s", err, stderr.String())
	}
	var manifest Manifest
	if err := json.Unmarshal(stdout.Bytes(), &manifest); err != nil {
		return Manifest{}, err
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
```

Add `/Users/matt/Dev/glade/internal/pluginhost/list.go`:

```go
package pluginhost

func (s Store) ListInstalled() ([]InstalledPlugin, error) {
	state, err := s.ReadInstalled()
	if err != nil {
		return nil, err
	}
	return state.Plugins, nil
}
```

- [ ] **Step 4: Add `glade plugins list`**

Create `/Users/matt/Dev/glade/internal/gladecli/plugins_command.go`:

```go
package gladecli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/glade-sh/glade/internal/pluginhost"
)

func runPlugins(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		printPluginsHelp(stdout)
		return nil
	}
	switch args[0] {
	case "list":
		return runPluginsList(ctx, stdout)
	default:
		return fmt.Errorf("unknown plugins command %q", args[0])
	}
}

func printPluginsHelp(w io.Writer) {
	fmt.Fprint(w, `Manage Glade plugins.

Usage:
  glade plugins <command> [flags]

Commands:
  list              List installed plugins.
  link              Link a local plugin executable.
  install           Install a plugin from the registry or archive.
  remove            Remove an installed plugin.
  doctor            Check installed plugins.
  which             Show the plugin that owns a command.
`)
}

func runPluginsList(ctx context.Context, stdout io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	plugins, err := pluginhost.NewStore(pluginhost.DefaultRoot()).ListInstalled()
	if err != nil {
		return err
	}
	if len(plugins) == 0 {
		_, err := fmt.Fprintln(stdout, "No plugins installed.")
		return err
	}
	for _, plugin := range plugins {
		link := ""
		if plugin.Linked {
			link = " linked"
		}
		if len(plugin.Commands) == 0 {
			return errors.New("installed plugin has no commands")
		}
		fmt.Fprintf(stdout, "%s %s%s %v\n", plugin.Name, plugin.Version, link, plugin.Commands)
	}
	return nil
}
```

Modify `/Users/matt/Dev/glade/internal/gladecli/cli.go` in the main command switch:

```go
	case "plugins":
		if err := runPlugins(ctx, args[1:], stdout); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
```

Modify `/Users/matt/Dev/glade/internal/cliui/help.go` to add `plugins` in the public command list:

```go
{
	Name:        "plugins",
	Usage:       "glade plugins <command>",
	Description: "Find, install, and manage Glade plugins.",
},
```

- [ ] **Step 5: Run focused tests**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost ./internal/gladecli -run 'TestManifest|TestStore|TestPluginsListEmpty' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/pluginhost
ok  	github.com/glade-sh/glade/internal/gladecli
```

- [ ] **Step 6: Commit**

```bash
cd /Users/matt/Dev/glade
git add internal/pluginhost/manifest.go internal/pluginhost/list.go internal/gladecli/plugins_command.go internal/gladecli/cli.go internal/cliui/help.go internal/gladecli/cli_test.go
git commit -m "feat: add glade plugins list"
```

## Task 4: Add Local Plugin Linking

**Files:**
- Create: `/Users/matt/Dev/glade/internal/pluginhost/link.go`
- Create: `/Users/matt/Dev/glade/internal/pluginhost/link_test.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/plugins_command.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`

- [ ] **Step 1: Write link tests**

Add `/Users/matt/Dev/glade/internal/pluginhost/link_test.go` with a test helper script:

```go
package pluginhost

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLinkExecutableStoresManifest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	root := t.TempDir()
	exe := filepath.Join(root, "glade-plugin-compat")
	script := `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "--json" ]; then
  printf '{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["compat"],"summary":"Compat commands."},{"path":["surface"],"summary":"Surface commands."}]}'
  exit 0
fi
echo "compat plugin"
`
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	plugin, err := store.LinkExecutable(context.Background(), exe, "link:test")
	if err != nil {
		t.Fatal(err)
	}
	if plugin.Name != "compat" || !plugin.Linked || len(plugin.Commands) != 2 {
		t.Fatalf("unexpected linked plugin: %#v", plugin)
	}
	state, err := store.ReadInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Plugins) != 1 || state.Plugins[0].Commands[1] != "surface" {
		t.Fatalf("unexpected installed state: %#v", state)
	}
}
```

- [ ] **Step 2: Run the link test and verify it fails**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost -run TestLinkExecutableStoresManifest -count=1
```

Expected:

```text
undefined: Store.LinkExecutable
```

- [ ] **Step 3: Add link support**

Add `/Users/matt/Dev/glade/internal/pluginhost/link.go`:

```go
package pluginhost

import (
	"context"
	"path/filepath"
)

func (s Store) LinkExecutable(ctx context.Context, executable, source string) (InstalledPlugin, error) {
	abs, err := filepath.Abs(executable)
	if err != nil {
		return InstalledPlugin{}, err
	}
	manifest, err := LoadManifestFromExecutable(ctx, abs)
	if err != nil {
		return InstalledPlugin{}, err
	}
	plugin := InstalledPlugin{
		Name:       manifest.Name,
		Version:    manifest.Version,
		Executable: abs,
		Manifest:   "",
		Source:     source,
		Linked:     true,
		Commands:   manifest.CommandRoots(),
	}
	state, err := s.ReadInstalled()
	if err != nil {
		return InstalledPlugin{}, err
	}
	state.Plugins = replaceInstalled(state.Plugins, plugin)
	if err := s.WriteInstalled(state); err != nil {
		return InstalledPlugin{}, err
	}
	return plugin, nil
}

func replaceInstalled(plugins []InstalledPlugin, plugin InstalledPlugin) []InstalledPlugin {
	out := plugins[:0]
	for _, existing := range plugins {
		if existing.Name == plugin.Name {
			continue
		}
		out = append(out, existing)
	}
	return append(out, plugin)
}
```

- [ ] **Step 4: Add `glade plugins link --exec <path>`**

Modify `/Users/matt/Dev/glade/internal/gladecli/plugins_command.go`:

```go
	case "link":
		return runPluginsLink(ctx, args[1:], stdout)
```

Add:

```go
func runPluginsLink(ctx context.Context, args []string, stdout io.Writer) error {
	executable := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--exec":
			if i+1 >= len(args) {
				return errors.New("--exec requires a value")
			}
			executable = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown plugins link argument %q", args[i])
		}
	}
	if executable == "" {
		return errors.New("usage: glade plugins link --exec <plugin-executable>")
	}
	plugin, err := pluginhost.NewStore(pluginhost.DefaultRoot()).LinkExecutable(ctx, executable, "link:"+executable)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Linked plugin %s %s with commands %v.\n", plugin.Name, plugin.Version, plugin.Commands)
	return nil
}
```

- [ ] **Step 5: Add CLI link test**

Append to `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`:

```go
func TestPluginsLinkListsPlugin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	home := t.TempDir()
	t.Setenv("GLADE_HOME", home)
	exe := filepath.Join(home, "glade-plugin-compat")
	script := `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "--json" ]; then
  printf '{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["compat"],"summary":"Compat commands."}]}'
  exit 0
fi
`
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"plugins", "link", "--exec", exe}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("link exit=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"plugins", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("list exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "compat 0.1.0 linked") {
		t.Fatalf("unexpected list output:\n%s", stdout.String())
	}
}
```

Add imports used by the test if absent:

```go
	"os"
	"path/filepath"
	"runtime"
```

- [ ] **Step 6: Run tests**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost ./internal/gladecli -run 'TestLink|TestPluginsLink|TestPluginsList' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/pluginhost
ok  	github.com/glade-sh/glade/internal/gladecli
```

- [ ] **Step 7: Commit**

```bash
cd /Users/matt/Dev/glade
git add internal/pluginhost/link.go internal/pluginhost/link_test.go internal/gladecli/plugins_command.go internal/gladecli/cli_test.go
git commit -m "feat: support linking local glade plugins"
```

## Task 5: Add Plugin Command Dispatch

**Files:**
- Create: `/Users/matt/Dev/glade/internal/pluginhost/router.go`
- Create: `/Users/matt/Dev/glade/internal/pluginhost/runner.go`
- Create: `/Users/matt/Dev/glade/internal/pluginhost/router_test.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/cli.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`

- [ ] **Step 1: Write router and runner tests**

Add `/Users/matt/Dev/glade/internal/pluginhost/router_test.go`:

```go
package pluginhost

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFindByCommandRoot(t *testing.T) {
	state := InstalledState{Version: 1, Plugins: []InstalledPlugin{{
		Name: "compat", Commands: []string{"compat", "surface"},
	}}}
	plugin, ok := FindByCommandRoot(state, "surface")
	if !ok || plugin.Name != "compat" {
		t.Fatalf("expected compat plugin, got %#v ok=%t", plugin, ok)
	}
}

func TestRunPluginStreamsOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "glade-plugin-compat")
	script := `#!/bin/sh
echo "plugin stdout: $*"
echo "plugin stderr" >&2
`
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code, err := RunPlugin(context.Background(), InstalledPlugin{Executable: exe}, []string{"compat", "x"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout.String(), "plugin stdout: compat x") {
		t.Fatalf("stdout not streamed: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "plugin stderr") {
		t.Fatalf("stderr not streamed: %q", stderr.String())
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost -run 'TestFindByCommandRoot|TestRunPluginStreamsOutput' -count=1
```

Expected:

```text
undefined: FindByCommandRoot
```

- [ ] **Step 3: Add router and runner**

Add `/Users/matt/Dev/glade/internal/pluginhost/router.go`:

```go
package pluginhost

func FindByCommandRoot(state InstalledState, root string) (InstalledPlugin, bool) {
	for _, plugin := range state.Plugins {
		for _, command := range plugin.Commands {
			if command == root {
				return plugin, true
			}
		}
	}
	return InstalledPlugin{}, false
}
```

Add `/Users/matt/Dev/glade/internal/pluginhost/runner.go`:

```go
package pluginhost

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
)

func RunPlugin(ctx context.Context, plugin InstalledPlugin, args []string, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, plugin.Executable, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), "GLADE_PLUGIN_HOST=glade", "GLADE_PLUGIN_API_VERSION="+APIVersion)
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 1, err
}
```

- [ ] **Step 4: Dispatch unknown roots to plugins**

Modify `/Users/matt/Dev/glade/internal/gladecli/cli.go` near the unknown-command path. Before writing `unknown command`, read the installed state and try a plugin:

```go
	if code, ok := runInstalledPluginCommand(ctx, args, stdout, stderr); ok {
		return code
	}
```

Add helper in `/Users/matt/Dev/glade/internal/gladecli/plugins_command.go`:

```go
func runInstalledPluginCommand(ctx context.Context, args []string, stdout, stderr io.Writer) (int, bool) {
	if len(args) == 0 {
		return 0, false
	}
	store := pluginhost.NewStore(pluginhost.DefaultRoot())
	state, err := store.ReadInstalled()
	if err != nil {
		return 1, true
	}
	plugin, ok := pluginhost.FindByCommandRoot(state, args[0])
	if !ok {
		return 0, false
	}
	code, err := pluginhost.RunPlugin(ctx, plugin, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "glade: plugin %s failed: %v\n", plugin.Name, err)
		return 1, true
	}
	return code, true
}
```

- [ ] **Step 5: Add CLI dispatch tests**

Append to `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`:

```go
func TestUnknownCompatStillFailsWithoutPlugin(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("compat without plugin succeeded")
	}
	if !strings.Contains(stderr.String(), `unknown command "compat"`) {
		t.Fatalf("expected unknown command, got stderr:\n%s", stderr.String())
	}
}

func TestCompatDispatchesToLinkedPlugin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	home := t.TempDir()
	t.Setenv("GLADE_HOME", home)
	exe := filepath.Join(home, "glade-plugin-compat")
	script := `#!/bin/sh
if [ "$1" = "manifest" ] && [ "$2" = "--json" ]; then
  printf '{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["compat"],"summary":"Compat commands."}]}'
  exit 0
fi
echo "called plugin with $*"
`
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"plugins", "link", "--exec", exe}, &stdout, &stderr); code != 0 {
		t.Fatalf("link failed: %s", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code := Run(context.Background(), []string{"compat", "local-tests", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dispatch failed code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "called plugin with compat local-tests --help") {
		t.Fatalf("unexpected stdout:\n%s", stdout.String())
	}
}
```

- [ ] **Step 6: Run tests**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost ./internal/gladecli -run 'TestFindByCommandRoot|TestRunPluginStreamsOutput|TestUnknownCompatStillFailsWithoutPlugin|TestCompatDispatchesToLinkedPlugin' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/pluginhost
ok  	github.com/glade-sh/glade/internal/gladecli
```

- [ ] **Step 7: Commit**

```bash
cd /Users/matt/Dev/glade
git add internal/pluginhost/router.go internal/pluginhost/runner.go internal/pluginhost/router_test.go internal/gladecli/cli.go internal/gladecli/plugins_command.go internal/gladecli/cli_test.go
git commit -m "feat: dispatch glade commands to linked plugins"
```

## Task 6: Make Glade Tools A Plugin Binary

**Files:**
- Create: `/Users/matt/Dev/glade-tools/cmd/glade-plugin-compat/main.go`
- Create: `/Users/matt/Dev/glade-tools/internal/toolcli/manifest.go`
- Create: `/Users/matt/Dev/glade-tools/internal/toolcli/plugin_command_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/cli.go`
- Modify: `/Users/matt/Dev/glade-tools/README.md`

- [ ] **Step 1: Add manifest test**

Add `/Users/matt/Dev/glade-tools/internal/toolcli/plugin_command_test.go`:

```go
package toolcli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestManifestCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"manifest", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	var manifest struct {
		APIVersion string `json:"apiVersion"`
		Name       string `json:"name"`
		Commands   []struct {
			Path []string `json:"path"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.APIVersion != "glade.plugin.v1" || manifest.Name != "compat" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	joined := stdout.String()
	for _, root := range []string{`"compat"`, `"surface"`, `"local-tests"`} {
		if !strings.Contains(joined, root) {
			t.Fatalf("manifest missing %s:\n%s", root, joined)
		}
	}
}
```

- [ ] **Step 2: Run test and verify it fails**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/toolcli -run TestManifestCommand -count=1
```

Expected:

```text
glade-tools: usage: glade-tools ...
```

- [ ] **Step 3: Add manifest command**

Add `/Users/matt/Dev/glade-tools/internal/toolcli/manifest.go`:

```go
package toolcli

import (
	"encoding/json"
	"io"
)

type pluginManifest struct {
	APIVersion          string                  `json:"apiVersion"`
	Name                string                  `json:"name"`
	Version             string                  `json:"version"`
	Summary             string                  `json:"summary"`
	Commands            []pluginCommandManifest `json:"commands"`
	MinimumGladeVersion string                  `json:"minimumGladeVersion"`
	Source              string                  `json:"source"`
}

type pluginCommandManifest struct {
	Path    []string `json:"path"`
	Summary string   `json:"summary"`
}

func writePluginManifest(w io.Writer) error {
	manifest := pluginManifest{
		APIVersion:          "glade.plugin.v1",
		Name:                "compat",
		Version:             "0.1.0",
		Summary:             "Compatibility fixtures, surface ledgers, project scanners, and generated support reports.",
		MinimumGladeVersion: "0.1.0",
		Source:              "github.com/glade-sh/glade/tools",
		Commands: []pluginCommandManifest{
			{Path: []string{"compat"}, Summary: "Compatibility fixture and report commands."},
			{Path: []string{"surface"}, Summary: "Salesforce surface ledger commands."},
			{Path: []string{"local-tests"}, Summary: "Project local-test readiness scans."},
			{Path: []string{"post-parity"}, Summary: "Unsupported surface scans for Salesforce projects."},
			{Path: []string{"examples"}, Summary: "Example-project readiness scans."},
			{Path: []string{"dashboard"}, Summary: "Compatibility dashboard generation."},
			{Path: []string{"gaps"}, Summary: "Known-gap document generation."},
			{Path: []string{"stdlib"}, Summary: "Standard library coverage generation."},
		},
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(manifest)
}
```

Modify `/Users/matt/Dev/glade-tools/internal/toolcli/cli.go` at the top of `Run` after the empty/help check:

```go
	if args[0] == "manifest" {
		if len(args) == 2 && args[1] == "--json" {
			if err := writePluginManifest(stdout); err != nil {
				fmt.Fprintf(stderr, "glade-tools: %v\n", err)
				return 1
			}
			return 0
		}
		fmt.Fprintln(stderr, "glade-tools: usage: glade-plugin-compat manifest --json")
		return 1
	}
```

- [ ] **Step 4: Add plugin binary main**

Add `/Users/matt/Dev/glade-tools/cmd/glade-plugin-compat/main.go`:

```go
package main

import (
	"context"
	"os"

	"github.com/glade-sh/glade/tools/internal/toolcli"
)

func main() {
	os.Exit(toolcli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
```

- [ ] **Step 5: Run tests and build both binaries**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/toolcli -run TestManifestCommand -count=1
go build ./cmd/glade-plugin-compat
go build ./cmd/glade-tools
```

Expected:

```text
ok  	github.com/glade-sh/glade/tools/internal/toolcli
```

Both build commands exit 0.

- [ ] **Step 6: Commit**

```bash
cd /Users/matt/Dev/glade-tools
git add cmd/glade-plugin-compat/main.go internal/toolcli/manifest.go internal/toolcli/plugin_command_test.go internal/toolcli/cli.go README.md
git commit -m "feat: expose glade tools as compat plugin"
```

## Task 7: Prove End-To-End Local Linking

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`
- No production files unless the test exposes a real bug.

- [ ] **Step 1: Build the plugin binary**

```bash
cd /Users/matt/Dev/glade-tools
go build -o /tmp/glade-plugin-compat ./cmd/glade-plugin-compat
```

Expected:

```text
```

No output. Exit 0.

- [ ] **Step 2: Link the binary in a temp Glade home**

```bash
cd /Users/matt/Dev/glade
tmp="$(mktemp -d)"
GLADE_HOME="$tmp" go run ./cmd/glade plugins link --exec /tmp/glade-plugin-compat
```

Expected output contains:

```text
Linked plugin compat 0.1.0
```

- [ ] **Step 3: List the linked plugin**

```bash
cd /Users/matt/Dev/glade
GLADE_HOME="$tmp" go run ./cmd/glade plugins list
```

Expected output contains:

```text
compat 0.1.0 linked
```

- [ ] **Step 4: Run plugin help through Glade**

```bash
cd /Users/matt/Dev/glade
GLADE_HOME="$tmp" go run ./cmd/glade compat local-tests --help
```

Expected output contains:

```text
Report local Apex test execution readiness.
```

- [ ] **Step 5: Run surface help through Glade**

```bash
cd /Users/matt/Dev/glade
GLADE_HOME="$tmp" go run ./cmd/glade surface --help
```

Expected output contains surface usage text from `glade-tools`.

- [ ] **Step 6: Commit only if production or tests changed**

If this task only runs the manual proof, do not commit. If it exposes a bug and you patch it, commit the exact touched files:

```bash
cd /Users/matt/Dev/glade
git add internal/pluginhost/router.go internal/pluginhost/runner.go internal/gladecli/cli.go internal/gladecli/plugins_command.go internal/gladecli/cli_test.go
git commit -m "fix: support linked compat plugin dispatch"
```

## Task 7A: Move Docs Inventory Into The Compat Plugin

**Files:**
- Move: `/Users/matt/Dev/glade/internal/apexdocs` to `/Users/matt/Dev/glade-tools/internal/apexdocs`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/compat_command.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/docs_snapshot.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/surfaceledger/docs_snapshot_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/*.go`
- Modify: `/Users/matt/Dev/glade/docs/ARCHITECTURE.md`
- Modify: `/Users/matt/Dev/glade/internal/repoguard/repo_standards_test.go`

- [ ] **Step 1: Prove `apexdocs` has no product callers**

Run:

```bash
cd /Users/matt/Dev/glade
rg -n "internal/apexdocs|apexdocs\\." /Users/matt/Dev/glade /Users/matt/Dev/glade-tools
```

Expected:

- Product hits are limited to `/Users/matt/Dev/glade/internal/apexdocs`, docs describing architecture, and tests in that package.
- Tooling hits come from `/Users/matt/Dev/glade-tools/internal/toolcli`, `/Users/matt/Dev/glade-tools/internal/surfaceledger`, and `/Users/matt/Dev/glade-tools/internal/capability`.

- [ ] **Step 2: Move the package**

Run:

```bash
mkdir -p /Users/matt/Dev/glade-tools/internal/apexdocs
cp /Users/matt/Dev/glade/internal/apexdocs/*.go /Users/matt/Dev/glade-tools/internal/apexdocs/
cd /Users/matt/Dev/glade
git rm -r internal/apexdocs
```

- [ ] **Step 3: Rewrite imports in `glade-tools`**

Replace:

```go
"github.com/glade-sh/glade/internal/apexdocs"
```

with:

```go
"github.com/glade-sh/glade/tools/internal/apexdocs"
```

Run:

```bash
cd /Users/matt/Dev/glade-tools
rg -l "github.com/glade-sh/glade/internal/apexdocs" internal | xargs perl -0pi -e 's#github\\.com/glade-sh/glade/internal/apexdocs#github.com/glade-sh/glade/tools/internal/apexdocs#g'
gofmt -w internal/apexdocs internal/toolcli internal/surfaceledger internal/capability
```

- [ ] **Step 4: Remove product architecture reference**

Edit `/Users/matt/Dev/glade/docs/ARCHITECTURE.md`. Delete the bullet that lists `internal/apexdocs` as a product package. Add one sentence in the tooling boundary paragraph:

```text
Salesforce docs inventory extraction lives in the compat plugin because it feeds ledgers and generated support reports, not runtime execution.
```

- [ ] **Step 5: Add guard coverage**

Add a repoguard assertion that `/Users/matt/Dev/glade/internal/apexdocs` must not exist and that product code must not import `github.com/glade-sh/glade/tools/internal/apexdocs`.

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/repoguard -run Test -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/repoguard
```

- [ ] **Step 6: Verify both repos**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/apexdocs ./internal/capability ./internal/surfaceledger ./internal/toolcli -count=1

cd /Users/matt/Dev/glade
go test ./internal/gladecli ./internal/repoguard -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/tools/internal/apexdocs
ok  	github.com/glade-sh/glade/tools/internal/capability
ok  	github.com/glade-sh/glade/tools/internal/surfaceledger
ok  	github.com/glade-sh/glade/tools/internal/toolcli
ok  	github.com/glade-sh/glade/internal/gladecli
ok  	github.com/glade-sh/glade/internal/repoguard
```

- [ ] **Step 7: Commit**

```bash
cd /Users/matt/Dev/glade
git add docs/ARCHITECTURE.md internal/repoguard/repo_standards_test.go
git add -u internal/apexdocs
git commit -m "chore: move docs inventory out of product runtime"

cd /Users/matt/Dev/glade-tools
git add internal/apexdocs internal/toolcli internal/surfaceledger internal/capability
git commit -m "feat: own Salesforce docs inventory in compat plugin"
```

## Task 7B: Move Advisory Performance Scan To A First-Party Plugin

**Files:**
- Move: `/Users/matt/Dev/glade/internal/perfscan` to `/Users/matt/Dev/glade-tools/internal/perfscan`
- Create: `/Users/matt/Dev/glade-tools/cmd/glade-plugin-performance/main.go`
- Create: `/Users/matt/Dev/glade-tools/internal/perftool/cli.go`
- Create: `/Users/matt/Dev/glade-tools/internal/perftool/cli_test.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/cli.go`
- Modify: `/Users/matt/Dev/glade/internal/cliui/help.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`
- Modify: `/Users/matt/Dev/glade/docs/INSTALL.md`
- Modify: `/Users/matt/Dev/glade/docs/LOCAL_TESTING.md`
- Modify: `/Users/matt/Dev/glade/docs/DOGFOOD_CHECKLIST.md`
- Modify: `/Users/matt/Dev/glade/docs/COMPATIBILITY.md`
- Modify: `/Users/matt/Dev/glade/internal/repoguard/repo_standards_test.go`

- [ ] **Step 1: Prove current command behavior before moving**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/perfscan ./internal/gladecli -run 'Test.*Performance|TestInspectPerformance' -count=1
go run ./cmd/glade inspect performance --help
```

Expected:

- tests pass if the current tree is green.
- help prints `glade inspect performance` usage.

- [ ] **Step 2: Move scanner package**

Run:

```bash
mkdir -p /Users/matt/Dev/glade-tools/internal/perfscan
cp /Users/matt/Dev/glade/internal/perfscan/*.go /Users/matt/Dev/glade-tools/internal/perfscan/
cd /Users/matt/Dev/glade
git rm -r internal/perfscan
```

- [ ] **Step 3: Add the performance plugin CLI wrapper**

Create `/Users/matt/Dev/glade-tools/internal/perftool/cli.go`:

```go
package perftool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/glade-sh/glade/tools/internal/perfscan"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 2 && args[0] == "manifest" && args[1] == "--json" {
		_ = writeManifest(stdout)
		return 0
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printHelp(stdout)
		return 0
	}
	if args[0] != "performance" {
		fmt.Fprintf(stderr, "glade-plugin-performance: unknown command %q\n", args[0])
		return 1
	}
	if err := runPerformance(ctx, args[1:], stdout); err != nil {
		fmt.Fprintf(stderr, "glade-plugin-performance: %v\n", err)
		return 1
	}
	return 0
}

func writeManifest(w io.Writer) error {
	return json.NewEncoder(w).Encode(map[string]any{
		"apiVersion":          "glade.plugin.v1",
		"name":                "performance",
		"version":             "0.1.0",
		"summary":             "Advisory Salesforce project performance scans.",
		"minimumGladeVersion": "0.1.0",
		"source":              "github.com/glade-sh/glade/tools",
		"commands": []map[string]any{
			{"path": []string{"performance"}, "summary": "Scan a project for advisory performance risks."},
		},
	})
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, `Scan Salesforce projects for advisory performance risks.

Usage:
  glade performance scan [--project <root>] [--trace <path>] [--json] [--top <n>]
`)
}

func runPerformance(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 || args[0] != "scan" {
		return errors.New("usage: glade performance scan [--project <root>] [--trace <path>] [--json] [--top <n>]")
	}
	root := "."
	tracePath := ""
	jsonOut := false
	topN := 0
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--trace":
			if i+1 >= len(args) {
				return errors.New("--trace requires a value")
			}
			tracePath = args[i+1]
			i++
		case "--json":
			jsonOut = true
		case "--top":
			if i+1 >= len(args) {
				return errors.New("--top requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed < 0 {
				return errors.New("--top must be a non-negative integer")
			}
			topN = parsed
			i++
		default:
			return fmt.Errorf("unknown performance scan argument %q", args[i])
		}
	}
	report, err := perfscan.AnalyzeProject(perfscan.Options{ProjectRoot: root, TracePath: tracePath, TopN: topN})
	if err != nil {
		return err
	}
	if jsonOut {
		return perfscan.WriteJSON(w, report)
	}
	return perfscan.WriteMarkdown(w, report)
}
```

Create `/Users/matt/Dev/glade-tools/cmd/glade-plugin-performance/main.go`:

```go
package main

import (
	"context"
	"os"

	"github.com/glade-sh/glade/tools/internal/perftool"
)

func main() {
	os.Exit(perftool.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
```

- [ ] **Step 4: Remove base `inspect performance`**

In `/Users/matt/Dev/glade/internal/gladecli/cli.go`, remove the `perfscan` import and the `runInspectPerformance` branch. `glade inspect` remains for symbols.

In `/Users/matt/Dev/glade/internal/cliui/help.go`, remove `glade inspect performance` usage and examples. Add a short note in the plugin docs instead:

```text
Performance scans are provided by the first-party performance plugin: `glade plugins install @glade/performance`, then `glade performance scan --project .`.
```

- [ ] **Step 5: Update tests**

In `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`, replace the `inspect performance` success tests with a no-plugin message test:

```go
func TestInspectPerformanceMovedToPlugin(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "performance", "--project", "."}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("inspect performance unexpectedly succeeded without plugin")
	}
	if !strings.Contains(stderr.String(), "performance scans are provided by the performance plugin") {
		t.Fatalf("stderr missing plugin guidance:\n%s", stderr.String())
	}
}
```

Add `/Users/matt/Dev/glade-tools/internal/perftool/cli_test.go`:

```go
package perftool

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"manifest", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"name":"performance"`) || !strings.Contains(stdout.String(), `"performance"`) {
		t.Fatalf("unexpected manifest:\n%s", stdout.String())
	}
}
```

- [ ] **Step 6: Add guard coverage**

Add repoguard assertions:

- `/Users/matt/Dev/glade/internal/perfscan` must not exist.
- `/Users/matt/Dev/glade/internal/gladecli` must not import `internal/perfscan`.
- product help must not list `inspect performance`.

- [ ] **Step 7: Verify**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan ./internal/perftool -count=1
go build -o /tmp/glade-plugin-performance ./cmd/glade-plugin-performance

cd /Users/matt/Dev/glade
go test ./internal/gladecli ./internal/repoguard -count=1
tmp="$(mktemp -d)"
GLADE_HOME="$tmp" go run ./cmd/glade plugins link --exec /tmp/glade-plugin-performance
GLADE_HOME="$tmp" go run ./cmd/glade performance scan --help
```

Expected:

- performance plugin tests pass.
- glade product tests pass.
- `glade performance scan --help` prints plugin help after link.

- [ ] **Step 8: Commit**

```bash
cd /Users/matt/Dev/glade
git add -u internal/perfscan internal/gladecli internal/cliui docs/INSTALL.md docs/LOCAL_TESTING.md docs/DOGFOOD_CHECKLIST.md docs/COMPATIBILITY.md internal/repoguard/repo_standards_test.go
git commit -m "chore: move advisory performance scan to plugin"

cd /Users/matt/Dev/glade-tools
git add internal/perfscan internal/perftool cmd/glade-plugin-performance
git commit -m "feat: add performance scan plugin"
```

## Task 7C: Put Describe Catalog Import In Base Glade

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/orgdescribe/orgdescribe.go`
- Modify: `/Users/matt/Dev/glade/internal/orgdescribe/orgdescribe_test.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/cli.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`
- Modify: `/Users/matt/Dev/glade/internal/cliui/help.go`
- Modify: `/Users/matt/Dev/glade/docs/schema.md` or `/Users/matt/Dev/glade/docs/storage-schema.md`

- [ ] **Step 1: Add a command test**

Add a test in `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`:

```go
func TestSchemaImportDescribeWritesSchema(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "describe.json")
	output := filepath.Join(root, "schema.json")
	data := `{"objects":[{"name":"Account","label":"Account","labelPlural":"Accounts","fields":[{"name":"Id","type":"id","label":"Account ID","nillable":false},{"name":"Name","type":"string","label":"Account Name","nillable":false,"createable":true,"updateable":true}]}]}`
	if err := os.WriteFile(input, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"schema", "import", "describe", "--input", input, "--output", output}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	written, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), `"name": "Account"`) {
		t.Fatalf("schema output missing Account:\n%s", string(written))
	}
}
```

- [ ] **Step 2: Implement `glade schema import describe`**

In the schema command dispatcher, add:

```text
glade schema import describe --input <describe.json> --output <schema.json>
```

Implementation:

- read a JSON `orgdescribe.Catalog`.
- call `Catalog.ToSchema()`.
- write formatted JSON to `--output`, or stdout when `--output` is absent.
- do not add any live org login or REST capture.

- [ ] **Step 3: Update help**

Add help entry:

```text
glade schema import describe --input <describe.json> [--output <schema.json>]
```

State that capture from an org belongs to a plugin.

- [ ] **Step 4: Verify**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/orgdescribe ./internal/gladecli -run 'TestSchemaImportDescribe|TestCatalog' -count=1
go run ./cmd/glade schema import describe --help
```

Expected:

- tests pass.
- help prints the import command.

- [ ] **Step 5: Commit**

```bash
cd /Users/matt/Dev/glade
git add internal/orgdescribe internal/gladecli internal/cliui docs/storage-schema.md
git commit -m "feat: import describe catalogs into local schema"
```

## Task 7D: Fold Product Test Selectors Into `glade test`

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/gladecli/test_command.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`
- Modify: `/Users/matt/Dev/glade/internal/apextest`
- Modify: `/Users/matt/Dev/glade/internal/watch` if class-file or shard planning reuses selection code.
- Modify: `/Users/matt/Dev/glade/docs/LOCAL_TESTING.md`
- Modify: `/Users/matt/Dev/glade-tools/internal/compat/local_tests.go`

- [ ] **Step 1: Add product selector tests**

Add tests in `/Users/matt/Dev/glade/internal/gladecli/cli_test.go` that prove:

- `glade test --class AccountServiceTest` runs only that class.
- `glade test --class AccountServiceTest --method testCreatesAccount` runs only that method.
- `glade test --class-file tests.txt` reads class names from the file.
- `glade test --shard-count 2 --shard-index 1` runs a deterministic subset.

Use an existing small testdata project. If no current testdata project has enough classes, create a small fixture under `/Users/matt/Dev/glade/internal/gladecli/testdata/test-selection`.

- [ ] **Step 2: Implement exact class and method selection**

In `/Users/matt/Dev/glade/internal/gladecli/test_command.go`, add flags:

```text
--class <name>
--method <name>
--class-file <path>
--shard-count <n>
--shard-index <i>
--duration-history <path>
--write-class-shards <dir>
```

Rules:

- `--method` requires `--class`.
- `--class-file` contains one class name per line; ignore blank lines and `#` comments.
- `--class` and `--class-file` cannot both be passed.
- shard selection sorts class names before selecting.
- duration history is optional and only weights shard planning when present.

- [ ] **Step 3: Keep readiness-only features in plugin**

Do not move these from `/Users/matt/Dev/glade-tools/internal/compat/local_tests.go`:

- `Ready` scoring.
- `TopFailures` grouping.
- corpus baselines and expected outcomes.
- `BlockersOnly`.
- compatibility capability IDs.
- post-parity and surface linkage.

After base Glade owns exact selection, simplify plugin local-tests so it shells to or calls the same selection behavior through `apextest` options instead of carrying duplicate selection parsing.

- [ ] **Step 4: Verify product test command**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/gladecli ./internal/apextest ./internal/watch -run 'Test.*Class|Test.*Shard|TestRunTest' -count=1
go run ./cmd/glade test --help
```

Expected:

- tests pass.
- help lists `--class`, `--method`, `--class-file`, `--shard-count`, `--shard-index`.

- [ ] **Step 5: Verify plugin still reports readiness**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/compat ./internal/toolcli -run 'Test.*LocalTests|Test.*ClassShard' -count=1
go run ./cmd/glade-tools local-tests --project ../glade/testdata/local-tests/basic --class AccountServiceTest --json
```

Expected:

- tests pass.
- JSON output still contains `target`, `ready`, `summary`, and `outcomes`.

- [ ] **Step 6: Commit**

```bash
cd /Users/matt/Dev/glade
git add internal/gladecli/test_command.go internal/gladecli/cli_test.go internal/apextest internal/watch docs/LOCAL_TESTING.md
git commit -m "feat: add exact test selection to glade test"

cd /Users/matt/Dev/glade-tools
git add internal/compat/local_tests.go internal/toolcli/compat_command.go
git commit -m "chore: keep local-test readiness on product selectors"
```

## Task 8: Add Local Archive Install

**Files:**
- Create: `/Users/matt/Dev/glade/internal/pluginhost/archive.go`
- Create: `/Users/matt/Dev/glade/internal/pluginhost/install.go`
- Create: `/Users/matt/Dev/glade/internal/pluginhost/install_test.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/plugins_command.go`

- [ ] **Step 1: Define archive format**

The archive root must contain:

```text
plugin.json
checksums.txt
bin/glade-plugin-<name>
```

`plugin.json` is the manifest. `checksums.txt` contains SHA-256 rows:

```text
<hex>  bin/glade-plugin-<name>
<hex>  plugin.json
```

- [ ] **Step 2: Write install tests**

Add `/Users/matt/Dev/glade/internal/pluginhost/install_test.go`:

```go
package pluginhost

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallArchiveInstallsExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper uses sh")
	}
	root := t.TempDir()
	archive := filepath.Join(root, "compat.tar.gz")
	manifest := []byte(`{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["compat"],"summary":"Compat commands."}]}` + "\n")
	exe := []byte("#!/bin/sh\nif [ \"$1\" = \"manifest\" ]; then cat plugin.json; exit 0; fi\necho compat\n")
	checksums := []byte(fmt.Sprintf("%x  plugin.json\n%x  bin/glade-plugin-compat\n", sha256.Sum256(manifest), sha256.Sum256(exe)))
	writeTestArchive(t, archive, map[string][]byte{
		"plugin.json":              manifest,
		"checksums.txt":            checksums,
		"bin/glade-plugin-compat":  exe,
	})
	store := NewStore(filepath.Join(root, "home"))
	plugin, err := store.InstallArchive(context.Background(), archive)
	if err != nil {
		t.Fatal(err)
	}
	if plugin.Name != "compat" || plugin.Linked {
		t.Fatalf("unexpected plugin: %#v", plugin)
	}
	if _, err := os.Stat(plugin.Executable); err != nil {
		t.Fatal(err)
	}
}

func writeTestArchive(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	for name, data := range files {
		mode := int64(0o644)
		if name == "bin/glade-plugin-compat" {
			mode = 0o755
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
}
```

- [ ] **Step 3: Run tests and verify failure**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost -run TestInstallArchiveInstallsExecutable -count=1
```

Expected:

```text
undefined: Store.InstallArchive
```

- [ ] **Step 4: Implement archive install**

Add `/Users/matt/Dev/glade/internal/pluginhost/archive.go` and `/Users/matt/Dev/glade/internal/pluginhost/install.go`.

Implementation rules:

- Reject absolute tar paths.
- Reject `..` path segments.
- Extract into a temp directory under the plugin store.
- Parse `plugin.json`.
- Verify every file listed in `checksums.txt`.
- Mark files under `bin/` executable with mode from tar.
- Move the extracted directory to `~/.glade/plugins/<name>/<version>`.
- Update `installed.json`.

The key install signature:

```go
func (s Store) InstallArchive(ctx context.Context, archivePath string) (InstalledPlugin, error)
```

The installed executable path:

```go
filepath.Join(s.root, "plugins", manifest.Name, manifest.Version, "bin", "glade-plugin-"+manifest.Name)
```

- [ ] **Step 5: Add `glade plugins install <archive>`**

Modify `/Users/matt/Dev/glade/internal/gladecli/plugins_command.go`:

```go
	case "install":
		return runPluginsInstall(ctx, args[1:], stdout)
```

Add:

```go
func runPluginsInstall(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: glade plugins install <plugin-archive>")
	}
	plugin, err := pluginhost.NewStore(pluginhost.DefaultRoot()).InstallArchive(ctx, args[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Installed plugin %s %s with commands %v.\n", plugin.Name, plugin.Version, plugin.Commands)
	return nil
}
```

- [ ] **Step 6: Run focused tests**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost ./internal/gladecli -run 'TestInstallArchive|TestPlugins' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/pluginhost
ok  	github.com/glade-sh/glade/internal/gladecli
```

- [ ] **Step 7: Commit**

```bash
cd /Users/matt/Dev/glade
git add internal/pluginhost/archive.go internal/pluginhost/install.go internal/pluginhost/install_test.go internal/gladecli/plugins_command.go
git commit -m "feat: install glade plugins from archives"
```

## Task 9: Add Registry Install By Name

**Files:**
- Create: `/Users/matt/Dev/glade/internal/pluginhost/registry.go`
- Create: `/Users/matt/Dev/glade/internal/pluginhost/registry_test.go`
- Modify: `/Users/matt/Dev/glade/internal/pluginhost/install.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/plugins_command.go`

- [ ] **Step 1: Define registry JSON**

Registry index shape:

```json
{
  "version": 1,
  "plugins": [
    {
      "name": "compat",
      "version": "0.1.0",
      "summary": "Compatibility fixtures, surface ledgers, and maintenance scanners.",
      "assets": [
        {
          "os": "darwin",
          "arch": "arm64",
          "url": "https://github.com/glade-sh/glade-tools/releases/download/v0.1.0/glade-plugin-compat_0.1.0_darwin_arm64.tar.gz",
          "sha256": "64 lowercase hex characters"
        }
      ]
    },
    {
      "name": "performance",
      "version": "0.1.0",
      "summary": "Advisory Salesforce project performance scans.",
      "assets": [
        {
          "os": "darwin",
          "arch": "arm64",
          "url": "https://github.com/glade-sh/glade-tools/releases/download/v0.1.0/glade-plugin-performance_0.1.0_darwin_arm64.tar.gz",
          "sha256": "64 lowercase hex characters"
        }
      ]
    }
  ]
}
```

Use `GLADE_PLUGIN_REGISTRY_URL` in tests and local development. Default to `https://plugins.glade.sh/index.json` in production code.

- [ ] **Step 2: Write registry tests with `httptest`**

Test cases:

- named plugin resolves the current OS/arch asset.
- missing plugin returns `plugin "x" was not found in registry`.
- checksum mismatch fails before extraction.
- install by name writes `installed.json`.

Use a local tar.gz archive body from Task 8 and serve it through `httptest.Server`.

- [ ] **Step 3: Implement registry install**

Add `RegistryIndex`, `RegistryPlugin`, and `RegistryAsset` types in `/Users/matt/Dev/glade/internal/pluginhost/registry.go`.

Add:

```go
func RegistryURL() string
func FetchRegistry(ctx context.Context, url string) (RegistryIndex, error)
func (idx RegistryIndex) AssetFor(name, goos, goarch string) (RegistryPlugin, RegistryAsset, bool)
func (s Store) InstallFromRegistry(ctx context.Context, name string) (InstalledPlugin, error)
```

Download assets to:

```text
~/.glade/plugins/downloads/<name>-<version>-<os>-<arch>.tar.gz
```

Verify the asset SHA-256 before calling `InstallArchive`.

- [ ] **Step 4: Let `plugins install` accept names**

In `/Users/matt/Dev/glade/internal/gladecli/plugins_command.go`, detect local files first:

```go
	if _, statErr := os.Stat(args[0]); statErr == nil {
		plugin, err := pluginhost.NewStore(pluginhost.DefaultRoot()).InstallArchive(ctx, args[0])
		...
	}
	plugin, err := pluginhost.NewStore(pluginhost.DefaultRoot()).InstallFromRegistry(ctx, args[0])
```

- [ ] **Step 5: Run tests**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost ./internal/gladecli -run 'TestRegistry|TestInstall' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/pluginhost
ok  	github.com/glade-sh/glade/internal/gladecli
```

- [ ] **Step 6: Commit**

```bash
cd /Users/matt/Dev/glade
git add internal/pluginhost/registry.go internal/pluginhost/registry_test.go internal/pluginhost/install.go internal/gladecli/plugins_command.go
git commit -m "feat: install glade plugins from registry"
```

## Task 10: Package Glade Tools As First-Party Plugins

**Files:**
- Create: `/Users/matt/Dev/glade-tools/scripts/build-plugin-archives.sh`
- Create: `/Users/matt/Dev/glade-tools/plugins/compat/plugin.json`
- Create: `/Users/matt/Dev/glade-tools/plugins/performance/plugin.json`
- Modify: `/Users/matt/Dev/glade-tools/README.md`
- Modify: `/Users/matt/Dev/glade-tools/.gitignore` if release artifacts need ignoring.

- [ ] **Step 1: Add source manifests**

Add `/Users/matt/Dev/glade-tools/plugins/compat/plugin.json` with the same contents returned by `glade-plugin-compat manifest --json`.

Add `/Users/matt/Dev/glade-tools/plugins/performance/plugin.json` with the same contents returned by `glade-plugin-performance manifest --json`.

- [ ] **Step 2: Add build script**

Add `/Users/matt/Dev/glade-tools/scripts/build-plugin-archives.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

version="${1:?usage: scripts/build-plugin-archives.sh <version>}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out="$root/dist/plugins"
mkdir -p "$out"

platforms=(
  "darwin arm64"
  "darwin amd64"
  "linux arm64"
  "linux amd64"
)

plugins=(
  "compat ./cmd/glade-plugin-compat plugins/compat/plugin.json"
  "performance ./cmd/glade-plugin-performance plugins/performance/plugin.json"
)

for platform in "${platforms[@]}"; do
  read -r goos goarch <<<"$platform"
  for plugin in "${plugins[@]}"; do
    read -r name package manifest <<<"$plugin"
    work="$(mktemp -d)"
    mkdir -p "$work/bin"
    GOOS="$goos" GOARCH="$goarch" go build -o "$work/bin/glade-plugin-$name" "$root/$package"
    cp "$root/$manifest" "$work/plugin.json"
    (
      cd "$work"
      shasum -a 256 plugin.json "bin/glade-plugin-$name" > checksums.txt
      tar -czf "$out/glade-plugin-${name}_${version}_${goos}_${goarch}.tar.gz" plugin.json checksums.txt "bin/glade-plugin-$name"
    )
    rm -rf "$work"
  done
done
(
  cd "$out"
  shasum -a 256 *.tar.gz > SHA256SUMS
)
```

- [ ] **Step 3: Run packaging**

```bash
cd /Users/matt/Dev/glade-tools
chmod +x scripts/build-plugin-archives.sh
scripts/build-plugin-archives.sh 0.1.0
ls dist/plugins
```

Expected output includes:

```text
glade-plugin-compat_0.1.0_darwin_arm64.tar.gz
glade-plugin-performance_0.1.0_darwin_arm64.tar.gz
SHA256SUMS
```

- [ ] **Step 4: Install local archives through Glade**

```bash
cd /Users/matt/Dev/glade
tmp="$(mktemp -d)"
compat_archive="/Users/matt/Dev/glade-tools/dist/plugins/glade-plugin-compat_0.1.0_$(go env GOOS)_$(go env GOARCH).tar.gz"
performance_archive="/Users/matt/Dev/glade-tools/dist/plugins/glade-plugin-performance_0.1.0_$(go env GOOS)_$(go env GOARCH).tar.gz"
GLADE_HOME="$tmp" go run ./cmd/glade plugins install "$compat_archive"
GLADE_HOME="$tmp" go run ./cmd/glade plugins install "$performance_archive"
GLADE_HOME="$tmp" go run ./cmd/glade compat local-tests --help
GLADE_HOME="$tmp" go run ./cmd/glade performance scan --help
```

Expected output contains:

```text
Report local Apex test execution readiness.
Scan Salesforce projects for advisory performance risks.
```

- [ ] **Step 5: Commit**

```bash
cd /Users/matt/Dev/glade-tools
git add scripts/build-plugin-archives.sh plugins/compat/plugin.json plugins/performance/plugin.json README.md .gitignore
git commit -m "build: package first-party plugin archives"
```

Do not commit `dist/plugins` archives unless release policy says to commit built artifacts.

## Task 11: Add Plugin Removal, Which, And Doctor

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/pluginhost/store.go`
- Create: `/Users/matt/Dev/glade/internal/pluginhost/doctor.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/plugins_command.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`

- [ ] **Step 1: Add tests**

Add tests covering:

- `glade plugins which compat` prints the owning plugin and executable.
- `glade plugins remove compat` removes it from `installed.json`.
- `glade plugins doctor` fails when executable is missing.
- `glade plugins doctor` succeeds when executable responds to `manifest --json`.

- [ ] **Step 2: Implement store removal**

Add:

```go
func (s Store) Remove(name string) error
```

Rules:

- Remove the entry from `installed.json`.
- If the plugin is not linked, remove `~/.glade/plugins/<name>`.
- If the plugin is linked, do not delete the executable.

- [ ] **Step 3: Implement `which`**

`glade plugins which compat` reads installed state, calls `FindByCommandRoot`, and prints:

```text
compat is provided by compat 0.1.0
executable: /path/to/glade-plugin-compat
```

- [ ] **Step 4: Implement `doctor`**

For each installed plugin:

- stat the executable.
- run `manifest --json`.
- validate the manifest.
- verify at least one command root in installed state still appears in the manifest.

Output:

```text
compat 0.1.0 ok
```

Missing executable output:

```text
compat 0.1.0 missing executable: /path/to/glade-plugin-compat
```

- [ ] **Step 5: Run tests**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost ./internal/gladecli -run 'TestPluginsWhich|TestPluginsRemove|TestPluginsDoctor' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/pluginhost
ok  	github.com/glade-sh/glade/internal/gladecli
```

- [ ] **Step 6: Commit**

```bash
cd /Users/matt/Dev/glade
git add internal/pluginhost/store.go internal/pluginhost/doctor.go internal/gladecli/plugins_command.go internal/gladecli/cli_test.go
git commit -m "feat: manage installed glade plugins"
```

## Task 12: Add Plugin Lock And Restore For CI

**Files:**
- Create: `/Users/matt/Dev/glade/internal/pluginhost/lock.go`
- Create: `/Users/matt/Dev/glade/internal/pluginhost/lock_test.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/plugins_command.go`
- Create: `/Users/matt/Dev/glade/docs/PLUGINS.md`

- [ ] **Step 1: Define lock file**

Project lock path:

```text
glade.plugins.lock.json
```

Shape:

```json
{
  "version": 1,
  "plugins": [
    {
      "name": "compat",
      "version": "0.1.0",
      "source": "registry:compat",
      "commands": ["compat", "surface", "local-tests"]
    },
    {
      "name": "performance",
      "version": "0.1.0",
      "source": "registry:performance",
      "commands": ["performance"]
    }
  ]
}
```

- [ ] **Step 2: Implement `glade plugins lock`**

Command:

```bash
glade plugins lock
```

Writes `glade.plugins.lock.json` in the current working directory from installed state. It excludes linked plugins unless `--include-linked` is passed. Linked plugins are not reproducible in CI by default.

- [ ] **Step 3: Implement `glade plugins restore`**

Command:

```bash
glade plugins restore
```

Reads `glade.plugins.lock.json` and installs each plugin from the registry by name and version. Version pinning must be exact.

- [ ] **Step 4: Add tests**

Use a temp working directory and temp `GLADE_HOME`. Use `httptest` registry from Task 9. Assert:

- lock writes installed plugin names and versions.
- restore installs the exact version.
- linked plugins are skipped without `--include-linked`.

- [ ] **Step 5: Run tests**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost ./internal/gladecli -run 'TestPluginLock|TestPluginsRestore' -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/pluginhost
ok  	github.com/glade-sh/glade/internal/gladecli
```

- [ ] **Step 6: Commit**

```bash
cd /Users/matt/Dev/glade
git add internal/pluginhost/lock.go internal/pluginhost/lock_test.go internal/gladecli/plugins_command.go docs/PLUGINS.md
git commit -m "feat: add glade plugin lock restore"
```

## Task 13: Migrate Product Docs To Plugin UX

**Files:**
- Modify: `/Users/matt/Dev/glade/README.md`
- Modify: `/Users/matt/Dev/glade/docs/ARCHITECTURE.md`
- Modify: `/Users/matt/Dev/glade/docs/COMPATIBILITY.md`
- Modify: `/Users/matt/Dev/glade/docs/LOCAL_TESTING.md`
- Modify: `/Users/matt/Dev/glade/docs/DOGFOOD_CHECKLIST.md`
- Modify: `/Users/matt/Dev/glade/docs/INSTALL.md`
- Modify: `/Users/matt/Dev/glade-tools/README.md`

- [ ] **Step 1: Search old wording**

```bash
cd /Users/matt/Dev/glade
rg -n "glade-tools|~/Dev/glade-tools|sibling .*tools|compat surface|glade compat" README.md docs internal/cliui internal/gladecli
```

Record each live product doc reference. Do not edit generated reports unless the text is part of the public narrative.

- [ ] **Step 2: Rewrite usage examples**

Replace:

```bash
glade-tools local-tests --project . --parallel auto --json
```

with:

```bash
glade plugins install @glade/compat
glade compat local-tests --project . --parallel auto --json
```

Replace surface-ledger examples with:

```bash
glade plugins install @glade/compat
glade surface refresh --docs "$GLADE_SALESFORCE_DOCS_SOURCE" --out tmp/surface
```

Replace advisory performance scan examples with:

```bash
glade plugins install @glade/performance
glade performance scan --project . --json > reports/glade-performance.json
glade performance scan --project . --trace reports/slow-test-trace.json > reports/glade-performance.md
```

Replace describe import examples with the base product command:

```bash
glade schema import describe --input reports/org-describe.json --output schema/local.schema.json
```

- [ ] **Step 3: Preserve product boundary language**

Docs must say:

```text
The product repo contains the plugin manager, command router, runtime, local schema import, and product test runner. Maintenance scanners, advisory performance scans, docs inventory, fixtures, and generated ledgers ship as plugins and do not live in the product runtime packages.
```

- [ ] **Step 4: Add `glade-tools` migration note**

In `/Users/matt/Dev/glade-tools/README.md`, say:

```text
This repository builds the first-party compat plugin. The old `glade-tools` binary remains as a migration wrapper. New usage goes through `glade plugins install @glade/compat` or `glade plugins link --exec <path>`.
```

- [ ] **Step 5: Run doc and guard checks**

```bash
cd /Users/matt/Dev/glade
rg -n "glade-tools" README.md docs | tee /tmp/glade-tools-doc-refs.txt
go test ./internal/gladecli ./internal/repoguard -count=1
```

Expected:

- Remaining `glade-tools` hits are migration notes or generated-report provenance.
- Tests pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/matt/Dev/glade
git add README.md docs/ARCHITECTURE.md docs/COMPATIBILITY.md docs/LOCAL_TESTING.md docs/DOGFOOD_CHECKLIST.md docs/INSTALL.md
git commit -m "docs: describe plugin-based maintenance tooling"
```

```bash
cd /Users/matt/Dev/glade-tools
git add README.md
git commit -m "docs: document compat plugin migration"
```

## Task 14: Add Repo Guards For The New Boundary

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/repoguard/repo_standards_test.go`
- Create: `/Users/matt/Dev/glade/internal/repoguard/testdata/plugin_boundary_allowlist.txt` if the current guard style uses testdata.

- [ ] **Step 1: Add guard test cases**

Guard rules:

- `glade` may contain `internal/pluginhost`.
- `glade` may contain `docs/PLUGINS.md`.
- `glade` must not contain `docs/fixtures`.
- `glade` must not contain `internal/apexdocs`.
- `glade` must not contain `internal/perfscan`.
- `glade` must not contain `internal/surfaceledger`.
- `glade` must not contain `internal/projectscan`.
- `glade` must not contain `internal/compat`.
- `glade` must not import `github.com/glade-sh/glade/tools`.
- `glade` public help must not list `inspect performance`.
- `glade` public docs must route performance scans through `glade performance scan`.

- [ ] **Step 2: Run guard test and verify current state**

```bash
cd /Users/matt/Dev/glade
go test ./internal/repoguard -run Test -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/repoguard
```

- [ ] **Step 3: Commit**

```bash
cd /Users/matt/Dev/glade
git add internal/repoguard/repo_standards_test.go internal/repoguard/testdata/plugin_boundary_allowlist.txt
git commit -m "test: guard plugin maintenance boundary"
```

## Task 15: Final Verification Packet

**Files:**
- No planned source edits.

- [ ] **Step 1: Verify Glade plugin host**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost ./internal/gladecli ./internal/repoguard -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/internal/pluginhost
ok  	github.com/glade-sh/glade/internal/gladecli
ok  	github.com/glade-sh/glade/internal/repoguard
```

- [ ] **Step 2: Verify Glade smoke**

```bash
cd /Users/matt/Dev/glade
scripts/smoke.sh
```

Expected: exits 0.

- [ ] **Step 3: Verify no plugin installed behavior**

```bash
cd /Users/matt/Dev/glade
tmp="$(mktemp -d)"
GLADE_HOME="$tmp" go run ./cmd/glade plugins list
GLADE_HOME="$tmp" go run ./cmd/glade compat
```

Expected:

```text
No plugins installed.
```

and:

```text
unknown command "compat"
```

- [ ] **Step 4: Verify linked plugin behavior**

```bash
cd /Users/matt/Dev/glade-tools
go build -o /tmp/glade-plugin-compat ./cmd/glade-plugin-compat
go build -o /tmp/glade-plugin-performance ./cmd/glade-plugin-performance

cd /Users/matt/Dev/glade
tmp="$(mktemp -d)"
GLADE_HOME="$tmp" go run ./cmd/glade plugins link --exec /tmp/glade-plugin-compat
GLADE_HOME="$tmp" go run ./cmd/glade plugins link --exec /tmp/glade-plugin-performance
GLADE_HOME="$tmp" go run ./cmd/glade plugins list
GLADE_HOME="$tmp" go run ./cmd/glade compat local-tests --help
GLADE_HOME="$tmp" go run ./cmd/glade surface --help
GLADE_HOME="$tmp" go run ./cmd/glade performance scan --help
```

Expected:

- `plugins link` prints `Linked plugin compat 0.1.0`.
- `plugins link` prints `Linked plugin performance 0.1.0`.
- `plugins list` prints `compat 0.1.0 linked` and `performance 0.1.0 linked`.
- `compat local-tests --help` prints local-test help.
- `surface --help` prints surface help.
- `performance scan --help` prints performance scan help.

- [ ] **Step 5: Verify archive install behavior**

```bash
cd /Users/matt/Dev/glade-tools
scripts/build-plugin-archives.sh 0.1.0

cd /Users/matt/Dev/glade
tmp="$(mktemp -d)"
compat_archive="/Users/matt/Dev/glade-tools/dist/plugins/glade-plugin-compat_0.1.0_$(go env GOOS)_$(go env GOARCH).tar.gz"
performance_archive="/Users/matt/Dev/glade-tools/dist/plugins/glade-plugin-performance_0.1.0_$(go env GOOS)_$(go env GOARCH).tar.gz"
GLADE_HOME="$tmp" go run ./cmd/glade plugins install "$compat_archive"
GLADE_HOME="$tmp" go run ./cmd/glade plugins install "$performance_archive"
GLADE_HOME="$tmp" go run ./cmd/glade plugins doctor
GLADE_HOME="$tmp" go run ./cmd/glade compat local-tests --help
GLADE_HOME="$tmp" go run ./cmd/glade performance scan --help
```

Expected:

- install prints `Installed plugin compat 0.1.0`.
- install prints `Installed plugin performance 0.1.0`.
- doctor prints `compat 0.1.0 ok` and `performance 0.1.0 ok`.
- compat help prints local-test help.
- performance help prints performance scan help.

- [ ] **Step 6: Verify glade-tools still works during migration**

```bash
cd /Users/matt/Dev/glade-tools
go run ./cmd/glade-tools --help
go run ./cmd/glade-plugin-compat manifest --json
go run ./cmd/glade-plugin-performance manifest --json
go test ./...
```

Expected:

- old help still prints.
- manifest JSON contains `"apiVersion": "glade.plugin.v1"`.
- performance manifest JSON contains `"name": "performance"`.
- tests pass.

- [ ] **Step 7: Capture final status**

```bash
cd /Users/matt/Dev/glade
git status --short

cd /Users/matt/Dev/glade-tools
git status --short
```

Expected: only intentional files are modified.

## Rollout Order

1. Land Tasks 1-5 in `/Users/matt/Dev/glade`.
2. Land Task 6 in `/Users/matt/Dev/glade-tools`.
3. Prove Task 7 end-to-end with a linked compat binary.
4. Land Tasks 7A-7D to correct the product/plugin boundary.
5. Land Tasks 8-12 in `/Users/matt/Dev/glade`.
6. Land Task 10 in `/Users/matt/Dev/glade-tools`.
7. Rewrite docs in Task 13.
8. Land guard and final verification in Tasks 14-15.

## Risks And Counters

- `glade-tools` imports `github.com/glade-sh/glade/internal/...`. Keep its module path under `github.com/glade-sh/glade/tools` until public SDK packages exist. External plugins must use process execution or future public APIs, not Glade internals.
- Plugin command roots can collide with product commands. The manifest validator rejects all core roots.
- Third-party plugins are arbitrary executables. `plugins install` must show source, version, checksum, and path before install once interactive prompts exist. CI can pass `--yes`.
- Registry availability can block installs. `plugins lock` plus local archive install gives CI a dry path.
- Big fixture data can bloat archives. Keep data packs inside plugin releases, not in `/Users/matt/Dev/glade`.
- Moving `inspect performance` breaks the old product command. This repo is pre-release; prefer the clean plugin boundary and make the error message point to `glade plugins install @glade/performance`.
- Moving `apexdocs` can break `glade-tools` imports because the module path still sits under `github.com/glade-sh/glade/tools`. Rewrite imports in one pass and run the focused package set before touching docs.
- Local-test selection can fork if copied twice. Move product-grade selectors into `glade test`, then let readiness plugin code call or mirror only the product selector contract.
- Describe catalog import is base only when it is file-to-local-schema conversion. Any live Salesforce org capture or authentication must remain a plugin.
- Old docs can drift. Run `rg -n "glade-tools|glade compat|compat surface"` after docs migration.

## Done State

The work is complete when these commands tell the same story:

```bash
cd /Users/matt/Dev/glade
tmp="$(mktemp -d)"
GLADE_HOME="$tmp" go run ./cmd/glade plugins list
GLADE_HOME="$tmp" go run ./cmd/glade compat
```

Output:

```text
No plugins installed.
error[GLADECLI001]: unknown command "compat"
```

Then:

```bash
cd /Users/matt/Dev/glade-tools
go build -o /tmp/glade-plugin-compat ./cmd/glade-plugin-compat
go build -o /tmp/glade-plugin-performance ./cmd/glade-plugin-performance

cd /Users/matt/Dev/glade
GLADE_HOME="$tmp" go run ./cmd/glade plugins link --exec /tmp/glade-plugin-compat
GLADE_HOME="$tmp" go run ./cmd/glade plugins link --exec /tmp/glade-plugin-performance
GLADE_HOME="$tmp" go run ./cmd/glade plugins list
GLADE_HOME="$tmp" go run ./cmd/glade compat local-tests --help
GLADE_HOME="$tmp" go run ./cmd/glade performance scan --help
```

Output:

```text
compat 0.1.0 linked
performance 0.1.0 linked
Report local Apex test execution readiness.
Scan Salesforce projects for advisory performance risks.
```

One command face. Separate tool weight. The door swings clean.
