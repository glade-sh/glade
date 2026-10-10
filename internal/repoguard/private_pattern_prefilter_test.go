package repoguard

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"regexp/syntax"
	"strings"
	"testing"
	"unicode"
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

func TestPrefilterEveryPrivatePatternHasRequiredLiterals(t *testing.T) {
	patterns := privateExamplePackagePatterns()
	if len(patterns) != 22 {
		t.Fatalf("pattern count = %d, want 22", len(patterns))
	}
	for _, pattern := range patterns {
		if len(pattern.requiredLiterals()) == 0 {
			t.Errorf("pattern %q has no required literals", pattern.label)
		}
		for _, literal := range pattern.requiredLiterals() {
			if literal.text == "" {
				t.Errorf("pattern %q has an empty required literal", pattern.label)
			}
		}
	}
}

func TestPrefilterRequiredLiteralDerivation(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []privatePackageLiteral
	}{
		{name: "literal", source: `marker`, want: []privatePackageLiteral{{text: "marker"}}},
		{name: "folded literal", source: `(?i)mArKeR`, want: []privatePackageLiteral{{text: "MARKER", fold: true}}},
		{name: "capture", source: `(marker)`, want: []privatePackageLiteral{{text: "marker"}}},
		{name: "selective concat", source: `ab.*marker`, want: []privatePackageLiteral{{text: "marker"}}},
		{name: "shortest alternative controls selection", source: `(ab|lengthy).*marker`, want: []privatePackageLiteral{{text: "marker"}}},
		{name: "alternate", source: `alpha|beta`, want: []privatePackageLiteral{{text: "alpha"}, {text: "beta"}}},
		{name: "mixed folding", source: `(?i:alpha)|beta`, want: []privatePackageLiteral{{text: "ALPHA", fold: true}, {text: "beta"}}},
		{name: "empty alternative", source: `alpha|`},
		{name: "character class", source: `[a-z]`},
		{name: "repeat", source: `(?:marker)+`},
		{name: "optional", source: `(?:marker)?`},
		{name: "anchors", source: `^$`},
		{name: "word boundary", source: `\b`},
		{name: "empty", source: ``},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := syntax.Parse(test.source, syntax.Perl)
			if err != nil {
				t.Fatal(err)
			}
			got := requiredPrivatePackageLiterals(parsed.Simplify())
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("required literals = %#v, want %#v", got, test.want)
			}
			if len(got) == 0 {
				for _, text := range []string{"", "a", "marker", "unrelated"} {
					if !privatePackageLiteralsMayMatch(got, text, foldPrivatePackageText(text)) {
						t.Errorf("pattern without required literals rejected %q", text)
					}
				}
			}
		})
	}
}

func TestPrefilterSimpleFoldMatchesSyntax(t *testing.T) {
	for _, text := range []string{"", "AbCdEfZz", "KkK Ssſ", "Σσς", "123_-\"'.:/() \n", "Åå ΩωΩ"} {
		want := strings.Map(func(r rune) rune {
			minimum := r
			for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
				if next < minimum {
					minimum = next
				}
			}
			return minimum
		}, text)
		if got := foldPrivatePackageText(text); got != want {
			t.Errorf("fold(%q) = %q, want %q", text, got, want)
		}
		parsed, err := syntax.Parse(`(?i)`+regexp.QuoteMeta(text), syntax.Perl)
		if err != nil {
			t.Fatal(err)
		}
		if parsed.Op == syntax.OpLiteral && parsed.Flags&syntax.FoldCase != 0 && string(parsed.Rune) != want {
			t.Errorf("syntax literal for %q = %q, want %q", text, string(parsed.Rune), want)
		}
	}
	exact := []privatePackageLiteral{{text: "AbCd"}}
	if !privatePackageLiteralsMayMatch(exact, "before AbCd after", "BEFORE ABCD AFTER") {
		t.Fatal("exact literal was absent from raw text")
	}
	if privatePackageLiteralsMayMatch(exact, "before abcd after", "BEFORE ABCD AFTER") {
		t.Fatal("exact literal used folded text")
	}
}

type prefilterPositiveSample struct {
	text     string
	foldable bool
}

func prefilterPositiveSamples(pattern privatePackagePattern) []prefilterPositiveSample {
	if prefix, ok := strings.CutSuffix(pattern.label, " namespace"); ok {
		return []prefilterPositiveSample{
			{text: prefix + "__Thing", foldable: true},
			{text: strings.ToLower(prefix) + "__Thing", foldable: true},
			{text: "z" + strings.ToLower(prefix) + "__Thing", foldable: true},
			{text: `"namespace": "` + prefix + `"`, foldable: true},
			{text: `Namespace: "` + prefix + `"`, foldable: true},
			{text: `Namespace = "` + prefix + `"`, foldable: true},
			{text: `SetCurrentNamespace("` + prefix + `")`, foldable: true},
			{text: `Type.forName('` + prefix + `')`, foldable: true},
			{text: prefix + ".Thing"},
		}
	}
	samples := []prefilterPositiveSample{{text: pattern.label, foldable: true}}
	if pattern.label == privateExamplePackagePatternSet[9].label {
		samples = append(samples,
			prefilterPositiveSample{text: "Object." + pattern.label + "__Thing", foldable: true},
			prefilterPositiveSample{text: pattern.label + ".Thing", foldable: true})
	}
	return samples
}

