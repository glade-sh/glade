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

// SOURCE ONLY: imported API67 metadata; uncompiled and unexecuted.
// Formula bodies are inspected through describe metadata and never evaluated.
// Root owns admission, diagnostics, actual red, product tests and integration.
func TestExecImportedHTMLFormulaMetadataAPI67(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"namespace":"","sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}
`,
		"force-app/main/default/classes/GladeHTMLFormula67Proof.cls": `@IsTest private class GladeHTMLFormula67Proof {
    @IsTest static void importedPlainTextControl() {
        Schema.DescribeFieldResult field = HTMLProbe__c.PlainText__c.getDescribe();
        System.assertEquals(false, field.isCalculated());
        System.assertEquals(null, field.getCalculatedFormula());
        System.assertEquals(false, field.isHtmlFormatted());
    }
    @IsTest static void importedNumberFormulaControl() {
        Schema.DescribeFieldResult field = HTMLProbe__c.NumberFormula__c.getDescribe();
        System.assertEquals(true, field.isCalculated());
        System.assertEquals('1 + 1', field.getCalculatedFormula());
        System.assertEquals(false, field.isHtmlFormatted());
    }
    @IsTest static void importedHyperlinkFocal() {
        Schema.DescribeFieldResult field = HTMLProbe__c.Hyperlink__c.getDescribe();
        System.assertEquals(true, field.isCalculated());
        System.assertEquals('HYPERLINK("https://example.invalid", "Open")', field.getCalculatedFormula());
        System.assertEquals(true, field.isHtmlFormatted());
    }
    @IsTest static void importedImageFocal() {
        Schema.DescribeFieldResult field = HTMLProbe__c.Image__c.getDescribe();
        System.assertEquals(true, field.isCalculated());
        System.assertEquals('IMAGE("https://example.invalid/icon.png", "Icon")', field.getCalculatedFormula());
        System.assertEquals(true, field.isHtmlFormatted());
    }
}
`,
		"force-app/main/default/classes/GladeHTMLFormula67Proof.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>
`,
		"force-app/main/default/objects/HTMLProbe__c/HTMLProbe__c.object-meta.xml": `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>HTML Probe</label><pluralLabel>HTML Probes</pluralLabel><nameField><label>HTML Probe Name</label><type>Text</type></nameField><sharingModel>ReadWrite</sharingModel></CustomObject>
`,
		"force-app/main/default/objects/HTMLProbe__c/fields/PlainText__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>PlainText__c</fullName><label>Plain Text</label><length>40</length><type>Text</type></CustomField>
`,
		"force-app/main/default/objects/HTMLProbe__c/fields/NumberFormula__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>NumberFormula__c</fullName><label>Number Formula</label><type>Number</type><precision>12</precision><scale>2</scale><formula>1 + 1</formula></CustomField>
`,
		"force-app/main/default/objects/HTMLProbe__c/fields/Hyperlink__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Hyperlink__c</fullName><label>Hyperlink</label><type>Text</type><formula>HYPERLINK("https://example.invalid", "Open")</formula></CustomField>
`,
		"force-app/main/default/objects/HTMLProbe__c/fields/Image__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Image__c</fullName><label>Image</label><type>Text</type><formula>IMAGE("https://example.invalid/icon.png", "Icon")</formula></CustomField>
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
	describe, err := registry.Describe("HTMLProbe__c")
	if err != nil {
		t.Fatal(err)
	}
	definition := sobject.ToObjectDefinition(describe)
	for _, required := range []struct {
		name    string
		typ     storage.FieldType
		display string
		formula string
	}{
		{"PlainText__c", storage.FieldString, "STRING", ""},
		{"NumberFormula__c", storage.FieldCalculated, "DOUBLE", `1 + 1`},
		{"Hyperlink__c", storage.FieldCalculated, "STRING", `HYPERLINK("https://example.invalid", "Open")`},
		{"Image__c", storage.FieldCalculated, "STRING", `IMAGE("https://example.invalid/icon.png", "Icon")`},
	} {
		field, exists := definition.Fields[required.name]
		if !exists || field.Type != required.typ || field.DisplayType != required.display || field.Formula != required.formula {
			t.Fatalf("imported %s profile = type %q display %q formula %q (exists=%v), want type %q display %q formula %q", required.name, field.Type, field.DisplayType, field.Formula, exists, required.typ, required.display, required.formula)
		}
	}
	index := typesys.Build(p, s)
	base := apextest.Options{
		SelectedClasses:       []string{"GladeHTMLFormula67Proof"},
		RuntimeRESTAPIVersion: "67.0",
		Parallelism:          1,
		NoDiskCache:          true,
	}
	discovered := apextest.Discover(index, base)
	methods := map[string]bool{
		"importedPlainTextControl": false,
		"importedNumberFormulaControl": false,
		"importedHyperlinkFocal": false,
		"importedImageFocal": false,
	}
	if len(discovered) != len(methods) {
		t.Fatalf("discovered %d cases, want exactly %d", len(discovered), len(methods))
	}
	for _, testCase := range discovered {
		seen, known := methods[testCase.MethodName]
		if !known || seen || testCase.ClassName != "GladeHTMLFormula67Proof" || testCase.APIVersion != "67.0" || !testCase.SourceContextBound {
			t.Fatalf("case %s.%s source binding = API %q bound=%v known=%v seen=%v, want unique exact API67 bound method", testCase.ClassName, testCase.MethodName, testCase.APIVersion, testCase.SourceContextBound, known, seen)
		}
		methods[testCase.MethodName] = true
	}
	for _, selected := range []struct {
		name    string
		method  string
		control bool
	}{
		{"case01ImportedPlainTextControl", "importedPlainTextControl", true},
		{"case02ImportedNumberFormulaControl", "importedNumberFormulaControl", true},
		{"case03ImportedHyperlinkFocal", "importedHyperlinkFocal", false},
		{"case04ImportedImageFocal", "importedImageFocal", false},
	} {
		selected := selected
		passed := t.Run(selected.name, func(t *testing.T) {
			options := base
			options.SelectedMethod = selected.method
			run := apextest.Run(index, options)
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 {
				t.Fatalf("%s: %+v; %+v", selected.method, got, run)
			}
		})
		// A failed control stops attribution; both independent focals remain selectable.
		if !passed && selected.control {
			return
		}
	}
}
