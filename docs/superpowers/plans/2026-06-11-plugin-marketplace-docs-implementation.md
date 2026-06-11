# Plugin Marketplace And Docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement scoped Glade plugin coordinates, curated marketplace catalog installs, third-party registry/archive install paths, first-party plugin registry output, and a cohesive main docs-site plugin section.

**Architecture:** Keep the installed plugin runtime executable-based. Add a catalog layer above archive installation: scoped package coordinates identify marketplace entries, while plugin manifests keep path-safe runtime names such as `compat` and `performance`. Implement in parallel squads with non-overlapping files first, then integrate CLI and lock behavior after the pluginhost APIs are in place.

**Tech Stack:** Go 1.26, standard-library HTTP/JSON/tar/gzip/SHA-256, VitePress docs, bash for release archive builds, no npm transport for plugins.

---

## Controller Instructions For GPT-5.5 Medium

Run this from a clean local `main` in `/Users/matt/Dev/glade`. Use one controller agent and parallel implementation subagents. The user explicitly wants parallel subagents; use the parallel lanes below and do not let two agents edit the same file set at the same time.

Create isolated work:

```bash
cd /Users/matt/Dev/glade
git status --short --branch
git worktree add .worktrees/plugin-marketplace-docs -b codex/plugin-marketplace-docs main
cd .worktrees/plugin-marketplace-docs
```

For `/Users/matt/Dev/glade-tools`, create a branch in that repo:

```bash
cd /Users/matt/Dev/glade-tools
git status --short --branch
git switch -c codex/plugin-marketplace-docs
```

If either command reports local changes, stop and report the exact files before editing.

## Parallel Squad Map

Dispatch Wave 1 in parallel:

- **Squad A:** Pluginhost catalog foundations. Owns `internal/pluginhost/*`.
- **Squad C:** Product docs and VitePress information architecture. Owns `docs/*`, `site/*`, `README.md`.
- **Squad D:** `glade-tools` first-party registry index generation. Owns `/Users/matt/Dev/glade-tools/*`.

After Squad A lands, dispatch Wave 2:

- **Squad B:** Product CLI wiring. Owns `internal/gladecli/*` and `internal/cliui/help.go`.
- **Squad A2:** Lock-file canonicalization and restore. Owns `internal/pluginhost/lock*`.

After Waves 1 and 2 land, dispatch Wave 3:

- **Squad I:** Integration smoke and cross-repo polish. Owns only fixes required by failing integration tests.

Review every squad with two checks:

1. Spec compliance: compare the patch to `docs/superpowers/specs/2026-06-11-plugin-marketplace-docs-design.md`.
2. Code quality: small boundaries, no npm transport, no hidden dependency on `/Users/matt/Dev/glade-tools` from the product repo.

Commit after each squad. Use these commit messages:

- `feat: add plugin marketplace catalog model`
- `feat: add plugin marketplace cli commands`
- `docs: restructure plugin marketplace docs`
- `feat: emit first-party plugin registry index`
- `feat: canonicalize plugin lock files`
- `test: prove plugin marketplace integration`

## File Responsibility Map

Product repo `/Users/matt/Dev/glade`:

- Create `internal/pluginhost/coordinate.go`: parse and validate plugin coordinates such as `@glade/compat@0.1.0`.
- Create `internal/pluginhost/coordinate_test.go`: parser and alias tests.
- Modify `internal/pluginhost/model.go`: registry metadata fields, installed plugin metadata, lock metadata.
- Modify `internal/pluginhost/registry.go`: marketplace search/info, alias resolution, custom registry URL, catalog/manifest validation, scoped storage keys.
- Modify `internal/pluginhost/registry_test.go`: marketplace install and search tests.
- Modify `internal/pluginhost/install.go`: add `InstallArchiveWithOptions` while preserving `InstallArchive`.
- Create `internal/pluginhost/remote.go`: direct URL archive download with required SHA-256.
- Create `internal/pluginhost/remote_test.go`: remote archive install tests.
- Modify `internal/pluginhost/store.go`: installed plugin identity matching and scoped storage deletion.
- Modify `internal/pluginhost/lock.go`: canonical lock fields and exact restore.
- Modify `internal/pluginhost/lock_test.go`: canonical lock and restore tests.
- Modify `internal/gladecli/cli.go`: pass stderr into `runPlugins`.
- Modify `internal/gladecli/plugins_command.go`: `search`, `info`, install flags, remote URL installs, `--registry`, `--sha256`, `--yes`, trust warnings.
- Modify `internal/gladecli/cli_test.go`: CLI contracts for search/info/install flags.
- Modify `internal/cliui/help.go`: scoped plugin examples.
- Modify `docs/PLUGINS.md`, `docs/INSTALL.md`, `docs/LOCAL_TESTING.md`, `docs/APEX_PARSER.md`, `docs/DOGFOOD_CHECKLIST.md`, `docs/POST_PARITY_TODO.md`, `README.md`: replace short-only install examples with canonical `@glade/*` installs while keeping alias notes.
- Modify `site/.vitepress/config.ts`: add a Plugins sidebar group.
- Replace `site/docs-src/guide/plugins.md`: overview page.
- Create `site/docs-src/guide/plugins/first-party.md`.
- Create `site/docs-src/guide/plugins/marketplace.md`.
- Create `site/docs-src/guide/plugins/install-manage.md`.
- Create `site/docs-src/guide/plugins/build.md`.
- Create `site/docs-src/guide/plugins/publish.md`.
- Create `site/docs-src/guide/plugins/manifest.md`.
- Create `site/docs-src/guide/plugins/lock-ci.md`.

Tools repo `/Users/matt/Dev/glade-tools`:

- Modify `scripts/build-plugin-archives.sh`: optionally write `index.json` when `PLUGIN_ASSET_BASE_URL` is set.
- Modify `README.md`: document canonical `@glade/*` installs and registry index output.
- Modify `plugins/compat/plugin.json` and `plugins/performance/plugin.json` only if docs URLs or source URLs need to be added to manifest output. Do not change manifest `name`; it remains path-safe.

## Data Model Rules

Use these names. Do not invent alternate fields.

```go
type PluginRef struct {
	Name    string
	Version string
}

type InstallOptions struct {
	CanonicalName string
	RegistryURL   string
	Publisher     string
	Trust         string
	AssetSHA256   string
	AssetOS       string
	AssetArch     string
	Source        string
}
```

Add fields to `InstalledPlugin`:

```go
CanonicalName string `json:"canonicalName,omitempty"`
StorageName   string `json:"storageName,omitempty"`
Registry      string `json:"registry,omitempty"`
Publisher     string `json:"publisher,omitempty"`
Trust         string `json:"trust,omitempty"`
AssetSHA256   string `json:"assetSha256,omitempty"`
AssetOS       string `json:"assetOS,omitempty"`
AssetArch     string `json:"assetArch,omitempty"`
```

Add fields to `RegistryPlugin`:

```go
Aliases             []string `json:"aliases,omitempty"`
Publisher           string   `json:"publisher,omitempty"`
Trust               string   `json:"trust,omitempty"`
Commands            []string `json:"commands,omitempty"`
DocsURL             string   `json:"docsURL,omitempty"`
SourceURL           string   `json:"sourceURL,omitempty"`
MinimumGladeVersion string   `json:"minimumGladeVersion,omitempty"`
```

Add fields to `LockedPlugin`:

```go
Registry   string `json:"registry,omitempty"`
OS         string `json:"os,omitempty"`
Arch       string `json:"arch,omitempty"`
SHA256     string `json:"sha256,omitempty"`
Trust      string `json:"trust,omitempty"`
Publisher  string `json:"publisher,omitempty"`
```

Keep `Manifest.Name` path-safe. A marketplace name like `@glade/compat` maps to runtime manifest name `compat`. Reject catalog installs when `manifest.Name` does not match the package segment of the canonical coordinate.