func prefilterMixedCase(text string) string {
	runes := []rune(text)
	for index, r := range runes {
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
	}{{ascii: "kK", fold: "K"}, {ascii: "sS", fold: "ſ"}} {
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

func TestPrefilterPrivatePatternPositiveVariants(t *testing.T) {
	for _, pattern := range privateExamplePackagePatterns() {
		t.Run(pattern.label, func(t *testing.T) {
			for _, sample := range prefilterPositiveSamples(pattern) {
				for _, text := range prefilterPositiveVariants(sample) {
					if !pattern.re.MatchString(text) {
						t.Fatalf("positive variant %q does not match regexp", text)
					}
					if !pattern.mayMatch(text, foldPrivatePackageText(text)) {
						t.Errorf("prefilter rejected positive variant %q", text)
					}
				}
			}
		})
	}
}

func TestPrefilterPrivatePatternRandomAbsenceIsConservative(t *testing.T) {
	patterns := privateExamplePackagePatterns()
	fragments := []string{"_", "-", `"`, "'", ".", ":", "/", "(", ")", " ", "\n", "K", "ſ", "\xff", "\xfe", "\x00"}
	for _, r := range "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" {
		fragments = append(fragments, string(r))
	}
	for _, pattern := range patterns {
		for _, literal := range pattern.requiredLiterals() {
			fragments = append(fragments, literal.text, prefilterMixedCase(literal.text))
			runes := []rune(literal.text)
			if len(runes) > 1 {
				fragments = append(fragments, string(runes[:len(runes)/2]), string(runes[len(runes)/2:]))
			}
		}
	}
	random := rand.New(rand.NewSource(20261009))
	for iteration := 0; iteration < 6000; iteration++ {
		var text strings.Builder
		for remaining := random.Intn(12); remaining > 0; remaining-- {
			text.WriteString(fragments[random.Intn(len(fragments))])
		}
		raw := text.String()
		folded := foldPrivatePackageText(raw)
		for _, pattern := range patterns {
			if !pattern.mayMatch(raw, folded) && pattern.re.MatchString(raw) {
				t.Fatalf("iteration %d: prefilter rejected regexp match %q for %q", iteration, raw, pattern.label)
			}
		}
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

func prefilterUnfilteredFindings(recorder *prefilterFindingRecorder, rel, text string) string {
	recorder.Helper()
	first := ""
	for _, pattern := range privateExamplePackagePatternSet {
		if pattern.re.MatchString(text) {
			recorder.Errorf("%s contains private example package marker %q", rel, pattern.label)
			if first == "" {
				first = pattern.label
			}
		}
	}
	return first
}

func TestRepoGuardPrefilterFindingsEquivalent(t *testing.T) {
	texts := []string{"", "macrodata-apex refinement-local", "partial marker", "K ſ", "\x00", "\xff"}
	var allSamples []string
	for _, pattern := range privateExamplePackagePatterns() {
		for _, sample := range prefilterPositiveSamples(pattern) {
			texts = append(texts, prefilterPositiveVariants(sample)...)
			allSamples = append(allSamples, sample.text)
			texts = append(texts, "\xff"+sample.text+"\xfe", "\x00"+sample.text)
		}
		for _, literal := range pattern.requiredLiterals() {
			runes := []rune(literal.text)
			middle := len(runes) / 2
			left, right := string(runes[:middle]), string(runes[middle:])
			texts = append(texts, left, right,
				strings.ToLower(left)+strings.ToUpper(right),
				strings.ToUpper(left)+strings.ToLower(right),
				left+" "+right, left+"\n"+right)
		}
		if prefix, ok := strings.CutSuffix(pattern.label, " namespace"); ok {
			texts = append(texts, strings.ToLower(prefix)+".Thing")
		}
	}
	texts = append(texts, strings.Join(allSamples, "\n"))
	for left, right := 0, len(allSamples)-1; left < right; left, right = left+1, right-1 {
		allSamples[left], allSamples[right] = allSamples[right], allSamples[left]
	}
	texts = append(texts, strings.Join(allSamples, "\n"))

	var actual, expected prefilterFindingRecorder
	for index, text := range texts {
		rel := fmt.Sprintf("docs/prefilter-%d.md", index)
		checkPrivateExamplePackageText(&actual, rel, text)
		wantFirst := prefilterUnfilteredFindings(&expected, rel, text)
		if got := privateExamplePackageFinding(text); got != wantFirst {
			t.Errorf("text %d: first finding = %q, want %q", index, got, wantFirst)
		}
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("ordered findings differ:\nfiltered: %#v\nunfiltered: %#v", actual, expected)
	}
}
