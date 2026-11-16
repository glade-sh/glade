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

// streetNativeDirs are the custom Street captures. The value is the most pairs
// Glade may match that the native rows do not; every such pair is pinned in the
// fixture's residuals.tsv, so the count cannot grow unseen.
var streetNativeDirs = map[string]int{
	"testdata/conformance/fuzzy_street_c4":  0,
	"testdata/conformance/fuzzy_street_c5":  0,
	"testdata/conformance/fuzzy_street_c8":  0,
	"testdata/conformance/fuzzy_street_c9":  8,
	"testdata/conformance/fuzzy_street_c10": 0,
}

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
// NATIVE fuzzy_street_c5/org.tsv:2 anonymous 62.0
// NATIVE fuzzy_street_c5/org.tsv:9 istest 67.0
// NATIVE fuzzy_street_c8/org.tsv:2 anonymous 62.0
// NATIVE fuzzy_street_c8/org.tsv:9 istest 67.0
// NATIVE fuzzy_street_c9/org.tsv:2 anonymous 62.0
// NATIVE fuzzy_street_c9/org.tsv:9 istest 67.0
// Every pair of each street panel's values through a custom Street rule
// (Datacloud.FindDuplicates, anonymous and @IsTest, APIs 62.0 and 67.0, two
// repeats, all identical per case). Fit panels and holdout panels are scored
// the same way; RUN-FACTS.txt names the panels that shaped the model. A pair
// where Glade differs from the native rows is pinned in residuals.tsv with
// both sides.
func TestFuzzyStreetNativeRows(t *testing.T) {
	for dir, maxExtra := range streetNativeDirs {
		t.Run(filepath.Base(dir), func(t *testing.T) { streetNativeRows(t, dir, maxExtra) })
	}
}

func streetNativeRows(t *testing.T, dir string, maxExtra int) {
	var panels map[string][]string
	body, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &panels); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
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
	// matchblanks.txt names the panels captured under a Match Blank Fields item.
	matchBlanks := map[string]bool{}
	if body, err := os.ReadFile(filepath.Join(dir, "matchblanks.txt")); err == nil {
		for _, name := range strings.Fields(string(body)) {
			if panels[name] == nil {
				t.Fatalf("matchblanks.txt names the unknown panel %q", name)
			}
			matchBlanks[name] = true
		}
	}
	residuals := map[string][2]string{}
	if body, err := os.ReadFile(filepath.Join(dir, "residuals.tsv")); err == nil {
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
	extra := 0
	for _, name := range names {
		values := panels[name]
		nat := streetNativePanel(t, native[name])
		for i, a := range values {
			for j, b := range values {
				if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
					// A blank street never reaches the scorer; the rule path decides.
					continue
				}
				pair := streetPanelDigits[i:i+1] + streetPanelDigits[j:j+1]
				want := "-"
				if v, ok := nat[pair]; ok {
					want = v
				}
				got := "-"
				if match, score := FuzzyScore("Street", a, b, matchBlanks[name]); match {
					got = fmt.Sprint(int(math.Floor(fuzzyItemScore("Street", score) + 0.5)))
				}
				if want == "-" && got != "-" {
					extra++
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
			}
		}
	}
	for key := range residuals {
		if !used[key] {
			t.Errorf("unused residual %s", key)
		}
	}
	if extra > maxExtra {
		t.Errorf("Glade matches %d pairs the native rows do not; at most %d", extra, maxExtra)
	}
}

