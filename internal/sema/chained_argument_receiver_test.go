package sema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The local contract is that a call argument uses the same receiver type as
// the same expression at statement level. The direct Date rejection text is
// captured in surf_collections_strings/org.tsv (dth_date_gettime, API 62/67).
func TestChainedArgumentReceiver(t *testing.T) {
	const dateRejection = "Method does not exist or incorrect signature: void getTime() from the type Date"
	cases := []struct {
		name        string
		declaration string
		statement   string
		wantDate    bool
		wantClean   bool
	}{
		{"datetime/first", "Datetime lastDate = Datetime.now();", "Datetime q = Datetime.newInstance(lastDate.addDays(2).getTime());", false, true},
		{"datetime/standalone", "Datetime lastDate = Datetime.now();", "Long t = lastDate.addDays(2).getTime();", false, true},
		{"datetime/later", "Datetime lastDate = Datetime.now();", "System.assertEquals(0L, lastDate.addDays(2).getTime());", false, true},
		{"date/first", "Date lastDate = Date.today();", "Datetime q = Datetime.newInstance(lastDate.addDays(2).getTime());", true, false},
		{"date/standalone", "Date lastDate = Date.today();", "Long t = lastDate.addDays(2).getTime();", true, false},
		{"date/direct", "Date lastDate = Date.newInstance(2024, 1, 15);", "Long t = lastDate.getTime();", true, false},
		{"date/null", "Date lastDate = null;", "Long t = lastDate.getTime();", true, false},
		{"object/first", "Object lastDate = Date.today();", "Datetime q = Datetime.newInstance(lastDate.addDays(2).getTime());", false, false},
		{"object/standalone", "Object lastDate = Date.today();", "Long t = lastDate.addDays(2).getTime();", false, false},
	}
	for _, api := range []string{"50.0", "62.0", "67.0"} {
		for _, tc := range cases {
			body := tc.declaration + "\n" + tc.statement
			check := func(t *testing.T, result Result) {
				t.Helper()
				if tc.wantClean && result.HasErrors() {
					t.Fatalf("valid Datetime chain rejected: %+v", result.Diagnostics)
				}
				var dateDiagnostic bool
				for _, d := range result.Diagnostics {
					dateDiagnostic = dateDiagnostic || d.NativeMessage == dateRejection
					if !tc.wantDate && (d.NativeMessage == dateRejection || strings.Contains(d.Message, "Date.getTime")) {
						t.Fatalf("call was incorrectly checked on a Date receiver: %+v", d)
					}
				}
				if tc.wantDate && !dateDiagnostic {
					t.Fatalf("missing Date rejection %q: %+v", dateRejection, result.Diagnostics)
				}
			}
			t.Run(api+"/"+tc.name+"/anonymous", func(t *testing.T) {
				check(t, AnalyzeAnonymous(typesys.Index{}, body, api))
			})
			t.Run(api+"/"+tc.name+"/isTest", func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "ReceiverProbe.cls")
				writeSemaFile(t, path, "@IsTest private class ReceiverProbe { @IsTest static void probe() {\n"+body+"\n} }")
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				if len(index.Diagnostics) != 0 {
					t.Fatalf("index diagnostics: %+v", index.Diagnostics)
				}
				check(t, Analyze(index))
			})
		}
	}
}

func TestUnresolvedDottedDateHasNoInferredType(t *testing.T) {
	model := semaTypeMemberViewFromMembers(map[string]typeMembers{})
	scope := map[string]string{"holder": "Object"}
	if got := inferSemaFieldAccessType("holder.lastDate", scope, model); got != "" {
		t.Fatalf("unresolved field inferred as %q from its name", got)
	}
}

func TestDateGetTimeAnonymousNativeConformance(t *testing.T) {
	const dir = "../apextest/testdata/conformance/surf_collections_strings"
	const caseID = "dth_date_gettime"
	source, err := os.ReadFile(filepath.Join(dir, caseID+".apex"))
	if err != nil {
		t.Fatal(err)
	}
	table, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(string(table), "\n") {
		if !strings.HasPrefix(line, caseID+"\t") {
			continue
		}
		row := strings.SplitN(line, "\t", 4)
		if len(row) != 4 || row[2] != "anonymous" {
			t.Fatalf("invalid Date.getTime capture row: %v", row)
		}
		var outcome string
		if err := json.Unmarshal([]byte(row[3]), &outcome); err != nil {
			t.Fatalf("decode Date.getTime capture outcome: %v", err)
		}
		api := row[1]
		want, ok := strings.CutPrefix(outcome, "COMPILE|")
		if !ok || seen[api] {
			t.Fatalf("invalid Date.getTime capture row: %v", row)
		}
		seen[api] = true
		t.Run(api, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, string(source), api)
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].NativeMessage != want {
				t.Fatalf("want native compile error %q, got %+v", want, result.Diagnostics)
			}
		})
	}
	if !seen["62.0"] || !seen["67.0"] {
		t.Fatalf("missing Date.getTime capture rows, APIs present: %v", seen)
	}
}
