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

// SOURCE ONLY: uncompiled/unexecuted imported API67 unique Text describe metadata.
// Keep actual imported CaseSensitive values; never inject or gate on the focal flag.
// Root owns independent acceptance, admission, execution, product and integration.
func TestExecImportedCaseSensitiveTextMetadataAPI67(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"namespace":"","sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}
`,
		"force-app/main/default/classes/GladeCaseSensitive67Proof.cls": `@IsTest private class GladeCaseSensitive67Proof {
    @IsTest static void importedCaseInsensitiveControl() {
        Schema.DescribeFieldResult field = CaseProbe__c.Insensitive__c.getDescribe();
        System.assertEquals('Insensitive__c', field.getName());
        System.assertEquals(Schema.DisplayType.STRING, field.getType());
        System.assertEquals(true, field.isUnique());
        System.assertEquals(false, field.isCaseSensitive());
    }
    @IsTest static void importedCaseSensitiveFocal() {
        Schema.DescribeFieldResult field = CaseProbe__c.Sensitive__c.getDescribe();
        System.assertEquals('Sensitive__c', field.getName());
        System.assertEquals(Schema.DisplayType.STRING, field.getType());
        System.assertEquals(true, field.isUnique());
        System.assertEquals(true, field.isCaseSensitive());
    }
}
`,
		"force-app/main/default/classes/GladeCaseSensitive67Proof.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>
`,
		"force-app/main/default/objects/CaseProbe__c/CaseProbe__c.object-meta.xml": `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Case Probe</label><pluralLabel>Case Probes</pluralLabel><nameField><label>Case Probe Name</label><type>Text</type></nameField><sharingModel>ReadWrite</sharingModel></CustomObject>
`,
		"force-app/main/default/objects/CaseProbe__c/fields/Insensitive__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Insensitive__c</fullName><label>Insensitive</label><length>40</length><type>Text</type><unique>true</unique><caseSensitive>false</caseSensitive><externalId>false</externalId><required>false</required></CustomField>
`,
		"force-app/main/default/objects/CaseProbe__c/fields/Sensitive__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Sensitive__c</fullName><label>Sensitive</label><length>40</length><type>Text</type><unique>true</unique><caseSensitive>true</caseSensitive><externalId>false</externalId><required>false</required></CustomField>
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
	describe, err := registry.Describe("CaseProbe__c")
	if err != nil {
		t.Fatal(err)
	}
	definition := sobject.ToObjectDefinition(describe)
	for _, name := range []string{"Insensitive__c", "Sensitive__c"} {
		field, exists := definition.Fields[name]
		if !exists || field.Type != storage.FieldString || field.DisplayType != "STRING" || field.Length != 40 || !field.Unique || field.ExternalID || field.Required || field.Formula != "" {
			t.Fatalf("imported %s profile invalid: exists=%v field=%+v", name, exists, field)
		}
		// Observe the real importer output without requiring the missing focal value.
		t.Logf("CASESENSITIVE_IMPORTED_FIELD name=%s caseSensitive=%t", name, field.CaseSensitive)
	}
	index := typesys.Build(p, s)
	base := apextest.Options{
		SelectedClasses:       []string{"GladeCaseSensitive67Proof"},
		RuntimeRESTAPIVersion: "67.0",
		Parallelism:          1,
		NoDiskCache:          true,
	}
	discovered := apextest.Discover(index, base)
	methods := map[string]bool{
		"importedCaseInsensitiveControl": false,
		"importedCaseSensitiveFocal": false,
	}
	if len(discovered) != len(methods) {
		t.Fatalf("discovered %d cases, want exactly %d", len(discovered), len(methods))
	}
	classFile := filepath.Join(root, "force-app/main/default/classes/GladeCaseSensitive67Proof.cls")
	for _, testCase := range discovered {
		seen, known := methods[testCase.MethodName]
		if !known || seen || testCase.ClassName != "GladeCaseSensitive67Proof" || testCase.File != classFile || testCase.APIVersion != "67.0" || !testCase.SourceContextBound {
			t.Fatalf("case %s.%s source binding = file %q API %q bound=%v known=%v seen=%v, want unique exact imported API67 bound method", testCase.ClassName, testCase.MethodName, testCase.File, testCase.APIVersion, testCase.SourceContextBound, known, seen)
		}
		methods[testCase.MethodName] = true
	}
	for _, selected := range []struct {
		name   string
		method string
	}{
		{"case01ImportedCaseInsensitiveControl", "importedCaseInsensitiveControl"},
		{"case02ImportedCaseSensitiveFocal", "importedCaseSensitiveFocal"},
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
			// A failed control stops focal attribution; a failed focal stops this top.
			return
		}
	}
}