## Task 1: Coordinate Parser And Alias Resolution

**Parallel lane:** Squad A.

**Files:**
- Create: `internal/pluginhost/coordinate.go`
- Create: `internal/pluginhost/coordinate_test.go`

- [ ] **Step 1: Write failing parser tests**

Create `internal/pluginhost/coordinate_test.go`:

```go
package pluginhost

import "testing"

func TestParsePluginRefAcceptsScopedNamesAndVersions(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantVersion string
	}{
		{input: "@glade/compat", wantName: "@glade/compat"},
		{input: "@glade/compat@0.1.0", wantName: "@glade/compat", wantVersion: "0.1.0"},
		{input: "@acme/quality-tools@1.2.3", wantName: "@acme/quality-tools", wantVersion: "1.2.3"},
		{input: "compat", wantName: "@glade/compat"},
		{input: "performance", wantName: "@glade/performance"},
	}
	for _, tt := range tests {
		got, err := ParsePluginRef(tt.input)
		if err != nil {
			t.Fatalf("ParsePluginRef(%q) error: %v", tt.input, err)
		}
		if got.Name != tt.wantName || got.Version != tt.wantVersion {
			t.Fatalf("ParsePluginRef(%q)=%#v, want name=%q version=%q", tt.input, got, tt.wantName, tt.wantVersion)
		}
	}
}

func TestParsePluginRefRejectsUnsafeNames(t *testing.T) {
	for _, input := range []string{
		"",
		"@",
		"@glade",
		"@glade/",
		"@glade/../compat",
		"@glade/com/pat",
		"@glade/compat@../1.0.0",
		"../compat",
		"https://example.com/plugin.tar.gz",
	} {
		if _, err := ParsePluginRef(input); err == nil {
			t.Fatalf("ParsePluginRef(%q) succeeded, want error", input)
		}
	}
}

func TestPluginRefManifestNameAndStorageKey(t *testing.T) {
	ref, err := ParsePluginRef("@acme/quality@1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if got := ref.ManifestName(); got != "quality" {
		t.Fatalf("ManifestName=%q, want quality", got)
	}
	if got := ref.StorageName(); got != "acme__quality" {
		t.Fatalf("StorageName=%q, want acme__quality", got)
	}
}
```

- [ ] **Step 2: Run the red test**

```bash
go test ./internal/pluginhost -run 'TestParsePluginRef|TestPluginRef' -count=1
```

Expected: compile failure because `ParsePluginRef` and `PluginRef` do not exist.

- [ ] **Step 3: Implement the parser**

Create `internal/pluginhost/coordinate.go`:

```go
package pluginhost

import (
	"fmt"
	"strings"
)

type PluginRef struct {
	Name    string
	Version string
}

var firstPartyAliases = map[string]string{
	"compat":      "@glade/compat",
	"performance": "@glade/performance",
}

func ParsePluginRef(input string) (PluginRef, error) {
	raw := strings.TrimSpace(input)
	if raw == "" || strings.Contains(raw, "://") || strings.ContainsAny(raw, `/\`) && !strings.HasPrefix(raw, "@") {
		return PluginRef{}, fmt.Errorf("invalid plugin coordinate %q", input)
	}
	if canonical, ok := firstPartyAliases[raw]; ok {
		raw = canonical
	}
	name, version := raw, ""
	if strings.HasPrefix(raw, "@") {
		if at := strings.LastIndex(raw[1:], "@"); at >= 0 {
			split := at + 1
			name = raw[:split]
			version = raw[split+1:]
		}
	} else if at := strings.LastIndex(raw, "@"); at > 0 {
		name = raw[:at]
		version = raw[at+1:]
	}
	ref := PluginRef{Name: name, Version: version}
	if err := ref.Validate(); err != nil {
		return PluginRef{}, err
	}
	return ref, nil
}

func (r PluginRef) Validate() error {
	if strings.HasPrefix(r.Name, "@") {
		parts := strings.Split(r.Name[1:], "/")
		if len(parts) != 2 {
			return fmt.Errorf("plugin coordinate %q must be @scope/name", r.Name)
		}
		if err := validatePluginPathToken("plugin scope", parts[0]); err != nil {
			return err
		}
		if err := validatePluginPathToken("plugin package", parts[1]); err != nil {
			return err
		}
	} else if err := validatePluginPathToken("plugin name", r.Name); err != nil {
		return err
	}
	if r.Version != "" {
		if err := validatePluginPathToken("plugin version", r.Version); err != nil {
			return err
		}
	}
	return nil
}

func (r PluginRef) ManifestName() string {
	if strings.HasPrefix(r.Name, "@") {
		return strings.SplitN(r.Name[1:], "/", 2)[1]
	}
	return r.Name
}

func (r PluginRef) StorageName() string {
	if strings.HasPrefix(r.Name, "@") {
		parts := strings.SplitN(r.Name[1:], "/", 2)
		return parts[0] + "__" + parts[1]
	}
	return r.Name
}
```

- [ ] **Step 4: Run the green test**

```bash
go test ./internal/pluginhost -run 'TestParsePluginRef|TestPluginRef' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pluginhost/coordinate.go internal/pluginhost/coordinate_test.go
git commit -m "feat: add plugin coordinate parser"
```

## Task 2: Marketplace Catalog Model And Search

**Parallel lane:** Squad A after Task 1, or same Squad A if no second agent is available.

**Files:**
- Modify: `internal/pluginhost/model.go`
- Modify: `internal/pluginhost/registry.go`
- Modify: `internal/pluginhost/registry_test.go`

- [ ] **Step 1: Write failing catalog tests**

Append to `internal/pluginhost/registry_test.go`:

```go
func TestRegistryFindsScopedNameAliasAndSearchResults(t *testing.T) {
	index := RegistryIndex{Version: 1, Plugins: []RegistryPlugin{{
		Name:      "@glade/compat",
		Aliases:   []string{"compat"},
		Version:   "0.1.0",
		Publisher: "glade",
		Trust:     "first-party",
		Summary:   "Compatibility fixtures.",
		Commands:  []string{"compat", "surface"},
		DocsURL:   "https://glade.sh/guide/plugins/compat",
		Assets: []RegistryAsset{{
			OS: runtime.GOOS, Arch: runtime.GOARCH, URL: "https://example.test/compat.tar.gz", SHA256: strings.Repeat("a", 64),
		}},
	}}}
	ref, err := ParsePluginRef("compat")
	if err != nil {
		t.Fatal(err)
	}
	plugin, asset, ok := index.AssetForRef(ref, runtime.GOOS, runtime.GOARCH)
	if !ok {
		t.Fatal("expected compat alias to resolve")
	}
	if plugin.Name != "@glade/compat" || asset.URL == "" {
		t.Fatalf("unexpected plugin/asset: %#v %#v", plugin, asset)
	}
	results := index.Search("surface")
	if len(results) != 1 || results[0].Name != "@glade/compat" {
		t.Fatalf("unexpected search results: %#v", results)
	}
}

func TestRegistryUnsupportedPlatformErrorNamesAvailablePlatforms(t *testing.T) {
	index := RegistryIndex{Version: 1, Plugins: []RegistryPlugin{{
		Name: "@acme/quality", Version: "1.2.0",
		Assets: []RegistryAsset{{OS: "linux", Arch: "amd64", URL: "https://example.test/q.tar.gz", SHA256: strings.Repeat("b", 64)}},
	}}}
	ref, err := ParsePluginRef("@acme/quality")
	if err != nil {
		t.Fatal(err)
	}
	err = index.NotFoundErrorForRef(ref, runtime.GOOS, runtime.GOARCH)
	if err == nil || !strings.Contains(err.Error(), "available platforms: linux/amd64") {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run the red test**

```bash
go test ./internal/pluginhost -run 'TestRegistryFindsScopedNameAliasAndSearchResults|TestRegistryUnsupportedPlatformError' -count=1
```

Expected: compile failure for `AssetForRef`, `Search`, and `NotFoundErrorForRef`.

- [ ] **Step 3: Add registry metadata fields**

Modify `internal/pluginhost/model.go`:

```go
type RegistryPlugin struct {
	Name                string          `json:"name"`
	Aliases             []string        `json:"aliases,omitempty"`
	Version             string          `json:"version"`
	Publisher           string          `json:"publisher,omitempty"`
	Trust               string          `json:"trust,omitempty"`
	Summary             string          `json:"summary,omitempty"`
	Commands            []string        `json:"commands,omitempty"`
	DocsURL             string          `json:"docsURL,omitempty"`
	SourceURL           string          `json:"sourceURL,omitempty"`
	MinimumGladeVersion string          `json:"minimumGladeVersion,omitempty"`
	Assets              []RegistryAsset `json:"assets"`
}
```

- [ ] **Step 4: Add registry lookup helpers**

Add to `internal/pluginhost/registry.go`:

```go
func (idx RegistryIndex) AssetForRef(ref PluginRef, goos, goarch string) (RegistryPlugin, RegistryAsset, bool) {
	for _, plugin := range idx.Plugins {
		if !registryPluginMatchesRef(plugin, ref) {
			continue
		}
		if ref.Version != "" && plugin.Version != ref.Version {
			continue
		}
		for _, asset := range plugin.Assets {
			if asset.OS == goos && asset.Arch == goarch {
				return plugin, asset, true
			}
		}
	}
	return RegistryPlugin{}, RegistryAsset{}, false
}

