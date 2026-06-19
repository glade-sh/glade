# Org Package Artifact Capture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a full org-connected package artifact workflow so local Glade projects can depend on installed Salesforce package contracts without checking in the package source.

**Architecture:** Keep live Salesforce capture in the first-party `@glade/orgpackage` plugin from the sibling `glade-tools` repo. Keep artifact format, dependency loading, semantic checks, code intel, runtime boundaries, and optional shims in base Glade. The capture plugin reads org facts through `sf`, normalizes them into Glade's existing package artifact format, and local projects consume the artifact through `project.managedPackageDependencies`.

**Tech Stack:** Go 1.26, Salesforce CLI `sf api request rest`, Glade `internal/packageartifact`, Glade `internal/typesys`, Glade `internal/orgdescribe`, Glade plugin host, `glade-tools` plugin packaging.

---

## Product Boundaries

- Base `glade` owns artifact format, validation, diff, indexing, runtime behavior, local shims, docs, and helpful plugin delegation.
- `glade-tools` owns live org capture, Salesforce API request batching, installed package discovery, org fact normalization, and plugin packaging.
- The plugin command root is `orgpackage`, not `package`. Glade's plugin router dispatches by the first command token, and `package` is already a built-in root.
- Add `glade package capture` only as a bridge. It delegates to the installed `orgpackage` plugin when present and prints install guidance when absent. It must not contain live Salesforce logic.

## File Structure

### Base Glade

- Modify `internal/packageartifact/artifact.go`
  - Add artifact schema version, capture provenance, metadata name arrays, optional Lightning bundle summaries, and a captured-artifact builder.
  - Keep old `labels` and `staticResources` count fields for compatibility.
- Modify `internal/packageartifact/artifact_test.go`
  - Cover captured artifact construction, metadata names, provenance, validation, and diff behavior.
- Modify `internal/typesys/symbols.go`
  - Preserve capture metadata in dependency summaries and index code-intel symbols from named artifact rows.
- Modify `internal/typesys/symbols_test.go`
  - Cover captured artifact dependency load with Apex, schema, labels, static resources, and Lightning bundle rows.
- Modify `internal/project/project.go`
  - Validate artifact schema version and expose capture provenance in dependency metadata.
- Modify `internal/project/project_test.go`
  - Cover version mismatch, schema-version mismatch, and capture metadata load.
- Modify `internal/apextest/project_setup.go`
  - Register captured package members as runtime-unsupported unless a configured shim overrides them.
- Modify `internal/apextest/runner_test.go`
  - Cover captured package compile success, runtime boundary error, and shim override.
- Modify `internal/config/config.go`
  - Add optional `project.packageShims` with namespace-to-source-root entries.
- Modify `internal/config/config_test.go`
  - Cover shim config parsing and relative path resolution.
- Modify `internal/gladecli/cli.go`
  - Add `glade package capture` bridge to the plugin host.
  - Print capture provenance in `glade package info`.
- Modify `internal/gladecli/cli_test.go`
  - Cover bridge messaging and enriched package info output.
- Modify `docs/CONFIG.md`, `docs/INSTALL.md`, `site/docs-src/guide/configuration.md`, `site/docs-src/guide/cli-reference.md`
  - Document artifact dependencies, capture plugin usage, and shim roots.
- Modify `site/tests/theme.test.mjs`
  - Keep rendered docs assertions current.

### Sibling `glade-tools`

- Create `/Users/matt/Dev/glade-tools/cmd/glade-plugin-orgpackage/main.go`
  - Thin entrypoint for the new plugin binary.
- Create `/Users/matt/Dev/glade-tools/plugins/orgpackage/plugin.json`
  - Packaged plugin manifest.
- Modify `/Users/matt/Dev/glade-tools/scripts/build-plugin-archives.sh`
  - Build `orgpackage` for all plugin targets and include it in `index.json`.
- Modify `/Users/matt/Dev/glade-tools/internal/toolcli/cli.go`
  - Route `orgpackage` commands and keep legacy compatibility commands intact.
- Modify `/Users/matt/Dev/glade-tools/internal/toolcli/manifest.go`
  - Add a separate orgpackage manifest writer.
- Create `/Users/matt/Dev/glade-tools/internal/toolcli/orgpackage_command.go`
  - Parse command flags and call the capture package.
- Create `/Users/matt/Dev/glade-tools/internal/toolcli/orgpackage_command_test.go`
  - Cover CLI parsing, JSON output, write failures, and help text.
- Create `/Users/matt/Dev/glade-tools/internal/orgpackage/model.go`
  - Define normalized capture rows independent of raw Salesforce JSON.
- Create `/Users/matt/Dev/glade-tools/internal/orgpackage/sfcli.go`
  - Define the `sf` request runner interface and subprocess implementation.
- Create `/Users/matt/Dev/glade-tools/internal/orgpackage/client.go`
  - Build REST and Tooling API query URLs and handle query pagination.
- Create `/Users/matt/Dev/glade-tools/internal/orgpackage/client_test.go`
  - Cover URL encoding, queryMore handling, and API-version selection.
- Create `/Users/matt/Dev/glade-tools/internal/orgpackage/discover.go`
  - Discover installed package identity, namespace, version, and org provenance.
- Create `/Users/matt/Dev/glade-tools/internal/orgpackage/tooling.go`
  - Capture `ApexClass.SymbolTable` rows, labels, static resources, and component inventory.
- Create `/Users/matt/Dev/glade-tools/internal/orgpackage/describe.go`
  - Capture namespaced objects, namespaced fields on standard objects, record types, and relationships.
- Create `/Users/matt/Dev/glade-tools/internal/orgpackage/convert.go`
  - Convert normalized capture rows into `packageartifact.Artifact`.
- Create `/Users/matt/Dev/glade-tools/internal/orgpackage/convert_test.go`
  - Cover SymbolTable-to-ApexType conversion, schema conversion, metadata names, and deterministic hashing.
- Create `/Users/matt/Dev/glade-tools/internal/orgpackage/report.go`
  - Produce human and JSON summaries.
- Create `/Users/matt/Dev/glade-tools/internal/orgpackage/testdata/*.json`
  - Store small fake Salesforce API responses for deterministic tests.

---

## Task 1: Extend The Artifact Format In Base Glade

**Files:**
- Modify: `internal/packageartifact/artifact.go`
- Modify: `internal/packageartifact/artifact_test.go`

- [ ] **Step 1: Write the failing tests**

Add these tests to `internal/packageartifact/artifact_test.go`:

