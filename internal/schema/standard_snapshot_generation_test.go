package schema

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestStandardSnapshotCompletenessGeneration(t *testing.T) {
	root := t.TempDir()
	snapshot := Schema{Objects: []Object{{Name: "Account", CompleteSnapshot: true, Fields: []Field{{Name: "Known__c", Type: "Text"}}}, {Name: "Contact", Partial: true, CompleteSnapshot: true}, {Name: "Lead", CompleteSnapshot: true}}}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.ReplaceAll(string(raw), `"completeSnapshot"`, `"fieldsComplete"`))
	path := filepath.Join(root, "snapshot.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	overlay := filepath.Join(root, "objects", "Account", "Account.object-meta.xml")
	if err := os.MkdirAll(filepath.Dir(overlay), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlay, []byte(`<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Account</label></CustomObject>`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadProject(project.Project{Root: root, ObjectFiles: []string{overlay}, SchemaSnapshot: path, SchemaSnapshotSHA256: fmt.Sprintf("%x", sum)})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]map[string]any{}
	for _, o := range got.Objects {
		b, err := json.Marshal(o)
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		byName[o.Name] = v
	}
	for _, name := range []string{"Account", "Lead"} {
		if byName[name]["completeSnapshot"] != true {
			t.Errorf("%s: complete snapshot provenance missing from serialized object: %v", name, byName[name])
		}
	}
	if byName["Contact"]["completeSnapshot"] == true {
		t.Errorf("partial snapshot incorrectly marked complete: %v", byName["Contact"])
	}
	var legacy Object
	if err := json.Unmarshal([]byte(`{"name":"Account"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"completeSnapshot":true`) {
		t.Fatalf("legacy object must remain open: %s", encoded)
	}
}
