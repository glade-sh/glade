package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

const workspaceMetadataXML = `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>Seed</label><protected>false</protected><values><field>Flag__c</field><value xsi:type="xsd:boolean">true</value></values></CustomMetadata>`

// A project that loads a sibling project as a package dependency must read the
// sibling's custom metadata, including when both projects declare a record
// with the same name and source-unit discovery runs.
func TestMetadataGenerationReadsSiblingProjectCustomMetadata(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		name := "unique"
		if duplicate {
			name = "duplicate-record"
		}
		t.Run(name, func(t *testing.T) {
			workspace := t.TempDir()
			alpha := filepath.Join(workspace, "ProjAlpha")
			beta := filepath.Join(workspace, "ProjBeta")
			writeFile(t, filepath.Join(alpha, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true,"package":"ProjAlpha","dependencies":[{"package":"ProjBeta"}]}]}`)
			writeFile(t, filepath.Join(beta, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"sfdx-source/beta/main","default":true,"package":"ProjBeta"}]}`)
			writeFile(t, filepath.Join(alpha, "force-app/main/default/classes/AlphaTest.cls"), `@IsTest private class AlphaTest { @IsTest static void check() { System.assertEquals(1, 1); } }`)
			betaSchema := filepath.Join(beta, "sfdx-source/beta/main/schema")
			writeFile(t, filepath.Join(betaSchema, "objects/Widget__mdt/Widget__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Widget</label><pluralLabel>Widgets</pluralLabel></CustomObject>`)
			writeFile(t, filepath.Join(betaSchema, "objects/Widget__mdt/fields/Flag__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Flag__c</fullName><label>Flag</label><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`)
			writeFile(t, filepath.Join(betaSchema, "customMetadata/Widget__mdt.Seed.md-meta.xml"), workspaceMetadataXML)
			if duplicate {
				writeFile(t, filepath.Join(alpha, "force-app/main/default/objects/Widget__mdt/Widget__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Widget</label><pluralLabel>Widgets</pluralLabel></CustomObject>`)
				writeFile(t, filepath.Join(alpha, "force-app/main/default/objects/Widget__mdt/fields/Flag__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Flag__c</fullName><label>Flag</label><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`)
				writeFile(t, filepath.Join(alpha, "force-app/main/default/customMetadata/Widget__mdt.Seed.md-meta.xml"), workspaceMetadataXML)
			}
			p, err := project.Load(alpha)
			if err != nil {
				t.Fatal(err)
			}
			loaded := false
			for _, dep := range p.ManagedPackageDependencies {
				loaded = loaded || (dep.Status == "loaded" && dep.Project != nil)
			}
			if !loaded {
				t.Fatalf("sibling dependency was not loaded: %#v %#v", p.ManagedPackageDependencies, p.DependencyDiagnostics)
			}
			index := loadTestIndex(t, alpha)
			run := Run(index, Options{ProjectSnapshot: &p, NoDiskCache: true, Parallelism: 1})
			if problem := firstRunProblem(run); strings.Contains(problem, "outside source root") || strings.Contains(problem, "CompileError") {
				t.Fatalf("sibling project metadata rejected: %q", problem)
			}
			if s := run.Summary(); s.Failed != 0 || s.Passed == 0 {
				t.Fatalf("summary = %#v", s)
			}
		})
	}
}
