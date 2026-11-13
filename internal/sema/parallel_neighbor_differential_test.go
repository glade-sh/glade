package sema

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The dotted-target neighbor is not a native capture. Its diagnostic bytes
// can be compared with the same test over the base revision.
func TestParallelUncapturedAssignmentNeighbor(t *testing.T) {
	root := t.TempDir()
	classes := filepath.Join(root, "force-app/main/default/classes")
	if err := os.MkdirAll(classes, 0755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("Neighbor%d", i)
		src := fmt.Sprintf("public class %s { public void run() { unknownOwner.child = 1; Integer count = 'text'; } }", name)
		writeSemaFile(t, filepath.Join(classes, name+".cls"), src)
		writeSemaFile(t, filepath.Join(classes, name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	}
	index, ok := methodBodyWorkersIndex(root)
	if !ok {
		t.Fatal("load project")
	}
	serial, _ := bodiesWithGOMAXPROCS(t, index, 1)
	parallel, _ := bodiesWithGOMAXPROCS(t, index, 4)
	if len(serial) < 12 {
		t.Fatalf("expected diagnostic sentinels, got %d", len(serial))
	}
	if !reflect.DeepEqual(serial, parallel) {
		t.Fatalf("diagnostics differ: serial=%v parallel=%v", serial, parallel)
	}
	for i := range serial {
		serial[i].File = filepath.Base(serial[i].File)
	}
	data, err := json.Marshal(serial)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("normalized diagnostics SHA256 %x (%d diagnostics)", sha256.Sum256(data), len(serial))
}