// NATIVE fuzzy_street_c4/org.tsv, fuzzy_street_c5/org.tsv: one captured row per
// rule the custom Street scorer applies, all through FuzzyScore. The panel is
// named beside each group.
func TestFuzzyStreetCapturedRules(t *testing.T) {
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
		{"41 Harlan Ave", "41\nHarlan Ave", true, 77},
		// Letters after the number (cl1, cl2, nl1).
		{"100 Vane St", "100 A Vane St", true, 83},
		{"100 Vane St", "100 B Vane St", false, 0},
		{"100 Vane St", "100 E Vane St", true, 92},
		{"71 E Vane St", "71 E E Vane St", true, 83},
		{"71 E B Vane St", "71 B E Vane St", true, 86},
		{"71 A Vane", "71 E Vane", true, 88},
		// Identical streets report the section average (cs1, nm1).
		{"Test Street", "Test Street", true, 88},
		{"Test", "Test", true, 86},
		{"500 Calder Rd", "500 Calder Rd", true, 100},
		{"Morven Tully Harbor Tower", "Morven Tully Harbor Tower", true, 100},
		// Edit similarity 0 when the distance is the candidate's length
		// (cu1, hu1, duplicate_fuzzy_matching).
		{"77 Wren Str", "77 Wren St", true, 78},
		{"77 Wren St", "77 Wren Str", true, 60},
		{"19 Birkin Dr", "19 Birkin Drv", true, 82},
		{"15 Elm", "15 Elm Drive", true, 50},
		{"15 Elm Drive", "15 Elm", true, 75},
		{"77 Wren Ave", "77 Wren Av", true, 80},
		{"77 Wren Xq", "77 Wren St", false, 0},
		// Number words and ordinals in the name (fuzzy_street_c10 o1 to o8,
		// e1 to e4): a name of numbers only, or after a directional, is
		// compared as one string; an ordinal first word is no street number.
		{"500 Twenty One St", "500 Twenty First St", false, 0},
		{"500 Twenty One St", "500 Twenty One Street", false, 0},
		{"500 Twenty First Street", "500 Twenty One Street", true, 88},
		{"300 E Twenty Third St", "300 E Twenty Third Street", false, 0},
		{"300 E Twenty Third Street", "300 Twenty Third Street", true, 45},
		{"500 Twenty First Street", "500 Twenty Tenth Street", true, 80},
		{"500 Twenty First St", "500 Twenty First Street", true, 88},
		{"Third Ave", "3rd Ave", true, 88},
		{"101st St", "101st St", true, 88},
		{"77 Wren St", "77 Wren St Xq", false, 0},
		{"14 Dana Blvd Jr", "14 Dana Jr Blvd", false, 0},
		// The similarity gate is 87 (nt1, nd4).
		{"32 Lvlofeer Rd", "32 Lvlofeer Rdxq", true, 80},                             // 2 edits in 16: 87.50
		{"35 Stcesoimtapar Rd Suite 5", "35 Stcesoimtapar Rdxqzj Suite 5", true, 64}, // 4 in 31: 87.10
		{"33 Erdugunowymist Rd", "33 Erdugunowymist Rdxqz", false, 0},                // 3 in 23: 86.96
		{"31 Hakeonl Rd", "31 Hakeonl Rdxq", false, 0},                               // 2 in 15: 86.67
		{"84 Sorrel St N", "84 Sorrel Strt N", true, 75},
		{"84 Sorrel St", "84 Sorrel Strt", false, 0},
		// Match key: five characters of the number and of the name (nk1).
		{"7 Morven Rd", "7 Morvex Rd", true, 91},
		{"7 Morven Rd", "7 Morvxn Rd", false, 0},
		{"12345 Morven Rd", "123456 Morven Rd", true, 85},
		{"12345 Morven Rd", "12346 Morven Rd", false, 0},
		// Street words left out of the key (ng1, ng7, np1, np3, nq1).
		{"121 Tully Ln W", "121 Tully Ln N", true, 96},
		{"101 Park Ln W", "101 Park Ln N", false, 0},
		{"101 Park Ln W", "101 W Park Ln", true, 85},
		{"101 Park Ln E", "101 Park Ln SE", true, 96},
		{"103 Park Ave", "103 Park Avn", true, 60},
		{"103 Park Ln Suite 5", "103 Park Ln Suite 6", false, 0},
		{"Morven Tully West", "Morven Tully Wests", true, 81},
		{"Morven Tower West", "Morven Tower Wests", false, 0},
		// A street that does not split scores the edit similarity (nh1, nn1,
		// ns1, nr1, nm1).
		{"62 Garnet Ave #5", "62 Garnet Ave#5", true, 93},
		{"62 Garnet Ave#5", "62 Garnet Ave #5", true, 47},
		{"62 Garnet Ave#5", "62 Garnet Ave/5", true, 100},
		{"62 Garnet Ave-5", "62 Garnet Ave 5", true, 47},
		{"72A Vane St", "72-A Vane St", true, 92},
		{"72-A Vane St", "72A Vane St", true, 46},
		{"72A Vane St", "72 A Vane St", false, 0},
		{"72-74 Vane St", "7274 Vane St", true, 84},
		{"Suite 9 73 Morrow Rd", "Ste 9 73 Morrow Rd", true, 90},
		{"Suite 9 73 Morrow Rd", "Suite 8 73 Morrow Rd", false, 0},
		{"Suite 9\n73 Morrow Rd", "Suite 9 73 Morrow Rd", true, 95},
		{"RR 4 Box 22", "RR 4 Box 23", true, 91},
		{"RR 4 Box 22", "RR 5 Box 22", false, 0},
		{"Morven Tully Harbor Tower", "Morven Tully Harbor Towers", true, 96},
		{"Morven Tully Tower", "Morven Tully Towers", true, 81},
		// PO Box (cp1, nr1).
		{"PO Box 410", "P.O. Box 410", true, 100},
		{"P O Box 410", "PO Box 410", true, 95},
		{"PO Box 410", "PO Box 411", false, 0},
		{"PO Box 410", "POB 410", false, 0},
		{"POB 410", "POB 410", true, 88},
		{"PO Box 410", "PO Box410", true, 90},
		{"PO Box410", "PO Box 410", true, 90},
		{"PO Box 52", "POBox52", false, 0},
		{"PO Box A", "PO Box A", false, 0},
		// Number words (nw1).
		{"200 Ten St", "200 10 St", true, 100},
		{"200 Twenty First St", "200 20 1st St", true, 100},
		{"200 Twentieth St", "200 20th St", false, 0},
		// Words before a suite value (nf1, duplicate_fuzzy_matching).
		{"600 Montgomery St Fl 3", "600 Montgomery St", true, 89},
		{"305 Corwin Rd Room 3", "305 Corwin Rd", true, 83},
		{"302 Corwin Rd Level 3", "302 Corwin Rd", false, 0},
		// A compass word before the name (sd1, nd4).
		{"500 North Calder Rd", "500 Northwest Calder Rd", true, 91},
		{"85 North Sorrel Dr", "85 Northern Sorrel Dr", false, 0},
		// fuzzy_street_c8: the key reads a line break as a word separator,
		// the score deletes it (qa1, qa2). With no street number the break
		// still finds the same street without it.
		{"Calder Rd Suite 200", "Calder Rd\nSuite 200", true, 69},
		{"Calder Rd\nSuite 200", "Calder Rd Suite 200", true, 69},
		{"Calder Rd\nSuite 200", "Calder Rd,\nSuite 200", true, 88},
		{"Calder Rd\nSuite 200", "Calder RdSuite 200", false, 0},
		{"Calder Rd Suite200", "Calder Rd\nSuite200", true, 94},
		{"Calder Rd #200", "Calder Rd\n#200", true, 92},
		{"Calder Rd\n#200", "Calder Rd #200", true, 46},
		{"Oak St Suite 200", "Oak St\nSuite 200", true, 64},
		{"Calder Plaza Suite 200", "Calder Plaza\nSuite 200", true, 84},
		// A name of four words does not split (qb1, qb3).
		{"100 N Martin Luther King Jr Blvd", "100 N Martin Luther King Blvd", true, 45},
		{"100 N Martin Luther King Blvd", "100 N Martin Luther King Jr Blvd", true, 91},
		{"100 N Martin Luther King Blvd Suite 5", "100 N Martin Luther King Jr Blvd Suite 5", true, 93},
		{"100 N Martin Luther King Jr Blvd", "100 N Martin Luther King Blvd Suite 5", false, 0},
		{"14 Kingsley Blvd Jr", "14 Kingsley King Jr Blvd", true, 72},
		{"14 Hawthorne Meadow Blvd Jr", "14 Hawthorne Meadow Blvd", true, 44},
		{"14 Dana Blvd Jr", "14 Dana Jr Blvd", false, 0},
		// A rural route reports 100 for itself (qc1); Rt, Rte and Rd are left
		// out of the key (qc2, qj1).
		{"RR 4", "RR 4", true, 100},
		{"Rural Route 4", "Rural Route 4", true, 100},
		{"HC 4", "HC 4", true, 88},
		{"Rural Rt 4", "Rural Rte 4", true, 80},
		{"Rural Rt 4", "Rural Rd 4", true, 72},
		{"604 Sts Ln W", "604 Sts Ln N", false, 0},
		{"615 Pk Ln W", "615 Pk Ln N", false, 0},
		// Accented letters fold, other characters outside ASCII are deleted
		// (qh1, qh2, qm1, qd1).
		{"7 Bor\u00fcden Rd", "7 Boruden Rd", true, 100},
		{"7 Dun\u00f8wick Rd", "7 Dunwick Rd", true, 100},
		{"Fu\u00dfweg 5", "Fuweg 5", true, 88},
		{"Friedrichstra\u00dfe 43", "Friedrichstr. 43", true, 78},
		{"12 Hauptstra\u00dfe S\u00fcd", "12 Hauptstrasse Sud", true, 90},
		{"7 Ob\u2019rien Rd", "7 Obrien Rd", true, 100},
		// Suffix words (qe1, qe2): Aly is the suffix Alley; Lp and Loop, Pkwy
		// and Parkway stay in the name.
		{"12 Hazel Aly", "12 Hazel Alley", true, 93},
		{"1 Infinite Lp", "1 Infinity Loop", true, 82},
		{"1600 Amphitheatre Pkwy", "1600 Amphitheater Parkway", true, 81},
		{"2 Quarry Lp", "2 Quarri Loop", true, 79},
		// Rue is a word of the name (qf1).
		{"7 Rue Royale", "7 Rue Royal", true, 92},
		{"12 Rue de Rivoli", "12 Rue Rivoli", true, 82},
		{"12 Rue Rivoli", "12 Ave Rivoli", true, 85},
		{"12 Rue Rivoli", "12 Rivoli", false, 0},
		{"Rue Royale", "Rue Royale", true, 86},
		// fuzzy_street_c9 (r1, r7): a street name on one side only is a
		// mismatch; a street with no key word finds nothing; one word that is
		// not a number is the name; characters outside ASCII are deleted, so
		// two Cyrillic or CJK streets with one house number are one street.
		{"12", "12 St", false, 0},
		{"12", "12 Park", false, 0},
		{"12 St", "12 Rd", false, 0},
		{"12 Park", "12 Park St", true, 85},
		{"12 Park", "12 Park Street", true, 50},
		{"12 Park Street", "12 Park", true, 75},
		{"Park St", "Park St", false, 0},
		{"\u6e2f\u533a", "\u6e2f\u533a", false, 0},
		{"\u6e2f\u533a", "\u5317\u533a", false, 0},
		{"3F", "3F", true, 86},
		{"\u6e2f\u533a 3F", "\u5317\u533a 3F", true, 86},
		{"\u0422\u0432\u0435\u0440\u0441\u043a\u0430\u044f 5", "\u0410\u0440\u0431\u0430\u0442\u0441\u043a\u0430\u044f 5", true, 100},
		{"\u0422\u0432\u0435\u0440\u0441\u043a\u0430\u044f 5", "\u0422\u0432\u0435\u0440\u0441\u043a\u0430\u044f 7", false, 0},
		{"12 \u0391\u03b8\u03b7\u03bd\u03ce\u03bd", "12", true, 100},
		{"12 \u0391\u03b8\u03b7\u03bd\u03ce\u03bd St", "12", false, 0},
	} {
		match, score := FuzzyScore("Street", c.a, c.b, false)
		got := int(math.Floor(score + 0.5))
		if match != c.match || (match && got != c.conf) {
			t.Errorf("%q / %q: got %v %d, native %v %d", c.a, c.b, match, got, c.match, c.conf)
		}
	}
	// Match Blank Fields scores the same sections (fuzzy_street_c8 qd2, qg2):
	// the floor words are suites there too, and the name similarity decides
	// where the street has no number.
	for _, c := range []struct {
		a, b  string
		match bool
		conf  int
	}{
		{"200 Park Ave Fl 5", "200 Park Ave Floor 5", true, 93},
		{"Hauptstra\u00dfe 5", "Hauptstr. 5", true, 86},
		{"Hauptstrasse 5", "Hauptstr. 5", true, 76},
		{"Haupt Str. 5", "Hauptstr. 5", true, 92},
		{"Fu\u00dfweg 5", "Fuweg 5", true, 100},
		{"3F", "3F", true, 100},
		{"Park St", "Park Street", false, 0},
		{"12", "12 Park", false, 0},
	} {
		match, score := FuzzyScore("Street", c.a, c.b, true)
		got := int(math.Floor(score + 0.5))
		if match != c.match || (match && got != c.conf) {
			t.Errorf("MatchBlanks %q / %q: got %v %d, native %v %d", c.a, c.b, match, got, c.match, c.conf)
		}
	}
	// The shared scorer the standard rules call keeps its behavior.
	for _, c := range [][2]string{{"500 Calder Rd\nSuite 200", "500 Calder Rd Suite 200"}, {"100 Vane St", "100 A Vane St"}, {"101 Park Ln W", "101 Park Ln N"}} {
		_, base := fuzzyStreet(c[0], c[1], false)
		_, custom := FuzzyScore("Street", c[0], c[1], false)
		if base == custom {
			t.Errorf("%q / %q: shared and custom scorers agree; the custom rules should differ", c[0], c[1])
		}
	}
}
