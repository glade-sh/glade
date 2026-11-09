package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestCDCRefreshTriggerCompileNativeAPIRoutes(t *testing.T) {
	f, err := os.Open("testdata/conformance/b7cdc_refresh/compile-org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 7 {
		t.Fatalf("got %d rows; want 6", len(rows)-1)
	}
	for _, row := range rows[1:] {
		api, object, want := row[0], row[1], row[2]
		t.Run(api+"/"+object, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":%q}`, api))
			dir := filepath.Join(root, "force-app", "main", "default", "triggers")
			writeFile(t, filepath.Join(dir, "B7CompileProbe.trigger"), fmt.Sprintf("trigger B7CompileProbe on %s (after insert) {}\n", object))
			writeFile(t, filepath.Join(dir, "B7CompileProbe.trigger-meta.xml"), fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexTrigger>`, api))
			index := loadTestIndex(t, root)
			result := sema.Analyze(index)
			accepted := !index.HasErrors() && !result.HasErrors()
			if accepted != (want == "accepted") {
				t.Fatalf("native %s; Glade accepted=%v index=%v sema=%v", want, accepted, index.Diagnostics, result.Diagnostics)
			}
		})
	}
}
