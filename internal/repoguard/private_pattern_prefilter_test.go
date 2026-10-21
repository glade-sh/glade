package repoguard

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"regexp/syntax"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestPrefilterSourceAvoidsPrivateMarkers(t *testing.T) {
	const filename = "private_pattern_prefilter_test.go"
	source, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, term := range forbiddenCorpusTerms() {
		if strings.Contains(strings.ToLower(text), term) {
			t.Errorf("%s contains forbidden corpus term %q", filename, term)
		}
	}
	checkPrivateExamplePackageText(t, filename, text)
}

func TestPrivateNameTableIsWellFormed(t *testing.T) {
	patterns := privateExamplePackagePatterns()
	if len(patterns) != 24 {
		t.Fatalf("pattern count = %d, want 24", len(patterns))
	}
	label := regexp.MustCompile(`^private-pattern-[0-9]{2}$`)
	labels := map[string]bool{}
	digests := map[[sha256.Size]byte]bool{}
	for _, pattern := range patterns {
		if !label.MatchString(pattern.label) || labels[pattern.label] {
			t.Errorf("label %q is not a unique private-pattern-NN label", pattern.label)
		}
		labels[pattern.label] = true
		if digests[pattern.digest] {
			t.Errorf("pattern %q repeats a digest", pattern.label)
		}
		digests[pattern.digest] = true
		if pattern.length <= 0 || pattern.kind > privateNamePhrase {
			t.Errorf("pattern %q has length %d and kind %d", pattern.label, pattern.length, pattern.kind)
		}
	}
}

// TestPrivateNameFoldingMatchesRegexp checks the one-byte rune form against
// the case folding and word characters of package regexp, for every rune.
func TestPrivateNameFoldingMatchesRegexp(t *testing.T) {
	for r := rune(0); r <= unicode.MaxRune; r++ {
		var letter byte // the ASCII letter (?i) matches r with, if any
		for next := unicode.SimpleFold(r); ; next = unicode.SimpleFold(next) {
			if 'a' <= next && next <= 'z' {
				letter = byte(next)
			}
			if next == r {
				break
			}
		}
		scanned := privateRuneByte(r)
		if got := privateRuneFold[scanned]; got != letter && (letter != 0 || privateLetter(got)) {
			t.Errorf("rune %U folds to %q, want %q", r, got, letter)
		}
		if privateRuneWord[scanned] != syntax.IsWordChar(r) {
			t.Errorf("rune %U word = %v, want %v", r, privateRuneWord[scanned], syntax.IsWordChar(r))
		}
	}
	for _, test := range []struct {
		pattern string
		text    string
		want    bool
	}{
		{pattern: `(?i)k`, text: "\u212a", want: true},
		{pattern: `(?i)s`, text: "\u017f", want: true},
		{pattern: `(?i)[A-Z_]`, text: "\u212a", want: true},
		{pattern: `(?i)[[:alnum:]_]`, text: "\u017f", want: true},
		{pattern: `(?i)\bk`, text: "\u212a", want: false},
		{pattern: `(?i)a\b`, text: "a\u212a", want: true},
		{pattern: `(?i)i`, text: "\u0130\u0131", want: false},
	} {
		if got := regexp.MustCompile(test.pattern).MatchString(test.text); got != test.want {
			t.Errorf("%s on %q = %v, want %v", test.pattern, test.text, got, test.want)
		}
	}
}

// privateStandInNames are public stand-ins for the guarded names, at least one
// per kind, hashed with the same function. Each spells k or s, which (?i) also
// matches with U+212A and U+017F, and some share prefixes to cover the
// shortest-first order.
var privateStandInNames = []struct {
	kind privateNameKind
	name string
}{
	{kind: privateNameHyphenated, name: "desk-kit"},
	{kind: privateNameHyphenated, name: "sky-desk-kit-set"},
	{kind: privateNameWord, name: "kestrel"},
	{kind: privateNameWordPrefix, name: "skiff"},
	{kind: privateNameWordPrefix, name: "skiffkeeper"},
	{kind: privateNamePackage, name: "kits"},
	{kind: privateNameProduct, name: "kitesurfer"},
	{kind: privateNameNamespace, name: "KS"},
	{kind: privateNamePhrase, name: "ks desk pattern"},
}

