package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStandardChildRelationshipSurvivesProjectParentAlias(t *testing.T) {
	placeholder := schema.Object{Name: "Parent__c", Partial: true}
	parent := schema.Object{Name: "hed__Parent__c", Fields: []schema.Field{{Name: "hed__Marker__c", Type: "Text"}}}
	contact := schema.Object{Name: "Contact", Fields: []schema.Field{{
		Name: "Parent__c", Type: "Lookup", ReferenceTo: []string{"Parent__c"},
		RelationshipName: "Parent__r", ChildRelationshipName: "Contacts1__r", ChildRelationshipNameInferred: true,
	}}}
	for _, test := range []struct {
		name    string
		objects []schema.Object
	}{
		{"placeholder child parent", []schema.Object{placeholder, contact, parent}},
		{"parent child placeholder", []schema.Object{parent, contact, placeholder}},
		{"child placeholder parent", []schema.Object{contact, placeholder, parent}},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := buildSemaTypeMemberView(typesys.Index{Project: typesys.ProjectInfo{Namespace: "hed"}, Objects: test.objects})
			for _, parentName := range []string{"Parent__c", "hed__Parent__c"} {
				member, ok := semaResolveField(model, parentName, "Contacts1__r", map[string]bool{})
				if !ok || member.member.Type != "List<Contact>" {
					t.Fatalf("%s child relationship = %#v, %v; want List<Contact>", parentName, member, ok)
				}
				if _, ok := semaResolveField(model, parentName, "Marker__c", map[string]bool{}); !ok {
					t.Fatalf("%s lost declared parent field", parentName)
				}
			}
		})
	}
}

func TestStandardChildRelationshipKeepsExplicitForeignParent(t *testing.T) {
	model := buildSemaTypeMemberView(typesys.Index{
		Project: typesys.ProjectInfo{Namespace: "hed"},
		Objects: []schema.Object{
			{Name: "hed__Parent__c"},
			{Name: "foreign__Parent__c"},
			{Name: "Contact", Fields: []schema.Field{{
				Name: "ForeignParent__c", Type: "Lookup", ReferenceTo: []string{"foreign__Parent__c"},
				ChildRelationshipName: "ForeignContacts__r", ChildRelationshipNameInferred: true,
			}}},
		},
	})
	if _, ok := semaResolveField(model, "hed__Parent__c", "ForeignContacts__r", map[string]bool{}); ok {
		t.Fatal("foreign parent relationship leaked into project parent")
	}
	member, ok := semaResolveField(model, "foreign__Parent__c", "ForeignContacts__r", map[string]bool{})
	if !ok || member.member.Type != "List<Contact>" {
		t.Fatalf("explicit foreign parent relationship = %#v, %v", member, ok)
	}
}
