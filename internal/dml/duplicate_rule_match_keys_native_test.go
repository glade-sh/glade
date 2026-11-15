package dml

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const ruleMatchKeysFixture = "../apextest/testdata/conformance/duplicate_rule_match_keys"

type ruleMatchKeysRule struct {
	Object string `json:"object"`
	Filter string `json:"filter"`
	Items  []struct {
		Field  string `json:"field"`
		Method string `json:"method"`
	} `json:"items"`
}

// ruleMatchKeysFind runs one stored record and one candidate through the
// shared duplicate-rule path and reports "1@<confidence>" or "0@".
func ruleMatchKeysFind(t *testing.T, name string, rule ruleMatchKeysRule, stored, candidate map[string]string) string {
	t.Helper()
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, rule.Object)
	storage.EnsureStandardObject(&org, "Account")
	state := org.Objects[rule.Object]
	if state.Records == nil {
		state.Records = map[storage.ID]storage.Record{}
	}
	id := storage.ID(standardObjectPrefix[rule.Object] + "000000000001")
	state.Records[id] = standardRowRecord(rule.Object, stored, id, nil)
	org.Objects[rule.Object] = state
	duplicate := storage.DuplicateRule{Name: "ProbeRule", Object: rule.Object, Active: true}
	if len(rule.Items) == 0 {
		duplicate.MatchRules = []storage.MatchingRule{{Object: rule.Object, Name: name, Status: "Active"}}
	} else {
		matching := storage.MatchingRule{Name: name, Object: rule.Object, Status: "Active", BooleanFilter: rule.Filter}
		for _, item := range rule.Items {
			matching.Items = append(matching.Items, storage.MatchingRuleItem{Field: item.Field, Method: item.Method, BlankBehavior: "NullNotAllowed"})
		}
		duplicate.MatchRules = []storage.MatchingRule{matching}
	}
	result, supported := FindDuplicateMatches(&org, duplicate, standardRowRecord(rule.Object, candidate, "", nil), nil)
	if !supported {
		t.Fatalf("%s: rule is not enforced", name)
	}
	got := "0@"
	for _, match := range result.Matches {
		for i := range match.Records {
			got = fmt.Sprintf("1@%.1f", math.Floor(match.Confidences[i]+0.5))
		}
	}
	return got
}

// ruleMatchKeysResiduals are the native rows the shared path does not yet
// reproduce, with the value Glade reports. None of them is a key: each is a
// scorer the key leaves to its owner.
var ruleMatchKeysResiduals = map[string]string{
	// The custom First Name method scores "Kristen" and "Cristen" 100 natively
	// (Jaro-Winkler 0.90 here).
	"C1.Kone.f3": "1@95.0", "C1.Ksix.f3": "1@98.0", "C1.Keight.f3": "1@99.0",
	// "Kate" and "Katie" score 100 natively behind the initial key; here
	// Jaro-Winkler 0.95.
	"C1.Kthree.f6": "1@99.0", "C1.Kfour.f6": "1@99.0", "C1.Kseven.f6": "1@99.0", "C1.Keight.f6": "1@99.0", "C1.Knine.f6": "1@99.0",
	// "Alex" and "Xander" are a Name Variant pair natively; the table lacks it.
	"C1.Keight.f4": "1@95.0", "B2.s_cmp_c.8": "0@", "B2.s_cmp_l.8": "0@",
	// Keyboard Distance of "Test 1" and "Last 1" (96 natively).
	"C1.Ktwo.l5b": "1@97.0", "C1.Kthree.l5b": "1@99.0", "C1.Kfive.l5b": "1@99.0",
}