func (idx RegistryIndex) Search(query string) []RegistryPlugin {
	q := strings.ToLower(strings.TrimSpace(query))
	var out []RegistryPlugin
	for _, plugin := range idx.Plugins {
		haystack := strings.ToLower(strings.Join(append([]string{
			plugin.Name, plugin.Version, plugin.Publisher, plugin.Trust, plugin.Summary, plugin.DocsURL, plugin.SourceURL,
		}, append(plugin.Aliases, plugin.Commands...)...), " "))
		if q == "" || strings.Contains(haystack, q) {
			out = append(out, plugin)
		}
	}
	return out
}

func registryPluginMatchesRef(plugin RegistryPlugin, ref PluginRef) bool {
	if plugin.Name == ref.Name {
		return true
	}
	for _, alias := range plugin.Aliases {
		if alias == ref.Name || firstPartyAliases[alias] == ref.Name {
			return true
		}
	}
	return false
}

func (idx RegistryIndex) NotFoundErrorForRef(ref PluginRef, goos, goarch string) error {
	var platforms []string
	for _, plugin := range idx.Plugins {
		if !registryPluginMatchesRef(plugin, ref) {
			continue
		}
		for _, asset := range plugin.Assets {
			platforms = append(platforms, asset.OS+"/"+asset.Arch)
		}
	}
	if len(platforms) > 0 {
		return fmt.Errorf("plugin %q has no asset for %s/%s; available platforms: %s", ref.Name, goos, goarch, strings.Join(platforms, ", "))
	}
	return fmt.Errorf("plugin %q was not found in registry; run `glade plugins search %s`", ref.Name, ref.ManifestName())
}
```

Keep existing `AssetFor`, `AssetForVersion`, and `NotFoundError` wrappers for backward tests. Have them call the new helpers.

- [ ] **Step 5: Run full pluginhost tests**

```bash
go test ./internal/pluginhost -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/pluginhost/model.go internal/pluginhost/registry.go internal/pluginhost/registry_test.go
git commit -m "feat: add plugin marketplace catalog model"
```

## Task 3: Scoped Registry Install, Catalog Validation, And Remote Archive URLs

**Parallel lane:** Squad A. Start after Task 2.

**Files:**
- Modify: `internal/pluginhost/model.go`
- Modify: `internal/pluginhost/install.go`
- Create: `internal/pluginhost/remote.go`
- Create: `internal/pluginhost/remote_test.go`
- Modify: `internal/pluginhost/registry.go`
- Modify: `internal/pluginhost/registry_test.go`
- Modify: `internal/pluginhost/store.go`

- [ ] **Step 1: Write failing install metadata tests**

Append to `internal/pluginhost/registry_test.go`:

```go
func TestInstallFromRegistryScopedNameStoresCanonicalMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive executable mode is unix-specific")
	}
	root := t.TempDir()
	body := makePluginArchive(t, root, "compat", "0.1.0")
	sum := sha256.Sum256(body)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			fmt.Fprintf(w, `{"version":1,"plugins":[{"name":"@glade/compat","aliases":["compat"],"version":"0.1.0","publisher":"glade","trust":"first-party","docsURL":"https://glade.sh/guide/plugins/compat","assets":[{"os":%q,"arch":%q,"url":%q,"sha256":"%x"}]}]}`,
				runtime.GOOS, runtime.GOARCH, server.URL+"/compat.tar.gz", sum)
		case "/compat.tar.gz":
			w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("GLADE_PLUGIN_REGISTRY_URL", server.URL+"/index.json")
	store := NewStore(filepath.Join(root, "home"))

	plugin, err := store.InstallFromRegistry(context.Background(), "@glade/compat")
	if err != nil {
		t.Fatal(err)
	}
	if plugin.Name != "compat" || plugin.CanonicalName != "@glade/compat" || plugin.StorageName != "glade__compat" {
		t.Fatalf("unexpected plugin metadata: %#v", plugin)
	}
	if plugin.Registry != server.URL+"/index.json" || plugin.Trust != "first-party" || plugin.AssetSHA256 == "" {
		t.Fatalf("missing registry metadata: %#v", plugin)
	}
	if _, err := os.Stat(filepath.Join(root, "home", "plugins", "glade__compat", "0.1.0", "plugin.json")); err != nil {
		t.Fatalf("scoped storage missing: %v", err)
	}
}

