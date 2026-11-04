package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestMissingSchemaNullReceiverNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/missing_schema_null_receiver"
	table, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()
	reader := csv.NewReader(table)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		t.Fatalf("native rows=%d, err=%v", len(rows), err)
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	index := loadTestIndex(t, root)
	org := orgFromIndex(index)
	storage.EnsureStandardObject(&org, "Account")
	for _, row := range rows[1:] {
		t.Run(row[1]+"/"+row[0], func(t *testing.T) {
			if row[2] != "anonymous" || row[3] != "OBSERVED" {
				t.Fatalf("invalid native row: %q", row)
			}
			var want struct {
				Compiled         bool   `json:"compiled"`
				Success          bool   `json:"success"`
				ExceptionMessage string `json:"exceptionMessage"`
			}
			if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(filepath.Join(fixture, row[0]+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			analysis := sema.AnalyzeAnonymous(index, string(body), row[1])
			if got := !analysis.HasErrors(); got != want.Compiled {
				t.Errorf("native compiled=%t; Glade=%t: %v", want.Compiled, got, analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			_, err = runner.newMachine().Execute(program)
			message := ""
			if err != nil {
				message = err.Error()
			}
			if row[0] == "dynquery_missing_ns_column" {
				// Documented divergence, not a defect: Glade treats a namespaced custom column as a
				// possibly installed managed-package field and returns null. A native org without the
				// package rejects it (rows kept in org.tsv); with the package installed it accepts it.
				// Glade has no package metadata to tell the cases apart. Assert the native row and the
				// tolerance explicitly instead of skipping, so the row stays visible and pinned.
				if want.Success || !strings.Contains(want.ExceptionMessage, "No such column 'nsq__Key__c'") {
					t.Errorf("native row changed: success=%t message=%q", want.Success, want.ExceptionMessage)
				}
				if err != nil {
					t.Errorf("installed-package tolerance lost: Glade rejected the column: %v", err)
				}
				return
			}
			if (err == nil) != want.Success || strings.TrimPrefix(trimContext216(strings.TrimSpace(message)), "System.") != strings.TrimPrefix(want.ExceptionMessage, "System.") {
				t.Errorf("native success=%t message=%q; Glade success=%t message=%q", want.Success, want.ExceptionMessage, err == nil, message)
			}
		})
	}
}

func trimContext216(message string) string {
	if i := strings.Index(message, " (context: "); i >= 0 {
		return message[:i]
	}
	return message
}
