package dml

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const fuzzyNameKeysDir = "testdata/conformance/fuzzy_name_keys/"

type fuzzyNameKeysCase struct {
	Kind   string     `json:"kind"`
	Object string     `json:"object"`
	Rule   string     `json:"rule"`
	Fields []string   `json:"fields"`
	Item   string     `json:"item"`
	Values []string   `json:"values"`
	Ops    []string   `json:"ops"`
	Pairs  [][]string `json:"pairs"`
}

// fuzzyNameKeysResiduals are the native pairs the scorers do not reproduce
// yet, keyed "case/candidate=>existing" with the native and Glade confidence
// (0 for no match). Every one reaches the Keyboard Distance step with two
// substitutions or unequal lengths ("Test 1" and "Last 1", "Harlow Jr" and
// "Harlowe"), whose costs these rows do not settle. None is a Glade match
// that native does not report.
var fuzzyNameKeysResiduals = map[string][2]float64{
	"ln1/Test 1=>Last 1":          {96, 97},
	"ln1/Last 1=>Test 1":          {96, 97},
	"suf1/Harlow Jr=>Harlow III":  {97, 0},
	"suf1/Harlow Jr=>Harlow.":     {97, 0},
	"suf1/Harlow Jr=>Harlowe":     {96, 0},
	"suf1/Harlow Jr.=>Harlow Sr":  {96, 0},
	"suf1/Harlow Jr.=>Harlow II":  {96, 0},
	"suf1/Harlow Jr.=>Harlow III": {97, 0},
	"suf1/Harlow Jr.=>Harlow IV":  {97, 0},
	"suf1/Harlow Jr.=>Harlow.":    {95, 0},
	"suf1/Harlow, Jr.=>Harlow Sr": {95, 0},
	"suf1/Harlow, Jr.=>Harlow II": {96, 0},
	"suf1/Harlow Sr=>Harlow Jr.":  {96, 0},
	"suf1/Harlow Sr=>Harlow, Jr.": {95, 0},
	"suf1/Harlow Sr=>Harlow.":     {97, 0},
	"suf1/Harlow Sr=>Harlowe":     {96, 0},
	"suf1/Harlow II=>Harlow Jr.":  {96, 0},
	"suf1/Harlow II=>Harlow, Jr.": {96, 0},
	"suf1/Harlow II=>Harlow IV":   {97, 98},
	"suf1/Harlow III=>Harlow Jr":  {97, 0},
	"suf1/Harlow III=>Harlow Jr.": {97, 0},
	"suf1/Harlow III=>Harlow IV":  {97, 0},
	"suf1/Harlow IV=>Harlow Jr.":  {97, 0},
	"suf1/Harlow IV=>Harlow II":   {97, 98},
	"suf1/Harlow IV=>Harlow III":  {97, 0},
	"suf1/Harlow IV=>Harlow.":     {97, 0},
	"suf1/Harlow IV=>Harlowe":     {96, 0},
	"suf1/Harlow.=>Harlow Jr":     {97, 0},
	"suf1/Harlow.=>Harlow Jr.":    {95, 0},
	"suf1/Harlow.=>Harlow Sr":     {97, 0},
	"suf1/Harlow.=>Harlow IV":     {97, 0},
	"suf1/Harlowe=>Harlow Jr":     {96, 0},
	"suf1/Harlowe=>Harlow Sr":     {96, 0},
	"suf1/Harlowe=>Harlow IV":     {96, 0},
}

func fuzzyNameKeysData(t *testing.T) (map[string]fuzzyNameKeysCase, map[string]string) {
	t.Helper()
	raw, err := os.ReadFile(fuzzyNameKeysDir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]fuzzyNameKeysCase{}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(fuzzyNameKeysDir + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma, r.LazyQuotes = '\t', true
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(rows[0], ",") != "api,route,repeat,group,method,observation" {
		t.Fatalf("header %v", rows[0])
	}
	observations := map[string]string{}
	seen := map[string]bool{}
	count := map[string]int{}
	for _, row := range rows[1:] {
		key := strings.Join(row[:5], "/")
		if seen[key] || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "istest") || (row[2] != "1" && row[2] != "2") {
			t.Fatalf("invalid or duplicate observation %s", key)
		}
		seen[key] = true
		method := row[4]
		if _, ok := cases[method]; !ok {
			t.Fatalf("observation without case %s", method)
		}
		// Every API, route and repeat agrees; assert the shared observation.
		if prior, ok := observations[method]; ok && prior != row[5] {
			t.Fatalf("%s: observations disagree", key)
		}
		observations[method] = row[5]
		count[method]++
	}
	for method := range cases {
		if count[method] != 8 {
			t.Fatalf("%s: %d observations, want 8", method, count[method])
		}
	}
	return cases, observations
}

