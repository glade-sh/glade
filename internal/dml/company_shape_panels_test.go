package dml

import (
	"encoding/csv"
	"encoding/json"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

type companyFitPanel struct {
	values []string
	native map[[2]int]float64
}

// companyFitPanels reads the FuzzyPanel cases of company_name_fit whose method
// has one of the prefixes. Every API, route and repeat of a panel must report
// the same observation.
func companyFitPanels(t *testing.T, prefixes ...string) map[string]companyFitPanel {
	t.Helper()
	raw, err := os.ReadFile(companyFitDir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string]string
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(companyFitDir + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	quoted := regexp.MustCompile(`'([^']*)'`)
	observed, repeats := map[string]string{}, map[string]int{}
	for _, row := range rows[1:] {
		selected := false
		for _, prefix := range prefixes {
			selected = selected || strings.HasPrefix(row[4], prefix)
		}
		if !selected || !strings.Contains(cases[row[4]], "FuzzyPanel.") {
			continue
		}
		if prior, ok := observed[row[4]]; ok && prior != row[5] {
			t.Fatalf("%s: observations disagree across api, route or repeat", row[4])
		}
		observed[row[4]] = row[5]
		repeats[row[4]]++
	}
	out := map[string]companyFitPanel{}
	for name, observation := range observed {
		if repeats[name] != 8 {
			t.Fatalf("%s: %d observations, want 8", name, repeats[name])
		}
		src := cases[name]
		list := src[strings.Index(src, "new List<String>{")+len("new List<String>{"):]
		list = list[:strings.IndexByte(list, '}')]
		panel := companyFitPanel{native: map[[2]int]float64{}}
		for _, m := range quoted.FindAllStringSubmatch(list, -1) {
			panel.values = append(panel.values, m[1])
		}
		_, body, _ := strings.Cut(observation, ":")
		for _, entry := range strings.Split(body, ",") {
			i, e1 := strconv.ParseInt(entry[:1], 36, 0)
			j, e2 := strconv.ParseInt(entry[1:2], 36, 0)
			if e1 != nil || e2 != nil || int(i) >= len(panel.values) || int(j) >= len(panel.values) {
				t.Fatalf("%s: malformed entry %q", name, entry)
			}
			score := 100.0
			if len(entry) > 2 {
				if score, err = strconv.ParseFloat(entry[2:], 64); err != nil {
					t.Fatal(err)
				}
			}
			panel.native[[2]int{int(i), int(j)}] = score
		}
		out[name] = panel
	}
	return out
}

// NATIVE company_name_fit/org.tsv R1_* and R2_* (custom CompanyName rule, API
// 62.0 and 67.0, anonymous and @IsTest, two repeats; all observations agree).
// The panels vary the shapes the fitted scorer covers: letters around the
// match key, suffix words across qualifier forms, numbers and number markers,
// digit runs, single letters and joins. Every ordered pair is compared. The
// counts are captured matches, matches Glade reports, exact confidences and
// matches native does not report. Matches and exact confidences are floors;
// false matches are a ceiling.
func TestCompanyShapePanelCounts(t *testing.T) {
	want := map[string][4]int{
		"R1_dbl":  {65, 41, 41, 0},
		"R1_dir":  {190, 157, 61, 4},
		"R1_num":  {159, 152, 81, 30},
		"R1_dig":  {371, 371, 213, 2},
		"R1_join": {55, 55, 39, 2},
		"R1_qf2":  {65, 56, 40, 2},
		"R2_vow":  {240, 148, 45, 0},
		"R2_let":  {529, 526, 340, 0},
		"R2_glu":  {325, 325, 167, 0},
		"R2_par":  {352, 273, 109, 0},
	}
	panels := companyFitPanels(t, "R1_", "R2_")
	if len(panels) != len(want) {
		t.Fatalf("%d panels, want %d", len(panels), len(want))
	}
	for name, panel := range panels {
		hits, exact, extra := 0, 0, 0
		for i := range panel.values {
			for j := range panel.values {
				got, score := fuzzyCompany(panel.values[i], panel.values[j], false)
				native, matched := panel.native[[2]int{i, j}]
				switch {
				case got && matched:
					hits++
					if math.Round(score) == native {
						exact++
					}
				case got:
					extra++
				}
			}
		}
		w := want[name]
		t.Logf("%s: native %d, matched %d, exact confidence %d, not native %d", name, len(panel.native), hits, exact, extra)
		if len(panel.native) != w[0] || hits < w[1] || exact < w[2] || extra > w[3] {
			t.Errorf("%s: native %d matched %d exact %d not native %d, want %d, at least %d, at least %d, at most %d", name, len(panel.native), hits, exact, extra, w[0], w[1], w[2], w[3])
		}
	}
}

// Each row is one directed pair of an R1 or R2 panel, candidate first. The
// decision is checked against the captured panel and against Glade; a
// confidence is checked where Glade reproduces it.
func TestCompanyShapePanelRows(t *testing.T) {
	panels := companyFitPanels(t, "R1_", "R2_")
	for _, c := range []struct {
		panel, candidate, stored string
		match                    bool
		confidence               float64
	}{
		{"R1_dir", "Orbel Trade/West", "Orbel Trade/East", false, 0},
		{"R1_dir", "Orbel Trade West", "Orbel Trade/West", false, 0},
		{"R1_dir", "Orbel Trade/West", "Orbel Trade - West", false, 0},
		{"R1_qf2", "Orbel Trade/Midwest", "Orbel Trade/Mideast", true, 100},
		{"R1_qf2", "Orbel/West", "Orbel/East", false, 0},
		{"R1_qf2", "Orbel Trades/West", "Orbel Trade/West", false, 0},
		{"R1_qf2", "Orbel Trade (West)", "Orbel Trade c/o West", true, 0},
		{"R1_dbl", "Vexlett Packard", "Vexlet Packard", true, 100},
		{"R1_dbl", "Vexlet Packard", "Vexlettt Packard", true, 100},
		{"R1_dbl", "Vexlett Packard", "Vexllet Packard", false, 0},
		{"R1_dbl", "Vexlet Packard", "Vvexlet Packard", false, 0},
		{"R1_dbl", "Qell Systems", "Qel Systems", false, 0},
		{"R1_dbl", "Zorvex Marine Supply", "Zorvvex Marine Supply", false, 0},
		{"R1_dir", "Orbel Trade - West", "Orbel Trade / Best", true, 91},
		{"R1_dir", "Orbel Trade - Best", "Orbel Trade / West", true, 91},
		{"R1_dir", "Orbel Trade - Best", "Orbel Trade / East", false, 0},
		{"R1_dir", "Orbel Trade - East", "Orbel Trade / West", true, 0},
		{"R1_dir", "Orbel Trade - West", "Orbel Trade / East", false, 0},
		{"R1_dir", "Orbel Trade - South", "Orbel Trade / West", false, 0},
		{"R1_dir", "Orbel Trade - East", "Orbel Trade (West)", false, 0},
		{"R1_dir", "Orbel Trade (Best)", "Orbel Trade - West", false, 0},
		{"R1_dir", "Orbel Trade - Wast", "Orbel Trade (West)", true, 0},
		{"R2_vow", "Orbel Trade - East", "Orbel Trade / Oast", true, 0},
		{"R2_vow", "Orbel Trade - East", "Orbel Trade / Ease", false, 0},
		{"R2_vow", "Orbel Trade - Mast", "Orbel Trade / Oast", false, 0},
		{"R2_vow", "Orbel Trade - Mast", "Orbel Trade / Last", true, 91},
		{"R2_vow", "Orbel Trade - West", "Orbel Trade / Mast", false, 0},
		{"R2_par", "Zorvex Freight (East)", "Zorvex Freight / West", true, 0},
		{"R2_par", "Zorvex Freight / West", "Zorvex Freight (Wast)", true, 0},
		{"R2_par", "Zorvex Freight - West", "Zorvex Freight (Wast)", true, 0},
		{"R2_par", "Zorvex Freight - Best", "Zorvex Freight (West)", false, 0},
		{"R1_dig", "Test Account12", "Test Account#2", true, 0},
		{"R1_dig", "Test Account#1", "Test Account01", true, 0},
		{"R1_num", "Qorvan Depot 4 North", "Qorvan Depot 3 North East", true, 93},
		{"R1_num", "Qorvan Depot North", "Qorvan Depot No 3", true, 0},
		{"R1_num", "Qorvan Depot", "Qorvan Depot 3A", true, 96},
		{"R1_num", "Qorvan Depot 3", "Qorvan Depot 4 North", false, 0},
		{"R2_vow", "Zorvex Region 1 Oast", "Zorvex Region 2 Mast", true, 0},
		{"R2_vow", "Zorvex Region 2 Mast", "Zorvex Region 1 Oast", false, 0},
		{"R2_vow", "Zorvex Region1", "Zorvex Region 1 Oast", true, 0},
		{"R2_vow", "Zorvex Region1", "Zorvex Region 1 Mast", false, 0},
		{"R2_let", "Test Account-1", "Test Account II", true, 0},
		{"R2_let", "Test Account 1", "Test Account II", true, 97},
		{"R1_dig", "Test Account11", "Test Account22", true, 97},
		{"R1_dig", "Test Account12", "Test Account13", true, 100},
		{"R2_glu", "Test Account123", "Test Account124", true, 100},
		{"R2_glu", "Test Account124", "Test Account133", true, 0},
		{"R2_let", "Test Account_2", "Test Account D", true, 91},
		{"R2_let", "Test Account K", "Test Account-1", true, 91},
		{"R1_dig", "Test Account I", "Test Account_2", true, 94},
		{"R2_let", "Test Account_2", "Test Account E", true, 0},
		{"R1_join", "Tri-Zorv Paving", "Tri Zorv Painting", false, 0},
		{"R1_join", "Qell_Atlantic", "Qell Atlantic", false, 0},
	} {
		panel := panels[c.panel]
		i, j := -1, -1
		for k, v := range panel.values {
			if v == c.candidate {
				i = k
			}
			if v == c.stored {
				j = k
			}
		}
		if i < 0 || j < 0 {
			t.Fatalf("%s: %q or %q is not in the panel", c.panel, c.candidate, c.stored)
		}
		native, matched := panel.native[[2]int{i, j}]
		if matched != c.match || c.confidence != 0 && native != c.confidence {
			t.Fatalf("%s %q -> %q: row says %v %v, capture says %v %v", c.panel, c.candidate, c.stored, c.match, c.confidence, matched, native)
		}
		got, score := fuzzyCompany(c.candidate, c.stored, false)
		if got != c.match || c.confidence != 0 && math.Round(score) != c.confidence {
			t.Errorf("%s %q -> %q: got %v %v, want %v %v", c.panel, c.candidate, c.stored, got, score, c.match, c.confidence)
		}
	}
}
