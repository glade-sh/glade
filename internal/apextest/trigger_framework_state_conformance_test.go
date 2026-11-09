package apextest

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTriggerFrameworkStateConformance compares a synthetic trigger-framework
// project with the captured scratch org rows (APIs 62.0 and 67.0, named tests).
// Rows cover custom-setting rows invisible to tests, trigger exceptions and
// nested DML wrapped in DmlException, caller-only stack traces with no cause,
// and field defaults that apply on persistence rather than on `new`.
func TestTriggerFrameworkStateConformance(t *testing.T) {
	dir := filepath.Join("testdata", "trigger_framework_state")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	want := map[string]map[string]string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 5)
		if len(fields) != 5 || fields[0] == "api" || fields[1] != "isTest" || fields[3] != "observed" {
			continue
		}
		var payload struct {
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(fields[4]), &payload); err != nil {
			t.Fatalf("row %s/%s: %v", fields[0], fields[2], err)
		}
		if want[fields[2]] == nil {
			want[fields[2]] = map[string]string{}
		}
		want[fields[2]][fields[0]] = payload.Value
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(want) == 0 {
		t.Fatal("no captured rows")
	}
	for method, versions := range want {
		if versions["62.0"] != versions["67.0"] {
			t.Fatalf("captured rows for %s differ between 62.0 and 67.0", method)
		}
	}

	index := loadTestIndex(t, dir)
	if index.HasErrors() {
		t.Fatalf("synthetic project index: %v", index.Diagnostics)
	}
	run := Run(index, Options{SelectedClasses: []string{"C11Cells"}, NoDiskCache: true})
	got := map[string]string{}
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.Problem == nil {
				t.Fatalf("%s passed; every cell reports through a failing assertion", testCase.MethodName)
			}
			const marker = "Actual: "
			message := testCase.Problem.Message
			i := strings.Index(message, marker)
			if i < 0 {
				t.Fatalf("%s: unexpected problem %q", testCase.MethodName, message)
			}
			got[testCase.MethodName] = message[i+len(marker):]
		}
	}
	for method, versions := range want {
		if actual, ok := got[method]; !ok {
			t.Errorf("cell %s did not run", method)
		} else if actual != versions["67.0"] {
			t.Errorf("cell %s\n got: %q\nwant: %q", method, actual, versions["67.0"])
		}
	}
}
