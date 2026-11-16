package vm

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dlclark/regexp2"
)

var rtdLit = regexp.MustCompile(`'((?:[^'\\]|\\.)*)'`)

func rtdUnescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'u':
			if i+4 < len(s) {
				if v, err := strconv.ParseUint(s[i+1:i+5], 16, 32); err == nil {
					b.WriteRune(rune(v))
					i += 4
					continue
				}
			}
			b.WriteByte('u')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func rtdFindAll(src string, text string) string {
	re, err := regexp2.Compile(src, regexp2.None)
	if err != nil {
		return "compile-error"
	}
	re.MatchTimeout = 300 * time.Millisecond
	var out []string
	m, err := re.FindStringMatch(text)
	for n := 0; m != nil && err == nil && n < 200; n++ {
		out = append(out, fmt.Sprintf("%d:%d", m.Index, m.Length))
		m, err = re.FindNextMatch(m)
	}
	if err != nil {
		return "timeout"
	}
	return strings.Join(out, ",")
}

func TestRTDDifferential(t *testing.T) {
	pats := map[string]bool{}
	for _, f := range []string{"regex_java_divergence", "regex_conservative_split", "regex_numeric_backreferences"} {
		fh, err := os.Open("../apextest/testdata/" + f + "/cases.tsv")
		if err != nil {
			t.Fatal(err)
		}
		r := csv.NewReader(bufio.NewReader(fh))
		r.Comma = '\t'
		r.LazyQuotes = true
		r.FieldsPerRecord = -1
		recs, _ := r.ReadAll()
		fh.Close()
		for _, rec := range recs {
			for _, f := range rec {
				for _, m := range rtdLit.FindAllStringSubmatch(f, -1) {
					pats[rtdUnescape(m[1])] = true
				}
			}
		}
	}
	for _, p := range []string{`(?i)hello`, `(?i)\bstreet\b`, `\bfoo\b`, `\b\w+\b`, `(?i)[a-z]+@[a-z]+\.com`, `(?i)[a-z_-]+`, `(?i)[A-Za-z0-9_-]+`, `(?i)[a-z#-]`, `(?i)k`, `(?i)i`, `(?i)s`, `(?iu)s`, `(?i)[^k]`, `(?i)[a-z]`, `(?i)[^a-z]`, `\B`, `(?i)\bk\b`, `(?i)(\w+)\s\1`} {
		pats[p] = true
	}
	subjects := []string{
		"", "hello Street foo bar", "Hello STREET Foo", "café Street foo", "naïve straße FOO", "İstanbul hello",
		"Kelvin k K street", "ſstreet ſ hello", "a – foo – b", "price €5 foo   street",
		"αβ foo Σ street", "中文 foo", "foó bar", "a\r\nb\u0085c d", "x@y.com é@z.com",
		"foo_bar-baz 12 ab ab", "\U0001F600 foo \U0001F600", "i I ı İ s S", "‍foo‌", "john smith@example.com\n",
		"é-b_9", "José 5%", "a-b_c#d",
	}
	diffs, total := 0, 0
	for p := range pats {
		if validateJavaRegexSource(p, "StringException") != nil {
			continue
		}
		for _, s := range subjects {
			narrow := regexTraitsFor(p, s)
			a, _, e1 := compileRegexp2SourceLoops("Pattern.compile", p, narrow)
			b, _, e2 := compileRegexp2SourceLoops("Pattern.compile", p, regexTraitsAll)
			if e1 != nil || e2 != nil {
				continue
			}
			total++
			if ra, rb := rtdFindAll(a, s), rtdFindAll(b, s); ra != rb {
				diffs++
				if diffs < 30 {
					t.Logf("DIFF %q on %q: narrow=%s full=%s", p, s, ra, rb)
				}
			}
		}
	}
	t.Logf("patterns=%d pairs=%d diffs=%d", len(pats), total, diffs)
	if diffs > 0 {
		t.Fail()
	}
}