```go
func TestBuildCapturedArtifactPreservesOrgProvenanceAndMetadataNames(t *testing.T) {
	artifact, err := BuildCaptured(BuildCapturedOptions{
		Namespace:        "pkg",
		PackageName:      "Billing Core",
		Version:          "1.2.3.4",
		SourceAPIVersion: "65.0",
		Capture: CaptureProvenance{
			Source:      "org",
			OrgID:       "00Dxx0000000001",
			Username:    "builder@example.com",
			TargetOrg:   "packaging",
			APIVersion:  "65.0",
			CapturedAt:  mustParsePackageArtifactTime(t, "2026-06-19T12:00:00Z"),
			PackageID:   "033xx0000000001",
			InstalledID: "0A3xx0000000001",
		},
		ApexTypes: []ApexType{{
			Kind:      apexast.DeclarationClass,
			Name:      "BillingGateway",
			Namespace: "pkg",
			Modifiers: []string{"global"},
			Members: []ApexMember{{
				Kind:      apexast.DeclarationMethod,
				Name:      "authorize",
				Type:      "Boolean",
				Modifiers: []string{"global", "static"},
				Parameters: []apexast.Parameter{{
					Name: "amount",
					Type: "Decimal",
				}},
			}},
		}},
		Objects: []schema.Object{{
			Name: "pkg__Billing_Profile__c",
			Fields: []schema.Field{{
				Name: "pkg__External_Key__c",
				Type: "Text",
			}},
		}},
		LabelNames:          []string{"pkg__Billing_Error"},
		StaticResourceNames: []string{"pkg__BillingAssets"},
		LightningBundles: []LightningBundle{{
			Namespace: "pkg",
			Name:      "billingConsole",
			Type:      "lwc",
			Exposed:   true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.SchemaVersion != 2 {
		t.Fatalf("schema version = %d, want 2", artifact.SchemaVersion)
	}
	if artifact.Capture.OrgID != "00Dxx0000000001" || artifact.Capture.PackageID != "033xx0000000001" {
		t.Fatalf("capture provenance = %#v", artifact.Capture)
	}
	if artifact.Labels != 1 || len(artifact.LabelNames) != 1 || artifact.LabelNames[0] != "pkg__Billing_Error" {
		t.Fatalf("labels = %d %#v", artifact.Labels, artifact.LabelNames)
	}
	if artifact.StaticResources != 1 || len(artifact.StaticResourceNames) != 1 || artifact.StaticResourceNames[0] != "pkg__BillingAssets" {
		t.Fatalf("static resources = %d %#v", artifact.StaticResources, artifact.StaticResourceNames)
	}
	if len(artifact.LightningBundles) != 1 || artifact.LightningBundles[0].QualifiedName() != "pkg/billingConsole" {
		t.Fatalf("lightning bundles = %#v", artifact.LightningBundles)
	}
	if artifact.SourceHash == "" {
		t.Fatal("sourceHash is empty")
	}
	if !artifactHasCodeIntelSymbol(artifact, "metadata:label:pkg__Billing_Error") {
		t.Fatalf("missing label symbol in %#v", artifact.CodeIntelSymbols)
	}
	if !artifactHasCodeIntelSymbol(artifact, "metadata:static_resource:pkg__BillingAssets") {
		t.Fatalf("missing static resource symbol in %#v", artifact.CodeIntelSymbols)
	}
}

func TestValidateRejectsUnsupportedArtifactSchemaVersion(t *testing.T) {
	issues := Validate(Artifact{
		SchemaVersion: 99,
		Namespace:     "pkg",
		SourceHash:    "abc",
		BuiltAt:       mustParsePackageArtifactTime(t, "2026-06-19T12:00:00Z"),
	})
	if len(issues) != 1 || issues[0] != "unsupported artifact schemaVersion 99" {
		t.Fatalf("issues = %#v", issues)
	}
}

func artifactHasCodeIntelSymbol(artifact Artifact, id string) bool {
	for _, symbol := range artifact.CodeIntelSymbols {
		if symbol.ID == id {
			return true
		}
	}
	return false
}

func mustParsePackageArtifactTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
```

- [ ] **Step 2: Run the tests and verify they fail**

Run:

```bash
go test ./internal/packageartifact -run 'TestBuildCapturedArtifactPreservesOrgProvenanceAndMetadataNames|TestValidateRejectsUnsupportedArtifactSchemaVersion'
```

Expected: FAIL with undefined `BuildCapturedOptions`, `BuildCaptured`, `CaptureProvenance`, and `LightningBundle`.

- [ ] **Step 3: Add artifact structs and captured builder**

Add the new types near the top of `internal/packageartifact/artifact.go`:

```go
const CurrentSchemaVersion = 2

type CaptureProvenance struct {
	Source      string    `json:"source,omitempty"`
	OrgID       string    `json:"orgId,omitempty"`
	Username    string    `json:"username,omitempty"`
	TargetOrg   string    `json:"targetOrg,omitempty"`
	APIVersion  string    `json:"apiVersion,omitempty"`
	CapturedAt  time.Time `json:"capturedAt,omitempty"`
	PackageID   string    `json:"packageId,omitempty"`
	InstalledID string    `json:"installedId,omitempty"`
}

type LightningBundle struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Exposed   bool   `json:"exposed,omitempty"`
}

func (b LightningBundle) QualifiedName() string {
	if strings.TrimSpace(b.Namespace) == "" {
		return strings.TrimSpace(b.Name)
	}
	return strings.TrimSpace(b.Namespace) + "/" + strings.TrimSpace(b.Name)
}

type BuildCapturedOptions struct {
	Namespace           string
	PackageName         string
	Version             string
	SourceAPIVersion    string
	Capture             CaptureProvenance
	ApexTypes           []ApexType
	Objects             []schema.Object
	CustomMetadataRecords []schema.CustomMetadataRecord
	LabelNames          []string
	StaticResourceNames []string
	LightningBundles    []LightningBundle
}
```

Extend `Artifact`:

```go
type Artifact struct {
	SchemaVersion          int                           `json:"schemaVersion,omitempty"`
	Namespace              string                        `json:"namespace"`
	PackageName            string                        `json:"packageName,omitempty"`
	Version                string                        `json:"version,omitempty"`
	SourceRoot             string                        `json:"sourceRoot,omitempty"`
	SourceHash             string                        `json:"sourceHash,omitempty"`
	SourceAPIVersion       string                        `json:"sourceApiVersion,omitempty"`
	BuiltAt                time.Time                     `json:"builtAt"`
	Capture                CaptureProvenance             `json:"capture,omitempty"`
	ApexTypes              []ApexType                    `json:"apexTypes,omitempty"`
	Objects                []schema.Object               `json:"objects,omitempty"`
	CustomMetadataRecords  []schema.CustomMetadataRecord `json:"customMetadataRecords,omitempty"`
	LabelNames             []string                      `json:"labelNames,omitempty"`
	StaticResourceNames    []string                      `json:"staticResourceNames,omitempty"`
	LightningBundles       []LightningBundle             `json:"lightningBundles,omitempty"`
	Labels                 int                           `json:"labels"`
	StaticResources        int                           `json:"staticResources"`
	CodeIntelSymbolsVersion int                           `json:"codeIntelSymbolsVersion,omitempty"`
	CodeIntelSymbols        []CodeIntelSymbol             `json:"codeIntelSymbols,omitempty"`
	CodeIntelUsesVersion    int                           `json:"codeIntelUsesVersion,omitempty"`
	CodeIntelUses           []CodeIntelUse                `json:"codeIntelUses,omitempty"`
}
```

