package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestOrgFromIndexPreservesOnlyProjectCustomSchema(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"pkg","sourceApiVersion":"66.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/LocalConfig__mdt/LocalConfig__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Local Config</label><visibility>Public</visibility></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/objects/LocalRecord__c/LocalRecord__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Local Record</label><pluralLabel>Local Records</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	org := orgFromIndex(loadTestIndex(t, root))
	for _, name := range storage.KnownStandardObjectNames() {
		if strings.HasSuffix(strings.ToLower(name), "__mdt") {
			t.Errorf("project schema leaked into standard object catalog: %s", name)
		}
	}
	metadata, ok := storage.ResolveObjectName(org, "LocalConfig__mdt")
	if !ok {
		t.Fatal("declared custom metadata lost")
	}
	if got := org.Objects[metadata].Definition.KeyPrefix; got != "m00" {
		t.Errorf("sole declared metadata prefix = %s; want m00", got)
	}
	record, ok := storage.ResolveObjectName(org, "LocalRecord__c")
	if !ok || org.Objects[record].Definition.Fields["Name"].APIName != "Name" {
		t.Fatal("declared custom object or Name field lost")
	}
	for _, name := range []string{"Knowledge__ka", "Knowledge__kav", "Knowledge__Feed"} {
		if !storage.IsKnownStandardObject(name) {
			t.Errorf("platform Knowledge shape lost: %s", name)
		}
	}
}
