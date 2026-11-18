package dml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

// referenceCompanyWords and referenceCompanyKey are the allocation-heavy
// forms the fast paths replace.
func referenceCompanyWords(s string) []string {
	var words []string
	for _, token := range strings.Fields(strings.ReplaceAll(strings.ToLower(s), "-", " ")) {
		if token == "&" {
			words = append(words, "and")
			continue
		}
		var clean []rune
		for _, r := range token {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '&' || r == '/' {
				clean = append(clean, r)
			}
		}
		if len(clean) > 0 {
			words = append(words, string(clean))
		}
	}
	if len(words) > 1 && words[0] == "the" {
		words = words[1:]
	}
	if len(words) > 1 && companyDesignators[words[len(words)-1]] {
		words = words[:len(words)-1]
	}
	var out []string
	for _, w := range words {
		if !companyStopWords[w] {
			out = append(out, w)
		}
	}
	if len(out) == 0 {
		return words
	}
	return out
}

func referenceCompanyKey(words []string) string {
	var b strings.Builder
	for i, w := range words {
		if i == 2 {
			break
		}
		r := []rune(w)
		if len(r) > 5 {
			r = r[:5]
		}
		b.WriteString(string(r))
	}
	return b.String()
}

func companyWordValues(t *testing.T) []string {
	values := []string{"", " ", "&", "The", "the and of", "AT&T", "Q & V", "Partner", " PARTNER ", "partners\t", "Partner Co",
		"Élan Company", "Ünïcode Name Works", "Acme ١", "a\tb", "A  B", "a\u00a0b", "x-y_z", "Barnes&Noble/West", "Mc-Donald's"}
	raw, err := os.ReadFile(filepath.Join(fuzzyFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		var row fuzzyNativeRow
		if json.Unmarshal([]byte(strings.SplitN(line, "\t", 4)[3]), &row) != nil || !strings.Contains(row.Method, "CompanyName") {
			continue
		}
		for _, v := range row.Values {
			if v != nil {
				values = append(values, strings.Split(*v, "|")[0])
			}
		}
	}
	raw, err = os.ReadFile("testdata/company_name_fit/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(string(raw), -1) {
		values = append(values, m[1])
	}
	return values
}

func TestCompanyWordFastPathsMatchReference(t *testing.T) {
	values := companyWordValues(t)
	for _, s := range values {
		got, want := companyWords(s), referenceCompanyWords(s)
		if strings.Join(got, "|") != strings.Join(want, "|") || len(got) != len(want) {
			t.Errorf("%q: words %q, want %q", s, got, want)
		}
		if companyKey(want) != referenceCompanyKey(want) {
			t.Errorf("%q: key %q, want %q", s, companyKey(want), referenceCompanyKey(want))
		}
		if got, want := companyAlignedName(s), companyAlignedWords[strings.TrimSpace(foldText(s))]; got != want {
			t.Errorf("%q: aligned %v, want %v", s, got, want)
		}
	}
	for _, a := range values {
		for _, b := range values[:60] {
			if got, want := foldedEqual(a, b), foldText(a) == foldText(b); got != want {
				t.Errorf("%q / %q: folded equal %v, want %v", a, b, got, want)
			}
			if got, want := foldedEqual(a, a+" "), true; got != want {
				t.Errorf("%q: folded equal with trailing space %v", a, got)
			}
		}
	}
}

// referenceScoreWords is companyScoreWords' general (rune) punctuation pass.
func referenceScoreWords(s string, key bool) []string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == '&':
			b.WriteString(" and ")
		case r == '.':
		case !key && (r == '-' || r == '_'):
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	return companyScoreFields(b.String(), key)
}

func TestCompanyScoreFastPathsMatchReference(t *testing.T) {
	for _, s := range append(companyWordValues(t), "Acme (West", "Acme West)", " Acme - West ", "A/B (C)", "of the and a", "The Acme of Co") {
		for _, key := range []bool{false, true} {
			if got, want := companyScoreWords(s, key), referenceScoreWords(s, key); strings.Join(got, "|") != strings.Join(want, "|") || len(got) != len(want) {
				t.Errorf("%q key=%v: %q, want %q", s, key, got, want)
			}
		}
		if got, want := companyQualifierForm(s), splitCompanyQualifier(s).form; got != want {
			t.Errorf("%q: form %q, want %q", s, got, want)
		}
	}
}
