package compile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// A nested project retains its own namespace; a consumer's source-specific
// admission rule must not be imposed on that project's bundles.
func TestSchemaImportRespectsSourceNamespaceOwnership(t *testing.T) {
	root := t.TempDir()
	writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"local","default":true},{"path":"nested/force-app"}],"namespace":"fixtureown","sourceApiVersion":"67.0"}`)
	writeCompileFixtureFile(t, filepath.Join(root, "nested", "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"fixtureother","sourceApiVersion":"67.0"}`)
	object, err := os.ReadFile("testdata/pr163_namespaced_schema/object.xml")
	if err != nil {
		t.Fatal(err)
	}
	writeCompileFixtureFile(t, filepath.Join(root, "local", "main", "default", "objects", "CaptureNsK471__c", "CaptureNsK471__c.object-meta.xml"), string(object))
	for _, entry := range []struct{ directory, name string }{{"local", "consumer"}, {"nested/force-app", "foreign"}, {"nested/force-app", "foreignshort"}} {
		bundle := filepath.Join(root, entry.directory, "main", "default", "lwc", entry.name)
		objectName := "fixtureown__CaptureNsK471__c"
		if entry.name == "foreignshort" {
			objectName = "CaptureNsK471__c"
		}
		source := `import { LightningElement } from 'lwc'; import FIELD from '@salesforce/schema/` + objectName + `.Name'; export default class Probe extends LightningElement { field = FIELD; }`
		writeCompileFixtureFile(t, filepath.Join(bundle, entry.name+".js"), source)
		writeCompileFixtureFile(t, filepath.Join(bundle, entry.name+".html"), `<template><p>Test</p></template>`)
		writeCompileFixtureFile(t, filepath.Join(bundle, entry.name+".js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.SourceNamespaceForFile(filepath.Join(root, "nested", "force-app", "main", "default", "lwc", "foreign", "foreign.js"), p.Namespace); got != "fixtureother" {
		t.Fatalf("nested owner = %q", got)
	}
	repo, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	results, err := CompileBatch(p, Options{OutDir: filepath.Join(root, "dist"), RepoRoot: repo})
	if err != nil {
		t.Fatal(err)
	}
	consumer := results["local/main/default/lwc/consumer"]
	if consumer.Err == nil || !strings.Contains(consumer.Err.Error(), "LWC1504") {
		t.Fatalf("own source = %v, want LWC1504", consumer.Err)
	}
	foreign := results["nested/force-app/main/default/lwc/foreign"]
	if foreign.Err != nil {
		t.Errorf("foreign source lost prior resolution: %v", foreign.Err)
	}
	foreignShort := results["nested/force-app/main/default/lwc/foreignshort"]
	if foreignShort.Err == nil || !strings.Contains(foreignShort.Err.Error(), "Invalid reference") {
		t.Errorf("foreign source accepted consumer-only alias: %v", foreignShort.Err)
	}
}
