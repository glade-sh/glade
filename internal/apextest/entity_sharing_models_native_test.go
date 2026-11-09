package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Standard-object sharing metadata below reproduces the native scratch org's
// observed configuration; the three custom-object metadata files are native.
func TestEntitySharingModelsNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/entity_sharing_models"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
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
	if len(rows) != 5 {
		t.Fatalf("native rows=%d, want 4", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, row[0]))
			version := strings.TrimSuffix(row[0], ".0")
			for _, class := range []string{"EntitySharingProbe" + version, "EntitySharingProbe" + version + "Test"} {
				for _, suffix := range []string{".cls", ".cls-meta.xml"} {
					path := filepath.Join("classes", class+suffix)
					b, err := os.ReadFile(filepath.Join(dir, "force-app/main/default", path))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "force-app/main/default", path), string(b))
				}
			}
			for _, name := range []string{"OwnedRead__c", "OwnedPublic__c", "OwnedPrivate__c"} {
				path := filepath.Join("objects", name, name+".object-meta.xml")
				b, err := os.ReadFile(filepath.Join(dir, "force-app/main/default", path))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default", path), string(b))
			}
			for name, sharing := range map[string]string{"Account": "ReadWrite", "Case": "ReadWriteTransfer", "Order": "ReadWrite", "User": "Read"} {
				writeFile(t, filepath.Join(root, "force-app/main/default/objects", name, name+".object-meta.xml"), "<CustomObject><sharingModel>"+sharing+"</sharingModel></CustomObject>")
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("project diagnostics: %v", index.Diagnostics)
			}
			var message string
			if row[1] == "istest" {
				run := Run(index, Options{SelectedClasses: []string{"EntitySharingProbe" + version + "Test"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing assertion transport: %+v", run.Summary())
				}
				message = run.Suites[0].Cases[0].Problem.Message
			} else {
				source, err := os.ReadFile(filepath.Join(dir, "anonymous-"+version+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = row[0]
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				_, err = runner.execute(program)
				if err == nil {
					t.Fatal("missing assertion transport")
				}
				message = err.Error()
			}
			_, payload, ok := strings.Cut(message, "ENTITYOWD|")
			if !ok {
				t.Fatalf("missing observation: %s", message)
			}
			var got, want any
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("native=%v Glade=%v", want, got)
			}
		})
	}
}
