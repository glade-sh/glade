package dml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const residualPanelsFixture = "testdata/conformance/fuzzy_residual_panels"

// residualPanelMatches scores every ordered pair of one panel through the
// path its rule uses: the Company Name, Title and First Name scorers for the
// custom rules, and the shared duplicate-rule matcher for the standard
// Contact and Lead rules. The key is the candidate and existing index in
// base 36, as in the native observation.
func residualPanelMatches(t *testing.T, call string, values []string) map[string]string {
	t.Helper()
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	glade := map[string]string{}
	var scorer func(a, b string, matchBlanks bool) (bool, float64)
	field := 0
	switch {
	case strings.HasPrefix(call, "FuzzyPanel.run('Account', 'N3Co', 'Name'"):
		scorer = fuzzyCompany
	case strings.HasPrefix(call, "FuzzyPanel.run('Contact', 'N3Title', 'LastName+Title'"):
		scorer, field = fuzzyTitle, 1
	case strings.HasPrefix(call, "FuzzyPanel.run('Contact', 'N1First', 'FirstName+LastName'"):
		scorer = fuzzyFirstName
	}
	if scorer != nil {
		for i, a := range values {
			for j, b := range values {
				x, y := strings.Split(a, "|"), strings.Split(b, "|")
				if ok, c := scorer(x[field], y[field], false); ok {
					glade[string(digits[i])+string(digits[j])] = strconv.Itoa(int(c + 0.5))
				}
			}
		}
		return glade
	}
	row := standardNativeRow{}
	switch {
	case strings.HasPrefix(call, "FuzzyPanel.run('Contact', 'Standard_Contact_Duplicate_Rule', '"):
		row.Object, row.Rule = "Contact", "Standard_Contact_Match_Rule_v1_1"
	case strings.HasPrefix(call, "FuzzyPanel.run('Lead', 'Standard_Lead_Duplicate_Rule', '"):
		row.Object, row.Rule = "Lead", "Standard_Lead_Match_Rule_v1_0"
	default:
		t.Fatalf("no scorer for %s", call)
	}
	names := strings.Split(titleVariantQuoted.FindAllStringSubmatch(call, 3)[2][1], "+")
	for _, value := range values {
		parts := strings.Split(value, "|")
		if len(parts) != len(names) {
			t.Fatalf("value %q does not hold %v", value, names)
		}
		fields := map[string]string{}
		for i, name := range names {
			if parts[i] != "" {
				fields[name] = parts[i]
			}
		}
		row.Existing = append(row.Existing, fields)
		row.Candidates = append(row.Candidates, fields)
	}
	got, engine := standardRowMatches(t, row)
	if got == nil {
		t.Fatalf("rule is not enforced (%s)", engine)
	}
	for pair, confidence := range got {
		glade[string(digits[pair[0]])+string(digits[pair[1]])] = strconv.Itoa(int(confidence))
	}
	return glade
}

// NATIVE testdata/conformance/fuzzy_residual_panels/org.tsv: panels on the
// standard Contact and Lead rules (surnames, first names and the email path
// beside an equal email address; the phone section grid) and on custom Title,
// Company Name and First Name rules. Each panel inserts its values and asks
// Datacloud.FindDuplicates about every value; a token is the candidate and
// existing index (base 36) with the confidence when it is not 100. All eight
// observations of a panel (API 62.0 and 67.0, anonymous and @IsTest, two
// repeats) agree and every ordered pair is compared. residuals.tsv pins the
// pairs where Glade still differs ("-" is no match).
func TestFuzzyResidualNativePanels(t *testing.T) {
	var groups map[string]map[string]string
	body, err := os.ReadFile(filepath.Join(residualPanelsFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &groups); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(filepath.Join(residualPanelsFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if lines[0] != "api\troute\trepeat\tgroup\tmethod\tobservation" {
		t.Fatal("invalid native row header")
	}
	rows, seen := map[string]string{}, map[string]int{}
	for _, line := range lines[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 6 {
			t.Fatalf("invalid row %q", line)
		}
		key := f[3] + "/" + f[4]
		if old, ok := rows[key]; ok && old != f[5] {
			t.Fatalf("native observations differ: %s", key)
		}
		rows[key] = f[5]
		seen[key]++
	}
	residuals := map[string][2]string{}
	body, err = os.ReadFile(filepath.Join(residualPanelsFixture, "residuals.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 4 || f[2] == f[3] {
			t.Fatalf("invalid residual %q", line)
		}
		residuals[f[0]+"/"+f[1]] = [2]string{f[2], f[3]}
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	used := map[string]bool{}
	panels := 0
	for group, cases := range groups {
		if group == "canary" {
			continue
		}
		for method, call := range cases {
			observation, ok := rows[group+"/"+method]
			if !ok || seen[group+"/"+method] != 8 {
				t.Fatalf("%s/%s: %d native rows, want 8", group, method, seen[group+"/"+method])
			}
			quoted := titleVariantQuoted.FindAllStringSubmatch(call, -1)[3:]
			values := make([]string, len(quoted))
			for i, q := range quoted {
				values[i] = strings.ReplaceAll(q[1], `\'`, "'")
			}
			count, list, _ := strings.Cut(observation, ":")
			if n, err := strconv.Atoi(count); err != nil || n != len(values) {
				t.Fatalf("%s: observation holds %s values, the case %d", method, count, len(values))
			}
			native := map[string]string{}
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
			glade := residualPanelMatches(t, call, values)
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
	if panels != 18 {
		t.Fatalf("%d panels, want 18", panels)
	}
	if !t.Failed() {
		for _, key := range slices.Sorted(mapsKeys(residuals)) {
			if !used[key] {
				t.Errorf("unused residual %s", key)
			}
		}
	}
}
