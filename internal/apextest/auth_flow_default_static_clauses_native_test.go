package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestAuthFlowDefaultStaticClauseQueryNativeRoutes(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata/conformance/auth_flow_default_static_clauses", "org.tsv"))
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
	if len(rows) != 73 {
		t.Fatalf("native observations=%d, want 72", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		query := "SELECT " + row[2] + " FROM AuthProvider LIMIT 1"
		switch row[3] {
		case "where":
			query = "SELECT Id FROM AuthProvider WHERE " + row[2] + " = null LIMIT 1"
		case "order":
			query = "SELECT Id FROM AuthProvider ORDER BY " + row[2] + " LIMIT 1"
		case "select":
		default:
			t.Fatalf("unknown clause %q", row[3])
		}
		body := "List<AuthProvider> rows=[" + query + "];"
		t.Run(row[0]+"/"+row[1]+"/"+row[2]+"/"+row[3], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			if row[1] == "named" {
				path := filepath.Join(root, "force-app/main/default/classes/SyntheticStatic.cls")
				writeFile(t, path, "public class SyntheticStatic { public static void observe() { "+body+" } }")
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if row[1] == "anonymous" {
				analysis = sema.AnalyzeAnonymous(index, body, row[0])
			}
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
