package storage

import (
	"reflect"
	"testing"
)

func TestStubFieldsRetainCapturedWritePermissions(t *testing.T) {
	definition, ok := StandardObjectDefinition("QueueSobject")
	if !ok {
		t.Fatal("missing QueueSobject")
	}
	for _, name := range []string{"QueueId", "SobjectType"} {
		field := definition.Fields[name]
		if field.Createable == nil || !*field.Createable || field.Updateable == nil || *field.Updateable {
			t.Fatalf("%s flags create=%v update=%v", name, field.Createable, field.Updateable)
		}
	}
	id := definition.Fields["Id"]
	if id.Createable != nil && *id.Createable {
		t.Fatal("record Id unexpectedly createable")
	}
}

func TestStubDescribeEnrichmentPreservesExplicitFlagsAndFieldInventory(t *testing.T) {
	for _, name := range []string{"QueueSobject", "Account"} {
		t.Run(name, func(t *testing.T) {
			fields, ok := standardSObjectStubFieldsFor(name)
			if !ok {
				t.Fatal("missing stub")
			}
			definition := ObjectDefinition{APIName: name, Fields: map[string]Field{}}
			for key, value := range fields {
				field := cloneField(value)
				field.Createable = BoolFlag(false)
				field.Updateable = BoolFlag(true)
				definition.Fields[key] = field
			}
			before := len(definition.Fields)
			mergeStandardSObjectStubFields(&definition, []string{"PersonAccounts"})
			if len(definition.Fields) != before {
				t.Fatalf("field inventory changed: %d -> %d", before, len(definition.Fields))
			}
			for key, field := range definition.Fields {
				if field.Createable == nil || *field.Createable || field.Updateable == nil || !*field.Updateable {
					t.Fatalf("explicit flags overwritten for %s", key)
				}
			}
		})
	}
}

func TestStubWriteFlagsDoNotImportOtherMetadata(t *testing.T) {
	for _, object := range []string{"Group", "ObjectPermissions", "QueueSobject"} {
		t.Run(object, func(t *testing.T) {
			fields, ok := standardSObjectStubFieldsFor(object)
			if !ok {
				t.Fatal("missing stub")
			}
			definition := ObjectDefinition{APIName: object, Fields: map[string]Field{}}
			for name, field := range fields {
				definition.Fields[name] = cloneField(field)
			}
			enrichStandardStubWriteFlagsFromV2Describe(&definition)
			if len(fields) != len(definition.Fields) {
				t.Fatal("field inventory changed")
			}
			for name, before := range fields {
				after := definition.Fields[name]
				after.Createable = before.Createable
				after.Updateable = before.Updateable
				if !reflect.DeepEqual(before, after) {
					t.Fatalf("non-write metadata changed for %s", name)
				}
			}
		})
	}
}