func fuzzyNameKeysRule(c fuzzyNameKeysCase, action string) storage.DuplicateRule {
	return storage.DuplicateRule{
		Name: c.Rule, Object: c.Object, Active: true, ActionOnInsert: action, ActionOnUpdate: action, OperationsOnInsert: []string{"Alert"}, OperationsOnUpdate: []string{"Alert"},
		MatchRules: []storage.MatchingRule{{Object: c.Object, Name: c.Rule, Status: "Active", Items: []storage.MatchingRuleItem{{Field: c.Item, Method: c.Item, BlankBehavior: "NullNotAllowed"}}}},
	}
}

func fuzzyNameKeysRecord(c fuzzyNameKeysCase, value string) storage.Record {
	record := storage.Record{Object: c.Object, Fields: map[string]storage.Value{}}
	parts := strings.Split(value, "|")
	if len(parts) != len(c.Fields) {
		panic(value)
	}
	for i, field := range c.Fields {
		record.Fields[field] = storage.StringValue(parts[i])
	}
	return record
}

func fuzzyNameKeysOrg(c fuzzyNameKeysCase, action string, values ...string) (*storage.OrgState, []storage.ID) {
	org := &storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(org, c.Object)
	state := org.Objects[c.Object]
	state.Definition.DuplicateRules = append(state.Definition.DuplicateRules, fuzzyNameKeysRule(c, action))
	if state.Records == nil {
		state.Records = map[storage.ID]storage.Record{}
	}
	var ids []storage.ID
	for i, value := range values {
		record := fuzzyNameKeysRecord(c, value)
		record.ID = storage.ID(fmt.Sprintf("003%012d", i+1))
		state.Records[record.ID] = record
		ids = append(ids, record.ID)
	}
	org.Objects[c.Object] = state
	return org, ids
}

const fuzzyNameKeysDigits = "0123456789abcdefghijklmnopqrstuvwxyz"

// fuzzyNameKeysPanel parses "<count>:<pairs>"; a pair is the candidate
// index, the existing index and an optional confidence (100 when absent).
func fuzzyNameKeysPanel(t *testing.T, observation string, values int) map[[2]int]float64 {
	t.Helper()
	head, body, _ := strings.Cut(observation, ":")
	if n, err := strconv.Atoi(head); err != nil || n != values {
		t.Fatalf("panel count %q, want %d", head, values)
	}
	out := map[[2]int]float64{}
	for _, token := range strings.Split(body, ",") {
		if token == "" {
			continue
		}
		c, e := strings.IndexByte(fuzzyNameKeysDigits, token[0]), strings.IndexByte(fuzzyNameKeysDigits, token[1])
		if c < 0 || e < 0 || c >= values || e >= values {
			t.Fatalf("panel token %q", token)
		}
		confidence := 100.0
		if len(token) > 2 {
			var err error
			if confidence, err = strconv.ParseFloat(token[2:], 64); err != nil {
				t.Fatalf("panel token %q", token)
			}
		}
		out[[2]int{c, e}] = confidence
	}
	return out
}

type fuzzyNameKeysCount struct{ native, hit, miss, extra, confidence int }

func (c fuzzyNameKeysCount) String() string {
	return fmt.Sprintf("native=%d hit=%d miss=%d extra=%d conf_diff=%d", c.native, c.hit, c.miss, c.extra, c.confidence)
}

