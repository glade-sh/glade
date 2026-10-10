package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestQualifiedFieldTokenKeepsTypePropertiesAndUnknownFieldErrors(t *testing.T) {
	for _, namespace := range []string{"", "pkgx"} {
		t.Run(namespace, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Namespace = namespace
			name := "Owned__mdt"
			if namespace != "" {
				name = namespace + "__" + name
			}
			org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, Label: "Owned", KeyPrefix: "m00", Fields: map[string]storage.Field{
				"Date_Field__c": {APIName: "Date_Field__c", Type: storage.FieldReference, ReferenceTo: []string{"FieldDefinition"}},
			}}}
			machine := New(nil)
			machine.SetOrg(&org)
			token, err := machine.lookup("Schema.Owned__mdt.Date_Field__c")
			if err != nil || token.Type != "Schema.SObjectField" || token.Fields["object"].Text != name {
				t.Fatalf("field token = %#v, %v", token, err)
			}
			label, err := machine.lookup("Schema.Owned__mdt.label")
			if err != nil || label.Text != "Owned" {
				t.Fatalf("type label = %#v, %v", label, err)
			}
			fields, err := machine.lookup("Schema.Owned__mdt.fields")
			if err != nil || fields.Type != "Schema.SObjectFieldMap" {
				t.Fatalf("fields = %#v, %v", fields, err)
			}
			if _, err := machine.lookup("Schema.Owned__mdt.Missing__c"); err == nil {
				t.Fatal("unknown schema field accepted")
			}
		})
	}
}