func TestInstallFromRegistryRejectsCatalogManifestMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive executable mode is unix-specific")
	}
	root := t.TempDir()
	body := makePluginArchive(t, root, "wrong-name", "0.1.0")
	sum := sha256.Sum256(body)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			fmt.Fprintf(w, `{"version":1,"plugins":[{"name":"@acme/quality","version":"0.1.0","assets":[{"os":%q,"arch":%q,"url":%q,"sha256":"%x"}]}]}`,
				runtime.GOOS, runtime.GOARCH, server.URL+"/quality.tar.gz", sum)
		case "/quality.tar.gz":
			w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("GLADE_PLUGIN_REGISTRY_URL", server.URL+"/index.json")

	_, err := NewStore(filepath.Join(root, "home")).InstallFromRegistry(context.Background(), "@acme/quality")
	if err == nil || !strings.Contains(err.Error(), `manifest name "wrong-name" does not match catalog package "quality"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Write failing remote archive test**

Create `internal/pluginhost/remote_test.go`:

```go
package pluginhost

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallRemoteArchiveRequiresAndVerifiesSHA256(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive executable mode is unix-specific")
	}
	root := t.TempDir()
	body := makePluginArchive(t, root, "quality", "1.2.0")
	sum := sha256.Sum256(body)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer server.Close()
	store := NewStore(filepath.Join(root, "home"))

	if _, err := store.InstallRemoteArchive(context.Background(), server.URL+"/quality.tar.gz", "", InstallOptions{}); err == nil {
		t.Fatal("expected missing sha256 to fail")
	}
	plugin, err := store.InstallRemoteArchive(context.Background(), server.URL+"/quality.tar.gz", fmt.Sprintf("%x", sum), InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if plugin.Name != "quality" || !strings.HasPrefix(plugin.Source, "url:") || plugin.Trust != "unlisted" {
		t.Fatalf("unexpected plugin: %#v", plugin)
	}
}
```

- [ ] **Step 3: Run red tests**

```bash
go test ./internal/pluginhost -run 'TestInstallFromRegistryScopedNameStoresCanonicalMetadata|TestInstallFromRegistryRejectsCatalogManifestMismatch|TestInstallRemoteArchiveRequiresAndVerifiesSHA256' -count=1
```

Expected: compile failure for missing metadata fields, `InstallOptions`, and `InstallRemoteArchive`.

- [ ] **Step 4: Add install metadata fields**

Modify `internal/pluginhost/model.go` using the fields in the Data Model Rules section.

- [ ] **Step 5: Add `InstallArchiveWithOptions`**

Modify `internal/pluginhost/install.go`:

```go
type InstallOptions struct {
	CanonicalName string
	RegistryURL   string
	Publisher     string
	Trust         string
	AssetSHA256   string
	AssetOS       string
	AssetArch     string
	Source        string
	StorageName   string
}

func (s Store) InstallArchive(ctx context.Context, archivePath string) (InstalledPlugin, error) {
	return s.InstallArchiveWithOptions(ctx, archivePath, InstallOptions{})
}

func (s Store) InstallArchiveWithOptions(ctx context.Context, archivePath string, opts InstallOptions) (InstalledPlugin, error) {
	if err := ctx.Err(); err != nil {
		return InstalledPlugin{}, err
	}
	stagingParent := filepath.Join(s.root, "plugins", ".tmp")
	if err := os.MkdirAll(stagingParent, 0o755); err != nil {
		return InstalledPlugin{}, err
	}
	extracted, err := extractArchive(archivePath, stagingParent)
	if err != nil {
		return InstalledPlugin{}, err
	}
	defer func() {
		if extracted.dir != "" {
			os.RemoveAll(extracted.dir)
		}
	}()
	checksumsPath := filepath.Join(extracted.dir, "checksums.txt")
	checksums, err := readChecksums(checksumsPath)
	if err != nil {
		return InstalledPlugin{}, err
	}
	if err := verifyArchiveChecksums(extracted.dir, extracted.files, checksums); err != nil {
		return InstalledPlugin{}, err
	}
	manifestPath := filepath.Join(extracted.dir, "plugin.json")
	manifest, err := LoadManifestFromFile(manifestPath)
	if err != nil {
		return InstalledPlugin{}, err
	}
	exeName := "glade-plugin-" + manifest.Name
	relativeExe := filepath.ToSlash(filepath.Join("bin", exeName))
	if _, ok := checksums["plugin.json"]; !ok {
		return InstalledPlugin{}, fmt.Errorf("plugin archive missing checksum for plugin.json")
	}
	if _, ok := checksums[relativeExe]; !ok {
		return InstalledPlugin{}, fmt.Errorf("plugin archive missing checksum for %s", relativeExe)
	}
	extractedExe, ok := extracted.files[relativeExe]
	if !ok {
		return InstalledPlugin{}, fmt.Errorf("plugin archive missing executable %s", relativeExe)
	}
	if _, err := os.Stat(extractedExe.path); err != nil {
		return InstalledPlugin{}, err
	}
	storageName := opts.StorageName
	if storageName == "" {
		storageName = manifest.Name
	}
	if err := validatePluginPathToken("plugin storage name", storageName); err != nil {
		return InstalledPlugin{}, err
	}
	targetDir := filepath.Join(s.root, "plugins", storageName, manifest.Version)
	if err := os.RemoveAll(targetDir); err != nil {
		return InstalledPlugin{}, err
	}
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return InstalledPlugin{}, err
	}
	if err := os.Rename(extracted.dir, targetDir); err != nil {
		return InstalledPlugin{}, err
	}
	extracted.dir = ""
	executable := filepath.Join(targetDir, "bin", exeName)
	if err := os.Chmod(executable, extractedExe.mode|0o100); err != nil {
		return InstalledPlugin{}, err
	}
	source := opts.Source
	if source == "" {
		absArchive, err := filepath.Abs(archivePath)
		if err != nil {
			return InstalledPlugin{}, err
		}
		source = "archive:" + absArchive
	}
	plugin := InstalledPlugin{
		Name:          manifest.Name,
		CanonicalName: opts.CanonicalName,
		StorageName:   storageName,
		Version:       manifest.Version,
		Executable:    executable,
		Manifest:      filepath.Join(targetDir, "plugin.json"),
		Source:        source,
		Linked:        false,
		Commands:      manifest.CommandRoots(),
		Registry:      opts.RegistryURL,
		Publisher:     opts.Publisher,
		Trust:         opts.Trust,
		AssetSHA256:   opts.AssetSHA256,
		AssetOS:       opts.AssetOS,
		AssetArch:     opts.AssetArch,
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
```

This function is the full replacement for the current `InstallArchive` body. Keep `InstallArchive` as the wrapper shown above.

- [ ] **Step 6: Add storage-aware removal and replacement**

Modify `internal/pluginhost/store.go`:

```go
func (p InstalledPlugin) IdentityName() string {
	if p.CanonicalName != "" {
		return p.CanonicalName
	}
	return p.Name
}

func (p InstalledPlugin) StorageKey() string {
	if p.StorageName != "" {
		return p.StorageName
	}
	return p.Name
}
```

Use `StorageKey()` in `Remove`. Match removal requests against `Name`, `CanonicalName`, and first-party aliases:

```go
func installedPluginMatchesName(plugin InstalledPlugin, name string) bool {
	if plugin.Name == name || plugin.CanonicalName == name {
		return true
	}
	if canonical, ok := firstPartyAliases[name]; ok && plugin.CanonicalName == canonical {
		return true
	}
	return false
}
```

Update `replaceInstalled` to compare `IdentityName()`.

- [ ] **Step 7: Update registry install**

Modify `internal/pluginhost/registry.go`:

```go
func (s Store) InstallFromRegistryVersion(ctx context.Context, name, version string) (InstalledPlugin, error) {
	ref, err := ParsePluginRef(name)
	if err != nil {
		return InstalledPlugin{}, err
	}
	if version != "" {
		if err := validatePluginPathToken("plugin version", version); err != nil {
			return InstalledPlugin{}, err
		}
		ref.Version = version
	}
	return s.InstallFromRegistryURL(ctx, RegistryURL(), ref)
}

func (s Store) InstallFromRegistryURL(ctx context.Context, registryURL string, ref PluginRef) (InstalledPlugin, error) {
	index, err := FetchRegistry(ctx, registryURL)
	if err != nil {
		return InstalledPlugin{}, err
	}
	registryPlugin, asset, ok := index.AssetForRef(ref, runtime.GOOS, runtime.GOARCH)
	if !ok {
		return InstalledPlugin{}, index.NotFoundErrorForRef(ref, runtime.GOOS, runtime.GOARCH)
	}
	catalogRef, err := ParsePluginRef(registryPlugin.Name)
	if err != nil {
		return InstalledPlugin{}, err
	}
	if registryPlugin.Version == "" {
		return InstalledPlugin{}, fmt.Errorf("registry plugin %q is missing version", registryPlugin.Name)
	}
	if err := validatePluginPathToken("registry plugin version", registryPlugin.Version); err != nil {
		return InstalledPlugin{}, err
	}
	archivePath := filepath.Join(s.root, "plugins", "downloads", fmt.Sprintf("%s-%s-%s-%s.tar.gz", catalogRef.StorageName(), registryPlugin.Version, runtime.GOOS, runtime.GOARCH))
	if err := downloadRegistryAsset(ctx, asset, archivePath); err != nil {
		return InstalledPlugin{}, err
	}
	plugin, err := s.InstallArchiveWithOptions(ctx, archivePath, InstallOptions{
		CanonicalName: registryPlugin.Name,
		RegistryURL:   registryURL,
		Publisher:     registryPlugin.Publisher,
		Trust:         registryPlugin.Trust,
		AssetSHA256:   strings.ToLower(asset.SHA256),
		AssetOS:       asset.OS,
		AssetArch:     asset.Arch,
		Source:        "registry:" + registryPlugin.Name,
		StorageName:   catalogRef.StorageName(),
	})
	if err != nil {
		return InstalledPlugin{}, err
	}
	if plugin.Name != catalogRef.ManifestName() {
		return InstalledPlugin{}, fmt.Errorf("manifest name %q does not match catalog package %q", plugin.Name, catalogRef.ManifestName())
	}
	return plugin, nil
}
```

Ensure `InstallArchiveWithOptions` writes installed state once. Do not write installed state twice in `InstallFromRegistryVersion`.

- [ ] **Step 8: Add remote URL install**

Create `internal/pluginhost/remote.go`:

```go
package pluginhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s Store) InstallRemoteArchive(ctx context.Context, archiveURL, wantSHA256 string, opts InstallOptions) (InstalledPlugin, error) {
	want := strings.ToLower(strings.TrimSpace(wantSHA256))
	if want == "" {
		return InstalledPlugin{}, fmt.Errorf("remote plugin archive installs require --sha256")
	}
	if len(want) != sha256.Size*2 {
		return InstalledPlugin{}, fmt.Errorf("remote plugin archive sha256 must be %d hex characters", sha256.Size*2)
	}
	if _, err := hex.DecodeString(want); err != nil {
		return InstalledPlugin{}, fmt.Errorf("remote plugin archive sha256 must be hex")
	}
	downloadDir := filepath.Join(s.root, "plugins", "downloads")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return InstalledPlugin{}, err
	}
	archivePath := filepath.Join(downloadDir, "remote-"+want[:16]+".tar.gz")
	if err := downloadURLWithSHA256(ctx, archiveURL, want, archivePath); err != nil {
		return InstalledPlugin{}, err
	}
	if opts.Source == "" {
		opts.Source = "url:" + archiveURL
	}
	if opts.Trust == "" {
		opts.Trust = "unlisted"
	}
	opts.AssetSHA256 = want
	return s.InstallArchiveWithOptions(ctx, archivePath, opts)
}