// NATIVE fuzzy_name_keys/org.tsv: custom FirstName and LastName rules at API
// 62.0 and 67.0, anonymous and @IsTest, two repeats. Every ordered pair of
// every panel, and every op of every pair case, goes through the shared
// duplicate-rule matcher and the DML engine.
func TestFuzzyNameKeysNativeRows(t *testing.T) {
	cases, observations := fuzzyNameKeysData(t)
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)
	used := map[string]bool{}
	check := func(t *testing.T, key string, native, got float64, counts *fuzzyNameKeysCount) {
		t.Helper()
		if native == 0 && got == 0 {
			return
		}
		if native > 0 {
			counts.native++
		}
		switch {
		case native > 0 && got > 0 && native == got:
			counts.hit++
			return
		case native > 0 && got > 0:
			counts.hit++
			counts.confidence++
		case native > 0:
			counts.miss++
		default:
			counts.extra++
		}
		if residual, ok := fuzzyNameKeysResiduals[key]; ok && residual == [2]float64{native, got} && native > 0 {
			used[key] = true
			return
		}
		t.Errorf("%s: native=%v glade=%v", key, native, got)
	}
	for _, name := range names {
		c := cases[name]
		t.Run(name, func(t *testing.T) {
			var counts fuzzyNameKeysCount
			switch c.Kind {
			case "panel":
				native := fuzzyNameKeysPanel(t, observations[name], len(c.Values))
				org, ids := fuzzyNameKeysOrg(c, "Allow", c.Values...)
				index := map[storage.ID]int{}
				for i, id := range ids {
					index[id] = i
				}
				rule := fuzzyNameKeysRule(c, "Allow")
				for i, value := range c.Values {
					got := map[int]float64{}
					result, supported := FindDuplicateMatches(org, rule, fuzzyNameKeysRecord(c, value), nil)
					if !supported {
						t.Fatal("rule not supported")
					}
					for _, match := range result.Matches {
						for k, record := range match.Records {
							got[index[record.ID]] = math.Floor(match.Confidences[k] + 0.5)
						}
					}
					for j := range c.Values {
						n, g := native[[2]int{i, j}], got[j]
						if n == 0 && g == 0 {
							continue
						}
						check(t, fmt.Sprintf("%s/%s=>%s", name, value, c.Values[j]), n, g, &counts)
					}
				}
			case "pair":
				tokens := strings.Split(observations[name], "|")
				byKey := map[string]string{}
				for _, token := range tokens {
					if strings.HasPrefix(token, "ctl=") {
						continue
					}
					parts := strings.SplitN(token, ":", 3)
					if len(parts) != 3 {
						t.Fatalf("pair token %q", token)
					}
					byKey[parts[0]+":"+parts[1]] = parts[2]
				}
				for i, pair := range c.Pairs {
					stored, candidate := pair[0], pair[1]
					key := fmt.Sprintf("%s/%s=>%s", name, candidate, stored)
					org, _ := fuzzyNameKeysOrg(c, "Allow", stored)
					result, _ := FindDuplicateMatches(org, fuzzyNameKeysRule(c, "Allow"), fuzzyNameKeysRecord(c, candidate), nil)
					got := 0.0
					if len(result.Matches) == 1 && len(result.Matches[0].Records) == 1 {
						got = math.Floor(result.Matches[0].Confidences[0] + 0.5)
					}
					for _, op := range c.Ops {
						entry, ok := byKey[fmt.Sprintf("%d:%s", i, op)]
						if !ok {
							t.Fatalf("%s: no %s observation", key, op)
						}
						find, dml, _ := strings.Cut(entry, ";dml=")
						if op == "bulk" {
							// One statement never matches within its own batch.
							if find != "find=-" || dml != "0:S+1:S" {
								t.Fatalf("%s bulk: %s", key, entry)
							}
							if gotDML := fuzzyNameKeysDML(t, c, op, stored, candidate); gotDML != dml {
								t.Errorf("%s %s: native dml=%s glade dml=%s", key, op, dml, gotDML)
							}
							continue
						}
						native := 0.0
						if find != "find=0@" {
							count, confidence, _ := strings.Cut(strings.TrimPrefix(find, "find="), "@")
							var err error
							if native, err = strconv.ParseFloat(confidence, 64); count != "1" || err != nil {
								t.Fatalf("%s %s: %s", key, op, entry)
							}
						}
						if op == c.Ops[0] {
							check(t, key, native, got, &counts)
						}
						if native != got {
							continue // the find residual above covers the save outcome
						}
						if gotDML := fuzzyNameKeysDML(t, c, op, stored, candidate); gotDML != fuzzyNameKeysNativeDML(dml) {
							t.Errorf("%s %s: native dml=%s glade dml=%s", key, op, dml, gotDML)
						}
					}
				}
			default:
				t.Fatalf("kind %q", c.Kind)
			}
			t.Logf("PANEL %s %s", name, counts)
		})
	}
	for key := range fuzzyNameKeysResiduals {
		if !used[key] {
			t.Errorf("unused residual %s", key)
		}
	}
}

// fuzzyNameKeysNativeDML drops the confidence formatting difference between
// the two captures ("96" and "96.0") and maps a thrown statement to its code.
func fuzzyNameKeysNativeDML(dml string) string {
	if strings.HasPrefix(dml, "T/") {
		return "E/" + strings.TrimPrefix(dml, "T/")
	}
	if strings.HasPrefix(dml, "D/") {
		head, confidence, _ := strings.Cut(dml, "@")
		if value, err := strconv.ParseFloat(confidence, 64); err == nil {
			return fmt.Sprintf("%s@%g", head, value)
		}
	}
	return dml
}

