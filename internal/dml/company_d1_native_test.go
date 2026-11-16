package dml

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// companyD1Residuals are the captured D1 directed pairs the fitted scorer does
// not reproduce, keyed "panel/candidate=>stored". "miss" is a native match
// Glade does not report; the word scorer rejects it too. "word" is a pair the
// word scorer matches and native does not ("Zorvex Partner" and "Zorvex
// Partner Inc", "Zorvex Freight West" and "Zorvex Freight (West Coast)"); it
// is the base decision, not a fitted match. No residual is a match the fit
// adds: the test fails on any pair Glade matches that native and the word
// scorer do not.
var companyD1Residuals = map[string]string{
	"D1_dp/Qelvun Partner Group=>Qelvun Partner":             "word",
	"D1_dp/Qelvun Partner=>Qelvun Partner Group":             "word",
	"D1_dp/Zorvex Inc=>Zorvex Partnership":                   "miss",
	"D1_dp/Zorvex Partner Inc=>Zorvex Partner":               "word",
	"D1_dp/Zorvex Partner Inc=>Zorvex Partner LLC":           "word",
	"D1_dp/Zorvex Partner LLC=>Zorvex Partner":               "word",
	"D1_dp/Zorvex Partner LLC=>Zorvex Partner Inc":           "word",
	"D1_dp/Zorvex Partner=>Zorvex Partner Inc":               "word",
	"D1_dp/Zorvex Partner=>Zorvex Partner LLC":               "word",
	"D1_dp/Zorvex Partner=>Zorvex Partnership":               "miss",
	"D1_dp/Zorvex Partnership=>Zorvex":                       "miss",
	"D1_dp/Zorvex Partnership=>Zorvex Inc":                   "miss",
	"D1_dp/Zorvex Partnership=>Zorvex Partner":               "miss",
	"D1_dp/Zorvex=>Zorvex Partnership":                       "miss",
	"D1_jn/Zorvex Marine Supplies=>Zorvex-Marine Supply":     "miss",
	"D1_nd/Zorvex Region 1 East=>Zorvex Region 2 West":       "miss",
	"D1_nd/Zorvex Region 1 Sales=>Zorvex Region 10":          "miss",
	"D1_nd/Zorvex Region 1 West=>Zorvex Region 10":           "miss",
	"D1_nd/Zorvex Region 11 Sales=>Zorvex Region 1":          "miss",
	"D1_nd/Zorvex Region 2 East=>Zorvex Region 1 West":       "miss",
	"D1_nd/Zorvex Region1=>Zorvex Region 1 East":             "miss",
	"D1_nd/Zorvex Region2=>Zorvex Region 2 East":             "miss",
	"D1_qf/Zorvex Freight (West Coast)=>Zorvex Freight West": "word",
	"D1_qf/Zorvex Freight - East=>Zorvex Freight West":       "miss",
	"D1_qf/Zorvex Freight - West=>Zorvex Freights/West":      "miss",
	"D1_qf/Zorvex Freight East=>Zorvex Freight - West":       "miss",
	"D1_qf/Zorvex Freight East=>Zorvex Freight / West":       "miss",
	"D1_qf/Zorvex Freight West=>Zorvex Freight (West Coast)": "word",
}

// NATIVE company_name_fit/org.tsv D1_* (custom CompanyName rule, API 62.0 and
// 67.0, anonymous and @IsTest, two repeats; all observations agree). Every
// ordered pair of every panel is compared for its decision and confidence.
func TestCompanyNameD1Conformance(t *testing.T) {
	const dir = "testdata/company_name_fit/"
	b, err := os.ReadFile(dir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string]string
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(dir + "org.tsv")
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
		if !strings.HasPrefix(row[4], "D1_") {
			continue
		}
		if prior, ok := observed[row[4]]; ok && prior != row[5] {
			t.Fatalf("%s: observations disagree across api, route or repeat", row[4])
		}
		observed[row[4]] = row[5]
		repeats[row[4]]++
	}
	if len(observed) != 5 {
		t.Fatalf("%d D1 panels, want 5", len(observed))
	}
	used := map[string]bool{}
	// Exact confidences must not fall below the counts this fit reaches.
	exactFloor := map[string]int{"D1_nd": 87, "D1_nd2": 71, "D1_jn": 42, "D1_qf": 165, "D1_dp": 44}
	names := make([]string, 0, len(observed))
	for name := range observed {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if repeats[name] != 8 {
			t.Fatalf("%s: %d observations, want 8", name, repeats[name])
		}
		src := cases[name]
		list := src[strings.Index(src, "new List<String>{")+len("new List<String>{"):]
		list = list[:strings.IndexByte(list, '}')]
		var values []string
		for _, m := range quoted.FindAllStringSubmatch(list, -1) {
			values = append(values, m[1])
		}
		_, body, _ := strings.Cut(observed[name], ":")
		native := map[[2]int]float64{}
		for _, entry := range strings.Split(body, ",") {
			i, e1 := strconv.ParseInt(entry[:1], 36, 0)
			j, e2 := strconv.ParseInt(entry[1:2], 36, 0)
			if e1 != nil || e2 != nil || int(i) >= len(values) || int(j) >= len(values) {
				t.Fatalf("%s: malformed entry %q", name, entry)
			}
			score := 100.0
			if len(entry) > 2 {
				if score, err = strconv.ParseFloat(entry[2:], 64); err != nil {
					t.Fatal(err)
				}
			}
			native[[2]int{int(i), int(j)}] = score
		}
		hits, exact, misses, words := 0, 0, 0, 0
		for i := range values {
			for j := range values {
				got, score := fuzzyCompany(values[i], values[j], false)
				nativeScore, want := native[[2]int{i, j}]
				key := fmt.Sprintf("%s/%s=>%s", name, values[i], values[j])
				switch {
				case got && want:
					hits++
					if math.Round(score) == nativeScore {
						exact++
					}
				case want:
					misses++
					if residual := companyD1Residuals[key]; residual != "miss" {
						t.Errorf("%s: native %v, Glade no match", key, nativeScore)
					}
					used[key] = true
				case got:
					if base, _ := legacyFuzzyCompany(values[i], values[j], false); !base {
						t.Errorf("%s: Glade matches at %.0f, native and the word scorer do not", key, score)
						continue
					}
					words++
					if companyD1Residuals[key] != "word" {
						t.Errorf("%s: unlisted word-scorer match", key)
					}
					used[key] = true
				}
			}
		}
		t.Logf("%s: native %d, matched %d, exact confidence %d, missed %d, word-scorer matches %d", name, len(native), hits, exact, misses, words)
		if exact < exactFloor[name] {
			t.Errorf("%s: exact confidence %d, want at least %d", name, exact, exactFloor[name])
		}
	}
	for key := range companyD1Residuals {
		if !used[key] {
			t.Errorf("unused residual %s", key)
		}
	}
}
