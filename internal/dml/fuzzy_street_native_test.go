package dml

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const streetNativeDir = "testdata/conformance/fuzzy_street_c4"

const streetPanelDigits = "0123456789abcdefghijklmnopqrstuvwxyz"

// streetNativePanel parses a FuzzyPanel observation: candidate and existing
// index in base 36, then the confidence when it is not 100.
func streetNativePanel(t *testing.T, observation string) map[string]string {
	t.Helper()
	_, body, ok := strings.Cut(observation, ":")
	if !ok {
		t.Fatalf("invalid observation %q", observation)
	}
	out := map[string]string{}
	if body == "" {
		return out
	}
	for _, token := range strings.Split(body, ",") {
		conf := "100"
		if len(token) > 2 {
			var v float64
			if _, err := fmt.Sscanf(token[2:], "%g", &v); err != nil {
				t.Fatalf("invalid token %q", token)
			}
			conf = fmt.Sprint(int(v))
		}
		out[token[:2]] = conf
	}
	return out
}

// NATIVE fuzzy_street_c4/org.tsv:2 anonymous 62.0
// NATIVE fuzzy_street_c4/org.tsv:9 istest 67.0
// Every pair of each street panel's values through a custom Street rule
// (Datacloud.FindDuplicates, anonymous and @IsTest, APIs 62.0 and 67.0, two
// repeats, all identical per case). Fit panels and holdout panels are scored
// the same way; the holdout values were not used to fit. A pair where Glade
// differs from the native rows is pinned in residuals.tsv with both sides.
func TestFuzzyStreetC4NativeRows(t *testing.T) {
	var panels map[string][]string
	body, err := os.ReadFile(filepath.Join(streetNativeDir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &panels); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(filepath.Join(streetNativeDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if lines[0] != "api\troute\trepeat\tgroup\tmethod\tobservation" {
		t.Fatal("invalid native header")
	}
	native := map[string]string{}
	counts := map[string]int{}
	for _, line := range lines[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 6 || panels[f[4]] == nil {
			t.Fatalf("invalid row %q", line)
		}
		if f[3] != "fit" && f[3] != "holdout" {
			t.Fatalf("invalid group %q", f[3])
		}
		if old, ok := native[f[4]]; ok && old != f[5] {
			t.Fatalf("native observations differ across API, route or repeat: %s", f[4])
		}
		native[f[4]] = f[5]
		counts[f[4]]++
	}
	for name := range panels {
		if counts[name] != 8 {
			t.Fatalf("%s: %d observations, want 8 (2 APIs x 2 routes x 2 repeats)", name, counts[name])
		}
	}
	residuals := map[string][2]string{}
	if body, err := os.ReadFile(filepath.Join(streetNativeDir, "residuals.tsv")); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
			f := strings.Split(line, "\t")
			residuals[f[0]+"/"+f[1]] = [2]string{f[2], f[3]}
		}
	}
	used := map[string]bool{}
	var names []string
	for name := range panels {
		names = append(names, name)
	}
	sort.Strings(names)
	var report []string
	for _, name := range names {
		values := panels[name]
		nat := streetNativePanel(t, native[name])
		for i, a := range values {
			for j, b := range values {
				pair := streetPanelDigits[i:i+1] + streetPanelDigits[j:j+1]
				want := "-"
				if v, ok := nat[pair]; ok {
					want = v
				}
				got := "-"
				if match, score := FuzzyScore("Street", a, b, false); match {
					got = fmt.Sprint(int(math.Floor(fuzzyItemScore("Street", score) + 0.5)))
				}
				key := name + "/" + pair
				if r, ok := residuals[key]; ok {
					used[key] = true
					if r[0] != want {
						t.Errorf("%s: native %s changed under residual pin %s", key, want, r[0])
					}
					want = r[1]
				}
				if got != want {
					t.Errorf("%s %q -> %q: Glade %s, expected %s", key, a, b, got, want)
				}
				if os.Getenv("STREET_RESIDUALS") != "" && got != nat2(nat, pair) {
					report = append(report, fmt.Sprintf("%s\t%s\t%s\t%s", name, pair, nat2(nat, pair), got))
				}
			}
		}
	}
	for key := range residuals {
		if !used[key] {
			t.Errorf("unused residual %s", key)
		}
	}
	if path := os.Getenv("STREET_RESIDUALS"); path != "" {
		os.WriteFile(path, []byte("panel\tpair\tnative\tglade\n"+strings.Join(report, "\n")+"\n"), 0o644)
	}
}

func nat2(nat map[string]string, pair string) string {
	if v, ok := nat[pair]; ok {
		return v
	}
	return "-"
}

// NATIVE fuzzy_street_c4/org.tsv: one row per rule the custom Street scorer
// applies, all through FuzzyScore. Uncaptured neighbours keep base behavior.
func TestFuzzyStreetC4CapturedRules(t *testing.T) {
	type row struct {
		a, b  string
		match bool
		conf  int
	}
	for _, c := range []row{
		// Line break joined (cj1): the break is deleted, not a space.
		{"500 Calder Rd\nSuite 200", "500 Calder Rd Suite 200", true, 81},
		{"500 Calder Rd\nSuite 200", "500 Calder RdSuite 200", true, 100},
		{"500 Calder Rd,\nSuite 200", "500 Calder Rd\nSuite 200", true, 100},
		{"500 Calder Rd\nSuite 200", "500 Calder Rd\nFloor 2", false, 0},
		// A single letter after the number (cl1, cl2).
		{"100 Vane St", "100 A Vane St", true, 83},
		{"100 Vane St", "100 B Vane St", false, 0},
		{"100 Vane St", "100 E Vane St", true, 92},
		// Identical streets report the section average (cs1).
		{"Test Street", "Test Street", true, 88},
		{"Test", "Test", true, 86},
		{"500 Calder Rd", "500 Calder Rd", true, 100},
		// Unknown trailing word near a known suffix (cu1).
		{"77 Wren Str", "77 Wren St", true, 78},
		{"77 Wren Ave", "77 Wren Av", true, 80},
		{"77 Wren Xq", "77 Wren St", false, 0},
		{"77 Wren St", "77 Wren St Xq", false, 0},
		// PO Box (cp1).
		{"PO Box 410", "P.O. Box 410", true, 100},
		{"P O Box 410", "PO Box 410", true, 95},
		{"PO Box 410", "PO Box 411", false, 0},
		{"PO Box 410", "POB 410", false, 0},
		{"POB 410", "POB 410", true, 88},
	} {
		match, score := FuzzyScore("Street", c.a, c.b, false)
		got := int(math.Floor(score + 0.5))
		if match != c.match || (match && got != c.conf) {
			t.Errorf("%q / %q: got %v %d, native %v %d", c.a, c.b, match, got, c.match, c.conf)
		}
	}
	// The shared scorer the standard rules call keeps its behavior.
	for _, c := range [][2]string{{"500 Calder Rd\nSuite 200", "500 Calder Rd Suite 200"}, {"100 Vane St", "100 A Vane St"}} {
		_, base := fuzzyStreet(c[0], c[1], false)
		_, custom := FuzzyScore("Street", c[0], c[1], false)
		if base == custom {
			t.Errorf("%q / %q: shared and custom scorers agree; the custom rules should differ", c[0], c[1])
		}
	}
}
