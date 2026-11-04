package vm_test

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestLiteralCaseNativeConformance(t *testing.T) {
	cases := readQueryLiteralTable(t, "../literal_case/cases.tsv", "case|apex_literal|hex")
	rows := readQueryLiteralTable(t, "../literal_case/org.tsv", "source_api|route|case|outcome|hex")
	literals := map[string]string{}
	for _, c := range cases {
		literals[c[0]] = c[1]
	}
	if len(rows) != 18 {
		t.Fatalf("native rows: got %d, want 18", len(rows))
	}
	for _, row := range rows {
		row := row
		t.Run(row[0]+"/"+row[1]+"/"+row[2], func(t *testing.T) {
			if row[2] == "unterminated" {
				if _, err := vm.CompileAnonymousWithOptions("String value = 'Unterminated;", vm.CompileOptions{APIVersion: row[0]}); err == nil {
					t.Fatal("native rejected unterminated literal")
				}
				return
			}
			literal, ok := literals[row[2]]
			if !ok {
				t.Fatalf("unknown case %q", row[2])
			}
			source := fmt.Sprintf("System.assertEquals('%s', EncodingUtil.convertToHex(Blob.valueOf(%s)));", row[4], literal)
			if row[1] == "istest" {
				checkQueryLiteralProject(t, row[0], source, "accept", row[4])
				return
			}
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = row[0]
			machine := vm.New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
