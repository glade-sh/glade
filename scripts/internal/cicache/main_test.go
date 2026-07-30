package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckWorkflowRejectsRepeatedCheckout(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - uses: actions/checkout@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
      - uses: actions/checkout@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
`))
	if !hasProblem(errs, "fixture.yml/test: repeated checkout") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCICacheHasOneManifestLaneAndTestMetadata(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	manifestData, err := os.ReadFile(filepath.Join(repoRoot, "scripts", "ci-package-lanes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Lanes map[string][]string `json:"lanes"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	const packageName = "github.com/glade-sh/glade/scripts/internal/cicache"
	owners := 0
	for _, packages := range manifest.Lanes {
		for _, candidate := range packages {
			if candidate == packageName {
				owners++
			}
		}
	}
	if owners != 1 {
		t.Fatalf("%s lane owners = %d, want 1", packageName, owners)
	}
	command := exec.Command("go", "list", "-f", "{{if or .TestGoFiles .XTestGoFiles}}has-tests{{else}}no-tests{{end}}", packageName)
	command.Dir = repoRoot
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(output)) != "has-tests" {
		t.Fatalf("%s test metadata = %q, want has-tests", packageName, output)
	}
}

func TestCheckWorkflowRejectsDuplicateExactCacheSaveKey(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - uses: actions/cache/save@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: ~/.cache/go-build
          key: go-build-v1-${{ runner.os }}-${{ runner.arch }}-1.26.5-${{ hashFiles('go.sum') }}
      - uses: actions/cache/save@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: ~/.cache/go-build
          key: go-build-v1-${{ runner.os }}-${{ runner.arch }}-1.26.5-${{ hashFiles('go.sum') }}
`))
	if !hasProblem(errs, "fixture.yml/test: duplicate immutable cache save key") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRejectsRestoreAfterGoCachePopulation(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - run: go test ./...
      - uses: actions/cache/restore@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: ~/.cache/go-build
          key: go-build-v1-${{ runner.os }}-${{ runner.arch }}-1.26.5-${{ hashFiles('go.sum') }}
`))
	if !hasProblem(errs, "fixture.yml/test: cache restore for ~/.cache/go-build follows a command that can populate it") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRejectsResolvableGoCacheKeyWithoutDependencyOrToolchain(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - uses: actions/cache/restore@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: ~/.cache/go-build
          key: go-build-v1-${{ runner.os }}-${{ runner.arch }}
`))
	if !hasProblem(errs, "fixture.yml/test: Go cache key lacks a dependency hash") || !hasProblem(errs, "fixture.yml/test: Go cache key lacks a toolchain component") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRejectsCacheKeyWithStaleResolvedToolchain(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - uses: actions/setup-go@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          go-version: "1.27.0"
      - uses: actions/cache/restore@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: ~/.cache/go-build
          key: go-build-v1-${{ runner.os }}-${{ runner.arch }}-1.26.5-${{ hashFiles('go.sum') }}
`))
	if !hasProblem(errs, "fixture.yml/test: Go cache key lacks the resolved toolchain component 1.27.0") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRejectsNPMCacheKeyWithStaleResolvedToolchain(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "24"
      - uses: actions/cache/restore@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: /tmp/npm-cache
          key: npm-cache-v1-${{ runner.os }}-${{ runner.arch }}-22-${{ hashFiles('package-lock.json') }}
`))
	if !hasProblem(errs, "fixture.yml/test: npm cache key lacks the resolved toolchain component 24") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowDoesNotReadToolchainFromDependencyExpression(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "24"
      - uses: actions/cache/restore@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: /tmp/npm-cache
          key: npm-cache-v1-${{ runner.os }}-${{ runner.arch }}-${{ hashFiles('package-24-lock.json') }}
`))
	if !hasProblem(errs, "fixture.yml/test: npm cache key lacks the resolved toolchain component 24") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRejectsSaveKeyMissingVaryingMatrixDimension(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  build:
    strategy:
      matrix:
        archive: [linux_amd64, darwin_arm64]
    steps:
      - uses: actions/cache/save@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: /tmp/go-build
          key: release-go-v1-${{ runner.os }}-1.26.5-${{ hashFiles('go.sum') }}
`))
	if !hasProblem(errs, "fixture.yml/build: cache save key lacks varying matrix dimension archive") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRequiresExplicitSetupGoCachePolicy(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - uses: actions/setup-go@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          go-version: "1.26.5"
`))
	if !hasProblem(errs, "fixture.yml/test: setup-go must set cache: false") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowAcceptsBenchmarkIsolatedCacheAndUpload(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - uses: actions/checkout@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
      - uses: actions/setup-go@bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb # v1.0.0
        with:
          go-version: "1.26.5"
          cache: false
      - uses: actions/cache/restore@cccccccccccccccccccccccccccccccccccccccc # v1.0.0
        with:
          path: ~/.cache/go-build
          key: ${{ inputs.benchmark_cache_pair > 0 && format('bench-{0}-', inputs.benchmark_cache_pair) || '' }}go-build-v1-${{ runner.os }}-${{ runner.arch }}-1.26.5-${{ hashFiles('go.sum') }}
      - run: go test ./...
      - uses: actions/upload-artifact@dddddddddddddddddddddddddddddddddddddddd # v1.0.0
        with:
          name: test-log
          path: ci-artifacts/test.json
`))
	if len(errs) != 0 {
		t.Fatalf("problems = %s", strings.Join(errs, "; "))
	}
}

func TestCheckWorkflowResolvesCachePrimaryKeyForSaveValidation(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - uses: actions/setup-go@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          go-version: "1.26.5"
          cache: false
      - id: restore-build
        uses: actions/cache/restore@bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb # v1.0.0
        with:
          path: ~/.cache/go-build
          key: go-build-v1-${{ runner.os }}-${{ runner.arch }}-1.26.5-${{ hashFiles('go.sum') }}
      - uses: actions/cache/save@cccccccccccccccccccccccccccccccccccccccc # v1.0.0
        with:
          path: ~/.cache/go-build
          key: ${{ steps.restore-build.outputs.cache-primary-key }}
`))
	if len(errs) != 0 {
		t.Fatalf("problems = %s", strings.Join(errs, "; "))
	}
}

