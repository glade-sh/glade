package vm

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecNetworkExternalUserAccessNativeRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/network_external_user_access/"
	read := func(name string) [][]string {
		t.Helper()
		f, err := os.Open(fixture + name)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma = '\t'
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	expressions := map[string]string{}
	for _, row := range read("cases.tsv")[1:] {
		expressions[row[0]] = row[1]
	}
	rows := read("org.tsv")
	if len(rows) != 17 {
		t.Fatalf("native matrix: %d rows, want 16", len(rows)-1)
	}
	for _, row := range rows[1:] {
		for _, setup := range []string{"default_vm", "explicit_org"} {
			t.Run(strings.Join(row[:3], "/")+"/"+setup, func(t *testing.T) {
				expression, ok := expressions[row[2]]
				if !ok {
					t.Fatalf("missing expression: %s", row[2])
				}
				body := `String observed;
try { String result = ` + expression + `; observed = result == null ? 'NULL' : 'VALUE|' + result; }
catch (Exception e) { observed = 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage(); }
System.assertEquals('` + strings.ReplaceAll(row[3], "'", "\\'") + `', observed);`
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				if setup == "explicit_org" {
					org := storage.NewOrgState()
					storage.EnsureDeterministicPlatformData(&org)
					machine.SetOrg(&org)
				}
				if row[1] == "istest" {
					machine.EnableTestContext()
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
