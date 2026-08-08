package repoguard

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/watch"
)

var (
	helpCommandNamePattern     = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	markdownHeadingLinePattern = regexp.MustCompile(`^ {0,3}#{1,6}\s+`)
	builtHelpSubcommandPattern = regexp.MustCompile(`^\s{2}([a-z][a-z0-9-]*(?: [a-z][a-z0-9-]*)*)\s{2,}\S`)
)

func TestBuiltHelpMatchesAuthoritativeCheckedCommandCatalog(t *testing.T) {
	root := repoRoot(t)
	bin := buildGladeForHelpContract(t, root)

	live := parseBuiltCommandInventory(t, runBuiltGlade(t, bin, "help", "commands"))
	checked := parseCompatibilityCommandCatalog(t, readRepoFile(t, root, "docs/COMPATIBILITY.md"))
	if diff := diffStringSets(live, checked); diff != "" {
		t.Fatalf("built `glade help commands` and docs/COMPATIBILITY.md command catalog differ:\n%s", diff)
	}

	for _, command := range siteCLIReferenceHeadingCommands(t, readRepoFile(t, root, "site/docs-src/reference/cli.md")) {
		parts := strings.Fields(command)
		if !containsSortedString(live, parts[0]) {
			t.Errorf("site CLI reference claims unknown command %q", command)
			continue
		}
		if len(parts) == 1 {
			continue
		}

		help := runBuiltGlade(t, bin, append([]string{"help"}, parts...)...)
		if !containsSortedString(parseBuiltHelpCommandPaths(t, help), command) {
			t.Errorf("site CLI reference heading %q is not an exact command path in its live structured help", command)
		}
	}
}

func TestStartupCacheDocsNameLiveFormatVersion(t *testing.T) {
	root := repoRoot(t)
	want := startupcache.Version
	for _, rel := range []string{
		"docs/TEST_STARTUP_CACHE.md",
		"site/docs-src/guide/test-startup-cache.md",
	} {
		got := parseClaimedCacheVersion(t, rel, readRepoFile(t, root, rel))
		if got != want {
			t.Errorf("%s claims startup cache version %d; live implementation uses %d", rel, got, want)
		}
	}
}

func TestAffectedTestDocsDescribeLiveTransitiveSelection(t *testing.T) {
	root := t.TempDir()
	writeHelpContractFile(t, filepath.Join(root, "Helper.cls"), "public class Helper { public static void go() {} }")
	writeHelpContractFile(t, filepath.Join(root, "Service.cls"), "public class Service { void run() { Helper.go(); } }")
	writeHelpContractFile(t, filepath.Join(root, "ServiceTest.cls"), "@IsTest class ServiceTest { @IsTest static void runs() { Service service = new Service(); } }")
	writeHelpContractFile(t, filepath.Join(root, "OtherTest.cls"), "@IsTest class OtherTest { @IsTest static void runs() { System.assert(true); } }")

	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			helpContractClass(root, "Helper", false),
			helpContractClass(root, "Service", false),
			helpContractClass(root, "ServiceTest", true),
			helpContractClass(root, "OtherTest", true),
		},
	}
	selection := watch.SelectAffectedTests(index, []watch.Change{{
		Path: filepath.Join(root, "Helper.cls"),
		Op:   watch.ChangeModified,
		Kind: watch.FileKindApexClass,
		Name: "Helper",
	}})
	if selection.Mode != watch.SelectionDirect || len(selection.TestClasses) != 1 || selection.TestClasses[0] != "ServiceTest" {
		t.Fatalf("live transitive selection = %#v, want direct [ServiceTest]", selection)
	}

	repoRoot := repoRoot(t)
	for _, rel := range []string{
		"docs/LOCAL_TESTING.md",
		"site/docs-src/guide/affected-tests.md",
	} {
		section := affectedSelectionSection(t, rel, readRepoFile(t, repoRoot, rel))
		if !describesTransitiveSelection(section) {
			t.Errorf("%s does not describe the live transitive affected-test selection", rel)
		}
		if claimsDirectOnlySelection(section) {
			t.Errorf("%s claims affected tests come only from direct references", rel)
		}
	}
}

