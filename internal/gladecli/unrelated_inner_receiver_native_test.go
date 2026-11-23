package gladecli

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// The four rows exercise an unrelated inner type receiver and lexical controls
// through anonymous Apex and the test runner at the captured source versions.
// NATIVE anonymous 62.0 and 67.0: testdata/unrelated_inner_receiver/org.tsv:2-3.
// NATIVE istest 62.0 and 67.0: testdata/unrelated_inner_receiver/org.tsv:4-5.
func TestUnrelatedInnerReceiverNativeConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	const dir = "testdata/unrelated_inner_receiver"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 || strings.Join(rows[0], ",") != "id,api,route,compiled,success,outcome,class_api,trigger_api,residue" {
		t.Fatalf("invalid native matrix: %v", err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 9 {
			t.Fatalf("invalid row: %v", row)
		}
		key := row[1] + "/" + row[2]
		if seen[key] || (row[1] != "62.0" && row[1] != "67.0") || row[6] != row[1] {
			t.Fatalf("invalid row: %v", row)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":%q}`, row[1]))
			classes, err := filepath.Glob(filepath.Join(dir, "classes", "*.cls"))
			if err != nil || len(classes) != 3 {
				t.Fatalf("classes: %v %v", classes, err)
			}
			for _, class := range classes {
				source, err := os.ReadFile(class)
				if err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(root, "force-app", "main", "default", "classes", filepath.Base(class))
				writeTestFile(t, target, string(source))
				writeTestFile(t, target+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><status>Active</status></ApexClass>`)
			}
			var args []string
			switch row[2] {
			case "anonymous":
				if row[0] != "anonymous" || row[3] != "True" || row[4] != "True" || row[5] != "ok" || row[8] != "0" {
					t.Fatalf("invalid anonymous row: %v", row)
				}
				body, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				args = []string{"exec", "--project", root, "--json", string(body)}
			case "istest":
				if row[0] != "test" || row[3] != "true" || row[4] != "Passed" || row[5] != "check=Pass" {
					t.Fatalf("invalid test row: %v", row)
				}
				args = []string{"test", "--project", root, "--class", "ProbeConsumerTest", "--no-cache", "--no-serve", "--no-progress", "--json"}
			default:
				t.Fatalf("unknown route: %v", row)
			}
			var stdout, stderr bytes.Buffer
			if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
				t.Fatalf("native %s: exit=%d stderr=%s stdout=%s", row[5], code, stderr.String(), stdout.String())
			}
			if row[2] == "istest" {
				run, err := decodeTestRunJSON(stdout.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						count++
						if result.Status != testreport.StatusPass {
							t.Fatalf("result=%+v", result)
						}
					}
				}
				if count != 1 {
					t.Fatalf("executed %d tests, want 1", count)
				}
			}
		})
	}
}
