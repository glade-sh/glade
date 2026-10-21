package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestSOQLMixedConditionalNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "soql_mixed_conditional_native", "MixedConditionalTest", []string{"62.0", "67.0"}, 2)
}

func TestSOQLMixedConditionalStoredListNativeControls(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "soql_mixed_conditional_native")
	file, err := os.Open(filepath.Join(dir, "controls.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("incomplete native controls: %v", err)
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			body, err := os.ReadFile(filepath.Join(dir, "controls", row[1]+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[0]+`"}`)
			writeFile(t, filepath.Join(root, "force-app", row[1]+".cls"), string(body))
			writeFile(t, filepath.Join(root, "force-app", row[1]+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[0]+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if !analysis.HasErrors() {
				t.Fatalf("native rejected stored-list conditional: %s; Glade accepted it", row[2])
			}
		})
	}
}
