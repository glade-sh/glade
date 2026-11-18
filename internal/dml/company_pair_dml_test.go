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

	"github.com/glade-sh/glade/internal/storage"
)

const companyFitDir = "testdata/company_name_fit/"

type companyPairCase struct {
	rule  string
	ops   []string
	pairs [][2]string // stored, candidate
}

var (
	companyPairQuoted = regexp.MustCompile(`'([^']*)'`)
	companyPairList   = regexp.MustCompile(`new List<String>\{([^}]*)\}`)
)

// companyPairCases reads the FuzzyPair.run calls from cases.json. A call is
// FuzzyPair.run(object, rule, field, ops, pairs) where ops is a comma list or
// a list of strings and each pair is "stored=>candidate".
func companyPairCases(t *testing.T) map[string]companyPairCase {
	t.Helper()
	raw, err := os.ReadFile(companyFitDir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var sources map[string]string
	if err := json.Unmarshal(raw, &sources); err != nil {
		t.Fatal(err)
	}
	out := map[string]companyPairCase{}
	for name, src := range sources {
		if !strings.Contains(src, "FuzzyPair.run(") {
			continue
		}
		args := strings.SplitN(src, "FuzzyPair.run(", 2)[1]
		strs := companyPairQuoted.FindAllStringSubmatch(args[:strings.Index(args, "new List")], -1)
		if len(strs) < 3 {
			t.Fatalf("%s: malformed call", name)
		}
		c := companyPairCase{rule: strs[1][1]}
		lists := companyPairList.FindAllStringSubmatch(args, -1)
		var pairList string
		if len(strs) > 3 {
			c.ops = strings.Split(strs[3][1], ",")
			pairList = lists[0][1]
		} else {
			for _, m := range companyPairQuoted.FindAllStringSubmatch(lists[0][1], -1) {
				c.ops = append(c.ops, m[1])
			}
			pairList = lists[1][1]
		}
		for _, m := range companyPairQuoted.FindAllStringSubmatch(pairList, -1) {
			stored, candidate, ok := strings.Cut(m[1], "=>")
			if !ok {
				t.Fatalf("%s: malformed pair %q", name, m[1])
			}
			c.pairs = append(c.pairs, [2]string{stored, candidate})
		}
		out[name] = c
	}
	return out
}

// companyPairObservations returns each method's DML observation. Every API,
// route and repeat must agree.
func companyPairObservations(t *testing.T, methods map[string]companyPairCase) map[string]string {
	t.Helper()
	f, err := os.Open(companyFitDir + "org.tsv")
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
	out, count := map[string]string{}, map[string]int{}
	for _, row := range rows[1:] {
		if _, ok := methods[row[4]]; !ok {
			continue
		}
		if prior, ok := out[row[4]]; ok && prior != row[5] {
			t.Fatalf("%s: observations disagree across api, route or repeat", row[4])
		}
		out[row[4]] = row[5]
		count[row[4]]++
	}
	for name := range methods {
		if count[name] != 8 {
			t.Fatalf("%s: %d observations, want 8", name, count[name])
		}
	}
	return out
}

func companyPairRule(rule, action string) storage.DuplicateRule {
	return storage.DuplicateRule{
		Name: rule, Object: "Account", Active: true, ActionOnInsert: action, ActionOnUpdate: action,
		OperationsOnInsert: []string{"Alert"}, OperationsOnUpdate: []string{"Alert"},
		MatchRules: []storage.MatchingRule{{Object: "Account", Name: rule, Status: "Active", Items: []storage.MatchingRuleItem{{Field: "Name", Method: "CompanyName", BlankBehavior: "NullNotAllowed"}}}},
	}
}

func companyPairRecord(name string) storage.Record {
	return storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(name)}}
}

func companyPairOrg(rule string, stored ...string) *storage.OrgState {
	org := &storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(org, "Account")
	state := org.Objects["Account"]
	state.Definition.DuplicateRules = append(state.Definition.DuplicateRules, companyPairRule(rule, "Allow"))
	if state.Records == nil {
		state.Records = map[storage.ID]storage.Record{}
	}
	for i, name := range stored {
		record := companyPairRecord(name)
		record.ID = storage.ID(fmt.Sprintf("001%012d", i+1))
		state.Records[record.ID] = record
	}
	org.Objects["Account"] = state
	return org
}

