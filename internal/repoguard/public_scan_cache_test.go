package repoguard

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The public-data scan stores each tracked file's findings under the git
// blob hash of its content and the ruleset version, so unchanged files are
// not rescanned on the next run. These regressions pin that contract from
// both sides: a warm cache must still catch violations in files the store
// has not seen — newly added, modified, or carrying a violating compressed
// member — and a ruleset change must invalidate the stored records.

func publicScanPlantedNameSum(name string) string {
	digest := sha256.Sum256([]byte(campaignNameSalt + name))
	return hex.EncodeToString(digest[:])
}

func publicScanPlantedMatcher() *campaignNameMatch {
	return campaignNameMatcher([]campaignName{
		{campaignNamePrefix, len("glade-planted-"), publicScanPlantedNameSum("glade-planted-")},
	})
}

func publicScanGzipMember(t *testing.T, text string) string {
	t.Helper()
	var out bytes.Buffer
	writer := gzip.NewWriter(&out)
	if _, err := writer.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// publicScanDivergence reports the first position where two public-data
// scans disagree, comparing paths, private-capture flags, findings and
// decode errors. The matched text itself is never part of a scan, so this
// compares exactly what the guards report.
func publicScanDivergence(left, right []publicScanFile) (int, bool) {
	if len(left) != len(right) {
		return 0, false
	}
	for index := range left {
		if left[index].rel != right[index].rel ||
			left[index].privateCapture != right[index].privateCapture ||
			!slices.Equal(left[index].campaignPath, right[index].campaignPath) ||
			publicScanErrorText(left[index].err) != publicScanErrorText(right[index].err) ||
			!slices.EqualFunc(left[index].release, right[index].release, publicScanFindingsEqual) ||
			!slices.EqualFunc(left[index].campaign, right[index].campaign, publicScanFindingsEqual) {
			return index, false
		}
	}
	return 0, true
}

func publicScanFindingsEqual(left, right publicScanFinding) bool {
	return left.member == right.member && left.line == right.line && left.kind == right.kind
}

func publicScanErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestGitBlobHashMatchesGitFormat(t *testing.T) {
	// The hashes git itself produces for these blobs; the format is what lets
	// a record key survive across checkouts and match git's own object names.
	if got := gitBlobHash(""); got != "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391" {
		t.Fatalf("empty blob hash = %s, want git's empty-blob hash", got)
	}
	if got := gitBlobHash("hello\n"); got != "ce013625030ba8dba906f756967f9e9ca394464a" {
		t.Fatalf("blob hash of \"hello\\n\" = %s, want git's hash", got)
	}
}

func TestPublicScanBlobCacheSkipsUnchangedFilesAcrossRuns(t *testing.T) {
	files := []repoScanFile{
		{rel: "docs/one.md", text: "public notes\n"},
		{rel: "docs/two.json", text: "{\"owner\": \"public\"}\n"},
	}
	names := campaignNameMatcher(nil)
	dir := t.TempDir()
	countingDecode := func(counter *int) publicAssetDecoder {
		return func(rel, text string, visit func(string, string)) error {
			*counter++
			return scanPublicAsset(rel, text, visit)
		}
	}

	coldDecodes := 0
	cold := loadPublicScanBlobCache(dir, "test-v1")
	scans := scanTrackedFilesForPublicDataCached(files, names, countingDecode(&coldDecodes), cold, 2)
	cold.save()
	if coldDecodes != len(files) {
		t.Fatalf("cold run decodes = %d, want %d", coldDecodes, len(files))
	}

	warmDecodes := 0
	warm := loadPublicScanBlobCache(dir, "test-v1")
	rescanned := scanTrackedFilesForPublicDataCached(files, names, countingDecode(&warmDecodes), warm, 2)
	if warmDecodes != 0 {
		t.Fatalf("decodes on the warm run = %d, want 0: unchanged files must not be rescanned", warmDecodes)
	}
	if index, equal := publicScanDivergence(scans, rescanned); !equal {
		t.Fatalf("warm scan diverged from the cold scan at %d: %s vs %s", index, publicScanErrorText(scans[index].err), publicScanErrorText(rescanned[index].err))
	}
}

func TestPublicScanBlobCacheKeysCarryRulesetVersion(t *testing.T) {
	const blob = "0123456789abcdef0123456789abcdef01234567"
	if publicScanCacheKey("rules-v1", "docs/one.md", blob) == publicScanCacheKey("rules-v2", "docs/one.md", blob) {
		t.Fatal("cache key ignores the ruleset version; a ruleset change must invalidate stored records")
	}
	if publicScanCacheKey("rules-v1", "docs/one.md", blob) == publicScanCacheKey("rules-v1", "docs/other.md", blob) {
		t.Fatal("cache key ignores the tracked path; both rulesets read the path")
	}
	if publicScanCacheKey("rules-v1", "docs/one.md", blob) == publicScanCacheKey("rules-v1", "docs/one.md", "fedcba9876543210fedcba9876543210fedcba98") {
		t.Fatal("cache key ignores the blob hash; changed content must not reuse stored records")
	}
}

func TestPublicScanBlobCacheCatchesViolationsWithWarmCache(t *testing.T) {
	names := publicScanPlantedMatcher()
	clean := []repoScanFile{
		{rel: "docs/one.md", text: "public notes\n"},
		{rel: "docs/catalog.json.gz", text: publicScanGzipMember(t, "{\"email\":\"safe@example.com\"}")},
	}
	dir := t.TempDir()
	base := loadPublicScanBlobCache(dir, "test-v1")
	scanTrackedFilesForPublicDataCached(clean, names, scanPublicAsset, base, 2)
	base.save()
	warm := func(files []repoScanFile) []publicScanFile {
		cache := loadPublicScanBlobCache(dir, "test-v1")
		scans := scanTrackedFilesForPublicDataCached(files, names, scanPublicAsset, cache, 2)
		cache.save()
		return scans
	}

	t.Run("newly added file", func(t *testing.T) {
		files := append(slices.Clone(clean), repoScanFile{rel: "docs/added.go", text: "// tracked in " + "RO" + "AD-12\n"})
		scans := warm(files)
		got := scans[len(scans)-1].campaign
		if len(got) != 1 || got[0].member != "" || got[0].kind != "campaign roadmap reference" {
			t.Fatalf("campaign findings on the added file = %v, want one roadmap reference", got)
		}
	})

	t.Run("modified file", func(t *testing.T) {
		files := slices.Clone(clean)
		files[0] = repoScanFile{rel: "docs/one.md", text: "contact person" + "@gmail.com\n"}
		scans := warm(files)
		got := scans[0].release
		if len(got) != 1 || got[0].kind != "personal-provider mailbox" {
			t.Fatalf("release findings on the modified file = %v, want the planted mailbox", got)
		}
	})

	t.Run("compressed member", func(t *testing.T) {
		files := slices.Clone(clean)
		files[1] = repoScanFile{rel: "docs/catalog.json.gz", text: publicScanPlantedCatalog(t)}
		scans := warm(files)
		if scans[1].err != nil {
			t.Fatalf("planted gzip member failed to decode: %v", scans[1].err)
		}
		release := scans[1].release
		if len(release) != 1 || release[0].member != " (member 1)" || release[0].kind != "personal-provider mailbox" {
			t.Fatalf("release findings inside the compressed member = %v, want one mailbox in member 1", release)
		}
		campaign := scans[1].campaign
		if len(campaign) != 1 || campaign[0].member != " (member 1)" || campaign[0].kind != "campaign roadmap reference" {
			t.Fatalf("campaign findings inside the compressed member = %v, want one roadmap reference in member 1", campaign)
		}
	})
}

func TestPublicScanBlobCacheRulesetChangeInvalidatesEntries(t *testing.T) {
	cleanNames := campaignNameMatcher(nil)
	strictNames := publicScanPlantedMatcher()
	files := []repoScanFile{
		{rel: "docs/glade-planted-x/notes.md", text: "public notes\n"},
	}
	dir := t.TempDir()

	underOld := loadPublicScanBlobCache(dir, "rules-v1")
	scans := scanTrackedFilesForPublicDataCached(files, cleanNames, scanPublicAsset, underOld, 2)
	if len(scans[0].campaignPath) != 0 {
		t.Fatalf("the old ruleset already flagged the planted alias: %v", scans[0].campaignPath)
	}
	underOld.save()

	// The old run recorded this path and blob clean; the changed ruleset must
	// rescan the file rather than reuse the stored records.
	underNew := loadPublicScanBlobCache(dir, "rules-v2")
	rescanned := scanTrackedFilesForPublicDataCached(files, strictNames, scanPublicAsset, underNew, 2)
	if got := rescanned[0].campaignPath; len(got) != 1 || got[0] != "private org alias or host name" {
		t.Fatalf("records under the old ruleset suppressed the new ruleset's finding: %v", got)
	}
}

func TestPublicScanBlobCacheWarmRunMatchesColdScan(t *testing.T) {
	names := publicScanPlantedMatcher()
	catalog := publicScanPlantedCatalog(t)
	files := []repoScanFile{
		{rel: "docs/one.md", text: "public notes\n"},
		{rel: "docs/glade-planted-x/notes.md", text: "the " + "glade-planted-x checkout\n"},
		{rel: "docs/mail.md", text: "contact person" + "@gmail.com about " + "RO" + "AD-12\n"},
		{rel: "docs/catalog.json.gz", text: catalog},
		{rel: "docs/broken.json.gz", text: catalog[:len(catalog)-8]},
		{rel: "docs/plans/ORG-RUN.txt", text: "person" + "@gmail.com\n"},
		{rel: "src/decision.go", text: "package src\n\n// decision " + "D9\n"},
		{rel: "testdata/fixture.json", text: "{\"review_note\":\"tracked in " + "RO" + "AD-12\"}\n"},
	}
	cold := scanTrackedFilesForPublicData(files, names, scanPublicAsset)

	// The fixtures must produce findings on both sides of the comparison, so
	// equality is not vacuous.
	if cold[1].campaignPath[0] != "private org alias or host name" {
		t.Fatalf("planted alias finding = %v, want one private org alias or host name", cold[1].campaignPath)
	}
	if len(cold[2].release) != 1 || len(cold[2].campaign) != 1 {
		t.Fatalf("planted text findings = %v %v, want one mailbox and one roadmap reference", cold[2].release, cold[2].campaign)
	}
	if cold[4].err == nil {
		t.Fatal("planted corrupt asset did not fail the shared decode")
	}
	if len(cold[6].release) != 1 || cold[6].release[0].kind != "campaign reference in source comment" {
		t.Fatalf("planted comment finding = %v, want one campaign reference in source comment", cold[6].release)
	}
	if len(cold[7].release) != 1 || cold[7].release[0].kind != "campaign reference in fixture metadata" {
		t.Fatalf("planted fixture finding = %v, want one campaign reference in fixture metadata", cold[7].release)
	}

	dir := t.TempDir()
	cache := loadPublicScanBlobCache(dir, "test-v1")
	first := scanTrackedFilesForPublicDataCached(files, names, scanPublicAsset, cache, 2)
	cache.save()
	second := scanTrackedFilesForPublicDataCached(files, names, scanPublicAsset, loadPublicScanBlobCache(dir, "test-v1"), 2)
	if index, equal := publicScanDivergence(cold, first); !equal {
		t.Fatalf("first cached scan diverged from the uncached engine at %d: %+v vs %+v", index, cold[index], first[index])
	}
	if index, equal := publicScanDivergence(cold, second); !equal {
		t.Fatalf("warm cached scan diverged from the uncached engine at %d: %+v vs %+v", index, cold[index], second[index])
	}
}

func TestPublicScanBlobCacheRecordsAreCopies(t *testing.T) {
	names := publicScanPlantedMatcher()
	files := []repoScanFile{
		{rel: "docs/glade-planted-x/notes.md", text: "contact person" + "@gmail.com\n"},
	}
	dir := t.TempDir()
	cache := loadPublicScanBlobCache(dir, "test-v1")
	scans := scanTrackedFilesForPublicDataCached(files, names, scanPublicAsset, cache, 2)
	cache.save()
	record := cache.records[publicScanCacheKey(cache.version, files[0].rel, gitBlobHash(files[0].text))]

	// A guard mutating its scan cannot reach the stored record, and a
	// mutation of the stored record cannot reach a later scan.
	scans[0].campaignPath[0] = "mutated"
	scans[0].release[0].kind = "mutated"
	if record.CampaignPath[0] == "mutated" || record.Release[0].Kind == "mutated" {
		t.Fatalf("mutating a scan changed the stored record: %+v", record)
	}
	record.CampaignPath[0] = "mutated"
	warm := loadPublicScanBlobCache(dir, "test-v1")
	rescanned := scanTrackedFilesForPublicDataCached(files, names, scanPublicAsset, warm, 2)
	if rescanned[0].campaignPath[0] == "mutated" || rescanned[0].release[0].kind == "mutated" {
		t.Fatalf("mutating the stored record changed a warm scan: %+v", rescanned[0])
	}
	if rescanned[0].campaignPath[0] != "private org alias or host name" || rescanned[0].release[0].kind != "personal-provider mailbox" {
		t.Fatalf("warm scan findings = %v %v, want the planted alias and mailbox", rescanned[0].campaignPath, rescanned[0].release)
	}
}

func TestPublicScanBlobCacheFallsBackToFullScanWhenStoreMissingOrCorrupt(t *testing.T) {
	names := campaignNameMatcher(nil)
	files := []repoScanFile{
		{rel: "docs/mail.md", text: "contact person" + "@gmail.com\n"},
		{rel: "docs/notes.md", text: "public\n"},
	}
	dir := t.TempDir()

	t.Run("missing store", func(t *testing.T) {
		cache := loadPublicScanBlobCache(dir, "test-v1")
		scans := scanTrackedFilesForPublicDataCached(files, names, scanPublicAsset, cache, 2)
		if got := scans[0].release; len(got) != 1 || got[0].kind != "personal-provider mailbox" {
			t.Fatalf("findings with no stored records = %v, want the planted mailbox", got)
		}
	})

	t.Run("corrupt store", func(t *testing.T) {
		cache := loadPublicScanBlobCache(dir, "test-v1")
		if err := os.WriteFile(cache.path, []byte("\xff\xfe not a record store"), 0o644); err != nil {
			t.Fatal(err)
		}
		corrupt := loadPublicScanBlobCache(dir, "test-v1")
		scans := scanTrackedFilesForPublicDataCached(files, names, scanPublicAsset, corrupt, 2)
		if got := scans[0].release; len(got) != 1 || got[0].kind != "personal-provider mailbox" {
			t.Fatalf("findings over a corrupt store = %v, want a full scan reporting the mailbox", got)
		}
	})

	t.Run("null store", func(t *testing.T) {
		cache := loadPublicScanBlobCache(dir, "test-v1")
		if err := os.WriteFile(cache.path, []byte("null"), 0o644); err != nil {
			t.Fatal(err)
		}
		empty := loadPublicScanBlobCache(dir, "test-v1")
		scans := scanTrackedFilesForPublicDataCached(files, names, scanPublicAsset, empty, 2)
		if got := scans[0].release; len(got) != 1 || got[0].kind != "personal-provider mailbox" {
			t.Fatalf("findings over a null store = %v, want a full scan reporting the mailbox", got)
		}
	})

	t.Run("no cache location", func(t *testing.T) {
		cache := loadPublicScanBlobCache("", "test-v1")
		scans := scanTrackedFilesForPublicDataCached(files, names, scanPublicAsset, cache, 2)
		if got := scans[0].release; len(got) != 1 || got[0].kind != "personal-provider mailbox" {
			t.Fatalf("findings without a cache location = %v, want the planted mailbox", got)
		}
	})
}

// The record store's ruleset version must follow the rules it guards with.
// A manual constant silently keeps cached 'clean' records suppressing a
// new ruleset's findings whenever someone edits a rule, table or decoder
// and forgets to bump it, so the version is derived from the guard
// package's own sources and the running toolchain instead. These
// regressions pin that derivation and its effect on the store.

// publicScanOverrideRulesetInputs pins the version derivation's inputs for
// one test, so a ruleset edit is a byte difference the derivation must
// see, and restores the repository checkout's inputs afterwards.
func publicScanOverrideRulesetInputs(t *testing.T, inputs []publicScanRulesetInput, goVersion string) {
	t.Helper()
	previous := publicScanRulesetInputs
	publicScanRulesetInputs = func() ([]publicScanRulesetInput, string) {
		return inputs, goVersion
	}
	t.Cleanup(func() {
		publicScanRulesetInputs = previous
	})
}

func TestPublicScanRulesetVersionDerivation(t *testing.T) {
	rules := []publicScanRulesetInput{
		{rel: "internal/repoguard/public_campaign_test.go", data: "campaign rules\n"},
		{rel: "internal/repoguard/public_safety_test.go", data: "release rules\n"},
	}
	version := publicScanRulesetVersion(rules, "go1.26.11")
	if len(version) != 64 {
		t.Fatalf("ruleset version %q is not a SHA-256 digest", version)
	}
	if again := publicScanRulesetVersion(rules, "go1.26.11"); again != version {
		t.Fatalf("ruleset version is not deterministic: %q vs %q", version, again)
	}
	if reordered := publicScanRulesetVersion([]publicScanRulesetInput{rules[1], rules[0]}, "go1.26.11"); reordered != version {
		t.Fatalf("ruleset version depends on input order: %q vs %q; a reader refactor must not flap the store cold", reordered, version)
	}
	edited := slices.Clone(rules)
	edited[0].data += "// one more rule\n"
	if got := publicScanRulesetVersion(edited, "go1.26.11"); got == version {
		t.Fatal("ruleset version ignores a rule-source edit; stored records must not survive a ruleset change")
	}
	renamed := slices.Clone(rules)
	renamed[0].rel = "internal/repoguard/moved_test.go"
	if got := publicScanRulesetVersion(renamed, "go1.26.11"); got == version {
		t.Fatal("ruleset version ignores a rule-source rename")
	}
	if got := publicScanRulesetVersion(rules[:1], "go1.26.11"); got == version {
		t.Fatal("ruleset version ignores a dropped rule source")
	}
	if got := publicScanRulesetVersion(rules, "go1.27.0"); got == version {
		t.Fatal("ruleset version ignores the Go toolchain")
	}
}

func TestPublicScanRulesetInputsCoverGuardSources(t *testing.T) {
	inputs, goVersion := publicScanRulesetInputs()
	if goVersion != runtime.Version() {
		t.Fatalf("ruleset inputs carry Go version %q, want the running toolchain %q", goVersion, runtime.Version())
	}
	// The hashed sources are the checkout's own guard package: every Go file,
	// including the ones defining the rules, matchers, normalizers, tables
	// and the asset decoder, so none of them can silently drop out of the
	// version.
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), "internal", "repoguard"))
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			want = append(want, "internal/repoguard/"+entry.Name())
		}
	}
	slices.Sort(want)
	got := make([]string, 0, len(inputs))
	for _, input := range inputs {
		got = append(got, input.rel)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("ruleset inputs = %v, want every guard package source %v", got, want)
	}
	for _, input := range inputs {
		if input.data == "" {
			t.Fatalf("%s contributes no bytes to the ruleset version", input.rel)
		}
	}
	campaign := "internal/repoguard/public_campaign_test.go"
	for _, input := range inputs {
		if input.rel == campaign && input.data != readRepoFile(t, repoRoot(t), input.rel) {
			t.Fatal("ruleset inputs do not carry the checkout's bytes for the campaign rules")
		}
	}
	edited := slices.Clone(inputs)
	edited[0].data += "// one more rule\n"
	if publicScanRulesetVersion(edited, goVersion) == publicScanRulesetVersion(inputs, goVersion) {
		t.Fatal("ruleset version ignores the guard sources' bytes")
	}
}