// newPrivateNamePattern hashes a stand-in name the way the table was built.
func newPrivateNamePattern(label string, kind privateNameKind, name string) privatePackagePattern {
	folded := []byte(strings.ToLower(name))
	return privatePackagePattern{
		label:       label,
		kind:        kind,
		length:      len(folded),
		fingerprint: privateNameFingerprint(folded),
		digest:      privateNameSum(folded),
	}
}

func privateStandInPatterns() []privatePackagePattern {
	patterns := make([]privatePackagePattern, len(privateStandInNames))
	for index, standIn := range privateStandInNames {
		patterns[index] = newPrivateNamePattern(fmt.Sprintf("stand-in-%02d", index+1), standIn.kind, standIn.name)
	}
	return patterns
}

// privateNameRegexp is the regular expression a kind stands for, the form the
// guard used before its names were hashed. It is the oracle for these tests.
func privateNameRegexp(kind privateNameKind, name string) *regexp.Regexp {
	upper, lower := strings.ToUpper(name), strings.ToLower(name)
	switch kind {
	case privateNameHyphenated:
		return regexp.MustCompile(`(?i)` + name)
	case privateNameWord, privateNamePhrase:
		return regexp.MustCompile(`(?i)\b` + name + `\b`)
	case privateNameWordPrefix:
		return regexp.MustCompile(`(?i)\b` + name + `[[:alnum:]_]*\b`)
	case privateNamePackage:
		return regexp.MustCompile(`(?i)\b` + name + `(?:__|\b|\.)`)
	case privateNameProduct:
		return regexp.MustCompile(`(?i)\b` + name + `\b|` + name + `__|` + name + `[:/-]|\b` + name + `[A-Z_]`)
	case privateNameNamespace:
		return regexp.MustCompile(`(?i:\b` + upper + `__|\b` + lower + `__|\bz` + lower + `__|namespace"\s*:\s*"` + upper +
			`"|Namespace:\s*"` + upper + `"|Namespace\s*=\s*"` + upper + `"|SetCurrentNamespace\("` + upper +
			`"\)|Type\.forName\('` + upper + `')|\b` + upper + `\.[A-Za-z]`)
	}
	panic(fmt.Sprintf("unknown private name kind %d", kind))
}

func privateStandInRegexps() []*regexp.Regexp {
	expressions := make([]*regexp.Regexp, len(privateStandInNames))
	for index, standIn := range privateStandInNames {
		expressions[index] = privateNameRegexp(standIn.kind, standIn.name)
	}
	return expressions
}

