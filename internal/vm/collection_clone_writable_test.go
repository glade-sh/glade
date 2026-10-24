package vm

import "testing"

// Native collection_clone_writable observations cover the same List and Map
// routes through anonymous DML, test DML, nested triggers and Queueable DML.
func TestCollectionDeepCloneWritableRecordRoutes(t *testing.T) {
	for _, route := range []string{"list-default", "list-clear", "list-preserve", "map", "single"} {
		t.Run(route, func(t *testing.T) {
			machine := New(nil)
			source := Object("Account")
			source.Fields["Id"] = String("001000000000001AAA")
			source.Fields["Name"] = String("Original")
			source.Fields[sobjectReadOnlyField] = String("trigger record")
			source.Fields[sobjectErrorsField] = List(Object("Database.Error"))
			var copied Value
			var handled bool
			var err error
			switch route {
			case "single":
				copied, handled, err = machine.callSObjectMember(source, "clone", []Value{Bool(true), Bool(true), Bool(true), Bool(true)})
			case "map":
				collection := Map()
				collection.Type = "Map<String,Account>"
				collection.Map[mapKey(String("record"))] = source
				collection, handled, err = machine.callMapValueMember("", collection, "deepClone", nil, &Result{})
				copied = collection.Map[mapKey(String("record"))]
			default:
				collection := List(source, Null)
				collection.Type = "List<Account>"
				var args []Value
				if route == "list-clear" {
					args = []Value{Bool(false), Bool(false), Bool(false)}
				} else if route == "list-preserve" {
					args = []Value{Bool(true), Bool(true), Bool(true)}
				}
				collection, handled, err = machine.callListValueMember("", collection, "deepClone", args, &Result{})
				copied = collection.List[0]
				if collection.List[1].Kind != ValueNull {
					t.Fatal("null list element changed")
				}
			}
			if err != nil || !handled {
				t.Fatalf("clone: handled=%t, err=%v", handled, err)
			}
			if _, readOnly := sobjectReadOnlyReason(copied); readOnly {
				t.Error("deep clone retained trigger read-only marker")
			}
			if _, errors := copied.Fields[sobjectErrorsField]; errors {
				t.Error("deep clone retained source validation errors")
			}
			if copied.Ref == source.Ref {
				t.Fatal("clone shares source identity")
			}
			copied.Fields["Name"] = String("Changed")
			if source.Fields["Name"].Text != "Original" {
				t.Error("clone mutation changed source")
			}
			if _, readOnly := sobjectReadOnlyReason(source); !readOnly {
				t.Error("clone removed source write guard")
			}
			if _, errors := source.Fields[sobjectErrorsField]; !errors {
				t.Error("clone removed source validation errors")
			}
			if route == "list-default" || route == "list-clear" {
				if _, retained := copied.Fields["Id"]; retained {
					t.Error("clone retained Id despite preserveId=false")
				}
			} else if copied.Fields["Id"].Text != source.Fields["Id"].Text {
				t.Error("clone changed preserved Id")
			}
		})
	}
}

func TestMapDeepClonePreservesNonRecordValueRoutes(t *testing.T) {
	machine := New(nil)
	payload := Object("ClonePayload")
	payload.Fields["Name"] = String("Original")
	payload.Fields[sobjectReadOnlyField] = String("payload state")
	payload.Fields[sobjectErrorsField] = List(String("payload errors"))
	original := Map()
	original.Type = "Map<String,Object>"
	original.Map["payload"] = payload
	original.Map["absent"] = Null
	original.Map["scalar"] = String("Unchanged")
	original.Map["nested"] = List(String("Nested"))
	copy, handled, err := machine.callMapValueMember("", original, "deepClone", nil, &Result{})
	if err != nil || !handled {
		t.Fatalf("clone: handled=%t, err=%v", handled, err)
	}
	if copy.Map["absent"].Kind != ValueNull || copy.Map["scalar"].Text != "Unchanged" || copy.Map["nested"].List[0].Text != "Nested" {
		t.Fatal("map clone changed non-record values")
	}
	cloned := copy.Map["payload"]
	if cloned.Fields[sobjectReadOnlyField].Text != "payload state" || len(cloned.Fields[sobjectErrorsField].List) != 1 {
		t.Fatal("map clone stripped non-record payload fields")
	}
	cloned.Fields["Name"] = String("Changed")
	if payload.Fields["Name"].Text != "Original" || cloned.Ref == payload.Ref {
		t.Fatal("map clone retained payload identity")
	}
}
