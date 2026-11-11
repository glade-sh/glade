package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthFlowDefaultFieldPresenceNativeRoutes(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata/conformance/auth_flow_default_presence", "org.tsv"))
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
	if len(rows) != 13 {
		t.Fatalf("native observations=%d, want 12", len(rows)-1)
	}
	for i := 1; i < len(rows); i += 2 {
		a, b := rows[i], rows[i+1]
		if a[0] != b[0] || a[3] != b[3] {
			t.Fatalf("inconsistent native pair: %v %v", a, b)
		}
		t.Run(a[0], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+a[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			body := `Map<String,Schema.SObjectField> fields=Schema.getGlobalDescribe().get('AuthProvider').getDescribe().fields.getMap(); System.assertEquals(` + a[3] + `,fields.containsKey('FlowDefaultAccountId')); System.assertEquals(` + b[3] + `,fields.containsKey('FlowDefaultProfileId'));`
			runStandardFieldEvidenceRoutes(t, root, a[0], body)
		})
	}
}