// privateStandInFixtures are matching and non-matching texts for each kind.
var privateStandInFixtures = []struct {
	name string
	text string
	want bool
}{
	{name: "ks desk pattern", text: "// class (the ks desk pattern, where names overlap)", want: true},
	{name: "ks desk pattern", text: "KS DESK PATTERN", want: true},
	{name: "ks desk pattern", text: "k\u017f de\u017fk pattern", want: true},
	{name: "ks desk pattern", text: "xks desk pattern", want: false},
	{name: "ks desk pattern", text: "ks desk patterns", want: false},
	{name: "ks desk pattern", text: "_ks desk pattern", want: false},
	{name: "ks desk pattern", text: "ks desk pattern_", want: false},
	{name: "ks desk pattern", text: "ks  desk pattern", want: false},
	{name: "ks desk pattern", text: "ks\ndesk pattern", want: false},
	{name: "desk-kit", text: "desk-kit", want: true},
	{name: "desk-kit", text: "my-desk-kit-fork", want: true},
	{name: "desk-kit", text: "xdesk-kitx", want: true},
	{name: "desk-kit", text: "DESK-\u212aIT", want: true},
	{name: "desk-kit", text: "desk_kit", want: false},
	{name: "desk-kit", text: "desk-ki", want: false},
	{name: "desk-kit", text: "desk--kit", want: false},
	{name: "sky-desk-kit-set", text: "path/sky-desk-kit-set.md", want: true},
	{name: "sky-desk-kit-set", text: "sky-desk-kit-se", want: false},
	{name: "kestrel", text: "kestrel", want: true},
	{name: "kestrel", text: "(Kestrel)", want: true},
	{name: "kestrel", text: "kestrel.go", want: true},
	{name: "kestrel", text: "a\u212aestrel", want: true},
	{name: "kestrel", text: "kestrels", want: false},
	{name: "kestrel", text: "akestrel", want: false},
	{name: "kestrel", text: "kestrel_x", want: false},
	{name: "kestrel", text: "\u212aestrel", want: false},
	{name: "skiff", text: "skiff", want: true},
	{name: "skiff", text: "SkiffTools_v2", want: true},
	{name: "skiff", text: "-skiff9", want: true},
	{name: "skiff", text: "a\u017fkiff", want: true},
	{name: "skiff", text: "xskiff", want: false},
	{name: "skiff", text: "9skiff", want: false},
	{name: "skiff", text: "\u017fkiff", want: false},
	{name: "skiff", text: "skif", want: false},
	{name: "skiffkeeper", text: "SKIFFKEEPERS", want: true},
	{name: "skiffkeeper", text: "skiff keeper", want: false},
	{name: "kits", text: "kits", want: true},
	{name: "kits", text: "kits__c", want: true},
	{name: "kits", text: "Object.kits__Thing", want: true},
	{name: "kits", text: "kits.Thing", want: true},
	{name: "kits", text: "kits9", want: false},
	{name: "kits", text: "kits_x", want: false},
	{name: "kits", text: "xkits__c", want: false},
	{name: "kitesurfer", text: "kitesurfer", want: true},
	{name: "kitesurfer", text: "KitesurferPro", want: true},
	{name: "kitesurfer", text: "kitesurfer_", want: true},
	{name: "kitesurfer", text: "xkitesurfer__", want: true},
	{name: "kitesurfer", text: "xkitesurfer:", want: true},
	{name: "kitesurfer", text: "xkitesurfer/path", want: true},
	{name: "kitesurfer", text: "xkitesurfer-", want: true},
	{name: "kitesurfer", text: "xkitesurfer", want: false},
	{name: "kitesurfer", text: "kitesurfer9", want: false},
	{name: "kitesurfer", text: "xkitesurfer_", want: false},
	{name: "KS", text: "KS__Thing", want: true},
	{name: "KS", text: "ks__Thing", want: true},
	{name: "KS", text: "zks__Thing", want: true},
	{name: "KS", text: `"namespace": "KS"`, want: true},
	{name: "KS", text: `Namespace: "ks"`, want: true},
	{name: "KS", text: `Namespace = "KS"`, want: true},
	{name: "KS", text: `SetCurrentNamespace("KS")`, want: true},
	{name: "KS", text: `Type.forName('KS')`, want: true},
	{name: "KS", text: "type.forname('\u212a\u017f')", want: true},
	{name: "KS", text: "KS.Thing", want: true},
	{name: "KS", text: "KS_Thing", want: false},
	{name: "KS", text: "aKS__", want: false},
	{name: "KS", text: "ks.Thing", want: false},
	{name: "KS", text: "KS.9", want: false},
	{name: "KS", text: "\u212aS.Thing", want: false},
	{name: "KS", text: `Namespace: "KSX"`, want: false},
	{name: "KS", text: `Namespace: KS`, want: false},
	{name: "KS", text: `SetCurrentNamespace("KS"`, want: false},
	{name: "KS", text: `Type.forName("KS")`, want: false},
}

// TestPrivateNameKindsMatchFixtures shows each kind matching and refusing
// fixture texts exactly as its regular expression does.
func TestPrivateNameKindsMatchFixtures(t *testing.T) {
	patterns := privateStandInPatterns()
	expressions := privateStandInRegexps()
	covered := map[string]bool{}
	for _, fixture := range privateStandInFixtures {
		index := privateStandInIndex(fixture.name)
		if index < 0 {
			t.Fatalf("fixture names unknown stand-in %q", fixture.name)
		}
		covered[fixture.name] = true
		if got := expressions[index].MatchString(fixture.text); got != fixture.want {
			t.Fatalf("regexp for %q on %q = %v, want %v", fixture.name, fixture.text, got, fixture.want)
		}
		for _, prefilter := range []bool{true, false} {
			if got := matchPrivateNames(patterns, fixture.text, prefilter)[index]; got != fixture.want {
				t.Errorf("hashed %q on %q (prefilter %v) = %v, want %v", fixture.name, fixture.text, prefilter, got, fixture.want)
			}
		}
		if got := patterns[index].matches(fixture.text); got != fixture.want {
			t.Errorf("pattern %q matches %q = %v, want %v", fixture.name, fixture.text, got, fixture.want)
		}
	}
	if len(covered) != len(privateStandInNames) {
		t.Fatalf("fixtures cover %d of %d stand-ins", len(covered), len(privateStandInNames))
	}
}

