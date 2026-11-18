package gladecli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The same fixture can be run on the base and the branch at one path to
// compare the complete served org and runtime for an unchanged project.
func TestDAPUnchangedProjectNeighbor(t *testing.T) {
	root := os.Getenv("GLADE_DAP_NEIGHBOR_ROOT")
	if root == "" {
		root = t.TempDir()
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`)
	path := filepath.Join(root, "src", "classes", "Neighbor.cls")
	write(path, `public class Neighbor { public static Integer value() { return 3; } }`)
	write(path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	org, runtime, api, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(runtime.Methods) == 0 || len(runtime.Classes) == 0 {
		t.Fatal("neighbor project did not compile its class")
	}
	payload, err := json.Marshal(struct {
		Org     any
		Runtime any
		API     string
	}{org, compiledRuntimeToCache(runtime), api})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("neighbor-outcome %x\n", sha256.Sum256(payload))
}