func downloadURLWithSHA256(ctx context.Context, url, want, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download plugin archive: %s", resp.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), resp.Body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if got != want {
		return fmt.Errorf("remote plugin archive checksum mismatch for %s", url)
	}
	return os.Rename(tmpName, path)
}
```

- [ ] **Step 9: Run pluginhost tests**

```bash
go test ./internal/pluginhost -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/pluginhost/model.go internal/pluginhost/install.go internal/pluginhost/store.go internal/pluginhost/registry.go internal/pluginhost/registry_test.go internal/pluginhost/remote.go internal/pluginhost/remote_test.go
git commit -m "feat: install marketplace plugin archives"
```

## Task 4: Lock File Canonicalization And Exact Restore

**Parallel lane:** Squad A2 after Task 3.

**Files:**
- Modify: `internal/pluginhost/lock.go`
- Modify: `internal/pluginhost/lock_test.go`

- [ ] **Step 1: Write failing lock tests**

Append to `internal/pluginhost/lock_test.go`:

```go
func TestWriteLockFileUsesCanonicalRegistryIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glade.plugins.lock.json")
	state := InstalledState{Plugins: []InstalledPlugin{{
		Name:          "compat",
		CanonicalName: "@glade/compat",
		Version:       "0.1.0",
		Registry:      "https://plugins.glade.sh/index.json",
		Trust:         "first-party",
		Publisher:     "glade",
		AssetOS:       "darwin",
		AssetArch:     "arm64",
		AssetSHA256:   strings.Repeat("a", 64),
		Source:        "registry:@glade/compat",
		Commands:      []string{"compat"},
	}}}
	if err := WriteLockFile(path, state, false); err != nil {
		t.Fatal(err)
	}
	lock, err := ReadLockFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := lock.Plugins[0]
	if got.Name != "@glade/compat" || got.Registry == "" || got.SHA256 == "" || got.OS != "darwin" || got.Arch != "arm64" {
		t.Fatalf("unexpected lock plugin: %#v", got)
	}
}

func TestRestoreLockInstallsExactCanonicalVersions(t *testing.T) {
	lock := PluginLock{Version: 1, Plugins: []LockedPlugin{{
		Name: "@acme/quality", Version: "1.2.0", Registry: "https://plugins.example.test/index.json", SHA256: strings.Repeat("b", 64),
	}}}
	var gotName, gotVersion string
	err := NewStore(t.TempDir()).RestoreLock(context.Background(), lock, func(ctx context.Context, name, version string) (InstalledPlugin, error) {
		gotName, gotVersion = name, version
		return InstalledPlugin{Name: "quality", CanonicalName: name, Version: version}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotName != "@acme/quality" || gotVersion != "1.2.0" {
		t.Fatalf("restore installed %q %q", gotName, gotVersion)
	}
}
```

- [ ] **Step 2: Run red tests**

```bash
go test ./internal/pluginhost -run 'TestWriteLockFileUsesCanonicalRegistryIdentity|TestRestoreLockInstallsExactCanonicalVersions' -count=1
```

Expected: lock metadata assertions fail.

- [ ] **Step 3: Update lock writing**

Modify `WriteLockFile`:

```go
name := plugin.Name
if plugin.CanonicalName != "" {
	name = plugin.CanonicalName
}
lock.Plugins = append(lock.Plugins, LockedPlugin{
	Name:      name,
	Version:   plugin.Version,
	Registry:  plugin.Registry,
	OS:        plugin.AssetOS,
	Arch:      plugin.AssetArch,
	SHA256:    plugin.AssetSHA256,
	Trust:     plugin.Trust,
	Publisher: plugin.Publisher,
	Source:    plugin.Source,
	Commands:  append([]string(nil), plugin.Commands...),
})
```

Keep `Source` and `Commands` for backward compatibility.

- [ ] **Step 4: Run pluginhost tests**

```bash
go test ./internal/pluginhost -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pluginhost/lock.go internal/pluginhost/lock_test.go
git commit -m "feat: canonicalize plugin lock files"
```

## Task 5: CLI Marketplace Commands And Install Flags

**Parallel lane:** Squad B after Task 2 and Task 3 expose pluginhost APIs.

**Files:**
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/plugins_command.go`
- Modify: `internal/gladecli/cli_test.go`
- Modify: `internal/cliui/help.go`

- [ ] **Step 1: Write failing CLI tests**

Append to `internal/gladecli/cli_test.go`:

```go
func TestPluginsSearchAndInfoUseRegistry(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"version":1,"plugins":[{"name":"@glade/compat","aliases":["compat"],"version":"0.1.0","publisher":"glade","trust":"first-party","summary":"Compatibility fixtures.","commands":["compat","surface"],"docsURL":"https://glade.sh/guide/plugins/compat","assets":[{"os":"darwin","arch":"arm64","url":"https://example.test/compat.tar.gz","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}]}`)
	}))
	defer server.Close()
	t.Setenv("GLADE_PLUGIN_REGISTRY_URL", server.URL)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"plugins", "search", "surface"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("search exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "@glade/compat 0.1.0 first-party") {
		t.Fatalf("unexpected search output:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"plugins", "info", "@glade/compat"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("info exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "commands: compat, surface") || !strings.Contains(stdout.String(), "docs: https://glade.sh/guide/plugins/compat") {
		t.Fatalf("unexpected info output:\n%s", stdout.String())
	}
}