func privateStandInIndex(name string) int {
	for index, standIn := range privateStandInNames {
		if standIn.name == name {
			return index
		}
	}
	return -1
}

type prefilterPositiveSample struct {
	text     string
	foldable bool
}

func prefilterPositiveSamples(kind privateNameKind, name string) []prefilterPositiveSample {
	switch kind {
	case privateNameNamespace:
		return []prefilterPositiveSample{
			{text: name + "__Thing", foldable: true},
			{text: strings.ToLower(name) + "__Thing", foldable: true},
			{text: "z" + strings.ToLower(name) + "__Thing", foldable: true},
			{text: `"namespace": "` + name + `"`, foldable: true},
			{text: `Namespace: "` + name + `"`, foldable: true},
			{text: `Namespace = "` + name + `"`, foldable: true},
			{text: `SetCurrentNamespace("` + name + `")`, foldable: true},
			{text: `Type.forName('` + name + `')`, foldable: true},
			{text: name + ".Thing"},
		}
	case privateNamePackage:
		return []prefilterPositiveSample{
			{text: name, foldable: true},
			{text: "Object." + name + "__Thing", foldable: true},
			{text: name + ".Thing", foldable: true},
		}
	case privateNameProduct:
		return []prefilterPositiveSample{
			{text: name, foldable: true},
			{text: "x" + name + "__", foldable: true},
			{text: "x" + name + ":/", foldable: true},
			{text: name + "Pro", foldable: true},
		}
	case privateNameWordPrefix:
		return []prefilterPositiveSample{
			{text: name, foldable: true},
			{text: name + "Tools", foldable: true},
		}
	}
	return []prefilterPositiveSample{{text: name, foldable: true}}
}

// prefilterMixedCase alternates the case of ASCII letters. Other runes stay,
// so a folded rune keeps its place outside the ASCII word characters.
func prefilterMixedCase(text string) string {
	runes := []rune(text)
	for index, r := range runes {
		if r >= utf8.RuneSelf {
			continue
		}
		if index%2 == 0 {
			runes[index] = unicode.ToUpper(r)
		} else {
			runes[index] = unicode.ToLower(r)
		}
	}
	return string(runes)
}

func prefilterPositiveVariants(sample prefilterPositiveSample) []string {
	variants := []string{sample.text}
	if !sample.foldable {
		return variants
	}
	variants = append(variants, prefilterMixedCase(sample.text))
	for _, replacement := range []struct {
		ascii string
		fold  string
	}{{ascii: "kK", fold: "\u212a"}, {ascii: "sS", fold: "\u017f"}} {
		variant := strings.NewReplacer(replacement.ascii[:1], replacement.fold, replacement.ascii[1:], replacement.fold).Replace(sample.text)
		if variant == sample.text {
			continue
		}
		// Word boundaries use ASCII word characters, including around folded runes.
		if strings.HasPrefix(variant, replacement.fold) {
			variant = "A" + variant
		}
		if strings.HasSuffix(variant, replacement.fold) {
			variant += "A"
		}
		variants = append(variants, variant, prefilterMixedCase(variant))
	}
	return variants
}

// TestPrefilterPrivatePatternPositiveVariants checks every case and fold
// variant of each stand-in sample against the regular expression. A fold rune
// is not an ASCII word character, so a variant can move a word boundary; the
// hashed pattern must then refuse it just as the expression does.
func TestPrefilterPrivatePatternPositiveVariants(t *testing.T) {
	patterns := privateStandInPatterns()
	expressions := privateStandInRegexps()
	for index, standIn := range privateStandInNames {
		t.Run(patterns[index].label, func(t *testing.T) {
			foldedMatches := 0
			for _, sample := range prefilterPositiveSamples(standIn.kind, standIn.name) {
				if !expressions[index].MatchString(sample.text) {
					t.Fatalf("sample %q does not match regexp", sample.text)
				}
				for _, text := range prefilterPositiveVariants(sample) {
					want := expressions[index].MatchString(text)
					if got := matchPrivateNames(patterns, text, true)[index]; got != want {
						t.Errorf("hashed pattern on variant %q = %v, regexp %v", text, got, want)
					}
					if want && strings.ContainsAny(text, "\u212a\u017f") {
						foldedMatches++
					}
				}
			}
			if foldedMatches == 0 {
				t.Error("no variant with U+212A or U+017F matched")
			}
		})
	}
}

