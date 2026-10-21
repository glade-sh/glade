package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// The native empty-schema capture rejects the first statement's field
// initializer. Keep that rejection independent of later field reads and calls.
func TestMetadataConstructorNativeRejection(t *testing.T) {
	dir := "testdata/conformance/metadata_dependency_shapes"
	source, err := os.ReadFile(filepath.Join(dir, "sources/split.apex"))
	if err != nil {
		t.Fatal(err)
	}
	constructor, _, _ := strings.Cut(string(source), ";")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			f, err := os.Open(filepath.Join(dir, "api"+api[:2], "empty/org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			reader := csv.NewReader(f)
			reader.Comma = '\t'
			rows, err := reader.ReadAll()
			if err != nil || len(rows) < 2 || strings.Join(rows[1], "|") != "split|compile-rejected|Field does not exist: Words__c on MeadowWords__mdt" {
				t.Fatalf("missing native constructor rejection: %v; %v", rows, err)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			objectPath := "objects/MeadowWords__mdt/MeadowWords__mdt.object-meta.xml"
			object, err := os.ReadFile(filepath.Join(dir, "metadata/empty", objectPath))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default", objectPath), string(object))
			index := loadTestIndex(t, root)
			result := sema.AnalyzeAnonymous(index, constructor+";", api)
			if !result.HasErrors() {
				t.Fatalf("native rejected <%s>; Glade accepted constructor: %s", rows[1][2], constructor)
			}
		})
	}
}