func companyPairOutcome(r Result) string {
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

// companyPairDML runs one op of a pair through the DML engine: the stored name
// exists, the candidate is saved by the op.
func companyPairDML(t *testing.T, rule, op, stored, candidate string) string {
	t.Helper()
	org := companyPairOrg(rule, stored)
	engine := NewEngine(org)
	engine.Options.EnforceDuplicateRules = true
	var results []Result
	switch op {
	case "ins", "stmt":
		results = engine.Insert([]storage.Record{companyPairRecord(candidate)})
	case "allow":
		engine.Options.AllowDuplicateSave = true
		results = engine.Insert([]storage.Record{companyPairRecord(candidate)})
	case "ups":
		results = engine.Upsert([]storage.Record{companyPairRecord(candidate)})
	case "upd":
		// A second stored record, unrelated to both names, is updated to the candidate.
		other := companyPairRecord("Qvxjz")
		other.ID = storage.ID("001999999999999")
		state := org.Objects["Account"]
		state.Records[other.ID] = other
		org.Objects["Account"] = state
		updated := companyPairRecord(candidate)
		updated.ID = other.ID
		results = engine.Update([]storage.Record{updated})
	case "bulk":
		org = companyPairOrg(rule)
		engine = NewEngine(org)
		engine.Options.EnforceDuplicateRules = true
		results = engine.Insert([]storage.Record{companyPairRecord(stored), companyPairRecord(candidate)})
		var parts []string
		for i, r := range results {
			parts = append(parts, fmt.Sprintf("%d:%s", i, companyPairOutcome(r)))
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
	return companyPairOutcome(results[0])
}

// companyPairResiduals are the captured find confidences the fitted scorer does
// not reproduce, keyed "method/stored=>candidate" with native and Glade value.
var companyPairResiduals = map[string][2]float64{}

// NATIVE company_name_fit/org.tsv (B1_dco_*, B3_dco3_*): custom CompanyName
// rules at API 62.0 and 67.0, anonymous and @IsTest, two repeats. Every
// stored-to-candidate pair goes through the shared duplicate-rule matcher for
// its find result, and through the DML engine for each op's save outcome.
func TestCompanyFitPairDMLRows(t *testing.T) {
	cases := companyPairCases(t)
	if len(cases) != 27 {
		t.Fatalf("%d FuzzyPair methods, want 27", len(cases))
	}
	observations := companyPairObservations(t, cases)
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)
	used := map[string]bool{}
	finds, saves := 0, 0
	for _, name := range names {
		c := cases[name]
		byKey := map[string]string{}
		for _, token := range strings.Split(observations[name], "|") {
			if strings.HasPrefix(token, "ctl=") {
				if token != "ctl=ok;neg=0@;pos=1@100" {
					t.Errorf("%s: control %q", name, token)
				}
				continue
			}
			parts := strings.SplitN(token, ":", 3)
			if len(parts) != 3 {
				t.Fatalf("%s: token %q", name, token)
			}
			byKey[parts[0]+":"+parts[1]] = parts[2]
		}
		for i, pair := range c.pairs {
			stored, candidate := pair[0], pair[1]
			key := fmt.Sprintf("%s/%s=>%s", name, stored, candidate)
			org := companyPairOrg(c.rule, stored)
			result, _ := FindDuplicateMatches(org, companyPairRule(c.rule, "Allow"), companyPairRecord(candidate), nil)
			got := 0.0
			if len(result.Matches) == 1 && len(result.Matches[0].Records) == 1 {
				got = math.Floor(result.Matches[0].Confidences[0] + 0.5)
			}
			for _, op := range c.ops {
				entry, ok := byKey[fmt.Sprintf("%d:%s", i, op)]
				if !ok {
					t.Fatalf("%s: no %s observation", key, op)
				}
				find, dml, _ := strings.Cut(entry, ";dml=")
				if op == "bulk" {
					// One statement never matches within its own batch. B1
					// reports "0:S+1:S" and no find; B3 reports "S;a=S", the
					// two saves, after an ordinary find.
					if dml == "S;a=S" {
						dml = "0:S+1:S"
					} else if find != "find=-" {
						t.Fatalf("%s bulk: %s", key, entry)
					}
					if dml != "0:S+1:S" {
						t.Fatalf("%s bulk: %s", key, entry)
					}
					if gotDML := companyPairDML(t, c.rule, op, stored, candidate); gotDML != dml {
						t.Errorf("%s bulk: native dml=%s glade dml=%s", key, dml, gotDML)
					}
					saves++
					if find == "find=-" {
						continue
					}
				}
				native := 0.0
				if find != "find=0@" {
					count, confidence, _ := strings.Cut(strings.TrimPrefix(find, "find="), "@")
					var err error
					if native, err = strconv.ParseFloat(confidence, 64); count != "1" || err != nil {
						t.Fatalf("%s %s: %s", key, op, entry)
					}
				}
				if op == c.ops[0] {
					finds++
					if native != got {
						if r, ok := companyPairResiduals[key]; ok && r == [2]float64{native, got} {
							used[key] = true
						} else {
							t.Errorf("%s: native find=%v glade=%v", key, native, got)
						}
					}
				}
				if native != got {
					continue
				}
				saves++
				if gotDML := companyPairDML(t, c.rule, op, stored, candidate); gotDML != fuzzyNameKeysNativeDML(dml) {
					t.Errorf("%s %s: native dml=%s glade dml=%s", key, op, dml, gotDML)
				}
			}
		}
	}
	for key := range companyPairResiduals {
		if !used[key] {
			t.Errorf("unused residual %s", key)
		}
	}
	// B1: 4+4+4 pairs; B3: one pair per method, two methods per pair.
	if finds != 36 {
		t.Errorf("checked %d find rows, want 36", finds)
	}
	t.Logf("find rows %d, save outcomes %d", finds, saves)
}
