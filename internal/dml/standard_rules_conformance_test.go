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

	"github.com/glade-sh/glade/internal/storage"
)

const standardRulesFixture = "../apextest/testdata/conformance/duplicate_standard_rules"

type standardNativeRow struct {
	Object      string              `json:"object"`
	ExistObject string              `json:"exist_object"`
	Rule        string              `json:"rule"`
	Existing    []map[string]string `json:"existing"`
	Candidates  []map[string]string `json:"candidates"`
	Matches     [][3]float64        `json:"matches"`
}

// customRuleDefinitions are the custom matching rules the capture deployed.
var customRuleDefinitions = map[string]storage.MatchingRule{
	"FzName": {Items: []storage.MatchingRuleItem{{Field: "Name", Method: "CompanyName"}}},
	"FzOrNameCity": {BooleanFilter: "1 OR 2", Items: []storage.MatchingRuleItem{
		{Field: "Name", Method: "CompanyName"}, {Field: "BillingCity", Method: "City"}}},
	"FzThree": {BooleanFilter: "1 AND 2 AND 3", Items: []storage.MatchingRuleItem{
		{Field: "Name", Method: "CompanyName"}, {Field: "BillingStreet", Method: "Street"}, {Field: "BillingCity", Method: "City"}}},
	"FzMixOr": {BooleanFilter: "1 AND (2 OR 3)", Items: []storage.MatchingRuleItem{
		{Field: "Name", Method: "CompanyName"}, {Field: "BillingCity", Method: "City"}, {Field: "BillingPostalCode", Method: "Zip"}}},
	"FzOrFLP": {BooleanFilter: "(1 AND 2) OR 3", Items: []storage.MatchingRuleItem{
		{Field: "FirstName", Method: "FirstName"}, {Field: "LastName", Method: "LastName"}, {Field: "Phone", Method: "Phone"}}},
	"FzEmailOrLast": {BooleanFilter: "1 OR 2", Items: []storage.MatchingRuleItem{
		{Field: "Email", Method: "Exact"}, {Field: "LastName", Method: "LastName"}}},
	"FzFirstLast": {BooleanFilter: "1 AND 2", Items: []storage.MatchingRuleItem{
		{Field: "FirstName", Method: "FirstName"}, {Field: "LastName", Method: "LastName"}}},
	"FzPA": {BooleanFilter: "1 AND 2", Items: []storage.MatchingRuleItem{
		{Field: "FirstName", Method: "FirstName"}, {Field: "LastName", Method: "LastName"}}},
}

var standardObjectPrefix = map[string]string{"Account": "001", "Contact": "003", "Lead": "00Q", "PersonAccount": "001"}

func standardRowRecord(object string, fields map[string]string, id storage.ID, accounts map[string]storage.ID) storage.Record {
	record := storage.Record{ID: id, Object: object, Fields: map[string]storage.Value{}}
	for name, value := range fields {
		if name == "_Account" {
			record.Fields["AccountId"] = storage.IDValue(accounts[value])
			continue
		}
		record.Fields[name] = storage.StringValue(value)
	}
	return record
}

// standardRowMatches returns the (candidate, existing) pairs and confidences
// Glade reports for one native row through the shared duplicate-rule path.
func standardRowMatches(t *testing.T, row standardNativeRow) (map[[2]int]float64, string) {
	t.Helper()
	person := row.Object == "PersonAccount"
	existObject := row.ExistObject
	if existObject == "" {
		existObject = row.Object
	}
	storeObject := func(o string) string {
		if o == "PersonAccount" {
			return "Account"
		}
		return o
	}
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	for _, o := range []string{storeObject(row.Object), storeObject(existObject), "Account"} {
		storage.EnsureStandardObject(&org, o)
	}
	accounts := map[string]storage.ID{}
	names := map[string]bool{}
	for _, list := range [][]map[string]string{row.Existing, row.Candidates} {
		for _, fields := range list {
			if name := fields["_Account"]; name != "" {
				names[name] = true
			}
		}
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	account := org.Objects["Account"]
	if account.Records == nil {
		account.Records = map[storage.ID]storage.Record{}
	}
	for i, name := range sorted {
		id := storage.ID(fmt.Sprintf("001A%011d", i+1))
		accounts[name] = id
		account.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("ixacct-" + name)}}
	}
	org.Objects["Account"] = account
	state := org.Objects[storeObject(existObject)]
	if state.Records == nil {
		state.Records = map[storage.ID]storage.Record{}
	}
	index := map[storage.ID]int{}
	for i, fields := range row.Existing {
		id := storage.ID(fmt.Sprintf("%s%012d", standardObjectPrefix[existObject], i+1))
		record := standardRowRecord(storeObject(existObject), fields, id, accounts)
		if existObject == "PersonAccount" {
			record.Fields["IsPersonAccount"] = storage.BooleanValue(true)
			record.Fields["PersonEmail"] = storage.StringValue("person@example.com")
		}
		state.Records[id] = record
		index[id] = i
	}
	org.Objects[storeObject(existObject)] = state

	rule := storage.DuplicateRule{Name: "ProbeRule", Object: storeObject(row.Object), Active: true}
	if person {
		rule.Subtype = "PersonAccount"
	}
	if definition, ok := customRuleDefinitions[row.Rule]; ok {
		definition.Name, definition.Status, definition.Object = row.Rule, "Active", row.Object
		for i := range definition.Items {
			definition.Items[i].BlankBehavior = "NullNotAllowed"
		}
		rule.MatchRules = []storage.MatchingRule{definition}
	} else {
		matchObject := strings.TrimSuffix(strings.TrimPrefix(row.Rule, "Standard_"), "_Match_Rule_v1_0")
		matchObject = strings.TrimSuffix(matchObject, "_Match_Rule_v1_1")
		rule.MatchRules = []storage.MatchingRule{{Object: matchObject, Name: row.Rule, Status: "Active"}}
	}
	got := map[[2]int]float64{}
	engine := ""
	for c, fields := range row.Candidates {
		record := standardRowRecord(storeObject(row.Object), fields, "", accounts)
		if person {
			record.Fields["IsPersonAccount"] = storage.BooleanValue(true)
			record.Fields["PersonEmail"] = storage.StringValue("person@example.com")
		}
		result, supported := FindDuplicateMatches(&org, rule, record, nil)
		if !supported {
			return nil, "unsupported"
		}
		for _, match := range result.Matches {
			engine = match.Engine
			for i, matched := range match.Records {
				got[[2]int{c, index[matched.ID]}] = math.Floor(match.Confidences[i] + 0.5)
			}
		}
	}
	return got, engine
}