func TestPublicScanRulesetChangeForcesFullRescan(t *testing.T) {
	cleanNames := campaignNameMatcher(nil)
	strictNames := publicScanPlantedMatcher()
	files := []repoScanFile{
		{rel: "docs/glade-planted-x/notes.md", text: "public notes\n"},
	}
	ruleset := []publicScanRulesetInput{
		{rel: "internal/repoguard/public_campaign_test.go", data: "campaign rules\n"},
	}
	dir := t.TempDir()
	decodes := 0
	counting := func(rel, text string, visit func(string, string)) error {
		decodes++
		return scanPublicAsset(rel, text, visit)
	}

	publicScanOverrideRulesetInputs(t, ruleset, "testgo1")
	old := loadPublicScanBlobCache(dir, currentPublicScanRulesetVersion())
	scans := scanTrackedFilesForPublicDataCached(files, cleanNames, counting, old, 2)
	old.save()
	if decodes != 1 {
		t.Fatalf("cold run decodes = %d, want 1", decodes)
	}
	if len(scans[0].campaignPath) != 0 {
		t.Fatalf("the first ruleset already flagged the planted alias: %v", scans[0].campaignPath)
	}
	warm := loadPublicScanBlobCache(dir, currentPublicScanRulesetVersion())
	if len(warm.records) != 1 {
		t.Fatalf("warm records = %d, want the one scanned file", len(warm.records))
	}
	decodes = 0
	scanTrackedFilesForPublicDataCached(files, cleanNames, counting, warm, 2)
	if decodes != 0 {
		t.Fatalf("warm run decodes = %d, want 0: unchanged files are not rescanned", decodes)
	}

	// One rule-source byte changes: the derived version moves, so the stored
	// clean record cannot be read, and the strict ruleset's finding surfaces.
	edited := slices.Clone(ruleset)
	edited[0].data += "// one more name\n"
	publicScanOverrideRulesetInputs(t, edited, "testgo1")
	if updated := currentPublicScanRulesetVersion(); updated == old.version {
		t.Fatal("a rule-source edit did not change the ruleset version")
	}
	decodes = 0
	rescanned := scanTrackedFilesForPublicDataCached(files, strictNames, counting, loadPublicScanBlobCache(dir, currentPublicScanRulesetVersion()), 2)
	if decodes != 1 {
		t.Fatalf("decodes after the ruleset change = %d, want 1: stored records must not be reused", decodes)
	}
	if got := rescanned[0].campaignPath; len(got) != 1 || got[0] != "private org alias or host name" {
		t.Fatalf("records under the old ruleset suppressed the new ruleset's finding: %v", got)
	}
}