func TestPluginsInstallRemoteURLRequiresSHA256(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"plugins", "install", "https://example.test/plugin.tar.gz"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "remote plugin archive installs require --sha256") {
		t.Fatalf("stderr missing sha256 message: %q", stderr.String())
	}
}
```

Add imports if needed: `net/http`, `net/http/httptest`.

- [ ] **Step 2: Run red CLI tests**

```bash
go test ./internal/gladecli -run 'TestPluginsSearchAndInfoUseRegistry|TestPluginsInstallRemoteURLRequiresSHA256' -count=1
```

Expected: unknown `plugins search` and `plugins info`.

- [ ] **Step 3: Pass stderr into plugins**

Modify `internal/gladecli/cli.go`:

```go
case "plugins":
	if err := runPlugins(ctx, args[1:], stdout, stderr); err != nil {
		writeCommandError(stderr, args[0], err)
		return 1
	}
	return 0
```

Change `runPlugins` signature to:

```go
func runPlugins(ctx context.Context, args []string, stdout, stderr io.Writer) error
```

- [ ] **Step 4: Implement plugins command parsing**

Modify `internal/gladecli/plugins_command.go`:

```go
case "search":
	return runPluginsSearch(ctx, args[1:], stdout)
case "info":
	return runPluginsInfo(ctx, args[1:], stdout)
```

Replace `runPluginsInstall` argument parsing with:

```go
type pluginsInstallOptions struct {
	target   string
	registry string
	sha256   string
	yes      bool
}

func parsePluginsInstallArgs(args []string) (pluginsInstallOptions, error) {
	var opts pluginsInstallOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--registry":
			if i+1 >= len(args) {
				return opts, errors.New("--registry requires a value")
			}
			opts.registry = args[i+1]
			i++
		case "--sha256":
			if i+1 >= len(args) {
				return opts, errors.New("--sha256 requires a value")
			}
			opts.sha256 = args[i+1]
			i++
		case "--yes":
			opts.yes = true
		default:
			if opts.target != "" {
				return opts, fmt.Errorf("unexpected plugins install argument %q", args[i])
			}
			opts.target = args[i]
		}
	}
	if opts.target == "" {
		return opts, errors.New("usage: glade plugins install <plugin-name-or-archive> [--registry <url>] [--sha256 <hash>] [--yes]")
	}
	return opts, nil
}
```

Use helper checks:

```go
func isRemoteArchiveInstallArg(arg string) bool {
	return strings.HasPrefix(arg, "https://") || strings.HasPrefix(arg, "http://")
}
```

Registry override must not mutate the environment for the full process. Use the pluginhost API from Task 3:

```go
func (s Store) InstallFromRegistryURL(ctx context.Context, registryURL string, ref PluginRef) (InstalledPlugin, error)
```

- [ ] **Step 5: Implement search and info**

Add:

```go
func runPluginsSearch(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: glade plugins search <query>")
	}
	index, err := pluginhost.FetchRegistry(ctx, pluginhost.RegistryURL())
	if err != nil {
		return err
	}
	results := index.Search(args[0])
	if len(results) == 0 {
		fmt.Fprintf(stdout, "No plugins found for %q.\n", args[0])
		return nil
	}
	for _, plugin := range results {
		trust := plugin.Trust
		if trust == "" {
			trust = "community"
		}
		fmt.Fprintf(stdout, "%s %s %s %s\n", plugin.Name, plugin.Version, trust, plugin.Summary)
	}
	return nil
}

func runPluginsInfo(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: glade plugins info <name>")
	}
	ref, err := pluginhost.ParsePluginRef(args[0])
	if err != nil {
		return err
	}
	index, err := pluginhost.FetchRegistry(ctx, pluginhost.RegistryURL())
	if err != nil {
		return err
	}
	plugin, _, ok := index.AssetForRef(ref, runtime.GOOS, runtime.GOARCH)
	if !ok {
		return index.NotFoundErrorForRef(ref, runtime.GOOS, runtime.GOARCH)
	}
	fmt.Fprintf(stdout, "%s %s\n", plugin.Name, plugin.Version)
	fmt.Fprintf(stdout, "trust: %s\n", plugin.Trust)
	fmt.Fprintf(stdout, "publisher: %s\n", plugin.Publisher)
	fmt.Fprintf(stdout, "summary: %s\n", plugin.Summary)
	fmt.Fprintf(stdout, "commands: %s\n", strings.Join(plugin.Commands, ", "))
	if plugin.DocsURL != "" {
		fmt.Fprintf(stdout, "docs: %s\n", plugin.DocsURL)
	}
	if plugin.SourceURL != "" {
		fmt.Fprintf(stdout, "source: %s\n", plugin.SourceURL)
	}
	return nil
}
```

- [ ] **Step 6: Add trust warnings**

In `runPluginsInstall`, after installing a registry or URL plugin:

```go
if plugin.Trust == "community" || plugin.Trust == "unlisted" {
	if os.Getenv("CI") != "" && !opts.yes {
		return fmt.Errorf("plugin %s is %s; rerun with --yes or restore from a lock file in CI", plugin.IdentityName(), plugin.Trust)
	}
	fmt.Fprintf(stderr, "warning: plugin %s is %s; review its source before use\n", plugin.IdentityName(), plugin.Trust)
}
```

Do not warn for `first-party` or `verified-publisher`.

- [ ] **Step 7: Update help examples**

In `printPluginsHelp` and `internal/cliui/help.go`, include:

```text
  search            Search the plugin marketplace.
  info              Show marketplace plugin metadata.
  install           Install a plugin from the marketplace, registry, URL, or archive.
```

Examples:

```text
glade plugins install @glade/compat
glade plugins install @glade/performance
glade plugins search quality
```

- [ ] **Step 8: Run focused CLI tests**

```bash
go test ./internal/pluginhost ./internal/gladecli -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/gladecli/cli.go internal/gladecli/plugins_command.go internal/gladecli/cli_test.go internal/cliui/help.go internal/pluginhost/registry.go internal/pluginhost/registry_test.go
git commit -m "feat: add plugin marketplace cli commands"
```

## Task 6: Product Docs And Main Site Plugin IA

**Parallel lane:** Squad C. Can run in parallel with Squad A and Squad D. Do not touch Go files.

**Files:**
- Modify: `docs/PLUGINS.md`
- Modify: `docs/INSTALL.md`
- Modify: `docs/LOCAL_TESTING.md`
- Modify: `docs/APEX_PARSER.md`
- Modify: `docs/DOGFOOD_CHECKLIST.md`
- Modify: `docs/POST_PARITY_TODO.md`
- Modify: `README.md`
- Modify: `site/.vitepress/config.ts`
- Replace: `site/docs-src/guide/plugins.md`
- Create: `site/docs-src/guide/plugins/first-party.md`
- Create: `site/docs-src/guide/plugins/marketplace.md`
- Create: `site/docs-src/guide/plugins/install-manage.md`
- Create: `site/docs-src/guide/plugins/build.md`
- Create: `site/docs-src/guide/plugins/publish.md`
- Create: `site/docs-src/guide/plugins/manifest.md`
- Create: `site/docs-src/guide/plugins/lock-ci.md`

- [ ] **Step 1: Update install examples**

Replace short-only examples with canonical names:

```bash
glade plugins install @glade/compat
glade plugins install @glade/performance
```

Where short aliases help readability, add this sentence:

```markdown
The short aliases `compat` and `performance` resolve to `@glade/compat` and `@glade/performance`.
```

Run:

```bash
rg -n "glade plugins install (compat|performance)" README.md docs site/docs-src
```

Expected: no hits except sections that explicitly explain aliases.

- [ ] **Step 2: Replace `site/docs-src/guide/plugins.md` overview**

Use this page structure:

```markdown
# Plugins

Glade plugins are standalone executables installed and run through `glade plugins`.
The default Glade install stays small. First-party and marketplace plugins add
heavier workflows when a project needs them.

## First-party plugins

- `@glade/compat` - compatibility fixtures, surface ledgers, support reports, and parity scanners.
- `@glade/performance` - advisory Salesforce performance scans.

```bash
glade plugins install @glade/compat
glade plugins install @glade/performance
```

