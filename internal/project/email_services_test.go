package project

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEmailServicesDiscoveryOverlappingSourceRoots(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app"},{"path":"force-app/main"}]}`)
	a := filepath.Join(root, "force-app/main/default/emailservices/Alpha.xml")
	b := filepath.Join(root, "force-app/main/default/emailservices/Beta.xml")
	for _, path := range []string{b, a, filepath.Join(root, "force-app/main/default/other/Unrelated.xml")} {
		writeFile(t, path, `<EmailServicesFunction/>`)
	}
	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var fields struct {
		Files []string `json:"emailServiceFiles"`
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fields.Files, []string{a, b}) {
		t.Fatalf("email-service discovery/order/deduplication = %v, want %v", fields.Files, []string{a, b})
	}
}