func TestAffectedSelectionClaimClassifier(t *testing.T) {
	tests := []struct {
		name           string
		claim          string
		wantTransitive bool
		wantDirectOnly bool
	}{
		{
			name:           "transitive selection",
			claim:          "Changed classes select tests through direct and transitive references.",
			wantTransitive: true,
		},
		{
			name:           "instead of transitive",
			claim:          "Changed classes select tests through direct references instead of transitive references.",
			wantDirectOnly: true,
		},
		{
			name:           "rather than transitive",
			claim:          "Changed classes select tests through direct references rather than transitive references.",
			wantDirectOnly: true,
		},
		{
			name:           "stops before transitive",
			claim:          "Selection for changed classes selects tests through direct references and stops before transitive references.",
			wantDirectOnly: true,
		},
		{
			name:           "stopping before transitive",
			claim:          "Selection for changed classes selects tests through direct references, stopping before transitive references.",
			wantDirectOnly: true,
		},
		{
			name:           "not selected only from direct",
			claim:          "Changed classes select tests that reference them. Tests are not selected only from direct references; transitive dependents are included.",
			wantTransitive: true,
		},
		{
			name:           "not selected solely from direct",
			claim:          "Changed classes select tests that reference them. Tests are not selected solely from direct references; transitive dependents are included.",
			wantTransitive: true,
		},
		{
			name:           "not selected exclusively from direct",
			claim:          "Changed classes select tests that reference them. Tests are not selected exclusively from direct references; transitive dependents are included.",
			wantTransitive: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := describesTransitiveSelection(test.claim); got != test.wantTransitive {
				t.Errorf("describesTransitiveSelection() = %v, want %v", got, test.wantTransitive)
			}
			if got := claimsDirectOnlySelection(test.claim); got != test.wantDirectOnly {
				t.Errorf("claimsDirectOnlySelection() = %v, want %v", got, test.wantDirectOnly)
			}
		})
	}
}

func buildGladeForHelpContract(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "glade")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/glade")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOMAXPROCS=2")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build glade help authority: %v\n%s", err, output)
	}
	return bin
}

func runBuiltGlade(t *testing.T, bin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s %s: %v\n%s", bin, strings.Join(args, " "), err, output)
	}
	return string(output)
}

func parseBuiltCommandInventory(t *testing.T, output string) []string {
	t.Helper()
	var commands []string
	seen := make(map[string]struct{})
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !helpCommandNamePattern.MatchString(fields[0]) {
			continue
		}
		if _, ok := seen[fields[0]]; ok {
			t.Fatalf("built command inventory repeats %q", fields[0])
		}
		seen[fields[0]] = struct{}{}
		commands = append(commands, fields[0])
	}
	if len(commands) == 0 {
		t.Fatalf("built command inventory was empty:\n%s", output)
	}
	sort.Strings(commands)
	return commands
}

func parseCompatibilityCommandCatalog(t *testing.T, text string) []string {
	t.Helper()
	section := markdownSection(t, "docs/COMPATIBILITY.md", text, "Initial Matrix")
	var notes string
	for _, line := range strings.Split(section, "\n") {
		cells := markdownTableCells(line)
		if len(cells) < 3 || !strings.EqualFold(cells[0], "CLI surface") {
			continue
		}
		if !strings.EqualFold(cells[1], "supported") {
			t.Fatalf("docs/COMPATIBILITY.md CLI surface status = %q, want supported", cells[1])
		}
		notes = strings.Join(cells[2:], " | ")
		break
	}
	if notes == "" {
		t.Fatal("docs/COMPATIBILITY.md has no supported CLI surface row")
	}
	matches := regexp.MustCompile("`([a-z][a-z0-9-]*)`").FindAllStringSubmatch(notes, -1)
	if len(matches) == 0 {
		t.Fatal("docs/COMPATIBILITY.md CLI surface row names no commands")
	}
	commands := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if _, ok := seen[match[1]]; ok {
			t.Fatalf("docs/COMPATIBILITY.md CLI surface repeats %q", match[1])
		}
		seen[match[1]] = struct{}{}
		commands = append(commands, match[1])
	}
	sort.Strings(commands)
	return commands
}

