package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
)

// Native deployment answers distinguish rejected metadata from a valid shape
// outside Glade's supported grammar. In particular, nested NOT is accepted by
// Salesforce but deliberately keeps Glade's GLADEAUTO002 unsupported fallback.
func TestFlowLogic2ParserNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_logic2/parser_review"
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	cases := strings.Split(strings.TrimSpace(read("cases.tsv")), "\n")
	if len(cases) != 21 || cases[0] != "id\tfilter_logic_json\tlocal_expectation" {
		t.Fatal("incomplete parser case table")
	}
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			capture := read("native/api" + api + "/RUN-FACTS.txt")
			for _, fact := range []string{"api=" + api + ".0", "native_complete=true", "deployment_rows=20"} {
				if !strings.Contains("\n"+capture, "\n"+fact+"\n") {
					t.Fatalf("missing native fact: %s", fact)
				}
			}
			checkHash := func(path string) {
				t.Helper()
				if !strings.Contains(capture, fmt.Sprintf("source_sha256:%s=%x\n", path, sha256.Sum256([]byte(read(path))))) {
					t.Fatalf("capture source changed: %s", path)
				}
			}
			checkHash("cases.tsv")
			rows := strings.Split(strings.TrimSpace(read("native/api"+api+"/org.tsv")), "\n")
			if len(rows) != len(cases) || rows[0] != "context\tid\tobservation" {
				t.Fatal("incomplete parser native rows")
			}
			for i, line := range cases[1:] {
				cols := strings.Split(line, "\t")
				if len(cols) != 3 {
					t.Fatalf("invalid case: %s", line)
				}
				t.Run(cols[0], func(t *testing.T) {
					row := strings.SplitN(rows[i+1], "\t", 3)
					if len(row) != 3 || row[0] != "deployment" || row[1] != cols[0] {
						t.Fatalf("missing native row for %s", cols[0])
					}
					var native struct {
						Accepted bool     `json:"accepted"`
						Messages []string `json:"messages"`
					}
					if err := json.Unmarshal([]byte(row[2]), &native); err != nil {
						t.Fatal(err)
					}
					accepted := cols[2] == "supported" || cols[0] == "nested_not"
					if native.Accepted != accepted || (accepted && len(native.Messages) != 0) || (!accepted && len(native.Messages) == 0) {
						t.Fatalf("unexpected native acceptance: %#v", native)
					}
					path := "metadata/api" + api + "/" + cols[0] + "/flows/Parser_" + cols[0] + ".flow-meta.xml"
					checkHash(path)
					path = filepath.Join(fixture, path)
					idx, err := automation.LoadProject(project.Project{FlowFiles: []string{path}})
					if err != nil {
						t.Fatal(err)
					}
					if len(idx.Flows) != 1 || len(idx.Flows[0].Rules) != 1 {
						t.Fatalf("missing flow rule: %#v", idx)
					}
					rule := idx.Flows[0].Rules[0]
					if cols[2] == "supported" {
						if len(idx.Diagnostics) != 0 || !reflect.DeepEqual(rule.CriteriaGroups, [][]int{{0, 0}, {1, 0}}) {
							t.Fatalf("valid nested logic: rule=%#v diagnostics=%v", rule, idx.Diagnostics)
						}
						return
					}
					if len(idx.Diagnostics) != 1 || idx.Diagnostics[0].Code != "GLADEAUTO002" || idx.Diagnostics[0].File != path || !strings.Contains(idx.Diagnostics[0].Message, "start filter logic") {
						t.Fatalf("missing unsupported fallback: %v", idx.Diagnostics)
					}
					if rule.Formula != "" || len(rule.Criteria) != 2 || len(rule.CriteriaGroups) != 0 {
						t.Fatalf("unsupported logic did not retain AND fallback: %#v", rule)
					}
				})
			}
		})
	}
}
