package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

// Constructor initialization must retain ordinary custom-object provenance:
// stripInaccessible uses the defaulted marker without populating the field.
func TestConstructedDefaultsNewRoutePreservesCustomObjectProvenance(t *testing.T) {
	for _, name := range []string{"ParcelEntry__c", "parcel__ParcelEntry__c", "ParcelEntry__mdt", "parcel__ParcelEntry__mdt"} {
		t.Run(name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, Fields: map[string]storage.Field{
				"Ready__c": {APIName: "Ready__c", Type: storage.FieldBoolean, DefaultValue: "false"},
			}}}
			machine := New(nil)
			machine.SetOrg(&org)
			row, err := machine.constructValue(name, nil, nil, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			metadata := storage.IsCustomMetadataDefinition(org.Objects[name].Definition)
			value, present := row.Fields["Ready__c"]
			if metadata {
				if present {
					t.Fatalf("new metadata: schema default materialized: %v", value)
				}
			} else if !present || value.Kind != ValueBool || value.Bool || !isDefaultedSObjectField(row, "Ready__c") {
				t.Fatalf("new custom object lost default/provenance: present=%t value=%v defaulted=%t", present, value, isDefaultedSObjectField(row, "Ready__c"))
			}
		})
	}
}

// Loading locally named records into a namespaced schema must not invent an
// empty second type. Every construction route retains the canonical identity.
func TestMetadataNamespaceRoutesIgnoreEmptySchemaPlaceholders(t *testing.T) {
	routes := []struct{ name, expression string }{
		{"new", "new DefaultProbe__mdt()"},
		{"jsonTyped", "(DefaultProbe__mdt)JSON.deserialize('{}',DefaultProbe__mdt.class)"},
		{"jsonGeneric", "(DefaultProbe__mdt)JSON.deserialize('{\"attributes\":{\"type\":\"DefaultProbe__mdt\"}}',SObject.class)"},
		{"jsonMapDynamic", "(DefaultProbe__mdt)JSON.deserialize(JSON.serialize(new Map<String,Object>{'DeveloperName'=>'Product'}),Type.forName('Schema.DefaultProbe__mdt'))"},
		{"newSObject", "(DefaultProbe__mdt)DefaultProbe__mdt.SObjectType.newSObject()"},
		{"clone", "new DefaultProbe__mdt().clone(false,true,false,false)"},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Namespace = "sprout"

			org.Objects["sprout__DefaultProbe__mdt"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "sprout__DefaultProbe__mdt", Fields: map[string]storage.Field{
				"DeveloperName": {APIName: "DeveloperName", Type: storage.FieldString},
			}}}
			if err := storage.ApplyCustomMetadataRecords(&org, []schema.CustomMetadataRecord{{ObjectName: "DefaultProbe__mdt", DeveloperName: "Product"}}); err != nil {
				t.Fatal(err)
			}
			if _, exists := org.Objects["DefaultProbe__mdt"]; exists {
				t.Error("metadata loading created a spurious local schema alias")
			}
			machine := New(nil)
			machine.SetOrg(&org)
			program, err := CompileAnonymous("DefaultProbe__mdt row=" + route.expression + `;
Schema.SObjectType token=row.getSObjectType();
System.assertEquals('sprout__DefaultProbe__mdt',token.getDescribe().getName(),'token describe');
System.assertEquals('sprout__DefaultProbe__mdt',String.valueOf(token),'token display');
System.assertEquals('sprout__DefaultProbe__mdt',Type.forName('Schema.DefaultProbe__mdt').getName(),'reflection name');
Metadata.CustomMetadata dto=new Metadata.CustomMetadata();
dto.fullName=token.getDescribe().getName().removeEnd('__mdt')+'.Product';
System.assertEquals('sprout__DefaultProbe.Product',dto.fullName,'metadata DTO');`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
