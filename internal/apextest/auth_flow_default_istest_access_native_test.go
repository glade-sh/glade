package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestAuthFlowDefaultIsTestFieldNativeRoutes(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata/conformance/auth_flow_default_istest_access", "org.tsv"))
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
	if len(rows) != 25 {
		t.Fatalf("native observations=%d, want 24", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		body := `AuthProvider source=new AuthProvider(); Id value=source.` + row[2] + `;`
		if row[3] == "write" {
			body = `AuthProvider source=new AuthProvider(); source.` + row[2] + `=null;`
		} else if row[3] != "read" {
			t.Fatalf("unknown operation %q", row[3])
		}
		t.Run(row[0]+"/"+row[1]+"/"+row[2]+"/"+row[3], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			if row[1] != "istest" {
				t.Fatalf("unknown route %q", row[1])
			}
			path := filepath.Join(root, "force-app/main/default/classes/SyntheticStatic.cls")
			writeFile(t, path, "@IsTest private class SyntheticStatic { @IsTest static void observe() { "+body+" } }")
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			got := "accepted"
			if index.HasErrors() || analysis.HasErrors() {
				got = "rejected"
			}
			if got != row[4] {
				t.Errorf("got %s, native %s; index=%v diagnostics=%v", got, row[4], index.Diagnostics, analysis.Diagnostics)
			}
		})
	}
}
