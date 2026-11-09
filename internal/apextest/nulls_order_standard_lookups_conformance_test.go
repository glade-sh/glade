package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/soql"
)

const nullsOrderStandardLookupsFixture = "testdata/conformance/nulls_order_standard_lookups"

var nullsOrderTails = []string{"NULLS FIRST", "NULLS LAST", "ASC", "DESC", "DESC NULLS LAST"}

// N holds the null lookup, L and H the lower and higher parent Id.
var (
	nullsOrderHonoured = []string{"NLH", "LHN", "NLH", "NHL", "HLN"}
	nullsOrderIgnored  = []string{"NLH", "NLH", "NLH", "HLN", "HLN"}
)

// nullsOrderUnexplained is the one probed field whose rows match neither
// shape: every order returned insertion order. Glade does not model it.
var nullsOrderUnexplained = map[string]bool{"CommSubscriptionConsent.DataUsePurposeId": true}

type nullsOrderRow struct {
	api, object, field, source string
	observation                map[string]any
}

func nullsOrderRows(t *testing.T) []nullsOrderRow {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(nullsOrderStandardLookupsFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 933 || lines[0] != "api\tobject\tfield\tsource\tobservation" {
		t.Fatalf("incomplete nulls order rows: %d", len(lines))
	}
	var rows []nullsOrderRow
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 5)
		row := nullsOrderRow{api: cells[0], object: cells[1], field: cells[2], source: cells[3]}
		if err := json.Unmarshal([]byte(cells[4]), &row.observation); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}

func nullsOrderOf(observation map[string]any) []string {
	out := make([]string, len(nullsOrderTails))
	for i, tail := range nullsOrderTails {
		out[i], _ = observation[tail].(string)
	}
	return out
}

func nullsOrderEqual(a, b []string) bool {
	return strings.Join(a, ",") == strings.Join(b, ",")
}

// Native rows: nulls_order_standard_lookups org.tsv. Every probed lookup either
// honours the NULLS clause or ignores it, the same at 62.0 and 67.0, and the
// Glade classification follows every probed field.
func TestNullsOrderStandardLookupsClassification(t *testing.T) {
	probed := 0
	for _, row := range nullsOrderRows(t) {
		if _, skipped := row.observation["skip"]; skipped {
			continue
		}
		probed++
		name := row.object + "." + row.field
		order := nullsOrderOf(row.observation)
		if nullsOrderUnexplained[name] {
			continue
		}
		ignored := nullsOrderEqual(order, nullsOrderIgnored)
		if !ignored && !nullsOrderEqual(order, nullsOrderHonoured) {
			t.Errorf("%s %s: unclassified native order %v", row.api, name, order)
			continue
		}
		if got := soql.StandardLookupIgnoresNullsClause(row.object, row.field); got != ignored {
			t.Errorf("%s %s: native ignores the NULLS clause=%v; Glade=%v", row.api, name, ignored, got)
		}
	}
	if probed != 270 {
		t.Fatalf("probed rows %d", probed)
	}
}

// The hand-written probe classes (NullCoreHand62/67) replay in Glade: each
// natively probed core lookup returns the five native orders.
func TestNullsOrderStandardLookupsNativeConformance(t *testing.T) {
	want := map[string][]string{}
	for _, row := range nullsOrderRows(t) {
		if _, skipped := row.observation["skip"]; row.source == "hand" && !skipped {
			want[row.api+"|"+row.object+"|"+row.field] = nullsOrderOf(row.observation)
		}
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	classes := filepath.Join(nullsOrderStandardLookupsFixture, "project", "force-app", "main", "default", "classes")
	for _, api := range []string{"62", "67"} {
		for _, suffix := range []string{".cls", ".cls-meta.xml"} {
			body, err := os.ReadFile(filepath.Join(classes, "NullCoreHand"+api+suffix))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app", "main", "default", "classes", "NullCoreHand"+api+suffix), string(body))
		}
	}
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("project compilation: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	run := Run(index, Options{SelectedClasses: []string{"NullCoreHand62", "NullCoreHand67"}, NoDiskCache: true, Parallelism: 1})
	seen := 0
	for _, suite := range run.Suites {
		for _, tc := range suite.Cases {
			if tc.Problem == nil {
				t.Errorf("%s.%s: no observation", tc.ClassName, tc.MethodName)
				continue
			}
			_, capture, ok := strings.Cut(tc.Problem.Message, "CAPTURE|")
			parts := strings.SplitN(capture, "|", 3)
			if !ok || len(parts) != 3 {
				t.Errorf("%s.%s: %s", tc.ClassName, tc.MethodName, tc.Problem.Message)
				continue
			}
			key := strings.TrimPrefix(tc.ClassName, "NullCoreHand") + ".0|" + parts[0] + "|" + parts[1]
			native, captured := want[key]
			if !captured {
				continue
			}
			seen++
			var observation map[string]any
			if err := json.Unmarshal([]byte(strings.TrimSpace(parts[2])), &observation); err != nil {
				t.Errorf("%s: %v", key, err)
				continue
			}
			if got := nullsOrderOf(observation); !nullsOrderEqual(got, native) {
				t.Errorf("%s: native %v; Glade %v (%v)", key, native, got, observation["skip"])
			}
		}
	}
	if seen != len(want) {
		t.Fatalf("replayed %d of %d native rows", seen, len(want))
	}
}
