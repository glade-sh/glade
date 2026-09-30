package vm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestCustomSharingReasonRowCauseTokenProject(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"force-app/main/default/classes/GladeSharingReason67.cls": `@IsTest private class GladeSharingReason67 {
			@IsTest static void rowCauseTokenIsUsable() {
				String reason = String.valueOf(Schema.GladeSharing67__Share.RowCause.GladeOwnerReason__c);
				System.assertEquals('GladeOwnerReason__c', reason);
				GladeSharing67__Share share = new GladeSharing67__Share(RowCause = Schema.GladeSharing67__Share.RowCause.GladeOwnerReason__c);
				System.assertEquals('GladeOwnerReason__c', share.RowCause);
			}
		}`,
		"force-app/main/default/classes/GladeSharingReason67.cls-meta.xml":                                           `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`,
		"force-app/main/default/objects/GladeSharing67__c/GladeSharing67__c.object-meta.xml":                         `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><enableSharing>true</enableSharing><label>Glade Sharing 67</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Glade Sharing 67 Rows</pluralLabel><sharingModel>Private</sharingModel></CustomObject>`,
		"force-app/main/default/objects/GladeSharing67__c/sharingReasons/GladeOwnerReason__c.sharingReason-meta.xml": `<?xml version="1.0" encoding="UTF-8"?><SharingReason xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>GladeOwnerReason__c</fullName><label>Glade Owner Reason</label></SharingReason>`,
		"sfdx-project.json": `{"namespace":"","sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`,
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, object := range s.Objects {
		if object.Name == "GladeSharing67__c" {
			found = len(object.SharingReasons) == 1 && object.SharingReasons[0] == "GladeOwnerReason__c"
		}
	}
	if !found {
		t.Fatalf("sharing reason metadata was not loaded: %#v", s.Objects)
	}
	r := apextest.Run(typesys.Build(p, s), apextest.Options{NoDiskCache: true})
	if got := r.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 {
		t.Fatalf("custom sharing reason: %+v; %+v", got, r)
	}
}

func TestNamespacedCustomSharingReasonSchemaShareTokensProject(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"force-app/main/default/classes/NebulaSharingReason65.cls": `@IsTest private class NebulaSharingReason65 {
			@IsTest static void schemaShareTokensUseLocalNames() {
				Schema.SObjectField accessLevel = Schema.Log__Share.AccessLevel;
				System.assertEquals('AccessLevel', accessLevel.getDescribe().getName());
				System.assertEquals('LoggedByUser__c', String.valueOf(Schema.Log__Share.RowCause.LoggedByUser__c));
			}
		}`,
		"force-app/main/default/classes/NebulaSharingReason65.cls-meta.xml":                           `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>`,
		"force-app/main/default/objects/Log__c/Log__c.object-meta.xml":                                `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><enableSharing>true</enableSharing><label>Log</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Logs</pluralLabel><sharingModel>Private</sharingModel></CustomObject>`,
		"force-app/main/default/objects/Log__c/sharingReasons/LoggedByUser__c.sharingReason-meta.xml": `<?xml version="1.0" encoding="UTF-8"?><SharingReason xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>LoggedByUser__c</fullName><label>Log Created By User</label></SharingReason>`,
		"sfdx-project.json": `{"namespace":"Nebula","sourceApiVersion":"65.0","packageDirectories":[{"path":"force-app","default":true}]}`,
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	r := apextest.Run(typesys.Build(p, s), apextest.Options{NoDiskCache: true})
	if got := r.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 {
		problem := ""
		if len(r.Suites) > 0 && len(r.Suites[0].Cases) > 0 && r.Suites[0].Cases[0].Problem != nil {
			problem = r.Suites[0].Cases[0].Problem.Message
		}
		t.Fatalf("namespaced custom sharing reason: %+v; problem=%q", got, problem)
	}
}