// fuzzyNameKeysDML runs one op of a pair case through the DML engine with
// the rule at Alert: A is stored, then B is saved by the op.
func fuzzyNameKeysDML(t *testing.T, c fuzzyNameKeysCase, op, stored, candidate string) string {
	t.Helper()
	org, _ := fuzzyNameKeysOrg(c, "Allow", stored)
	engine := NewEngine(org)
	engine.Options.EnforceDuplicateRules = true
	var results []Result
	switch op {
	case "ins", "stmt":
		results = engine.Insert([]storage.Record{fuzzyNameKeysRecord(c, candidate)})
	case "allow":
		engine.Options.AllowDuplicateSave = true
		results = engine.Insert([]storage.Record{fuzzyNameKeysRecord(c, candidate)})
	case "ups":
		results = engine.Upsert([]storage.Record{fuzzyNameKeysRecord(c, candidate)})
	case "upd":
		// A second stored record, unrelated to both values, is updated to B.
		other := fuzzyNameKeysRecord(c, strings.Repeat("Qvxjz|", len(c.Fields)-1)+"Qvxjz")
		other.ID = storage.ID("003999999999999")
		state := org.Objects[c.Object]
		state.Records[other.ID] = other
		org.Objects[c.Object] = state
		updated := fuzzyNameKeysRecord(c, candidate)
		updated.ID = other.ID
		results = engine.Update([]storage.Record{updated})
	case "bulk":
		// Both records in one statement; neither is stored first.
		org, _ = fuzzyNameKeysOrg(c, "Allow")
		engine = NewEngine(org)
		engine.Options.EnforceDuplicateRules = true
		results = engine.Insert([]storage.Record{fuzzyNameKeysRecord(c, stored), fuzzyNameKeysRecord(c, candidate)})
		var parts []string
		for i, r := range results {
			parts = append(parts, fmt.Sprintf("%d:%s", i, fuzzyNameKeysOutcome(r)))
		}
		return strings.Join(parts, "+")
	default:
		t.Fatalf("op %q", op)
	}
	if len(results) != 1 {
		t.Fatalf("%s: %d results", op, len(results))
	}
	if op == "stmt" && !results[0].Success {
		return "E/" + results[0].StatusCode
	}
	return fuzzyNameKeysOutcome(results[0])
}

func fuzzyNameKeysOutcome(r Result) string {
	if r.Success {
		return "S"
	}
	for _, e := range r.Errors {
		if d := e.Duplicate; d != nil && len(d.Matches) == 1 && len(d.Matches[0].Records) == 1 {
			return fmt.Sprintf("D/%s/%v/1@%g", d.Rule, d.AllowSave, math.Floor(d.Matches[0].Confidences[0]+0.5))
		}
	}
	return "E/" + r.StatusCode
}

// An insert loop through a custom FirstName OR LastName rule: 50 candidates
// scanned against 2,000 existing contacts.
func BenchmarkCustomNameRuleInsertLoop(b *testing.B) {
	org := benchOrg("Contact", 2000)
	rule := storage.DuplicateRule{Name: "Custom", Object: "Contact", Active: true, MatchRules: []storage.MatchingRule{{Name: "Names", Object: "Contact", Status: "Active", BooleanFilter: "1 OR 2", Items: []storage.MatchingRuleItem{
		{Field: "FirstName", Method: "FirstName", BlankBehavior: "NullNotAllowed"}, {Field: "LastName", Method: "LastName", BlankBehavior: "NullNotAllowed"}}}}}
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		for i := 0; i < 50; i++ {
			c := storage.Record{Object: "Contact", Fields: map[string]storage.Value{
				"FirstName": storage.StringValue(fmt.Sprint(benchFirst[i%10], " ", i%3)), "LastName": storage.StringValue(benchSurname(i*7 + 3))}}
			FindDuplicateMatches(org, rule, c, nil)
		}
	}
}

// Local contract: the standard rules keep the whole-name first and last name
// keys (standardFirstName, standardLastName). The first-word key, Initials
// and the last-word key are captured for custom rules only.
func TestFuzzyNameKeysStandardPathUnchanged(t *testing.T) {
	for _, c := range []struct {
		a, b           string
		custom, before bool
	}{
		{"P", "P W G", true, false},
		{"PW", "P W G", true, false},
		{"Y R S", "Y", true, false},
		{"Billy Bob", "Billy", true, true},
		{"Jane.", "Jane", true, true},
		{"Jon", "John", true, true},
	} {
		if got, _ := fuzzyFirstName(c.a, c.b, false); got != c.custom {
			t.Errorf("custom first name %q/%q: %v", c.a, c.b, got)
		}
		if got, _ := standardFirstName(c.a, c.b); got != c.before {
			t.Errorf("standard first name %q/%q: %v", c.a, c.b, got)
		}
	}
	for _, c := range []struct {
		a, b           string
		custom, before bool
	}{
		{"Harlow", "Harlow III", true, false},
		{"Al-Rashid", "Alrashid", true, false},
		{"Test 1", "Last 1", true, false},
		{"42", "43", false, true},
		{"Smith", "Smyth", true, true},
	} {
		if got, _ := fuzzyLastName(c.a, c.b, false); got != c.custom {
			t.Errorf("custom last name %q/%q: %v", c.a, c.b, got)
		}
		if got, _ := standardLastName(c.a, c.b); got != c.before {
			t.Errorf("standard last name %q/%q: %v", c.a, c.b, got)
		}
	}
}