func TestPublicScanBlobCacheNeverReadAcrossVersions(t *testing.T) {
	names := publicScanPlantedMatcher()
	files := []repoScanFile{
		{rel: "docs/mail.md", text: "contact person" + "@gmail.com\n"},
	}
	ruleset := []publicScanRulesetInput{
		{rel: "internal/repoguard/public_safety_test.go", data: "release rules\n"},
	}
	dir := t.TempDir()

	publicScanOverrideRulesetInputs(t, ruleset, "testgo1")
	first := loadPublicScanBlobCache(dir, currentPublicScanRulesetVersion())
	scans := scanTrackedFilesForPublicDataCached(files, names, scanPublicAsset, first, 2)
	first.save()
	if got := scans[0].release; len(got) != 1 || got[0].kind != "personal-provider mailbox" {
		t.Fatalf("findings under the first version = %v, want the planted mailbox", got)
	}
	if _, err := os.Stat(first.path); err != nil {
		t.Fatalf("the first version's store was not written: %v", err)
	}

	// The toolchain moves: the derived version follows, and the warm store
	// built under the old version is never read under the new one.
	publicScanOverrideRulesetInputs(t, ruleset, "testgo2")
	second := currentPublicScanRulesetVersion()
	if second == first.version {
		t.Fatal("the derived ruleset version did not move with the toolchain")
	}
	foreign := loadPublicScanBlobCache(dir, second)
	if len(foreign.records) != 0 {
		t.Fatalf("records built under one ruleset version were read under another: %d records", len(foreign.records))
	}
	rescans := scanTrackedFilesForPublicDataCached(files, names, scanPublicAsset, foreign, 2)
	if got := rescans[0].release; len(got) != 1 || got[0].kind != "personal-provider mailbox" {
		t.Fatalf("findings under the new version = %v, want the planted mailbox", got)
	}
	foreign.save()
	if reloaded := loadPublicScanBlobCache(dir, first.version); len(reloaded.records) != 0 {
		t.Fatalf("the old version's store survived a save under the new version: %d records", len(reloaded.records))
	}
}