type standardBound struct {
	MinRecall          float64 `json:"min_recall"`
	MaxExtra           int     `json:"max_extra"`
	MaxConfidenceError float64 `json:"max_conf_mae"`
}

// Native Datacloud.FindDuplicates verdicts and match confidences for the
// standard Account, Contact and Lead matching rules, the cross-object Contact
// and Lead rules, custom rules with OR groups and three items, a Person Account
// rule, and the Company Name pairs of the Syllable Alignment probes (API 62.0
// and 67.0) are reproduced by the shared duplicate-rule matcher. Fit rows are
// the rows the model was built from; Holdout rows were captured after the
// model and scored once. The bounds in cases.json are the measured agreement;
// docs/DUPLICATE_FUZZY_MATCHING.md lists what stays unmatched.
func TestStandardRulesNativeRows(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(standardRulesFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Bounds map[string]standardBound `json:"bounds"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(standardRulesFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if lines[0] != "api\troute\tid\tvalue" {
		t.Fatal("invalid native row header")
	}
	seen := 0
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 {
			t.Fatalf("invalid native row: %q", line)
		}
		var row standardNativeRow
		if err := json.Unmarshal([]byte(cells[3]), &row); err != nil {
			t.Fatalf("%s: %v", cells[2], err)
		}
		name := strings.TrimSuffix(cells[0], ".0") + "/" + cells[2]
		bound, ok := cases.Bounds[name]
		if !ok {
			t.Errorf("%s: no bound in cases.json", name)
			continue
		}
		seen++
		t.Run(name, func(t *testing.T) {
			got, engine := standardRowMatches(t, row)
			if got == nil {
				t.Fatalf("rule is not enforced (%s)", engine)
			}
			native := map[[2]int]float64{}
			for _, m := range row.Matches {
				native[[2]int{int(m[0]), int(m[1])}] = m[2]
			}
			hit, extra, confidenceError := 0, 0, 0.0
			for pair, confidence := range got {
				if nativeConfidence, ok := native[pair]; ok {
					hit++
					confidenceError += math.Abs(confidence - nativeConfidence)
				} else {
					extra++
					if extra <= 5 {
						t.Logf("extra pair: candidate %v existing %v confidence %.0f", row.Candidates[pair[0]], row.Existing[pair[1]], confidence)
					}
				}
			}
			recall, mae := 1.0, 0.0
			if len(native) > 0 {
				recall = float64(hit) / float64(len(native))
			}
			if hit > 0 {
				mae = confidenceError / float64(hit)
			}
			t.Logf("recall=%.4f extra=%d conf_mae=%.3f native=%d glade=%d", recall, extra, mae, len(native), len(got))
			if recall+1e-9 < bound.MinRecall || extra > bound.MaxExtra || mae > bound.MaxConfidenceError+1e-9 {
				t.Errorf("recall=%.4f (min %.4f) extra=%d (max %d) conf_mae=%.3f (max %.3f); native=%d glade=%d", recall, bound.MinRecall, extra, bound.MaxExtra, mae, bound.MaxConfidenceError, len(native), len(got))
			}
		})
	}
	if seen != len(cases.Bounds) {
		t.Errorf("native rows %d; bounds %d", seen, len(cases.Bounds))
	}
}
