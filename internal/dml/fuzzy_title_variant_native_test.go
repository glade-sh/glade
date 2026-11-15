package dml

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const titleVariantFixture = "testdata/conformance/fuzzy_title_variant"

var titleVariantQuoted = regexp.MustCompile(`'((?:[^'\\]|\\.)*)'`)

// NATIVE fuzzy Title and First Name panels testdata/conformance/fuzzy_title_variant/org.tsv:
// each panel inserts its values as records under one custom matching rule (a
// Title item with an exact Last Name item, or a First Name item) and asks
// Datacloud.FindDuplicates about every value; each row lists the candidate and
// existing indexes (base 36) that matched, with the confidence when not 100.
// The 62.0 and 67.0 rows, both routes and both repeats agree; the 67.0
// anonymous rows are compared exactly, pair by pair, against the scorers.
// residuals.tsv pins the pairs where Glade still differs.
func TestFuzzyTitleVariantNativePanels(t *testing.T) {
	var groups map[string]map[string]string
	body, err := os.ReadFile(filepath.Join(titleVariantFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &groups); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(filepath.Join(titleVariantFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if lines[0] != "api\troute\trepeat\tgroup\tmethod\tobservation" {
		t.Fatal("invalid native row header")
	}
	rows := map[string]string{}
	for _, line := range lines[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 6 {
			t.Fatalf("invalid row %q", line)
		}
		key := f[3] + "/" + f[4]
		if f[0] != "67.0" || f[1] != "anonymous" {
			continue
		}
		if old, ok := rows[key]; ok && old != f[5] {
			t.Fatalf("native repeats differ: %s", key)
		}
		rows[key] = f[5]
	}
	residuals := map[string][2]string{}
	if body, err := os.ReadFile(filepath.Join(titleVariantFixture, "residuals.tsv")); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
			f := strings.Split(line, "\t")
			if len(f) != 4 || f[2] == f[3] {
				t.Fatalf("invalid residual %q", line)
			}
			residuals[f[0]+"/"+f[1]] = [2]string{f[2], f[3]}
		}
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	used := map[string]bool{}
	panels := 0
	for group, cases := range groups {
		for method, call := range cases {
			if !strings.HasPrefix(call, "FuzzyPanel.run('Contact'") {
				continue
			}
			isTitle := strings.Contains(call, "'LastName+Title'")
			isFirst := strings.Contains(call, "'FirstName+LastName'")
			if !isTitle && !isFirst {
				continue
			}
			observation, ok := rows[group+"/"+method]
			if !ok {
				t.Fatalf("%s/%s: missing native row", group, method)
			}
			quoted := titleVariantQuoted.FindAllStringSubmatch(call, -1)[3:]
			values := make([]string, len(quoted))
			for i, q := range quoted {
				v := strings.ReplaceAll(q[1], `\'`, "'")
				first, last, _ := strings.Cut(v, "|")
				if isTitle {
					values[i] = last
				} else {
					values[i] = first
				}
			}
			native := map[string]string{}
			if _, list, _ := strings.Cut(observation, ":"); list != "" {
				for _, token := range strings.Split(list, ",") {
					confidence := "100"
					if len(token) > 2 {
						f, err := strconv.ParseFloat(token[2:], 64)
						if err != nil {
							t.Fatalf("invalid token %q", token)
						}
						confidence = strconv.Itoa(int(f))
					}
					native[token[:2]] = confidence
				}
			}
			glade := map[string]string{}
			for i, a := range values {
				for j, b := range values {
					var ok bool
					var c float64
					if isTitle {
						ok, c = fuzzyTitle(a, b, false)
					} else {
						ok, c = fuzzyFirstName(a, b, false)
					}
					if ok {
						glade[string(digits[i])+string(digits[j])] = strconv.Itoa(int(c + 0.5))
					}
				}
			}
			keys := map[string]bool{}
			for k := range native {
				keys[k] = true
			}
			for k := range glade {
				keys[k] = true
			}
			ordered := make([]string, 0, len(keys))
			for k := range keys {
				ordered = append(ordered, k)
			}
			sort.Strings(ordered)
			for _, pair := range ordered {
				n, g := native[pair], glade[pair]
				if n == "" {
					n = "-"
				}
				if g == "" {
					g = "-"
				}
				want := n
				if r, ok := residuals[method+"/"+pair]; ok {
					used[method+"/"+pair] = true
					if r[0] != n {
						t.Errorf("%s %s: native %s changed under residual pin %s", method, pair, n, r[0])
					}
					want = r[1]
				}
				if g != want {
					i, j := strings.Index(digits, pair[:1]), strings.Index(digits, pair[1:])
					t.Errorf("%s %s (%q -> %q): native=%s Glade=%s pinned=%s", method, pair, values[i], values[j], n, g, want)
				}
			}
			panels++
		}
	}
	if panels == 0 {
		t.Fatal("no panels")
	}
	if !t.Failed() {
		for _, key := range slices.Sorted(mapsKeys(residuals)) {
			if !used[key] {
				t.Errorf("unused residual %s", key)
			}
		}
	}
}

func mapsKeys[V any](m map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// titleVariantFirstNamePairs are the Name Variant pairs the First Name panels
// of the title and variant fixture score 100 where Jaro-Winkler under the match
// key does not explain the score (the same rule as TestFuzzyNameTablesMatchNativeRows).
func titleVariantFirstNamePairs(t *testing.T) map[string]bool {
	t.Helper()
	var groups map[string]map[string]string
	body, err := os.ReadFile(filepath.Join(titleVariantFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &groups); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(filepath.Join(titleVariantFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	out := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		f := strings.Split(line, "\t")
		if f[0] != "67.0" || f[1] != "anonymous" || f[2] != "1" || !strings.Contains(groups[f[3]][f[4]], "'FirstName+LastName'") {
			continue
		}
		quoted := titleVariantQuoted.FindAllStringSubmatch(groups[f[3]][f[4]], -1)[3:]
		_, list, _ := strings.Cut(f[5], ":")
		for _, token := range strings.Split(list, ",") {
			if len(token) != 2 || token[0] == token[1] {
				continue
			}
			first := func(i int) string {
				name, _, _ := strings.Cut(quoted[strings.IndexByte(digits, token[i])][1], "|")
				return foldText(name)
			}
			x, y := first(0), first(1)
			score := jaroWinkler(x, y)
			if x == y || doubleMetaphone(x, firstNameKeyLength) == doubleMetaphone(y, firstNameKeyLength) && score >= firstNameJaroWinkler-1e-9 && math.Floor(averageWith100(100*score)+0.5) == 100 {
				continue
			}
			out[x+"\x00"+y] = true
		}
	}
	return out
}