func siteCLIReferenceHeadingCommands(t *testing.T, text string) []string {
	t.Helper()
	pattern := regexp.MustCompile("`glade ([a-z][a-z0-9-]*(?: [a-z][a-z0-9-]*)*)`")
	seen := make(map[string]struct{})
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || !markdownHeadingLinePattern.MatchString(line) {
			continue
		}
		for _, match := range pattern.FindAllStringSubmatch(line, -1) {
			if _, duplicate := seen[match[1]]; duplicate {
				t.Fatalf("site CLI reference repeats command heading %q", match[1])
			}
			seen[match[1]] = struct{}{}
		}
	}
	if len(seen) == 0 {
		t.Fatal("site CLI reference names no command headings")
	}
	commands := make([]string, 0, len(seen))
	for command := range seen {
		commands = append(commands, command)
	}
	sort.Strings(commands)
	return commands
}

func parseBuiltHelpCommandPaths(t *testing.T, output string) []string {
	t.Helper()
	paths := make(map[string]struct{})
	section := ""
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(line, " ") {
			section = strings.TrimSuffix(trimmed, ":")
			continue
		}
		switch section {
		case "Usage":
			fields := strings.Fields(line)
			if len(fields) < 2 || fields[0] != "glade" {
				continue
			}
			var command []string
			for _, field := range fields[1:] {
				if !helpCommandNamePattern.MatchString(field) {
					break
				}
				command = append(command, field)
			}
			if len(command) > 0 {
				paths[strings.Join(command, " ")] = struct{}{}
			}
		case "Subcommands":
			match := builtHelpSubcommandPattern.FindStringSubmatch(line)
			if len(match) != 2 {
				continue
			}
			root := builtHelpRootCommand(output)
			if root != "" {
				paths[root+" "+match[1]] = struct{}{}
			}
		}
	}
	if len(paths) == 0 {
		t.Fatalf("built command help exposed no structured command paths:\n%s", output)
	}
	out := make([]string, 0, len(paths))
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func builtHelpRootCommand(output string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "glade" && helpCommandNamePattern.MatchString(fields[1]) {
			return fields[1]
		}
	}
	return ""
}

func parseClaimedCacheVersion(t *testing.T, rel, text string) int {
	t.Helper()
	headings := map[string]string{
		"docs/TEST_STARTUP_CACHE.md":                "Freshness: how the cache stays up to date",
		"site/docs-src/guide/test-startup-cache.md": "When it is reused",
	}
	heading, ok := headings[rel]
	if !ok {
		t.Fatalf("no startup-cache documentation section configured for %s", rel)
	}
	section := markdownSection(t, rel, text, heading)
	matches := regexp.MustCompile(`(?i)cache(?:\s+format)?\s+version[^\n0-9]{0,80}([0-9]+)`).FindAllStringSubmatch(section, -1)
	versions := make(map[int]struct{})
	for _, match := range matches {
		version, err := strconv.Atoi(match[1])
		if err != nil {
			t.Fatalf("parse %s cache version: %v", rel, err)
		}
		versions[version] = struct{}{}
	}
	if len(versions) != 1 {
		t.Fatalf("%s startup-cache section names %d distinct current cache versions, want one", rel, len(versions))
	}
	for version := range versions {
		return version
	}
	return 0
}

func affectedSelectionSection(t *testing.T, rel, text string) string {
	t.Helper()
	headings := map[string]string{
		"docs/LOCAL_TESTING.md":                 "Run Tests Affected By Local Changes",
		"site/docs-src/guide/affected-tests.md": "Run Only Affected Tests",
	}
	heading, ok := headings[rel]
	if !ok {
		t.Fatalf("no affected-test documentation section configured for %s", rel)
	}
	return markdownSection(t, rel, text, heading)
}