func TestCheckWorkflowRejectsUnpinnedActionsAndIncompleteUploads(t *testing.T) {
	errs := checkWorkflow("ci.yml", []byte(`
jobs:
  test:
    steps:
      - uses: actions/checkout@v6
      - uses: actions/upload-artifact@bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb # v1.0.0
        with:
          name: test-log
`))
	if !hasProblem(errs, "ci.yml/test: action is not pinned to a full SHA") || !hasProblem(errs, "ci.yml/test: upload artifact lacks a name or path") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRejectsDuplicateExactCacheSaveKeyAcrossJobs(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  first:
    steps:
      - uses: actions/cache/save@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: /tmp/cache
          key: immutable-cache-key
  second:
    steps:
      - uses: actions/cache/save@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: /tmp/cache
          key: immutable-cache-key
`))
	if !hasProblem(errs, "fixture.yml/second: duplicate immutable cache save key") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRejectsRestoreAfterExactPathIsPopulated(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - run: mkdir -p /tmp/tool-cache
      - uses: actions/cache/restore@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: /tmp/tool-cache
          key: tool-cache-v1
`))
	if !hasProblem(errs, "fixture.yml/test: cache restore for /tmp/tool-cache follows a command that can populate it") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestNormalizedCachePathsOverlapOnlyAtDirectoryBoundaries(t *testing.T) {
	if !pathsOverlap("/tmp/cache", "/tmp/cache/build") {
		t.Fatal("nested cache paths must overlap")
	}
	if pathsOverlap("/tmp/cache", "/tmp/cache-other") {
		t.Fatal("prefix-only cache paths must not overlap")
	}
}

func TestCheckWorkflowAllowsBuildCacheRestoreAfterGoModDownload(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    env:
      GOMODCACHE: /tmp/go-mod
      GOCACHE: /tmp/go-build
    steps:
      - run: go mod download
      - uses: actions/cache/restore@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: ${{ env.GOCACHE }}
          key: go-build-v1-${{ runner.os }}-${{ runner.arch }}-1.26.5-${{ hashFiles('go.sum') }}
`))
	if len(errs) != 0 {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRejectsRestorePrefixThatDropsABIComponent(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - uses: actions/cache/restore@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: ~/.cache/go-build
          key: go-build-v1-${{ runner.os }}-${{ runner.arch }}-1.26.5-${{ hashFiles('go.sum') }}
          restore-keys: |
            go-build-v1-${{ runner.os }}-${{ hashFiles('go.sum') }}-
`))
	if !hasProblem(errs, "fixture.yml/test: restore key drops ABI component runner.arch") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowModelsSetupNodeImplicitNPMCache(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "22"
          cache: npm
`))
	if !hasProblem(errs, "fixture.yml/test: setup-node npm cache lacks cache-dependency-path") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRejectsSetupNodeCacheAfterNPMWork(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  test:
    steps:
      - run: npm ci
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: package-lock.json
`))
	if !hasProblem(errs, "fixture.yml/test: setup-node npm cache restore follows a command that can populate it") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCachePathsIncludeSetupNodeImplicitNPMCache(t *testing.T) {
	paths := cachePaths(job{Steps: []step{{
		Uses: "actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		With: map[string]any{
			"node-version":          "22",
			"cache":                 "npm",
			"cache-dependency-path": "site/package-lock.json",
		},
	}}})
	if len(paths) != 1 || paths[0] != "npm-cache://22/site/package-lock.json" {
		t.Fatalf("implicit npm cache paths = %v", paths)
	}
}

