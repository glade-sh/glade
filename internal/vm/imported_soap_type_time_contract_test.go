package vm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// SOURCE ONLY: uncompiled/unexecuted; exactly imported Text control then Time focal.
// Require actual imported storage.FieldTime, never a hand-assigned display label.
// Root owns admission/execution; no org interaction or frozen-package modification.
func TestExecImportedSOAPTypeTextThenTimeAPI67(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"namespace":"","sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}
`,
		"force-app/main/default/classes/GladeSOAPTypeTime67Proof.cls": `@IsTest private class GladeSOAPTypeTime67Proof {
    @IsTest static void importedTextControl() {
        System.assertEquals(Schema.SOAPType.String, EnumProbe__c.Text__c.getDescribe().getSoapType());
    }
    @IsTest static void importedTimeFocal() {
        System.assertEquals(Schema.SOAPType.Time, EnumProbe__c.StartTime__c.getDescribe().getSoapType());
    }
}
`,
		"force-app/main/default/classes/GladeSOAPTypeTime67Proof.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>
`,
		"force-app/main/default/objects/EnumProbe__c/EnumProbe__c.object-meta.xml": `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Enum Probe</label><pluralLabel>Enum Probes</pluralLabel><nameField><label>Enum Probe Name</label><type>Text</type></nameField><sharingModel>ReadWrite</sharingModel></CustomObject>
`,
		"force-app/main/default/objects/EnumProbe__c/fields/Text__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Text__c</fullName><label>Text</label><length>40</length><type>Text</type></CustomField>
`,
		"force-app/main/default/objects/EnumProbe__c/fields/StartTime__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>StartTime__c</fullName><label>Start Time</label><type>Time</type></CustomField>
`,
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if p.SourceAPIVersion != "67.0" {
		t.Fatalf("project source API = %q, want 67.0", p.SourceAPIVersion)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	registry := sobject.BuildDescribeRegistry(s)
	describe, err := registry.Describe("EnumProbe__c")
	if err != nil {
		t.Fatal(err)
	}
	definition := sobject.ToObjectDefinition(describe)
	for _, required := range []struct {
		name string
		typ  storage.FieldType
	}{
		{"Text__c", storage.FieldString},
		{"StartTime__c", storage.FieldTime},
	} {
		field, exists := definition.Fields[required.name]
		if !exists || field.Type != required.typ {
			t.Fatalf("imported %s storage type = %q (exists=%v), want %q", required.name, field.Type, exists, required.typ)
		}
	}
	index := typesys.Build(p, s)
	base := apextest.Options{
		SelectedClasses:       []string{"GladeSOAPTypeTime67Proof"},
		RuntimeRESTAPIVersion: "67.0",
		Parallelism:          1,
		NoDiskCache:          true,
	}
	discovered := apextest.Discover(index, base)
	if len(discovered) != 2 {
		t.Fatalf("discovered %d cases, want exactly 2", len(discovered))
	}
	for _, testCase := range discovered {
		if testCase.APIVersion != "67.0" || !testCase.SourceContextBound {
			t.Fatalf("case %s.%s source binding = API %q bound=%v, want API67 bound", testCase.ClassName, testCase.MethodName, testCase.APIVersion, testCase.SourceContextBound)
		}
	}
	for _, selected := range []struct {
		name   string
		method string
	}{
		{"case01ImportedTextControl", "importedTextControl"},
		{"case02ImportedTimeFocal", "importedTimeFocal"},
	} {
		selected := selected
		if !t.Run(selected.name, func(t *testing.T) {
			options := base
			options.SelectedMethod = selected.method
			run := apextest.Run(index, options)
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 {
				t.Fatalf("%s: %+v; %+v", selected.method, got, run)
			}
		}) {
			return
		}
	}
}
