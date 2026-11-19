package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthFlowDefaultDynamicQueryNativeRoutes(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata/conformance/auth_flow_default_query", "org.tsv"))
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
	observed := make(map[string]string, len(rows)-1)
	for _, row := range rows[1:] {
		observed[row[0]+"/"+row[1]+"/"+row[2]] = row[3]
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		for _, field := range []string{"FlowDefaultAccountId", "FlowDefaultProfileId"} {
			anonymous := observed[api+"/anonymous/"+field]
			named := observed[api+"/istest/"+field]
			if anonymous == "" || anonymous != named || (anonymous != "ok" && !strings.HasPrefix(anonymous, "System.QueryException: No such column")) {
				t.Fatalf("invalid native rows for %s/%s: %q %q", api, field, anonymous, named)
			}
			t.Run(api+"/"+field, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				wantError := "false"
				if anonymous != "ok" {
					wantError = "true"
				}
				body := `Boolean rejected=false; try { Database.query('SELECT ` + field + ` FROM AuthProvider LIMIT 1'); } catch(QueryException e) { rejected=true; } System.assertEquals(` + wantError + `,rejected);`
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}