func TestCheckWorkflowRequiresKnownArtifactUpload(t *testing.T) {
	errs := checkWorkflow("ci.yml", []byte(`
jobs:
  gladecli:
    steps:
      - uses: actions/checkout@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
`))
	if !hasProblem(errs, "ci.yml/gladecli: required artifact upload missing: go-test-gladecli") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRequiresEveryKnownArtifactPath(t *testing.T) {
	errs := checkWorkflow("ci.yml", []byte(`
jobs:
  gladecli:
    steps:
      - uses: actions/upload-artifact@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          name: go-test-gladecli
          path: ci-artifacts/go-test/test-gladecli.json
`))
	if !hasProblem(errs, "ci.yml/gladecli: required artifact upload missing path: ci-artifacts/go-test/resource-gladecli.json") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRejectsFalseRequiredArtifactUpload(t *testing.T) {
	errs := checkWorkflow("ci.yml", []byte(`
jobs:
  gladecli:
    steps:
      - uses: actions/upload-artifact@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        if: false
        with:
          name: go-test-gladecli
          path: |
            ci-artifacts/go-test/test-gladecli.json
            ci-artifacts/go-test/resource-gladecli.json
`))
	if !hasProblem(errs, "ci.yml/gladecli: required artifact upload is disabled: go-test-gladecli") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowDoesNotAcceptArtifactPathSubstring(t *testing.T) {
	errs := checkWorkflow("ci.yml", []byte(`
jobs:
  gladecli:
    steps:
      - uses: actions/upload-artifact@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          name: go-test-gladecli
          path: |
            ci-artifacts/go-test/test-gladecli.json.bak
            ci-artifacts/go-test/resource-gladecli.json
`))
	if !hasProblem(errs, "ci.yml/gladecli: required artifact upload missing path: ci-artifacts/go-test/test-gladecli.json") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckCacheEvidenceRejectsNegativeThreeSampleCache(t *testing.T) {
	problems, err := checkCacheEvidence([]byte(`{
  "version": 1,
  "samples": [
    {"cache":"go-build","sample":"one","observed_at":"2026-07-29T00:00:00Z","transfer_seconds":11,"avoided_work_seconds":10},
    {"cache":"go-build","sample":"two","observed_at":"2026-07-29T00:01:00Z","transfer_seconds":12,"avoided_work_seconds":10},
    {"cache":"go-build","sample":"three","observed_at":"2026-07-29T00:02:00Z","transfer_seconds":13,"avoided_work_seconds":10}
  ]
}`))
	if err != nil {
		t.Fatal(err)
	}
	if !hasProblem(problems, "cache evidence go-build is negative after 3 consecutive samples") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestCheckCacheEvidenceWaitsForThreeSamples(t *testing.T) {
	problems, err := checkCacheEvidence([]byte(`{
  "version": 1,
  "samples": [
    {"cache":"go-build","sample":"one","observed_at":"2026-07-29T00:00:00Z","transfer_seconds":11,"avoided_work_seconds":10},
    {"cache":"go-build","sample":"two","observed_at":"2026-07-29T00:01:00Z","transfer_seconds":12,"avoided_work_seconds":10}
  ]
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

func TestCheckCacheEvidenceRequiresThreeConsecutiveNegativeSamples(t *testing.T) {
	problems, err := checkCacheEvidence([]byte(`{
  "version": 1,
  "samples": [
    {"cache":"go-build","sample":"one","observed_at":"2026-07-29T00:00:00Z","transfer_seconds":11,"avoided_work_seconds":10},
    {"cache":"go-build","sample":"two","observed_at":"2026-07-29T00:01:00Z","transfer_seconds":12,"avoided_work_seconds":10},
    {"cache":"go-build","sample":"three","observed_at":"2026-07-29T00:02:00Z","transfer_seconds":1,"avoided_work_seconds":10},
    {"cache":"go-build","sample":"four","observed_at":"2026-07-29T00:03:00Z","transfer_seconds":13,"avoided_work_seconds":10},
    {"cache":"go-build","sample":"five","observed_at":"2026-07-29T00:04:00Z","transfer_seconds":14,"avoided_work_seconds":10}
  ]
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

func TestCheckWorkflowDirRejectsDuplicateImmutableSaveAcrossWorkflowFiles(t *testing.T) {
	dir := t.TempDir()
	workflow := `jobs:
  test:
    steps:
      - uses: actions/cache/save@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: /tmp/cache
          key: immutable-cache-key
`
	if err := os.WriteFile(filepath.Join(dir, "first.yml"), []byte(workflow), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "second.yml"), []byte(workflow), 0o600); err != nil {
		t.Fatal(err)
	}
	problems, err := checkWorkflowDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasProblem(problems, "second.yml/test: duplicate immutable cache save key") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestCheckWorkflowDirRejectsDuplicateImplicitNPMWriterAcrossWorkflowFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(`{"packages":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	workflow := `jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: package-lock.json
`
	if err := os.WriteFile(filepath.Join(dir, "first.yml"), []byte(workflow), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "second.yml"), []byte(workflow), 0o600); err != nil {
		t.Fatal(err)
	}
	problems, err := checkWorkflowDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasProblem(problems, "second.yml/test: duplicate implicit npm cache writer (already owned by first.yml/test)") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestCheckWorkflowDirRejectsImplicitNPMWritersWithDistinctIdenticalLockfiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"first-lock.json", "second-lock.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`{"packages":{"node_modules/shared":{"version":"1.0.0"}}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	workflowForLock := func(lockfile string) string {
		return `jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: ` + lockfile + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "first.yml"), []byte(workflowForLock("first-lock.json")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "second.yml"), []byte(workflowForLock("second-lock.json")), 0o600); err != nil {
		t.Fatal(err)
	}
	problems, err := checkWorkflowDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasProblem(problems, "second.yml/test: duplicate implicit npm cache writer (already owned by first.yml/test)") {
		t.Fatalf("same-content implicit writers accepted: %v", problems)
	}
}

func TestCheckWorkflowDirRejectsIdenticalImplicitNPMContentsAcrossEquivalentLinuxLabels(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(`{"packages":{"node_modules/shared":{"version":"1.0.0"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	workflowForRunner := func(runner string) string {
		return `jobs:
  test:
    runs-on: ` + runner + `
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: package-lock.json
`
	}
	if err := os.WriteFile(filepath.Join(dir, "first.yml"), []byte(workflowForRunner("ubuntu-latest")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "second.yml"), []byte(workflowForRunner("ubuntu-24.04")), 0o600); err != nil {
		t.Fatal(err)
	}
	problems, err := checkWorkflowDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasProblem(problems, "second.yml/test: duplicate implicit npm cache writer (already owned by first.yml/test)") {
		t.Fatalf("equivalent Linux setup-node writers accepted: %v", problems)
	}
}

func TestCheckCacheEvidenceRejectsTrailingJSONValue(t *testing.T) {
	_, err := checkCacheEvidence([]byte(`{"version":1,"samples":[]} {}`))
	if err == nil || !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("error = %v", err)
	}
}

func TestStrictEvidenceRejectsAbsentEmptyUnmappedDuplicateAndIncompleteSamples(t *testing.T) {
	identities := map[string]bool{"fixture.yml/test/explicit:/tmp/cache": true}
	for name, data := range map[string][]byte{
		"empty":     []byte(`{"version":1,"samples":[]}`),
		"unmapped":  []byte(`{"version":1,"samples":[{"cache":"unknown","sample":"one","observed_at":"2026-07-29T00:00:00Z","transfer_seconds":1,"avoided_work_seconds":2}]}`),
		"duplicate": []byte(`{"version":1,"samples":[{"cache":"fixture.yml/test/explicit:/tmp/cache","sample":"one","observed_at":"2026-07-29T00:00:00Z","transfer_seconds":1,"avoided_work_seconds":2},{"cache":"fixture.yml/test/explicit:/tmp/cache","sample":"one","observed_at":"2026-07-29T00:01:00Z","transfer_seconds":1,"avoided_work_seconds":2}]}`),
		"short":     []byte(`{"version":1,"samples":[{"cache":"fixture.yml/test/explicit:/tmp/cache","sample":"one","observed_at":"2026-07-29T00:00:00Z","transfer_seconds":1,"avoided_work_seconds":2},{"cache":"fixture.yml/test/explicit:/tmp/cache","sample":"two","observed_at":"2026-07-29T00:01:00Z","transfer_seconds":1,"avoided_work_seconds":2}]}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := checkStrictEvidence(data, identities); err == nil {
				t.Fatal("strict evidence accepted incomplete input")
			}
		})
	}
}

func TestStrictEvidenceRequiresMappedThreeSampleEvidenceForEveryCache(t *testing.T) {
	identities := map[string]bool{
		"fixture.yml/test/explicit:/tmp/cache":               true,
		"fixture.yml/test/implicit-npm:22/package-lock.json": true,
	}
	data := []byte(`{"version":1,"samples":[
{"cache":"fixture.yml/test/explicit:/tmp/cache","sample":"one","observed_at":"2026-07-29T00:00:00Z","transfer_seconds":1,"avoided_work_seconds":2},
{"cache":"fixture.yml/test/explicit:/tmp/cache","sample":"two","observed_at":"2026-07-29T00:01:00Z","transfer_seconds":1,"avoided_work_seconds":2},
{"cache":"fixture.yml/test/explicit:/tmp/cache","sample":"three","observed_at":"2026-07-29T00:02:00Z","transfer_seconds":1,"avoided_work_seconds":2}]}`)
	if _, err := checkStrictEvidence(data, identities); err == nil || !strings.Contains(err.Error(), "implicit-npm") {
		t.Fatalf("strict evidence error = %v", err)
	}
}

func TestCheckWorkflowRejectsReleaseIncludeSaveKeyWithoutUniqueProducer(t *testing.T) {
	errs := checkWorkflow("release.yml", []byte(`
jobs:
  build:
    strategy:
      matrix:
        include:
          - archive: linux_amd64
            artifact: linux-amd64
          - archive: darwin_arm64
            artifact: darwin-arm64
    steps:
      - uses: actions/cache@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: /tmp/go-build
          key: release-go-v1-${{ runner.os }}-1.26.5-${{ hashFiles('go.sum') }}
`))
	if !hasProblem(errs, "release.yml/build: cache save key does not uniquely identify matrix include producers") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestReleaseIncludeMatrixKeyCannotDropArchiveDimension(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.ReplaceAll(string(data), "-${{ matrix.archive }}-", "-")
	errs := checkWorkflow("release.yml", []byte(mutated))
	if !hasProblem(errs, "release.yml/build: cache save key does not uniquely identify matrix include producers") {
		t.Fatalf("archive-removal mutation accepted: %v", errs)
	}
}

func TestCheckWorkflowConservativelyRequiresDynamicMatrixDimension(t *testing.T) {
	errs := checkWorkflow("race.yml", []byte(`
jobs:
  race:
    strategy:
      matrix:
        package: ${{ fromJSON(needs.plan.outputs.packages) }}
    steps:
      - uses: actions/cache/save@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: /tmp/go-build
          key: race-go-v1-${{ runner.os }}-1.26.5-${{ hashFiles('go.sum') }}
`))
	if !hasProblem(errs, "race.yml/race: cache save key lacks varying matrix dimension package") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRejectsSelectorBoundExplicitSaveForOpaqueDynamicMatrix(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  race:
    strategy:
      matrix:
        package: ${{ fromJSON(needs.plan.outputs.packages) }}
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "22"
      - id: restore-npm
        uses: actions/cache/restore@bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb # v1.0.0
        with:
          path: /tmp/npm-cache
          key: race-npm-v1-${{ runner.os }}-${{ runner.arch }}-22-${{ hashFiles('package-lock.json') }}
      - uses: actions/cache/save@cccccccccccccccccccccccccccccccccccccccc # v1.0.0
        if: ${{ success() && matrix.package == './internal/gladecli' }}
        with:
          path: /tmp/npm-cache
          key: ${{ steps.restore-npm.outputs.cache-primary-key }}
`))
	if !hasProblem(errs, "cache save key lacks varying matrix dimension package") {
		t.Fatalf("opaque dynamic selector-bound save accepted: %v", errs)
	}
}

func TestCheckWorkflowRejectsAmbiguousSelectorBoundExplicitSaveWithoutMatrixKey(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  race:
    strategy:
      matrix:
        package: ${{ fromJSON(needs.plan.outputs.packages) }}
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "22"
      - id: restore-npm
        uses: actions/cache/restore@bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb # v1.0.0
        with:
          path: /tmp/npm-cache
          key: race-npm-v1-${{ runner.os }}-${{ runner.arch }}-22-${{ hashFiles('package-lock.json') }}
      - uses: actions/cache/save@cccccccccccccccccccccccccccccccccccccccc # v1.0.0
        if: ${{ matrix.package == './internal/gladecli' || matrix.package == './internal/server' }}
        with:
          path: /tmp/npm-cache
          key: ${{ steps.restore-npm.outputs.cache-primary-key }}
`))
	if !hasProblem(errs, "cache save key lacks varying matrix dimension package") {
		t.Fatalf("ambiguous selector-bound save accepted: %v", errs)
	}
}

func TestCheckWorkflowRejectsNegatedSelectorBoundExplicitSaveWithoutMatrixKey(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  race:
    strategy:
      matrix:
        package: ${{ fromJSON(needs.plan.outputs.packages) }}
    steps:
      - uses: actions/cache/save@cccccccccccccccccccccccccccccccccccccccc # v1.0.0
        if: ${{ !(matrix.package == './internal/gladecli') }}
        with:
          path: /tmp/npm-cache
          key: race-npm-v1-${{ runner.os }}-${{ runner.arch }}-22-${{ hashFiles('package-lock.json') }}
`))
	if !hasProblem(errs, "cache save key lacks varying matrix dimension package") {
		t.Fatalf("negated selector-bound save accepted: %v", errs)
	}
}

func TestCheckWorkflowRejectsUnsupportedSelectorBooleanForm(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  race:
    strategy:
      matrix:
        package: ${{ fromJSON(needs.plan.outputs.packages) }}
    steps:
      - uses: actions/cache/save@cccccccccccccccccccccccccccccccccccccccc # v1.0.0
        if: ${{ contains(fromJSON('["./internal/gladecli"]'), matrix.package) && matrix.package == './internal/gladecli' }}
        with:
          path: /tmp/npm-cache
          key: race-npm-v1-${{ runner.os }}-${{ runner.arch }}-22-${{ hashFiles('package-lock.json') }}
`))
	if !hasProblem(errs, "cache save key lacks varying matrix dimension package") {
		t.Fatalf("unsupported selector boolean form accepted: %v", errs)
	}
}

func TestCheckWorkflowRejectsDoubleQuotedExpressionStringSelector(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  race:
    strategy:
      matrix:
        package:
          - ./internal/gladecli
          - ./internal/server
    steps:
      - uses: actions/cache/save@cccccccccccccccccccccccccccccccccccccccc # v1.0.0
        if: ${{ matrix.package == "./internal/gladecli" }}
        with:
          path: /tmp/npm-cache
          key: race-npm-v1-${{ runner.os }}-${{ runner.arch }}-22-${{ hashFiles('package-lock.json') }}
`))
	if !hasProblem(errs, "cache save key lacks varying matrix dimension package") {
		t.Fatalf("double-quoted expression selector accepted: %v", errs)
	}
}

func TestCheckWorkflowRejectsCaseVariantIncludeRowsSelectedByOneStringEquality(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  race:
    strategy:
      matrix:
        include:
          - package: ./internal/gladecli
          - package: ./INTERNAL/GLADECLI
    steps:
      - uses: actions/cache/save@cccccccccccccccccccccccccccccccccccccccc # v1.0.0
        if: ${{ matrix.package == './internal/gladecli' }}
        with:
          path: /tmp/npm-cache
          key: race-npm-v1-${{ runner.os }}-${{ runner.arch }}-22-${{ hashFiles('package-lock.json') }}
`))
	if !hasProblem(errs, "cache save key does not uniquely identify matrix include producers") {
		t.Fatalf("case-insensitive duplicate writers accepted: %v", errs)
	}
}

func TestCheckWorkflowRejectsCoerciveIncludeRowsSelectedByNumericEquality(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  race:
    strategy:
      matrix:
        include:
          - shard: 0
          - shard: false
    steps:
      - uses: actions/cache/save@cccccccccccccccccccccccccccccccccccccccc # v1.0.0
        if: ${{ matrix.shard == 0 }}
        with:
          path: /tmp/go-build
          key: race-go-v1-${{ runner.os }}-1.26.5-${{ hashFiles('go.sum') }}
`))
	if !hasProblem(errs, "cache save key does not uniquely identify matrix include producers") {
		t.Fatalf("coercive duplicate writers accepted: %v", errs)
	}
}

func TestCheckWorkflowAllowsTwoDimensionPositiveStringSelectorWithOneEligibleRow(t *testing.T) {
	errs := checkWorkflow("fixture.yml", []byte(`
jobs:
  race:
    strategy:
      matrix:
        package:
          - ./internal/gladecli
          - ./internal/server
        platform:
          - linux
          - windows
    steps:
      - uses: actions/cache/save@cccccccccccccccccccccccccccccccccccccccc # v1.0.0
        if: ${{ matrix.package == './internal/gladecli' && matrix.platform == 'linux' }}
        with:
          path: /tmp/npm-cache
          key: race-npm-v1-${{ runner.os }}-${{ runner.arch }}-22-${{ hashFiles('package-lock.json') }}
`))
	if hasProblem(errs, "cache save key does not uniquely identify matrix include producers") ||
		hasProblem(errs, "cache save selector does not match a matrix include producer") {
		t.Fatalf("positive two-dimension selector rejected: %v", errs)
	}
}

func TestCheckWorkflowRejectsImplicitNPMCacheInDynamicMatrixWithoutSingleWriter(t *testing.T) {
	errs := checkWorkflow("race.yml", []byte(`
jobs:
  race:
    strategy:
      matrix:
        package: ${{ fromJSON(needs.plan.outputs.packages) }}
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: third_party/lwc/package-lock.json
`))
	if !hasProblem(errs, "race.yml/race: setup-node npm cache requires a deterministic single matrix writer") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRejectsImplicitNPMCacheWithAmbiguousMatrixWriter(t *testing.T) {
	errs := checkWorkflow("race.yml", []byte(`
jobs:
  race:
    strategy:
      matrix:
        package: ${{ fromJSON(needs.plan.outputs.packages) }}
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        if: ${{ matrix.package == './internal/gladecli' || matrix.package == './internal/server' }}
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: third_party/lwc/package-lock.json
`))
	if !hasProblem(errs, "race.yml/race: setup-node npm cache requires a deterministic single matrix writer") {
		t.Fatalf("problems = %v", errs)
	}
}

func TestCheckWorkflowRejectsImplicitNPMCacheWithNegatedMatrixWriter(t *testing.T) {
	errs := checkWorkflow("race.yml", []byte(`
jobs:
  race:
    strategy:
      matrix:
        package: ${{ fromJSON(needs.plan.outputs.packages) }}
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        if: ${{ !(matrix.package == './internal/gladecli') }}
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: third_party/lwc/package-lock.json
`))
	if !hasProblem(errs, "race.yml/race: setup-node npm cache requires a deterministic single matrix writer") {
		t.Fatalf("negated implicit npm selector accepted: %v", errs)
	}
}

func TestCacheIdentityChangesWhenSamePathPrimaryKeyChanges(t *testing.T) {
	workflowForKey := func(key string) workflow {
		return workflow{Jobs: map[string]job{
			"test": {Steps: []step{{Uses: "actions/cache@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", With: map[string]any{"path": "/tmp/cache", "key": key}}}},
		}}
	}
	first := cacheIdentities("fixture.yml", workflowForKey("cache-v1-a"))
	second := cacheIdentities("fixture.yml", workflowForKey("cache-v1-b"))
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("identity counts = %d/%d", len(first), len(second))
	}
	for identity := range first {
		if second[identity] {
			t.Fatalf("same-path key change retained evidence identity %q", identity)
		}
	}
}

func TestCacheIdentityChangesWhenWriterSelectorChanges(t *testing.T) {
	workflowForSelector := func(condition string) workflow {
		return workflow{Jobs: map[string]job{
			"race": {Steps: []step{{
				Uses: "actions/cache/save@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				If:   condition,
				With: map[string]any{"path": "/tmp/npm-cache", "key": "race-npm-v1"},
			}}},
		}}
	}
	first := cacheIdentities("race.yml", workflowForSelector("${{ matrix.package == './internal/gladecli' }}"))
	second := cacheIdentities("race.yml", workflowForSelector("${{ matrix.package == './internal/server' }}"))
	assertIdentitySetsDoNotOverlap(t, first, second)
}

func TestCacheIdentityNormalizesWriterSelectorWhitespace(t *testing.T) {
	workflowForSelector := func(condition string) workflow {
		return workflow{Jobs: map[string]job{
			"race": {Steps: []step{{
				Uses: "actions/cache/save@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				If:   condition,
				With: map[string]any{"path": "/tmp/npm-cache", "key": "race-npm-v1"},
			}}},
		}}
	}
	first := cacheIdentities("race.yml", workflowForSelector("${{ matrix.package == './internal/gladecli' }}"))
	second := cacheIdentities("race.yml", workflowForSelector("  ${{   matrix.package   ==   './internal/gladecli'   }}  "))
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("identity counts = %d/%d", len(first), len(second))
	}
	for identity := range first {
		if !second[identity] {
			t.Fatalf("selector whitespace changed evidence identity %q", identity)
		}
	}
}

func TestCacheIdentityChangesWhenActionPinChanges(t *testing.T) {
	workflowForAction := func(uses string) workflow {
		return workflow{Jobs: map[string]job{
			"race": {Steps: []step{{Uses: uses, With: map[string]any{"path": "/tmp/npm-cache", "key": "race-npm-v1"}}}},
		}}
	}
	first := cacheIdentities("race.yml", workflowForAction("actions/cache/save@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	second := cacheIdentities("race.yml", workflowForAction("actions/cache/save@bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"))
	assertIdentitySetsDoNotOverlap(t, first, second)
}

func TestCacheIdentitiesDirChangesWhenImplicitDependencyContentsChange(t *testing.T) {
	first := writeIdentityFixture(t, "package-lock.json", `{"packages":{}}`, `
jobs:
  test:
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: package-lock.json
`)
	second := writeIdentityFixture(t, "package-lock.json", `{"packages":{"node_modules/large":{"version":"99.0.0"}}}`, `
jobs:
  test:
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: package-lock.json
`)
	assertIdentitySetsDoNotOverlap(t, first, second)
}

func TestCacheIdentitiesDirChangesWhenExplicitHashFilesContentChanges(t *testing.T) {
	first := writeIdentityFixture(t, "go.sum", "module-a v1.0.0\n", `
jobs:
  test:
    steps:
      - uses: actions/cache/restore@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: /tmp/go-build
          key: go-build-v1-${{ runner.os }}-1.26.5-${{ hashFiles('go.sum') }}
`)
	second := writeIdentityFixture(t, "go.sum", "module-a v2.0.0\n", `
jobs:
  test:
    steps:
      - uses: actions/cache/restore@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          path: /tmp/go-build
          key: go-build-v1-${{ runner.os }}-1.26.5-${{ hashFiles('go.sum') }}
`)
	assertIdentitySetsDoNotOverlap(t, first, second)
}

func TestCacheIdentitiesDirResolvesCheckoutSubdirectoryInputs(t *testing.T) {
	repo := t.TempDir()
	workflowDir := filepath.Join(repo, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "package-lock.json"), []byte(`{"packages":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	workflow := `jobs:
  test:
    steps:
      - uses: actions/checkout@bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb # v1.0.0
        with:
          path: source
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: source/package-lock.json
`
	if err := os.WriteFile(filepath.Join(workflowDir, "ci.yml"), []byte(workflow), 0o600); err != nil {
		t.Fatal(err)
	}
	identities, err := cacheIdentitiesDir(workflowDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(identities) != 1 {
		t.Fatalf("identity count = %d, want 1", len(identities))
	}
}

func TestCacheIdentitiesDirResolvesRepositoryWorkflowDependencies(t *testing.T) {
	workflowDir := filepath.Join("..", "..", "..", ".github", "workflows")
	identities, err := cacheIdentitiesDir(workflowDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(identities) == 0 {
		t.Fatal("repository workflows produced no strict cache identities")
	}
}

func TestCacheIdentitiesDirRejectsMissingOrDynamicDependencyInputs(t *testing.T) {
	for name, dependencyPath := range map[string]string{
		"missing": "missing-package-lock.json",
		"dynamic": "${{ matrix.dependency_path }}",
	} {
		t.Run(name, func(t *testing.T) {
			repo := t.TempDir()
			workflow := `jobs:
  test:
    steps:
      - uses: actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa # v1.0.0
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: ` + dependencyPath + "\n"
			if err := os.WriteFile(filepath.Join(repo, "ci.yml"), []byte(workflow), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := cacheIdentitiesDir(repo); err == nil {
				t.Fatalf("strict identity accepted %s dependency input", name)
			}
		})
	}
}

func writeIdentityFixture(t *testing.T, dependencyPath, dependencyContents, workflow string) map[string]bool {
	t.Helper()
	repo := t.TempDir()
	fullDependencyPath := filepath.Join(repo, filepath.FromSlash(dependencyPath))
	if err := os.MkdirAll(filepath.Dir(fullDependencyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullDependencyPath, []byte(dependencyContents), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "ci.yml"), []byte(workflow), 0o600); err != nil {
		t.Fatal(err)
	}
	identities, err := cacheIdentitiesDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	return identities
}

func assertIdentitySetsDoNotOverlap(t *testing.T, first, second map[string]bool) {
	t.Helper()
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("identity counts = %d/%d", len(first), len(second))
	}
	for identity := range first {
		if second[identity] {
			t.Fatalf("configuration change retained evidence identity %q", identity)
		}
	}
}

func TestImplicitNPMCacheIdentityChangesWhenRaceMatrixProducerChanges(t *testing.T) {
	workflowForMatrix := func(value string) workflow {
		return workflow{Jobs: map[string]job{
			"race": {
				Strategy: strategy{Matrix: map[string]any{"package": value}},
				Steps:    []step{{Uses: "actions/setup-node@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", With: map[string]any{"cache": "npm", "node-version": "22", "cache-dependency-path": "third_party/lwc/package-lock.json"}}},
			},
		}}
	}
	first := cacheIdentities("race.yml", workflowForMatrix("${{ fromJSON(needs.plan.outputs.packages) }}"))
	second := cacheIdentities("race.yml", workflowForMatrix("${{ fromJSON(needs.plan.outputs.non_apextest_packages) }}"))
	for identity := range first {
		if second[identity] {
			t.Fatalf("race implicit npm producer change retained identity %q", identity)
		}
	}
}

func hasProblem(errs []string, want string) bool {
	for _, err := range errs {
		if strings.Contains(err, want) {
			return true
		}
	}
	return false
}