Add the builder:

```go
func BuildCaptured(opts BuildCapturedOptions) (Artifact, error) {
	namespace := strings.TrimSpace(opts.Namespace)
	if namespace == "" {
		return Artifact{}, errors.New("namespace is required")
	}
	builtAt := time.Now().UTC()
	if !opts.Capture.CapturedAt.IsZero() {
		builtAt = opts.Capture.CapturedAt.UTC()
	}
	artifact := Artifact{
		SchemaVersion:         CurrentSchemaVersion,
		Namespace:             namespace,
		PackageName:           strings.TrimSpace(opts.PackageName),
		Version:               strings.TrimSpace(opts.Version),
		SourceAPIVersion:      strings.TrimSpace(opts.SourceAPIVersion),
		BuiltAt:               builtAt,
		Capture:               opts.Capture,
		ApexTypes:             globalContractTypes(cloneApexTypes(opts.ApexTypes)),
		Objects:               cloneSchemaObjects(opts.Objects),
		CustomMetadataRecords: cloneCustomMetadataRecords(opts.CustomMetadataRecords),
		LabelNames:            sortedUniqueStrings(opts.LabelNames),
		StaticResourceNames:   sortedUniqueStrings(opts.StaticResourceNames),
		LightningBundles:      sortedLightningBundles(opts.LightningBundles),
	}
	artifact.Labels = len(artifact.LabelNames)
	artifact.StaticResources = len(artifact.StaticResourceNames)
	artifact.SourceHash = capturedSourceHash(artifact)
	artifact.CodeIntelSymbols = codeIntelSymbols(artifact, project.Project{})
	artifact.CodeIntelUses = codeIntelDeclarationUses(artifact.CodeIntelSymbols)
	if len(artifact.CodeIntelSymbols) > 0 {
		artifact.CodeIntelSymbolsVersion = 1
	}
	if len(artifact.CodeIntelUses) > 0 {
		artifact.CodeIntelUsesVersion = 1
	}
	return artifact, nil
}
```

Add helper functions in the same file. Use `json.Marshal` for `capturedSourceHash`; sort all list fields before hashing.

- [ ] **Step 4: Update existing build path**

In `Build`, set `SchemaVersion: CurrentSchemaVersion`. Fill `LabelNames` and `StaticResourceNames` from existing source files, then keep `Labels` and `StaticResources` as the counts:

```go
labels := packageLabelNames(namespace, p.LabelFiles)
resources := packageStaticResourceNames(namespace, p.StaticResourceFiles, p.StaticResourceMetas)
artifact := Artifact{
	SchemaVersion:       CurrentSchemaVersion,
	Namespace:           namespace,
	Version:             version,
	SourceRoot:          p.Root,
	SourceHash:          sourceHash,
	SourceAPIVersion:    p.SourceAPIVersion,
	BuiltAt:             time.Now().UTC(),
	ApexTypes:           globalContractTypes(apexTypes),
	Objects:             namespaceObjects(namespace, s.Objects),
	CustomMetadataRecords: namespaceCustomMetadataRecords(namespace, s.CustomMetadataRecords),
	LabelNames:          labels,
	StaticResourceNames: resources,
	Labels:              len(labels),
	StaticResources:     len(resources),
}
```

- [ ] **Step 5: Update validation**

In `Validate`, treat omitted schema version as version 1 for old files:

```go
schemaVersion := artifact.SchemaVersion
if schemaVersion == 0 {
	schemaVersion = 1
}
if schemaVersion > CurrentSchemaVersion {
	issues = append(issues, fmt.Sprintf("unsupported artifact schemaVersion %d", artifact.SchemaVersion))
}
```

- [ ] **Step 6: Run package artifact tests**

Run:

```bash
go test ./internal/packageartifact
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/packageartifact/artifact.go internal/packageartifact/artifact_test.go
git commit -m "feat: extend package artifact capture metadata"
```

---

## Task 2: Teach The Type Index About Captured Metadata

**Files:**
- Modify: `internal/typesys/symbols.go`
- Modify: `internal/typesys/symbols_test.go`
- Modify: `internal/project/project.go`
- Modify: `internal/project/project_test.go`

- [ ] **Step 1: Write dependency load tests**

Add a test to `internal/typesys/symbols_test.go` that writes a captured artifact with `labelNames`, `staticResourceNames`, one `lightningBundles` row, one global Apex type, and one schema object. Load a project with:

```yaml
project:
  managedPackageDependencies: ["pkg:artifact:../packages/pkg.glade-package.json:1.2.3"]
```

Assert:

```go
if len(idx.Dependencies) != 1 || idx.Dependencies[0].Status != "loaded" {
	t.Fatalf("dependencies = %#v", idx.Dependencies)
}
if idx.Dependencies[0].ApexTypes != 1 || idx.Dependencies[0].Objects != 1 || idx.Dependencies[0].Labels != 1 || idx.Dependencies[0].StaticResources != 1 {
	t.Fatalf("dependency summary = %#v", idx.Dependencies[0])
}
if !codeIntelIDPresent(idx.CodeIntelSymbols, "metadata:label:pkg__Billing_Error") {
	t.Fatalf("missing label symbol: %#v", idx.CodeIntelSymbols)
}
if !codeIntelIDPresent(idx.CodeIntelSymbols, "metadata:static_resource:pkg__BillingAssets") {
	t.Fatalf("missing static resource symbol: %#v", idx.CodeIntelSymbols)
}
```

- [ ] **Step 2: Write project metadata test**

Add a test to `internal/project/project_test.go` that loads an artifact with:

```json
{
  "schemaVersion": 99,
  "namespace": "pkg",
  "version": "1.2.3",
  "sourceHash": "abc",
  "builtAt": "2026-06-19T12:00:00Z"
}
```

Assert the dependency status is `load_error` and the diagnostic message contains `unsupported artifact schemaVersion 99`.

- [ ] **Step 3: Run tests and verify they fail**

Run:

```bash
go test ./internal/typesys ./internal/project -run 'Captured|ArtifactSchema'
```

Expected: FAIL until project artifact metadata validation calls `packageartifact.Validate`.

- [ ] **Step 4: Validate artifact metadata in project load**