// privateNameRandomTexts joins random fragments: structure, single runes, and
// the stand-in names whole, in halves, and in mixed case.
func privateNameRandomTexts(seed int64, count int) []string {
	fragments := []string{"_", "__", "-", `"`, "'", ".", ":", "/", "(", ")", " ", "\n", "\u212a", "\u017f", "\xff", "\xfe", "\x00", "é",
		"z", "Object.", ".Thing", `"namespace": "`, `Namespace: "`, `Namespace = "`, `SetCurrentNamespace("`, `Type.forName('`, `")`}
	for _, r := range "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" {
		fragments = append(fragments, string(r))
	}
	for _, standIn := range privateStandInNames {
		runes := []rune(standIn.name)
		fragments = append(fragments, standIn.name, strings.ToLower(standIn.name), prefilterMixedCase(standIn.name),
			string(runes[:len(runes)/2]), string(runes[len(runes)/2:]))
	}
	random := rand.New(rand.NewSource(seed))
	texts := make([]string, count)
	for index := range texts {
		var text strings.Builder
		for remaining := random.Intn(12); remaining > 0; remaining-- {
			text.WriteString(fragments[random.Intn(len(fragments))])
		}
		texts[index] = text.String()
	}
	return texts
}

// TestPrivateNameMatchersEqualRegexpsOnRandomText compares the hashed
// matchers, with and without the fingerprint prefilter, with the regular
// expressions on random texts, both ways.
func TestPrivateNameMatchersEqualRegexpsOnRandomText(t *testing.T) {
	patterns := privateStandInPatterns()
	expressions := privateStandInRegexps()
	matches := 0
	for iteration, text := range privateNameRandomTexts(20261009, 20000) {
		fast := matchPrivateNames(patterns, text, true)
		slow := matchPrivateNames(patterns, text, false)
		for index, expression := range expressions {
			want := expression.MatchString(text)
			if fast[index] != want || slow[index] != want {
				t.Fatalf("iteration %d: %q on %q: prefiltered %v, unfiltered %v, regexp %v",
					iteration, privateStandInNames[index].name, text, fast[index], slow[index], want)
			}
			if want {
				matches++
			}
		}
		if !reflect.DeepEqual(matchPrivateNames(privateExamplePackagePatternSet, text, true), matchPrivateNames(privateExamplePackagePatternSet, text, false)) {
			t.Fatalf("iteration %d: guarded names differ with and without the prefilter", iteration)
		}
	}
	if matches == 0 {
		t.Fatal("random texts never matched a stand-in name")
	}
}

type prefilterRecordedFinding struct {
	file    string
	label   string
	format  string
	message string
}

type prefilterFindingRecorder struct {
	findings []prefilterRecordedFinding
	helpers  int
}

func (recorder *prefilterFindingRecorder) Helper() {
	recorder.helpers++
}

func (recorder *prefilterFindingRecorder) Errorf(format string, args ...any) {
	recorder.findings = append(recorder.findings, prefilterRecordedFinding{
		file:    args[0].(string),
		label:   args[1].(string),
		format:  format,
		message: fmt.Sprintf(format, args...),
	})
}

func prefilterRegexpFindings(recorder *prefilterFindingRecorder, patterns []privatePackagePattern, expressions []*regexp.Regexp, rel, text string) string {
	recorder.Helper()
	first := ""
	for index, expression := range expressions {
		if expression.MatchString(text) {
			recorder.Errorf("%s contains private example package marker %q", rel, patterns[index].label)
			if first == "" {
				first = patterns[index].label
			}
		}
	}
	return first
}

