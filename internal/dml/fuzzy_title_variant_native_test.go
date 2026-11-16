package dml

import (
	"encoding/json"
	"maps"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
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

// NATIVE fuzzy_title_variant/org.tsv (f1s/f1s1): the standard Contact rule,
// one last name and email address, API 62.0 and 67.0, both routes, two
// repeats. The Name Variant entries added from the custom First Name panels
// hold under the standard rule too: "Dave", "David" and "Davy" find each
// other at 100, as do "Sandy" and "Xander", and "Marjorie" and "Peggy". Glade
// reports no pair native does not. The native pairs Glade lacks are other
// variants of "Alexander" ("Sandy" finds "Zander"), which no custom row backs.
func TestFuzzyTitleVariantStandardNames(t *testing.T) {
	t.Setenv("GLADE_STANDARD_DUPLICATE_RULES", "on")
	var groups map[string]map[string]string
	body, err := os.ReadFile(filepath.Join(titleVariantFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &groups); err != nil {
		t.Fatal(err)
	}
	call := groups["f1s"]["f1s1"]
	if !strings.Contains(call, "'Standard_Contact_Duplicate_Rule', 'FirstName+LastName+Email'") {
		t.Fatalf("case %q", call)
	}
	var names []string
	var records []storage.Record
	for _, q := range titleVariantQuoted.FindAllStringSubmatch(call, -1)[3:] {
		parts := strings.Split(q[1], "|")
		names = append(names, parts[0])
		records = append(records, storage.Record{Object: "Contact", Fields: map[string]storage.Value{
			"FirstName": storage.StringValue(parts[0]), "LastName": storage.StringValue(parts[1]), "Email": storage.StringValue(parts[2]),
		}})
	}
	body, err = os.ReadFile(filepath.Join(titleVariantFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	native := map[[2]string]int{}
	rows := 0
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		f := strings.Split(line, "\t")
		if f[3] != "f1s" || f[4] != "f1s1" {
			continue
		}
		rows++
		_, list, _ := strings.Cut(f[5], ":")
		row := map[[2]string]int{}
		for _, token := range strings.Split(list, ",") {
			confidence := 100
			if len(token) > 2 {
				v, err := strconv.ParseFloat(token[2:], 64)
				if err != nil {
					t.Fatalf("invalid token %q", token)
				}
				confidence = int(v)
			}
			row[[2]string{names[strings.IndexByte(digits, token[0])], names[strings.IndexByte(digits, token[1])]}] = confidence
		}
		if rows > 1 && !maps.Equal(row, native) {
			t.Fatal("native rows differ")
		}
		native = row
	}
	if rows != 8 {
		t.Fatalf("native rows %d", rows)
	}
	hit, exact := 0, 0
	for i, a := range names {
		matcher := standardDuplicateMatcher(nil, "Contact", "Contact", records[i])
		for j, b := range names {
			ok, confidence := matcher(records[j])
			n, found := native[[2]string{a, b}]
			switch {
			case ok && !found:
				t.Errorf("%s=>%s: Glade %v, native has no match", a, b, confidence)
			case ok:
				hit++
				if int(confidence+0.5) == n {
					exact++
				}
			}
		}
	}
	// 41 since the standard rules score both names without a key when the
	// email addresses agree (NATIVE duplicate_rule_match_keys/org.tsv).
	if len(native) != 51 || hit != 41 || exact != 41 {
		t.Errorf("native=%d hit=%d exact=%d, want 51, 41, 41", len(native), hit, exact)
	}
	for _, pair := range [][2]string{{"Dave", "Davy"}, {"Davy", "Dave"}, {"David", "Davy"}, {"Davy", "David"}, {"Sandy", "Xander"}, {"Xander", "Sandy"}, {"Marjorie", "Peggy"}, {"Peggy", "Marjorie"}} {
		if native[pair] != 100 {
			t.Errorf("%s=>%s: native %d", pair[0], pair[1], native[pair])
		}
		if ok, confidence := standardFirstName(pair[0], pair[1]); !ok || confidence != 100 {
			t.Errorf("%s=>%s: standard first name %v %v", pair[0], pair[1], ok, confidence)
		}
	}
}