In `loadManagedPackageArtifactMetadata`, unmarshal the full artifact enough to call `packageartifact.Validate`. Keep the lightweight namespace and version checks after validation.

```go
artifact, err := packageartifact.ReadJSON(path)
if err != nil {
	return newManagedPackageArtifactError("load_error", "dependency_load_error", err.Error())
}
if issues := packageartifact.Validate(artifact); len(issues) > 0 {
	return newManagedPackageArtifactError("load_error", "dependency_load_error", "managed package dependency artifact invalid: "+strings.Join(issues, "; "))
}
metadata.Namespace = artifact.Namespace
metadata.Version = artifact.Version
```

- [ ] **Step 5: Preserve code intel and dependency summary**

Confirm `appendArtifactDependency` already appends `artifact.CodeIntelSymbols` and `artifact.CodeIntelUses`. Add capture fields only to `DependencyInfo` if needed by JSON consumers:

```go
CaptureSource string `json:"captureSource,omitempty"`
CaptureOrgID  string `json:"captureOrgId,omitempty"`
```

Populate them from `artifact.Capture.Source` and `artifact.Capture.OrgID`.

- [ ] **Step 6: Run focused tests**

Run:

```bash
go test ./internal/typesys ./internal/project
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/typesys/symbols.go internal/typesys/symbols_test.go internal/project/project.go internal/project/project_test.go
git commit -m "feat: load captured package artifact metadata"
```

---