func describesTransitiveSelection(section string) bool {
	changed := regexp.MustCompile(`\bchang(?:e|ed|es|ing)\b`)
	subject := regexp.MustCompile(`\b(?:types?|class(?:es)?|symbols?|files?)\b`)
	tests := regexp.MustCompile(`\btests?\b`)
	link := regexp.MustCompile(`\b(?:select|selected|selects|find|finds|reach|reaches|depend|depends|dependent|dependents)\b`)
	for _, paragraph := range markdownParagraphs(section) {
		lower := strings.ToLower(paragraph)
		if !strings.Contains(lower, "transitive") || negatesTransitiveSelection(lower) {
			continue
		}
		if changed.MatchString(lower) && subject.MatchString(lower) && tests.MatchString(lower) && link.MatchString(lower) {
			return true
		}
	}
	return false
}

func negatesTransitiveSelection(text string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\b(?:does not|doesn't|cannot|can't|never)\b.{0,30}\b(?:select|include|follow|walk|reach)\w*\b.{0,50}\btransitive(?:ly)?\b`),
		regexp.MustCompile(`\b(?:does not|doesn't|cannot|can't|never)\b.{0,30}\btransitive(?:ly)?\b.{0,30}\b(?:select|include|follow|walk|reach|support)\w*\b`),
		regexp.MustCompile(`\b(?:not|no)\s+transitive(?:ly)?\b`),
		regexp.MustCompile(`\b(?:unsupported|excludes?|ignores?|without)\b.{0,80}\btransitive\b`),
		regexp.MustCompile(`\btransitive\b.{0,80}\b(?:is not|isn't|are not|aren't|unsupported|excluded|ignored)\b`),
		regexp.MustCompile(`\b(?:instead of|rather than)\b\s+(?:the\s+)?\btransitive\b`),
		regexp.MustCompile(`\b(?:stops?|stopping)\b\s+\bbefore\b\s+(?:the\s+)?\btransitive\b`),
	}
	for _, pattern := range patterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func negatesDirectOnlySelection(text string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\bnot\b\s+\bselected\b\s+\b(?:only|solely|exclusively)\b\s+\bfrom\b\s+(?:the\s+)?\bdirect\b\s+\breferences?\b`),
		regexp.MustCompile(`\bnot\b\s+\blimited\b\s+\bto\b\s+(?:the\s+)?\bdirect\b\s+\breferences?\b`),
		regexp.MustCompile(`\bnot\b\s+\b(?:only|solely|exclusively)\b\s+(?:the\s+)?\bdirect\b\s+\breferences?\b`),
	}
	for _, pattern := range patterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func claimsDirectOnlySelection(section string) bool {
	direct := regexp.MustCompile(`\bdirect\b`)
	tests := regexp.MustCompile(`\btests?\b`)
	selection := regexp.MustCompile(`\bselect(?:ed|ion|s)?\b`)
	references := regexp.MustCompile(`\breferences?\b`)
	exclusive := regexp.MustCompile(`(?:\b(?:only|solely|exclusively)\b.{0,30}\bdirect\b|\bdirect\b.{0,30}\b(?:only|alone)\b)`)
	for _, paragraph := range markdownParagraphs(section) {
		lower := strings.ToLower(paragraph)
		negatesTransitive := negatesTransitiveSelection(lower)
		if negatesDirectOnlySelection(lower) && !negatesTransitive {
			continue
		}
		if !direct.MatchString(lower) || !tests.MatchString(lower) || !selection.MatchString(lower) || !references.MatchString(lower) {
			continue
		}
		if !strings.Contains(lower, "transitive") || negatesTransitive || exclusive.MatchString(lower) {
			return true
		}
	}
	return false
}