## Marketplace plugins

```bash
glade plugins search quality
glade plugins info @acme/quality
glade plugins install @acme/quality
```

## Build and publish

Plugin authors ship an executable that supports `manifest --json` and package it
as a checksum-verified archive. See the build and publish guides.
```
```

- [ ] **Step 3: Add first-party plugin page**

Create `site/docs-src/guide/plugins/first-party.md` with sections:

```markdown
# First-Party Plugins

## `@glade/compat`

Install:

```bash
glade plugins install @glade/compat
```

Commands:

- `glade compat ...`
- `glade surface ...`
- `glade local-tests ...`
- `glade post-parity ...`
- `glade examples ...`
- `glade dashboard ...`
- `glade gaps ...`
- `glade stdlib ...`

## `@glade/performance`

Install:

```bash
glade plugins install @glade/performance
```

Commands:

- `glade performance scan --project .`
```
```

- [ ] **Step 4: Add marketplace page**

Create `site/docs-src/guide/plugins/marketplace.md` with:

```markdown
# Plugin Marketplace

The marketplace is a catalog of plugin archives. Glade downloads the archive for
your OS and CPU, verifies SHA-256, verifies the archive checksums, reads
`manifest --json`, then records the install.

## Trust labels

- `first-party`
- `verified-publisher`
- `community`
- `unlisted`

## Custom registries

```bash
glade plugins install @acme/quality --registry https://plugins.acme.com/index.json
```

## Direct archives

```bash
glade plugins install ./glade-plugin-quality_1.2.0_darwin_arm64.tar.gz
glade plugins install https://github.com/acme/glade-plugin-quality/releases/download/v1.2.0/glade-plugin-quality_1.2.0_darwin_arm64.tar.gz --sha256 <hash>
```
```

- [ ] **Step 5: Add build, publish, manifest, lock pages**

Create each page with the exact contract from `docs/superpowers/specs/2026-06-11-plugin-marketplace-docs-design.md`:

- `build.md`: executable plugin contract and archive layout.
- `publish.md`: PR-based marketplace publication flow.
- `manifest.md`: `glade.plugin.v1` JSON shape.
- `lock-ci.md`: `glade.plugins.lock.json`, `glade plugins lock`, `glade plugins restore`, CI `--yes`.
- `install-manage.md`: install/list/which/doctor/remove/link/search/info.

Every page must include at least one command block and no npm install command.

- [ ] **Step 6: Update sidebar**

Modify `site/.vitepress/config.ts` so the Workflows section points to the overview and add a Plugins group:

```ts
{
  text: 'Plugins',
  items: [
    { text: 'Overview', link: '/guide/plugins' },
    { text: 'First-Party Plugins', link: '/guide/plugins/first-party' },
    { text: 'Marketplace', link: '/guide/plugins/marketplace' },
    { text: 'Install And Manage', link: '/guide/plugins/install-manage' },
    { text: 'Build A Plugin', link: '/guide/plugins/build' },
    { text: 'Publish A Plugin', link: '/guide/plugins/publish' },
    { text: 'Manifest Reference', link: '/guide/plugins/manifest' },
    { text: 'Lock Files And CI', link: '/guide/plugins/lock-ci' }
  ]
}
```

- [ ] **Step 7: Build the site**

```bash
npm --prefix site run build
git diff --check
```

Expected: both pass.

- [ ] **Step 8: Commit**

```bash
git add README.md docs/PLUGINS.md docs/INSTALL.md docs/LOCAL_TESTING.md docs/APEX_PARSER.md docs/DOGFOOD_CHECKLIST.md docs/POST_PARITY_TODO.md site/.vitepress/config.ts site/docs-src/guide/plugins.md site/docs-src/guide/plugins
git commit -m "docs: restructure plugin marketplace docs"
```

## Task 7: First-Party Registry Index In glade-tools

**Parallel lane:** Squad D in `/Users/matt/Dev/glade-tools`. Can run in parallel with Squads A and C.

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/scripts/build-plugin-archives.sh`
- Modify: `/Users/matt/Dev/glade-tools/README.md`

- [ ] **Step 1: Write failing script smoke**

Run this before changes:

```bash
tmp=$(mktemp -d)
OUT_DIR="$tmp/dist" TARGETS="$(go env GOOS)/$(go env GOARCH)" PLUGIN_ASSET_BASE_URL="https://plugins.glade.sh/v0.1.0" /Users/matt/Dev/glade-tools/scripts/build-plugin-archives.sh 0.1.0
test -f "$tmp/dist/index.json"
```

Expected: FAIL because `index.json` is not written.

- [ ] **Step 2: Modify archive script to write index JSON**

In `scripts/build-plugin-archives.sh`, keep current archive output and append an index generation step when `PLUGIN_ASSET_BASE_URL` is set.

Add arrays while building archives:

```bash
INDEX_ROWS=()
```

Inside `build_archive`, after writing the archive checksum:

```bash
local archive_sum
archive_sum="$(sha256_file "$archive")"
printf "%s  %s\n" "$archive_sum" "$(basename "$archive")" >> "$OUT_DIR/checksums.txt"
INDEX_ROWS+=("$name|$VERSION|$goos|$goarch|$(basename "$archive")|$archive_sum")
```

Replace the existing checksum append with the version above.

After the target loop:

```bash
if [[ -n "${PLUGIN_ASSET_BASE_URL:-}" ]]; then
  {
    printf '{\n  "version": 1,\n  "plugins": [\n'
    for plugin_name in compat performance; do
      case "$plugin_name" in
        compat)
          canonical="@glade/compat"
          aliases='["compat"]'
          summary="Compatibility fixtures, surface ledgers, and maintenance scanners."
          commands='["compat","surface","local-tests","post-parity","examples","dashboard","gaps","stdlib"]'
          docs="https://glade.sh/guide/plugins/first-party"
          ;;
        performance)
          canonical="@glade/performance"
          aliases='["performance"]'
          summary="Advisory Salesforce performance scanner."
          commands='["performance"]'
          docs="https://glade.sh/guide/plugins/first-party"
          ;;
      esac
      [[ "$plugin_name" == "compat" ]] || printf ',\n'
      printf '    {\n'
      printf '      "name": "%s",\n' "$canonical"
      printf '      "aliases": %s,\n' "$aliases"
      printf '      "version": "%s",\n' "$VERSION"
      printf '      "publisher": "glade",\n'
      printf '      "trust": "first-party",\n'
      printf '      "summary": "%s",\n' "$summary"
      printf '      "commands": %s,\n' "$commands"
      printf '      "docsURL": "%s",\n' "$docs"
      printf '      "sourceURL": "https://github.com/glade-sh/glade-tools",\n'
      printf '      "minimumGladeVersion": "0.1.0",\n'
      printf '      "assets": [\n'
      first_asset=1
      for row in "${INDEX_ROWS[@]}"; do
        IFS='|' read -r row_name row_version row_goos row_goarch row_archive row_sum <<< "$row"
        [[ "$row_name" == "$plugin_name" ]] || continue
        [[ "$first_asset" -eq 1 ]] || printf ',\n'
        first_asset=0
        printf '        {"os":"%s","arch":"%s","url":"%s/%s","sha256":"%s"}' "$row_goos" "$row_goarch" "${PLUGIN_ASSET_BASE_URL%/}" "$row_archive" "$row_sum"
      done
      printf '\n      ]\n'
      printf '    }'
    done
    printf '\n  ]\n}\n'
  } > "$OUT_DIR/index.json"
fi
```

- [ ] **Step 3: Prove generated JSON parses**