## Task 3: Add Runtime Boundary And Package Shims

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/project/project.go`
- Modify: `internal/apextest/project_setup.go`
- Modify: `internal/apextest/runner_test.go`

- [ ] **Step 1: Write config tests for shims**

Add to `internal/config/config_test.go`:

```go
func TestParsePackageShimRoots(t *testing.T) {
	cfg, err := parseYAMLSubset(`
project:
  packageShims: ["pkg:test-support/package-shims/pkg"]
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Project.PackageShims) != 1 {
		t.Fatalf("package shims = %#v", cfg.Project.PackageShims)
	}
	shim := cfg.Project.PackageShims[0]
	if shim.Namespace != "pkg" || shim.SourceRoot != "test-support/package-shims/pkg" {
		t.Fatalf("shim = %#v", shim)
	}
}
```

- [ ] **Step 2: Write runtime boundary tests**

Add to `internal/apextest/runner_test.go`:

```go
func TestCapturedPackageMethodFailsWithNamedBoundaryWithoutShim(t *testing.T) {
	root := t.TempDir()
	writeRunnerFile(t, filepath.Join(root, "glade.yml"), `
project:
  packageDirs: [force-app]
  managedPackageDependencies: ["pkg:artifact:packages/pkg.glade-package.json:1.0"]
`)
	writeCapturedArtifact(t, filepath.Join(root, "packages/pkg.glade-package.json"), capturedBillingGatewayArtifact())
	writeRunnerFile(t, filepath.Join(root, "force-app/main/default/classes/BillingTest.cls"), `
@IsTest
private class BillingTest {
  @IsTest static void runs() {
    System.assertEquals(true, pkg.BillingGateway.authorize(1.00));
  }
}
`)
	report := runGladeTestAndReturnReport(t, root, "BillingTest")
	if report.Tests[0].Status != "failed" || !strings.Contains(report.Tests[0].Message, "captured package member has no local body") {
		t.Fatalf("report = %#v", report.Tests[0])
	}
}

func TestCapturedPackageMethodUsesConfiguredShim(t *testing.T) {
	root := t.TempDir()
	writeRunnerFile(t, filepath.Join(root, "glade.yml"), `
project:
  packageDirs: [force-app]
  managedPackageDependencies: ["pkg:artifact:packages/pkg.glade-package.json:1.0"]
  packageShims: ["pkg:test-support/package-shims/pkg"]
`)
	writeCapturedArtifact(t, filepath.Join(root, "packages/pkg.glade-package.json"), capturedBillingGatewayArtifact())
	writeRunnerFile(t, filepath.Join(root, "test-support/package-shims/pkg/classes/BillingGateway.cls"), `
global class BillingGateway {
  global static Boolean authorize(Decimal amount) {
    return amount > 0;
  }
}
`)
	writeRunnerFile(t, filepath.Join(root, "force-app/main/default/classes/BillingTest.cls"), `
@IsTest
private class BillingTest {
  @IsTest static void runs() {
    System.assertEquals(true, pkg.BillingGateway.authorize(1.00));
  }
}
`)
	report := runGladeTestAndReturnReport(t, root, "BillingTest")
	if report.Tests[0].Status != "passed" {
		t.Fatalf("report = %#v", report.Tests[0])
	}
}
```

- [ ] **Step 3: Run tests and verify they fail**

Run:

```bash
go test ./internal/config ./internal/apextest -run 'PackageShim|CapturedPackageMethod'
```

Expected: FAIL with unsupported config key `packageShims` and missing runtime behavior.

- [ ] **Step 4: Add package shim config**

Add:

```go
type PackageShim struct {
	Namespace  string `json:"namespace"`
	SourceRoot string `json:"sourceRoot"`
}
```

Extend `ProjectConfig`:

```go
PackageShims []PackageShim `json:"packageShims,omitempty"`
```

Add parser support:

```go
case "project.packageShims":
	values, err := parseInlineList(value)
	if err != nil {
		return Config{}, fmt.Errorf("glade.yml:%d: %w", lineNo+1, err)
	}
	shims, err := parsePackageShims(values)
	if err != nil {
		return Config{}, fmt.Errorf("glade.yml:%d: %w", lineNo+1, err)
	}
	cfg.Project.PackageShims = shims
```

Implement `parsePackageShims` with the same namespace duplicate checks as managed dependencies.

- [ ] **Step 5: Load shim projects**

In `project.Project`, add:

```go
PackageShims []PackageShim `json:"packageShims,omitempty"`
```

Load each shim root as a dependency-shaped project with the configured namespace. Mark shim types so they can override captured artifact runtime methods.

- [ ] **Step 6: Register captured package members as unsupported**

When `project_setup.go` registers methods from artifact dependency types and no shim method with the same signature exists, set:

```go
Unsupported: "captured package member has no local body; add a project.packageShims entry or run this behavior in Salesforce",
Dependency:  true,
```

Do not set `Unsupported` when the method came from a configured shim source root.

- [ ] **Step 7: Run runtime tests**

Run:

```bash
go test ./internal/config ./internal/project ./internal/apextest -run 'PackageShim|CapturedPackageMethod'
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go internal/project/project.go internal/apextest/project_setup.go internal/apextest/runner_test.go
git commit -m "feat: support local shims for captured package artifacts"
```

---

## Task 4: Add Product CLI Support And Plugin Bridge

**Files:**
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/cli_test.go`
- Modify: `internal/cliui/help.go`

- [ ] **Step 1: Write CLI tests**

Add tests:

```go
func TestPackageInfoPrintsCaptureProvenance(t *testing.T) {
	root := t.TempDir()
	artifactPath := filepath.Join(root, "pkg.glade-package.json")
	writeCapturedArtifact(t, artifactPath, capturedBillingGatewayArtifact())
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"package", "info", artifactPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"captureSource: org",
		"captureOrgId: 00Dxx0000000001",
		"targetOrg: packaging",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestPackageCaptureWithoutPluginPrintsInstallGuidance(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"package", "capture", "--target-org", "packaging", "--namespace", "pkg"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("code=0 stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "glade plugins install @glade/orgpackage") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "glade orgpackage capture") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
go test ./internal/gladecli -run 'PackageInfoPrintsCaptureProvenance|PackageCaptureWithoutPlugin'
```

Expected: FAIL because info does not print capture fields and `capture` is not recognized.

- [ ] **Step 3: Add `capture` subcommand bridge**

In `runPackage`, add:

```go
case "capture":
	return runPackageCaptureBridge(ctx, args[1:], w, progressW)
```

Implement:

```go
func runPackageCaptureBridge(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	state, err := pluginhost.NewStore(pluginhost.DefaultRoot()).ReadInstalled()
	if err != nil {
		return err
	}
	plugin, ok := pluginhost.FindByCommandRoot(state, "orgpackage")
	if !ok {
		return errors.New("package capture is provided by the orgpackage plugin; run `glade plugins install @glade/orgpackage`, then `glade orgpackage capture --target-org <alias> --namespace <namespace> --output <artifact.json>`")
	}
	code, err := pluginhost.RunPlugin(ctx, plugin, append([]string{"orgpackage", "capture"}, args...), stdout, stderr)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("orgpackage plugin exited with status %d", code)
	}
	return nil
}
```

- [ ] **Step 4: Add package info fields**

Extend `packageartifact.Info` with capture fields. Print them only when non-empty:

```go
if info.CaptureSource != "" {
	fmt.Fprintf(w, "captureSource: %s\n", info.CaptureSource)
}
if info.CaptureOrgID != "" {
	fmt.Fprintf(w, "captureOrgId: %s\n", info.CaptureOrgID)
}
if info.TargetOrg != "" {
	fmt.Fprintf(w, "targetOrg: %s\n", info.TargetOrg)
}
```

- [ ] **Step 5: Update help**

Add `capture` to package help:

```text
glade package capture --target-org packaging --namespace pkg --output pkg.glade-package.json
```

State that it delegates to `@glade/orgpackage`.

- [ ] **Step 6: Run CLI tests**

Run:

```bash
go test ./internal/gladecli
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/gladecli/cli.go internal/gladecli/cli_test.go internal/cliui/help.go
git commit -m "feat: add package capture plugin bridge"
```

---

## Task 5: Build The `orgpackage` Capture Model In `glade-tools`

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/orgpackage/model.go`
- Create: `/Users/matt/Dev/glade-tools/internal/orgpackage/convert_test.go`
- Create: `/Users/matt/Dev/glade-tools/internal/orgpackage/convert.go`

- [ ] **Step 1: Write conversion tests**

In `convert_test.go`, create a `Capture` with:

- one package identity row
- one Apex class SymbolTable row
- one object describe row
- one label name
- one static resource name
- one LWC bundle name

Assert the resulting `packageartifact.Artifact` has deterministic sorted data and a non-empty source hash:

```go
func TestConvertCaptureToArtifact(t *testing.T) {
	capture := Capture{
		Package: PackageIdentity{
			Namespace:   "pkg",
			Name:        "Billing Core",
			Version:     "1.2.3.4",
			PackageID:   "033xx0000000001",
			InstalledID: "0A3xx0000000001",
		},
		Org: OrgIdentity{
			OrgID:      "00Dxx0000000001",
			Username:   "builder@example.com",
			TargetOrg:  "packaging",
			APIVersion: "65.0",
		},
		ApexClasses: []ApexClassContract{{
			Name:      "BillingGateway",
			Namespace: "pkg",
			Visibility: "global",
			Methods: []ApexMethodContract{{
				Name:       "authorize",
				ReturnType: "Boolean",
				Visibility: "global",
				Static:     true,
				Parameters: []ApexParameterContract{{Name: "amount", Type: "Decimal"}},
			}},
		}},
		Objects: []orgdescribe.SObject{{
			Name: "pkg__Billing_Profile__c",
			Fields: []orgdescribe.Field{{Name: "pkg__External_Key__c", Type: "string", Nillable: true}},
		}},
		Labels:          []string{"pkg__Billing_Error"},
		StaticResources: []string{"pkg__BillingAssets"},
		LightningBundles: []LightningBundleContract{{
			Namespace: "pkg",
			Name:      "billingConsole",
			Type:      "lwc",
			Exposed:   true,
		}},
		CapturedAt: time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
	}
	artifact, err := Convert(capture)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Namespace != "pkg" || artifact.PackageName != "Billing Core" || artifact.Version != "1.2.3.4" {
		t.Fatalf("artifact identity = %#v", artifact)
	}
	if len(artifact.ApexTypes) != 1 || artifact.ApexTypes[0].Members[0].Name != "authorize" {
		t.Fatalf("apex types = %#v", artifact.ApexTypes)
	}
	if artifact.SourceHash == "" {
		t.Fatal("sourceHash is empty")
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/orgpackage -run TestConvertCaptureToArtifact
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Add normalized model**

Create `model.go`:

```go
package orgpackage

import (
	"time"

	"github.com/glade-sh/glade/internal/orgdescribe"
)

type Capture struct {
	Package          PackageIdentity
	Org              OrgIdentity
	ApexClasses      []ApexClassContract
	Objects          []orgdescribe.SObject
	Labels           []string
	StaticResources  []string
	LightningBundles []LightningBundleContract
	CapturedAt        time.Time
}

type PackageIdentity struct {
	Namespace   string
	Name        string
	Version     string
	PackageID   string
	InstalledID string
}

type OrgIdentity struct {
	OrgID      string
	Username   string
	TargetOrg  string
	APIVersion string
}

type ApexClassContract struct {
	Name       string
	Namespace  string
	Visibility string
	Abstract   bool
	Interface  bool
	Enum       bool
	SuperClass string
	Interfaces []string
	Methods    []ApexMethodContract
	Properties []ApexPropertyContract
	Constructors []ApexMethodContract
}

type ApexMethodContract struct {
	Name       string
	ReturnType string
	Visibility string
	Static     bool
	Abstract   bool
	Parameters []ApexParameterContract
}

type ApexPropertyContract struct {
	Name       string
	Type       string
	Visibility string
	Static     bool
}

type ApexParameterContract struct {
	Name string
	Type string
}

type LightningBundleContract struct {
	Namespace string
	Name      string
	Type      string
	Exposed   bool
}
```

- [ ] **Step 4: Implement conversion**

Create `convert.go` with `Convert(Capture) (packageartifact.Artifact, error)`. Map contracts into `packageartifact.BuildCapturedOptions`. Use:

```go
func apexTypeFromContract(row ApexClassContract, version string) packageartifact.ApexType
func apexMembersFromContract(row ApexClassContract) []packageartifact.ApexMember
func modifierList(visibility string, isStatic bool, isAbstract bool) []string
```

Only include Apex rows with `global` or `namespaceaccessible` visibility. Set `Dependency: true` on captured Apex types.

- [ ] **Step 5: Run conversion tests**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/orgpackage
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/matt/Dev/glade-tools
git add internal/orgpackage
git commit -m "feat: model org package capture artifacts"
```

---

## Task 6: Add `sf` REST Client And Query Pagination

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/orgpackage/sfcli.go`
- Create: `/Users/matt/Dev/glade-tools/internal/orgpackage/client.go`
- Create: `/Users/matt/Dev/glade-tools/internal/orgpackage/client_test.go`

- [ ] **Step 1: Write client tests**

Cover:

```go
func TestClientEncodesToolingQuery(t *testing.T)
func TestClientFollowsNextRecordsURL(t *testing.T)
func TestSFRunnerPassesTargetOrgAndURL(t *testing.T)
```

Use a fake runner:

```go
type fakeSFRunner struct {
	calls []sfCall
	out   map[string]string
}

func (f *fakeSFRunner) Request(ctx context.Context, call sfCall) ([]byte, error) {
	f.calls = append(f.calls, call)
	return []byte(f.out[call.URL]), nil
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/orgpackage -run 'Client|SFRunner'
```

Expected: FAIL with missing client types.

- [ ] **Step 3: Implement `sf` runner**

Create:

```go
type sfCall struct {
	TargetOrg string
	Method    string
	URL       string
	Body      string
}

type SFRunner interface {
	Request(ctx context.Context, call sfCall) ([]byte, error)
}

type ExecSFRunner struct {
	Bin string
}
```

`ExecSFRunner.Request` runs:

```bash
sf api request rest <url> --target-org <target> --method <method>
```

Pass stdin only when `Body` is non-empty.

- [ ] **Step 4: Implement client**

Create:

```go
type Client struct {
	Runner     SFRunner
	TargetOrg  string
	APIVersion string
}

func (c Client) ToolingQuery(ctx context.Context, soql string, out any) error
func (c Client) DataQuery(ctx context.Context, soql string, out any) error
func (c Client) Get(ctx context.Context, path string, out any) error
```

Use `/services/data/v<version>/tooling/query/?q=<escaped>` for Tooling queries and follow `nextRecordsUrl`.

- [ ] **Step 5: Run tests**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/orgpackage
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/matt/Dev/glade-tools
git add internal/orgpackage/sfcli.go internal/orgpackage/client.go internal/orgpackage/client_test.go
git commit -m "feat: add sf rest client for orgpackage capture"
```

---

## Task 7: Capture Installed Package Identity And Apex Contracts

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/orgpackage/discover.go`
- Create: `/Users/matt/Dev/glade-tools/internal/orgpackage/tooling.go`
- Create: `/Users/matt/Dev/glade-tools/internal/orgpackage/tooling_test.go`
- Add: `/Users/matt/Dev/glade-tools/internal/orgpackage/testdata/tooling-apex-class-symboltable.json`

- [ ] **Step 1: Write discovery and SymbolTable tests**

Use fake JSON for:

```sql
SELECT Id, SubscriberPackageId, SubscriberPackage.Name, SubscriberPackage.NamespacePrefix,
SubscriberPackageVersionId, SubscriberPackageVersion.MajorVersion,
SubscriberPackageVersion.MinorVersion, SubscriberPackageVersion.PatchVersion,
SubscriberPackageVersion.BuildNumber
FROM InstalledSubscriberPackage
WHERE SubscriberPackage.NamespacePrefix = 'pkg'
```

Use fake JSON for:

```sql
SELECT Id, Name, NamespacePrefix, SymbolTable, ManageableState
FROM ApexClass
WHERE NamespacePrefix = 'pkg' AND Status = 'Active'
```

Assert package version string `1.2.3.4` and one `ApexClassContract` with method `authorize(Decimal)`.

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/orgpackage -run 'Discover|SymbolTable'
```

Expected: FAIL until discover and tooling capture are implemented.

- [ ] **Step 3: Implement package discovery**

Create:

```go
func DiscoverPackage(ctx context.Context, client Client, namespace string) (PackageIdentity, error)
```

Return a typed error when no row exists:

```go
return PackageIdentity{}, fmt.Errorf("installed package namespace %q not found in target org", namespace)
```

- [ ] **Step 4: Implement SymbolTable conversion**

Define raw structs for the JSON shape returned by Tooling. Keep only fields used by the artifact:

```go
type rawSymbolTable struct {
	TableDeclaration rawSymbol `json:"tableDeclaration"`
	Methods          []rawMethod `json:"methods"`
	Constructors     []rawMethod `json:"constructors"`
	Properties       []rawVisibilitySymbol `json:"properties"`
	InnerClasses     []rawSymbolTable `json:"innerClasses"`
}
```

Map:

- `tableDeclaration.name` to class name
- `tableDeclaration.visibility` to class visibility
- `methods[].returnType` to method return type
- `methods[].parameters[]` to parameters
- `properties[].type` to fields/properties
- `innerClasses[]` to nested names `Outer.Inner`

- [ ] **Step 5: Filter visible contracts**

Keep rows where visibility is `GLOBAL` or annotations include `NamespaceAccessible`. For normal subscriber dependencies, global members matter most. Preserve `namespaceaccessible` rows because same-namespace extension projects need them.

- [ ] **Step 6: Run tests**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/orgpackage
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/matt/Dev/glade-tools
git add internal/orgpackage/discover.go internal/orgpackage/tooling.go internal/orgpackage/tooling_test.go internal/orgpackage/testdata
git commit -m "feat: capture installed package apex contracts"
```

---

## Task 8: Capture Schema, Metadata Names, And Lightning Inventory

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/orgpackage/describe.go`
- Create: `/Users/matt/Dev/glade-tools/internal/orgpackage/describe_test.go`
- Add: `/Users/matt/Dev/glade-tools/internal/orgpackage/testdata/describe-global.json`
- Add: `/Users/matt/Dev/glade-tools/internal/orgpackage/testdata/describe-account-with-package-field.json`

- [ ] **Step 1: Write describe tests**

Use fake responses to prove the capture includes:

- custom objects named `pkg__*`
- standard objects that have fields named `pkg__*`
- relationship names and record types
- external id, unique, formula, picklist, and reference metadata

Assert conversion through `orgdescribe.Catalog.ToSchema()` returns the expected `schema.Schema`.

- [ ] **Step 2: Write metadata inventory tests**

Use Tooling fake responses for:

```sql
SELECT Name, NamespacePrefix FROM ExternalString WHERE NamespacePrefix = 'pkg'
SELECT Name, NamespacePrefix FROM StaticResource WHERE NamespacePrefix = 'pkg'
SELECT DeveloperName, NamespacePrefix, MasterLabel, IsExposed FROM LightningComponentBundle WHERE NamespacePrefix = 'pkg'
SELECT DeveloperName, NamespacePrefix FROM AuraDefinitionBundle WHERE NamespacePrefix = 'pkg'
```

Assert names become `pkg__LabelName`, `pkg__ResourceName`, and `pkg/componentName`.

- [ ] **Step 3: Run tests and verify they fail**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/orgpackage -run 'Describe|Inventory'
```

Expected: FAIL until capture functions exist.

- [ ] **Step 4: Implement describe capture**

Create:

```go
func CaptureObjects(ctx context.Context, client Client, namespace string) ([]orgdescribe.SObject, error)
```

Algorithm:

1. Get global describe from `/services/data/v<version>/sobjects`.
2. Select object names that start with `pkg__`.
3. Query `FieldDefinition` or describe standard objects to find standard objects with fields starting with `pkg__`.
4. Fetch `/services/data/v<version>/sobjects/<name>/describe` for selected objects.
5. Convert JSON into `orgdescribe.SObject`.
6. Sort objects and fields by API name.

- [ ] **Step 5: Implement metadata inventory capture**

Create:

```go
func CaptureMetadataNames(ctx context.Context, client Client, namespace string) (labels []string, staticResources []string, err error)
func CaptureLightningBundles(ctx context.Context, client Client, namespace string) ([]LightningBundleContract, error)
```

Return empty slices when the target org does not expose a Tooling object. Include the skipped query in the JSON report warnings.

- [ ] **Step 6: Run tests**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/orgpackage
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/matt/Dev/glade-tools
git add internal/orgpackage/describe.go internal/orgpackage/describe_test.go internal/orgpackage/testdata
git commit -m "feat: capture package schema and metadata names"
```

---

## Task 9: Add The `orgpackage capture` CLI

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/toolcli/orgpackage_command.go`
- Create: `/Users/matt/Dev/glade-tools/internal/toolcli/orgpackage_command_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/cli.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/manifest.go`
- Create: `/Users/matt/Dev/glade-tools/cmd/glade-plugin-orgpackage/main.go`
- Create: `/Users/matt/Dev/glade-tools/plugins/orgpackage/plugin.json`

- [ ] **Step 1: Write CLI tests**

Cover:

```go
func TestOrgPackageCaptureRequiresTargetNamespaceAndOutput(t *testing.T)
func TestOrgPackageCaptureWritesArtifactAndJSONSummary(t *testing.T)
func TestOrgPackageManifestListsCaptureCommand(t *testing.T)
func TestPackagedOrgPackageManifestMatchesRuntimeCommands(t *testing.T)
```

Expected CLI:

```bash
glade orgpackage capture --target-org packaging --namespace pkg --output .glade/packages/pkg-1.2.3.glade-package.json --json
```

JSON summary shape:

```json
{
  "ok": true,
  "artifact": ".glade/packages/pkg-1.2.3.glade-package.json",
  "namespace": "pkg",
  "version": "1.2.3.4",
  "apexTypes": 1,
  "objects": 1,
  "labels": 1,
  "staticResources": 1,
  "warnings": []
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/toolcli -run 'OrgPackage'
```

Expected: FAIL because the command does not exist.

- [ ] **Step 3: Implement command parsing**

Accepted flags:

```text
--target-org <alias>
--namespace <namespace>
--output <path>
--api-version <version>
--sf-bin <path>
--json
--config-snippet
```

`--config-snippet` prints:

```yaml
project:
  managedPackageDependencies: ["pkg:artifact:.glade/packages/pkg-1.2.3.glade-package.json:1.2.3.4"]
```

- [ ] **Step 4: Wire capture execution**

Command sequence:

```go
pkg, err := orgpackage.DiscoverPackage(ctx, client, opts.Namespace)
apex, err := orgpackage.CaptureApexContracts(ctx, client, opts.Namespace)
objects, err := orgpackage.CaptureObjects(ctx, client, opts.Namespace)
labels, resources, err := orgpackage.CaptureMetadataNames(ctx, client, opts.Namespace)
bundles, err := orgpackage.CaptureLightningBundles(ctx, client, opts.Namespace)
artifact, err := orgpackage.Convert(capture)
err = packageartifact.WriteJSON(opts.Output, artifact)
```

- [ ] **Step 5: Add plugin manifest**

`plugins/orgpackage/plugin.json`:

```json
{
  "apiVersion": "glade.plugin.v1",
  "name": "orgpackage",
  "version": "0.1.0",
  "summary": "Capture installed package contracts from Salesforce orgs into Glade package artifacts.",
  "commands": [
    {"path": ["orgpackage"], "summary": "Capture and inspect org package artifacts."},
    {"path": ["orgpackage", "capture"], "summary": "Capture an installed package artifact from a Salesforce org."}
  ],
  "minimumGladeVersion": "0.1.0",
  "source": "github.com/glade-sh/glade/tools"
}
```

- [ ] **Step 6: Add binary entrypoint**

`cmd/glade-plugin-orgpackage/main.go`:

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

- [ ] **Step 7: Run CLI tests**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/toolcli ./internal/orgpackage
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
cd /Users/matt/Dev/glade-tools
git add internal/toolcli cmd/glade-plugin-orgpackage plugins/orgpackage
git commit -m "feat: add orgpackage capture plugin command"
```

---

## Task 10: Package And Publish The Plugin

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/scripts/build-plugin-archives.sh`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/plugin_command_test.go`
- Modify: `/Users/matt/Dev/glade-tools/dist/plugins/index.json` only when building release artifacts for a checked release step

- [ ] **Step 1: Write packaging tests**

Extend manifest tests to require command root `orgpackage`.

Add a script check test if shell tests exist. If not, add a Go test that reads `scripts/build-plugin-archives.sh` and asserts it contains `orgpackage` in:

- binary switch
- plugin loop
- registry `commands`

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/toolcli -run 'Manifest|Packaged'
```

Expected: FAIL until manifests and script include orgpackage.

- [ ] **Step 3: Update build script**

Add `orgpackage` to each plugin loop. Add ldflags case:

```bash
orgpackage)
  ldflags="-X github.com/glade-sh/glade/tools/internal/toolcli.pluginVersion=$VERSION"
  ;;
```

Add registry row:

```bash
orgpackage)
  canonical="@glade/orgpackage"
  aliases='["orgpackage"]'
  summary="Capture installed package contracts from Salesforce orgs into Glade package artifacts."
  commands='["orgpackage"]'
  docs="https://glade.sh/guide/plugins/first-party"
  ;;
```

- [ ] **Step 4: Build one local target**

Run:

```bash
cd /Users/matt/Dev/glade-tools
TARGETS=darwin/arm64 OUT_DIR=/tmp/glade-plugin-build scripts/build-plugin-archives.sh 0.1.0
tar -tzf /tmp/glade-plugin-build/glade-plugin-orgpackage_0.1.0_darwin_arm64.tar.gz
```

Expected archive entries:

```text
bin/
bin/glade-plugin-orgpackage
plugin.json
checksums.txt
```

- [ ] **Step 5: Commit**

```bash
cd /Users/matt/Dev/glade-tools
git add scripts/build-plugin-archives.sh internal/toolcli/plugin_command_test.go
git commit -m "build: package orgpackage plugin"
```

---

## Task 11: End-To-End Local Proof With Fake `sf`

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/orgpackage/e2e_test.go`
- Add: `/Users/matt/Dev/glade-tools/internal/orgpackage/testdata/fake-sf/*.json`
- Modify: `/Users/matt/Dev/glade-tools/internal/toolcli/orgpackage_command_test.go`

- [ ] **Step 1: Write fake `sf` script test**

Create a temp executable named `sf` that reads the requested URL and prints matching testdata JSON. Run:

```go
code := toolcli.Run(context.Background(), []string{
	"orgpackage", "capture",
	"--target-org", "packaging",
	"--namespace", "pkg",
	"--sf-bin", fakeSFPath,
	"--output", artifactPath,
	"--json",
}, &stdout, &stderr)
```

Assert:

- exit code is 0
- artifact file exists
- `packageartifact.Validate` returns no issues
- artifact can be loaded by a tiny Glade project through `managedPackageDependencies`

- [ ] **Step 2: Run e2e test and verify it fails**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/orgpackage ./internal/toolcli -run 'FakeSF|EndToEnd'
```

Expected: FAIL until all URLs and testdata are complete.

- [ ] **Step 3: Complete fake responses**

Provide JSON files for:

- org display or limits response used for org identity
- installed package query
- ApexClass SymbolTable query
- global describe
- object describe
- metadata name queries
- Lightning bundle query

- [ ] **Step 4: Run e2e tests**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/orgpackage ./internal/toolcli -run 'FakeSF|EndToEnd'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/matt/Dev/glade-tools
git add internal/orgpackage/e2e_test.go internal/orgpackage/testdata internal/toolcli/orgpackage_command_test.go
git commit -m "test: prove orgpackage capture with fake sf"
```

---

## Task 12: Documentation And Site Coverage

**Files:**
- Modify: `docs/CONFIG.md`
- Modify: `docs/INSTALL.md`
- Modify: `site/docs-src/guide/configuration.md`
- Modify: `site/docs-src/guide/cli-reference.md`
- Modify: `site/docs-src/guide/plugins/first-party.md` if present
- Modify: `site/tests/theme.test.mjs`
- Modify: `/Users/matt/Dev/glade-tools/README.md`

- [ ] **Step 1: Write docs test expectations**

In `site/tests/theme.test.mjs`, assert the docs include:

```js
assert.match(configuration, /packageShims/);
assert.match(cliReference, /glade orgpackage capture --target-org packaging --namespace pkg --output/);
assert.match(cliReference, /managedPackageDependencies: \["pkg:artifact:/);
```

- [ ] **Step 2: Run site tests and verify they fail**

Run:

```bash
npm --prefix site test -- --runInBand
```

Expected: FAIL until docs text exists.

- [ ] **Step 3: Update docs**

Add examples:

```bash
glade plugins install @glade/orgpackage
glade orgpackage capture \
  --target-org packaging \
  --namespace pkg \
  --output .glade/packages/pkg-1.2.3.glade-package.json \
  --json
glade package validate .glade/packages/pkg-1.2.3.glade-package.json
glade package info .glade/packages/pkg-1.2.3.glade-package.json
```

Add config:

```yaml
project:
  managedPackageDependencies:
    - "pkg:artifact:.glade/packages/pkg-1.2.3.glade-package.json:1.2.3.4"
  packageShims:
    - "pkg:test-support/package-shims/pkg"
```

State the runtime boundary:

```text
Captured artifacts provide contracts. They do not include package method bodies. Tests that execute package behavior need local shims or a hosted Salesforce run.
```

- [ ] **Step 4: Run docs tests**

Run:

```bash
npm --prefix site test -- --runInBand
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add docs/CONFIG.md docs/INSTALL.md site/docs-src site/tests/theme.test.mjs
git commit -m "docs: explain captured package artifacts"

cd /Users/matt/Dev/glade-tools
git add README.md
git commit -m "docs: add orgpackage capture workflow"
```

---

## Task 13: Full Verification Gate

**Files:**
- No source edits.

- [ ] **Step 1: Verify Glade core**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./internal/packageartifact ./internal/project ./internal/typesys ./internal/config ./internal/gladecli ./internal/apextest
```

Expected: PASS.

- [ ] **Step 2: Verify Glade broad surface if the focused gate passes**

Run:

```bash
cd /Users/matt/Dev/glade
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Verify tools**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/orgpackage ./internal/toolcli
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Verify plugin archive**

Run:

```bash
cd /Users/matt/Dev/glade-tools
TARGETS=darwin/arm64 OUT_DIR=/tmp/glade-plugin-build scripts/build-plugin-archives.sh 0.1.0
test -f /tmp/glade-plugin-build/glade-plugin-orgpackage_0.1.0_darwin_arm64.tar.gz
```

Expected: command exits 0.

- [ ] **Step 5: Verify no whitespace damage**

Run:

```bash
cd /Users/matt/Dev/glade
git diff --check
cd /Users/matt/Dev/glade-tools
git diff --check
```

Expected: no output.

---

## Self-Review

- Spec coverage: The plan covers artifact format, org capture through `sf`, local dependency consumption without source, runtime boundaries, shims, plugin packaging, docs, and verification.
- Boundary check: Live org access stays in `glade-tools` and the plugin. Base Glade only owns artifact and runtime consumption.
- Routing check: `orgpackage` is the plugin root. `glade package capture` is a bridge because plugin routing dispatches on the first token.
- Placeholder scan: No task depends on unnamed files or open-ended error handling.
- Type consistency: `Capture`, `PackageIdentity`, `ApexClassContract`, `BuildCapturedOptions`, and `CaptureProvenance` names match across tasks.