func markdownSection(t *testing.T, rel, text, wantHeading string) string {
	t.Helper()
	headingPattern := regexp.MustCompile(`^ {0,3}(#{1,6})\s+(.+?)\s*#*\s*$`)
	lines := strings.Split(text, "\n")
	type heading struct {
		line  int
		level int
		text  string
	}
	var headings []heading
	inFence := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		match := headingPattern.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		headings = append(headings, heading{
			line:  index,
			level: len(match[1]),
			text:  strings.TrimSpace(match[2]),
		})
	}
	var matches []heading
	for _, candidate := range headings {
		if strings.EqualFold(candidate.text, wantHeading) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		t.Fatalf("%s has no %q section", rel, wantHeading)
	}
	if len(matches) > 1 {
		t.Fatalf("%s repeats %q section %d times", rel, wantHeading, len(matches))
	}
	target := matches[0]
	start := target.line + 1
	end := len(lines)
	for _, candidate := range headings {
		if candidate.line > target.line && candidate.level <= target.level {
			end = candidate.line
			break
		}
	}
	section := strings.Join(lines[start:end], "\n")
	if strings.TrimSpace(section) == "" {
		t.Fatalf("%s has an empty %q section", rel, wantHeading)
	}
	return section
}

func markdownTableCells(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil
	}
	raw := strings.Split(strings.Trim(line, "|"), "|")
	cells := make([]string, len(raw))
	for index, cell := range raw {
		cells[index] = strings.TrimSpace(cell)
	}
	return cells
}

func markdownParagraphs(text string) []string {
	var paragraphs []string
	for _, block := range regexp.MustCompile(`\n\s*\n`).Split(text, -1) {
		block = strings.Join(strings.Fields(block), " ")
		if block != "" {
			paragraphs = append(paragraphs, block)
		}
	}
	return paragraphs
}

func TestHelpContractLinePatternsRetainExistingMatching(t *testing.T) {
	for _, test := range []struct {
		name    string
		pattern *regexp.Regexp
		text    string
		want    bool
	}{
		{name: "command name", pattern: helpCommandNamePattern, text: "cache-format", want: true},
		{name: "command name rejects space", pattern: helpCommandNamePattern, text: "cache format", want: false},
		{name: "markdown heading", pattern: markdownHeadingLinePattern, text: "### `glade test`", want: true},
		{name: "markdown heading rejects prose", pattern: markdownHeadingLinePattern, text: "Run glade test", want: false},
		{name: "subcommand line", pattern: builtHelpSubcommandPattern, text: "  cache clear  remove cached files", want: true},
		{name: "subcommand line rejects one space", pattern: builtHelpSubcommandPattern, text: " cache clear  remove cached files", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.pattern.MatchString(test.text); got != test.want {
				t.Fatalf("%s.MatchString(%q) = %v, want %v", test.pattern, test.text, got, test.want)
			}
		})
	}
}

func helpContractClass(root, name string, isTest bool) typesys.TypeSymbol {
	typ := typesys.TypeSymbol{
		Kind:   apexast.DeclarationClass,
		Name:   name,
		File:   filepath.Join(root, name+".cls"),
		IsTest: isTest,
	}
	if isTest {
		typ.Members = []typesys.MemberSymbol{{
			Kind:   apexast.DeclarationMethod,
			Name:   "runs",
			IsTest: true,
			Range:  diagnostic.Range{Start: diagnostic.Position{Line: 1, Column: 1}},
		}}
	}
	return typ
}

func writeHelpContractFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func diffStringSets(got, want []string) string {
	gotSet := make(map[string]struct{}, len(got))
	wantSet := make(map[string]struct{}, len(want))
	for _, value := range got {
		gotSet[value] = struct{}{}
	}
	for _, value := range want {
		wantSet[value] = struct{}{}
	}
	var unexpected, missing []string
	for value := range gotSet {
		if _, ok := wantSet[value]; !ok {
			unexpected = append(unexpected, value)
		}
	}
	for value := range wantSet {
		if _, ok := gotSet[value]; !ok {
			missing = append(missing, value)
		}
	}
	sort.Strings(unexpected)
	sort.Strings(missing)
	var lines []string
	if len(unexpected) > 0 {
		lines = append(lines, "live only: "+strings.Join(unexpected, ", "))
	}
	if len(missing) > 0 {
		lines = append(lines, "catalog only: "+strings.Join(missing, ", "))
	}
	return strings.Join(lines, "\n")
}

func containsSortedString(values []string, want string) bool {
	index := sort.SearchStrings(values, want)
	return index < len(values) && values[index] == want
}