```bash
tmp=$(mktemp -d)
OUT_DIR="$tmp/dist" TARGETS="$(go env GOOS)/$(go env GOARCH)" PLUGIN_ASSET_BASE_URL="https://plugins.glade.sh/v0.1.0" /Users/matt/Dev/glade-tools/scripts/build-plugin-archives.sh 0.1.0
node -e 'const fs=require("fs"); const idx=JSON.parse(fs.readFileSync(process.argv[1],"utf8")); if(idx.plugins.length!==2) process.exit(1); for (const p of idx.plugins) { if (!p.name.startsWith("@glade/")) process.exit(2); if (!p.assets.length) process.exit(3); }' "$tmp/dist/index.json"
```

Expected: PASS.

- [ ] **Step 4: Update glade-tools README**

Replace install examples:

```bash
glade plugins install @glade/compat
glade plugins install @glade/performance
```

Add registry output example:

```bash
OUT_DIR=dist/plugins TARGETS="darwin/arm64 linux/amd64" PLUGIN_ASSET_BASE_URL="https://plugins.glade.sh/v0.1.0" scripts/build-plugin-archives.sh 0.1.0
```

Mention that `plugins/*/plugin.json` manifest `name` remains `compat` or `performance`; scoped names live in the marketplace index.

- [ ] **Step 5: Run glade-tools tests**

```bash
go test ./internal/toolcli ./internal/perftool ./internal/perfscan ./internal/apexdocs ./internal/capability ./internal/surfaceledger -count=1 -timeout=10m
git diff --check
```

Expected: PASS.

- [ ] **Step 6: Commit in glade-tools**

```bash
cd /Users/matt/Dev/glade-tools
git add scripts/build-plugin-archives.sh README.md
git commit -m "feat: emit first-party plugin registry index"
```

## Task 8: Integration Proof And Product Docs Cross-Links

**Parallel lane:** Squad I after Squads A, B, C, and D land.

**Files:**
- Modify only files required by failing integration gates.

- [ ] **Step 1: Run product focused tests**

```bash
cd /Users/matt/Dev/glade/.worktrees/plugin-marketplace-docs
go test ./internal/pluginhost ./internal/gladecli ./internal/repoguard -count=1
```

Expected: PASS.

- [ ] **Step 2: Build docs site**

```bash
npm --prefix site run build
```

Expected: PASS.

- [ ] **Step 3: Build first-party registry and install through product**

```bash
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
go build -o "$tmp/bin/glade" ./cmd/glade
cd /Users/matt/Dev/glade-tools
OUT_DIR="$tmp/dist" TARGETS="$(go env GOOS)/$(go env GOARCH)" PLUGIN_ASSET_BASE_URL="http://127.0.0.1:65535/plugins" scripts/build-plugin-archives.sh 0.1.0
cd /Users/matt/Dev/glade/.worktrees/plugin-marketplace-docs
python3 -m http.server 8765 --directory "$tmp/dist" >"$tmp/http.log" 2>&1 &
server_pid=$!
trap 'kill "$server_pid" 2>/dev/null || true; rm -rf "$tmp"' EXIT
python3 - <<'PY' "$tmp/dist/index.json" "$tmp/dist/index.local.json"
import json, sys
src, dst = sys.argv[1], sys.argv[2]
idx = json.load(open(src))
for plugin in idx["plugins"]:
    for asset in plugin["assets"]:
        asset["url"] = "http://127.0.0.1:8765/" + asset["url"].rsplit("/", 1)[-1]
json.dump(idx, open(dst, "w"), indent=2)
PY
HOME="$tmp/home" GLADE_PLUGIN_REGISTRY_URL="http://127.0.0.1:8765/index.local.json" "$tmp/bin/glade" plugins install @glade/compat
HOME="$tmp/home" GLADE_PLUGIN_REGISTRY_URL="http://127.0.0.1:8765/index.local.json" "$tmp/bin/glade" plugins install @glade/performance
HOME="$tmp/home" "$tmp/bin/glade" plugins doctor
HOME="$tmp/home" "$tmp/bin/glade" compat local-tests --help >/dev/null
HOME="$tmp/home" "$tmp/bin/glade" performance scan --help >/dev/null
```

Expected:

```text
Installed plugin compat 0.1.0 ...
Installed plugin performance 0.1.0 ...
compat 0.1.0 ok
performance 0.1.0 ok
```

- [ ] **Step 4: Verify lock file canonical names**

Inside the same temp install:

```bash
HOME="$tmp/home" "$tmp/bin/glade" plugins lock
node -e 'const fs=require("fs"); const lock=JSON.parse(fs.readFileSync("glade.plugins.lock.json","utf8")); const names=lock.plugins.map(p=>p.name).sort(); if (names.join(",") !== "@glade/compat,@glade/performance") process.exit(1);'
rm glade.plugins.lock.json
```

Expected: PASS.

- [ ] **Step 5: Check no npm transport language slipped in**

```bash
rg -n "npm install|npm transport|Node dependency" docs site README.md internal /Users/matt/Dev/glade-tools/README.md
```

Expected: no hits except explicit “no npm transport” text in the design/plan docs.

- [ ] **Step 6: Run final product gates**

```bash
go test ./internal/pluginhost ./internal/gladecli ./internal/repoguard -count=1
scripts/smoke.sh
npm --prefix site run build
git diff --check
```

Expected: PASS.

- [ ] **Step 7: Run final glade-tools gates**

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/toolcli ./internal/perftool ./internal/perfscan ./internal/apexdocs ./internal/capability ./internal/surfaceledger -count=1 -timeout=10m
git diff --check
```

Expected: PASS.

- [ ] **Step 8: Commit integration fixes**

If Step 1-7 required fixes, stage the changed files reported by `git diff --name-only`:

```bash
git diff --name-only
git add $(git diff --name-only)
git commit -m "test: prove plugin marketplace integration"
```

If no fixes were required, do not create an empty commit.

## Final Merge And Cleanup

- [ ] **Step 1: Confirm both repos are clean or intentionally staged**

```bash
git -C /Users/matt/Dev/glade/.worktrees/plugin-marketplace-docs status --short --branch
git -C /Users/matt/Dev/glade-tools status --short --branch
```

- [ ] **Step 2: Merge product branch to local main**

```bash
cd /Users/matt/Dev/glade
git switch main
git merge --no-ff codex/plugin-marketplace-docs -m "merge plugin marketplace docs"
```

- [ ] **Step 3: Keep or merge glade-tools branch**

If `/Users/matt/Dev/glade-tools` is expected to land locally now:

```bash
cd /Users/matt/Dev/glade-tools
git switch main
git merge --no-ff codex/plugin-marketplace-docs -m "merge plugin marketplace index"
```

If the user wants a PR instead, push the branch and do not delete it.

- [ ] **Step 4: Verify merged state**

```bash
cd /Users/matt/Dev/glade
go test ./internal/pluginhost ./internal/gladecli ./internal/repoguard -count=1
scripts/smoke.sh
npm --prefix site run build
git diff --check

cd /Users/matt/Dev/glade-tools
go test ./internal/toolcli ./internal/perftool ./internal/perfscan ./internal/apexdocs ./internal/capability ./internal/surfaceledger -count=1 -timeout=10m
git diff --check
```

- [ ] **Step 5: Clean product worktree**

Only after merge and verification pass:

```bash
cd /Users/matt/Dev/glade
git worktree remove /Users/matt/Dev/glade/.worktrees/plugin-marketplace-docs
git worktree prune
git branch -d codex/plugin-marketplace-docs
```

Delete the `glade-tools` feature branch only after that repo has been merged or pushed.

## Failure Rules

- If `go test ./internal/pluginhost` fails, stop product CLI work and fix pluginhost first.
- If docs build fails, do not change Go code to satisfy docs.
- If archive install works only from a path and not through the registry index, the marketplace is not done.
- If lock files write `compat` instead of `@glade/compat` for registry installs, the lock work is not done.
- If any docs advise npm as plugin transport, remove that text.
- If product code imports or shells out to `/Users/matt/Dev/glade-tools`, revert that change. Product must not depend on `glade-tools`.