// Native C1 and B2 stage 2 rows: the match key belongs to the rule. A custom
// rule's First Name item beside a Last Name item keys on the initial
// (First_Name (1,1) Last_Name), and the standard Contact and Lead email path,
// keyed on Email alone, scores both names without a key; the phone path keys
// on First_Name (1,1) Last_Name Phone. Every row must equal its native
// observation, or its pinned residual.
func TestDuplicateRuleMatchKeysNativeRows(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(ruleMatchKeysFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Rules map[string]ruleMatchKeysRule `json:"rules"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(ruleMatchKeysFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if lines[0] != "api\troute\trepeat\tid\tvalue" {
		t.Fatal("invalid native row header")
	}
	seen := map[string]bool{}
	got := map[string]string{}
	exact, total := 0, 0
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 5)
		if len(cells) != 5 {
			t.Fatalf("invalid native row: %q", line)
		}
		var row struct {
			Rule      string            `json:"rule"`
			Stored    map[string]string `json:"stored"`
			Candidate map[string]string `json:"candidate"`
			Find      string            `json:"find"`
		}
		if err := json.Unmarshal([]byte(cells[4]), &row); err != nil {
			t.Fatalf("%s: %v", cells[3], err)
		}
		id := cells[3]
		if _, ok := got[id]; !ok {
			rule, ok := cases.Rules[row.Rule]
			if !ok {
				t.Fatalf("%s: unknown rule %s", id, row.Rule)
			}
			got[id] = ruleMatchKeysFind(t, row.Rule, rule, row.Stored, row.Candidate)
		}
		total++
		want := row.Find
		if residual, ok := ruleMatchKeysResiduals[id]; ok {
			seen[id] = true
			if residual == row.Find {
				t.Errorf("%s: residual %s now equals the native row; remove it", id, residual)
			}
			want = residual
		} else if got[id] == row.Find {
			exact++
		}
		if got[id] != want {
			t.Errorf("%s/%s/%s %s: native %s, want %s, got %s", cells[0], cells[1], cells[2], id, row.Find, want, got[id])
		}
	}
	for id := range ruleMatchKeysResiduals {
		if !seen[id] {
			t.Errorf("unused residual %s", id)
		}
	}
	t.Logf("%d of %d observations equal native (%d pairs)", exact, total, len(got))
}

// The key of each item follows the AND groups of the rule's filter: a First
// Name item takes the initial key only when every group holding it also holds
// a Last Name item.
func TestCustomRuleKeyModes(t *testing.T) {
	items := func(methods ...string) []storage.MatchingRuleItem {
		out := make([]storage.MatchingRuleItem, len(methods))
		for i, m := range methods {
			out[i] = storage.MatchingRuleItem{Field: m, Method: m}
		}
		return out
	}
	m, i := fuzzyKeyMethod, fuzzyKeyFirstInitial
	for _, c := range []struct {
		filter  string
		methods []string
		want    []fuzzyKeyMode
	}{
		{"", []string{"FirstName"}, []fuzzyKeyMode{m}},                                                  // C1 K1
		{"1 AND 2", []string{"FirstName", "LastName"}, []fuzzyKeyMode{i, m}},                            // K3
		{"", []string{"FirstName", "LastName"}, []fuzzyKeyMode{i, m}},                                   // no filter is AND
		{"1 AND 2", []string{"FirstName", "Exact"}, []fuzzyKeyMode{m, m}},                               // K6
		{"(1 AND 2) OR 3", []string{"FirstName", "LastName", "Exact"}, []fuzzyKeyMode{i, m, m}},         // K8
		{"(1 AND 2) OR (1 AND 3)", []string{"FirstName", "LastName", "Exact"}, []fuzzyKeyMode{m, m, m}}, // uncaptured: keeps the method key
		{"1 OR 2", []string{"FirstName", "LastName"}, []fuzzyKeyMode{m, m}},
		{"1 AND (2 OR 3)", []string{"FirstName", "LastName", "LastName"}, []fuzzyKeyMode{i, m, m}},
		{"1 AND 2", []string{"FirstName", "Phone"}, []fuzzyKeyMode{m, m}},
		{"1 AND", []string{"FirstName", "LastName"}, []fuzzyKeyMode{m, m}}, // unparsed filter
	} {
		methods := items(c.methods...)
		// An exact Last Name item is not a Last Name method.
		if got := customRuleKeyModes(storage.MatchingRule{BooleanFilter: c.filter, Items: methods}); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q %v: got %v, want %v", c.filter, c.methods, got, c.want)
		}
	}
	exactLast := []storage.MatchingRuleItem{{Field: "FirstName", Method: "FirstName"}, {Field: "LastName", Method: "Exact"}}
	if got := customRuleKeyModes(storage.MatchingRule{BooleanFilter: "1 AND 2", Items: exactLast}); got[0] != fuzzyKeyMethod {
		t.Errorf("an exact Last Name item gave the first name the initial key")
	}
}

// Differential: the shapes no row measures keep the origin/main verdicts and
// confidences (recorded from origin/main ad1577dbc6): a fuzzy First Name with
// an exact Last Name (the Help article's first-and-last-letter key, C1 K11 not
// captured), First Name beside Phone, and the standard Lead company path.
func TestDuplicateRuleMatchKeysUncapturedShapesKeepMain(t *testing.T) {
	rule := func(filter string, items ...[2]string) ruleMatchKeysRule {
		r := ruleMatchKeysRule{Object: "Contact", Filter: filter}
		for _, it := range items {
			r.Items = append(r.Items, struct {
				Field  string `json:"field"`
				Method string `json:"method"`
			}{it[0], it[1]})
		}
		return r
	}
	exactLast := rule("1 AND 2", [2]string{"FirstName", "FirstName"}, [2]string{"LastName", "Exact"})
	withPhone := rule("1 AND 2", [2]string{"FirstName", "FirstName"}, [2]string{"Phone", "Phone"})
	lead := ruleMatchKeysRule{Object: "Lead"}
	firsts := []struct {
		a, b                     string
		exactLast, phone, leadCo string
	}{
		{"Vorkel", "Volkel", "0@", "0@", "0@"},
		{"Kristen", "Cristen", "1@98.0", "1@98.0", "1@95.0"},
		{"Y", "Yancey", "0@", "0@", "0@"},
		{"Kate", "Katie", "0@", "0@", "0@"},
		{"Alex", "Xander", "0@", "0@", "0@"},
		{"Bob", "Robert", "1@100.0", "1@100.0", "1@100.0"},
		{"Jon", "Jo", "0@", "0@", "0@"},
		{"Cathy", "Kathy", "1@100.0", "1@100.0", "1@100.0"},
		{"Ray", "Rayh", "0@", "0@", "0@"},
		{"YR", "Y R", "0@", "0@", "0@"},
	}
	for _, c := range firsts {
		person := map[string]string{"FirstName": c.a, "LastName": "Zorwick", "Email": "probe@fixture.example", "Phone": "4155550123"}
		other := map[string]string{"FirstName": c.b, "LastName": "Zorwick", "Email": "probe@fixture.example", "Phone": "4155550123"}
		if got := ruleMatchKeysFind(t, "ExactLast", exactLast, person, other); got != c.exactLast {
			t.Errorf("first name and exact last name %s/%s: got %s, origin/main %s", c.a, c.b, got, c.exactLast)
		}
		if got := ruleMatchKeysFind(t, "WithPhone", withPhone, person, other); got != c.phone {
			t.Errorf("first name and phone %s/%s: got %s, origin/main %s", c.a, c.b, got, c.phone)
		}
		stored := map[string]string{"FirstName": c.a, "LastName": "Harlow", "Company": "Zqbeacon", "Title": "Director"}
		candidate := map[string]string{"FirstName": c.b, "LastName": "Harlow", "Company": "Zqbeacon", "Title": "Director"}
		if got := ruleMatchKeysFind(t, "Standard_Lead_Match_Rule_v1_0", lead, stored, candidate); got != c.leadCo {
			t.Errorf("standard Lead company path %s/%s: got %s, origin/main %s", c.a, c.b, got, c.leadCo)
		}
	}
}

// NATIVE duplicate_save_behavior calibration initials: on the standard email
// path an initial "K" finds "Karl" typed with ASCII letters only; typed with
// the Kelvin sign (U+212A) it finds nothing, although folding equates them.
func TestStandardInitialsNeedASCIIInitial(t *testing.T) {
	if ok, score := standardFirstNameScore("Karl", "K"); !ok || score != 100 {
		t.Fatalf("ASCII initial: %v %.0f", ok, score)
	}
	if ok, _ := standardFirstNameScore("Karl", "K"); ok {
		t.Fatal("a Kelvin-sign name matched an ASCII initial")
	}
	if _, ok := standardInitialKeyedFirst("Keith", "K"); ok {
		t.Fatal("phone path: a Kelvin-sign name matched an ASCII initial")
	}
}