func TestRepoGuardPrefilterFindingsEquivalent(t *testing.T) {
	patterns := privateStandInPatterns()
	expressions := privateStandInRegexps()
	texts := []string{"", "macrodata-apex refinement-local", "partial marker", "\u212a \u017f", "\x00", "\xff"}
	var allSamples []string
	for _, standIn := range privateStandInNames {
		for _, sample := range prefilterPositiveSamples(standIn.kind, standIn.name) {
			texts = append(texts, prefilterPositiveVariants(sample)...)
			allSamples = append(allSamples, sample.text)
			texts = append(texts, "\xff"+sample.text+"\xfe", "\x00"+sample.text)
		}
		runes := []rune(standIn.name)
		middle := len(runes) / 2
		left, right := string(runes[:middle]), string(runes[middle:])
		texts = append(texts, left, right,
			strings.ToLower(left)+strings.ToUpper(right),
			strings.ToUpper(left)+strings.ToLower(right),
			left+" "+right, left+"\n"+right)
	}
	texts = append(texts, strings.Join(allSamples, "\n"))
	for left, right := 0, len(allSamples)-1; left < right; left, right = left+1, right-1 {
		allSamples[left], allSamples[right] = allSamples[right], allSamples[left]
	}
	texts = append(texts, strings.Join(allSamples, "\n"))
	texts = append(texts, privateNameRandomTexts(7, 2000)...)

	var actual, expected prefilterFindingRecorder
	for index, text := range texts {
		rel := fmt.Sprintf("docs/prefilter-%d.md", index)
		checkPrivateNameText(&actual, patterns, rel, text)
		wantFirst := prefilterRegexpFindings(&expected, patterns, expressions, rel, text)
		if got := privateNameFinding(patterns, text); got != wantFirst {
			t.Errorf("text %d: first finding = %q, want %q", index, got, wantFirst)
		}
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("ordered findings differ:\nhashed: %#v\nregexp: %#v", actual, expected)
	}
}

// SHA-256 cannot reconstruct the removed wording. Use a synthetic phrase with
// the same kind and length to exercise the reporter without storing that text.
func TestPrivateNamePhraseReportsSyntheticComment(t *testing.T) {
	patterns := privateExamplePackagePatterns()
	pattern := patterns[22]
	if pattern.kind != privateNamePhrase || pattern.length != 15 || pattern.fingerprint != 0x88a4 ||
		fmt.Sprintf("%x", pattern.digest) != "dd538701916e4588093d6b1ac427561ad5250bc03ef812cb0940f99c66526414" {
		t.Fatal("removed comment phrase is missing from the hashed table")
	}
	standIn := newPrivateNamePattern(pattern.label, pattern.kind, "ks desk pattern")
	if standIn.length != pattern.length {
		t.Fatal("stand-in phrase length differs from the guarded phrase")
	}
	patterns[22] = standIn
	const rel = "internal/vm/synthetic_dispatch.go"
	const text = "// A local class uses the ks desk pattern when names overlap."
	var recorder prefilterFindingRecorder
	checkPrivateNameText(&recorder, patterns, rel, text)
	if len(recorder.findings) != 1 || recorder.findings[0].file != rel || recorder.findings[0].label != pattern.label {
		t.Fatalf("synthetic comment findings = %#v, want one phrase finding", recorder.findings)
	}
	if strings.Contains(recorder.findings[0].message, "ks desk pattern") {
		t.Fatal("finding message discloses the matched phrase")
	}
	var clean prefilterFindingRecorder
	checkPrivateNameText(&clean, patterns, rel, "// A local class takes precedence over a dependency class of the same name.")
	if len(clean.findings) != 0 {
		t.Fatalf("synthetic behavior comment findings = %#v, want none", clean.findings)
	}
}

func TestPrivateNamePhraseTableKeepsDiagnosticComment(t *testing.T) {
	pattern := privateExamplePackagePatterns()[23]
	if pattern.kind != privateNamePhrase || pattern.length != 30 || pattern.fingerprint != 0x1011 ||
		fmt.Sprintf("%x", pattern.digest) != "0063f9509f624b3f44c98edf0ec395238beaf73b2517d11b45486e151f46bcc7" {
		t.Fatal("removed diagnostic comment phrase is missing from the hashed table")
	}
}
