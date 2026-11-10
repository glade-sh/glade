package compile

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestOwnNamespaceSchemaAliases(t *testing.T) {
	objects := map[string]salesforceSchemaObject{
		"acme__Widget__c": {
			Fields:        map[string]bool{"Name": true, "acme__Size__c": true, "acme__Gadget__c": true},
			Relationships: map[string][]string{"acme__Gadget__r": {"acme__Gadget__c"}},
		},
		"acme__Gadget__c": {Fields: map[string]bool{"acme__Color__c": true}, Relationships: map[string][]string{}},
		"Account":         {Fields: map[string]bool{"Name": true, "acme__Tier__c": true, "other__Code__c": true}, Relationships: map[string][]string{}},
		"Gizmo__c":        {Fields: map[string]bool{"Name": true}, Relationships: map[string][]string{}},
		"acme__Gizmo__c":  {Fields: map[string]bool{"acme__Weight__c": true}, Relationships: map[string][]string{}},
	}
	addOwnNamespaceSchemaAliases(objects, "acme")

	widget, ok := objects["Widget__c"]
	if !ok {
		t.Fatal("Widget__c alias missing")
	}
	for _, field := range []string{"Size__c", "acme__Size__c", "Name"} {
		if !widget.Fields[field] {
			t.Errorf("Widget__c.%s missing", field)
		}
	}
	if got := widget.Relationships["Gadget__r"]; len(got) != 2 || got[0] != "acme__Gadget__c" || got[1] != "Gadget__c" {
		t.Errorf("Widget__c.Gadget__r = %v", got)
	}
	if !objects["Account"].Fields["Tier__c"] {
		t.Error("Account.Tier__c alias missing")
	}
	if objects["Account"].Fields["Code__c"] {
		t.Error("another namespace's field must keep its prefix")
	}
	if objects["Gizmo__c"].Fields["acme__Weight__c"] {
		t.Error("an object declared without the prefix must not be replaced")
	}
	if _, ok := objects["Name"]; ok {
		t.Error("a name without a custom suffix after the prefix must not be aliased")
	}

	plain := map[string]salesforceSchemaObject{"acme__Widget__c": {Fields: map[string]bool{}, Relationships: map[string][]string{}}}
	addOwnNamespaceSchemaAliases(plain, "")
	if len(plain) != 1 {
		t.Errorf("no namespace must add no aliases: %v", plain)
	}
}

func TestSalesforceImportSchemaFollowsMultiCurrencyOrgShape(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		root := t.TempDir()
		writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"64.0"}`)
		features := `[]`
		if enabled {
			features = `["MultiCurrency"]`
		}
		writeCompileFixtureFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"edition":"Developer","features":`+features+`}`)
		writeCompileFixtureFile(t, filepath.Join(root, "force-app", "main", "default", "objects", "Widget__c", "Widget__c.object-meta.xml"),
			`<?xml version="1.0" encoding="UTF-8"?><CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Widget</label><pluralLabel>Widgets</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := loadSalesforceImports(p, []string{"@salesforce/schema/Account.CurrencyIsoCode"})
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.Schema["Widget__c"].Fields["CurrencyIsoCode"]; got != enabled {
			t.Errorf("MultiCurrency=%v: Widget__c.CurrencyIsoCode = %v", enabled, got)
		}
		if got := cfg.Schema["Account"].Fields["CurrencyIsoCode"]; got != enabled {
			t.Errorf("MultiCurrency=%v: Account.CurrencyIsoCode = %v", enabled, got)
		}
	}
}
