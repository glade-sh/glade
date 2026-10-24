package vm

import (
	"reflect"
	"testing"
)

// The free coercer has no VM to invalidate. Its object paths must preserve
// identity and fields; scalar conversions may remove, but never add, objects.
func TestFlatObjectAliasIndexFreeCoercionRoutesPreserveObjects(t *testing.T) {
	routes := []struct {
		name string
		call func(string, Value) (Value, error)
	}{
		{"DirectCollection", coerceCollectionValue},
		{"FreeAssignableList", coerceAssignable},
		{"FreeAssignableQueryRecords", func(target string, rows Value) (Value, error) {
			wrapper := Map()
			wrapper.Map[mapKey(String("records"))] = rows
			return coerceAssignable(target, wrapper)
		}},
		{"EnsureAssignable", func(target string, rows Value) (Value, error) {
			return rows, ensureAssignable(target, rows)
		}},
		{"FreeConstructor", func(target string, rows Value) (Value, error) {
			return constructValue(target, rows.List)
		}},
		{"InstanceDefault", func(target string, rows Value) (Value, error) {
			return defaultValue(target, rows), nil
		}},
		{"StaticDefault", func(target string, rows Value) (Value, error) {
			return defaultStaticFieldValue("WriterDefaults", "Rows", target, rows), nil
		}},
	}
	for _, route := range routes {
		for _, target := range []string{"List<Account>", "List<Object>", "List<System.Object>"} {
			t.Run(route.name+"/"+target, func(t *testing.T) {
				machine := New(nil)
				row := Object("Account")
				row.Fields["Name"] = String("before")
				// Duplicate refs with detached fields exercise every occurrence.
				rows := List(row, cloneValuePreserveRefs(row))
				rows.Type = "List<Account>"
				before := cloneValuePreserveRefs(rows)
				warmAliasMutationIndex(t, machine, rows)
				key, _ := flatObjectAliasKey(rows)
				index := machine.flatAliasIndex[key]
				coerced, err := route.call(target, rows)
				if err != nil {
					t.Fatal(err)
				}
				for resultIndex, list := range []Value{rows, coerced} {
					if len(list.List) != len(before.List) {
						t.Fatal("coercion changed list length")
					}
					for i, item := range list.List {
						clonedDefault := resultIndex == 1 && (route.name == "InstanceDefault" || route.name == "StaticDefault")
						identityOK := item.Ref == before.List[i].Ref
						if clonedDefault {
							identityOK = item.Ref != 0 && item.Ref != before.List[i].Ref && &list.List[0] != &rows.List[0]
						}
						if item.Kind != ValueObject || !identityOK || !reflect.DeepEqual(item.Fields, before.List[i].Fields) {
							t.Fatalf("slot %d changed object identity or fields", i)
						}
					}
				}
				if machine.flatAliasIndex[key] != index {
					t.Fatal("identity-preserving free coercion discarded the index")
				}
				updated := cloneValuePreserveRefs(row)
				updated.Fields["Name"] = String("after")
				handled, changed := machine.tryIndexedFlatObjectAliasRoot(rows, snapshotAlias(row), updated, nil)
				if !handled || !changed {
					t.Fatal("retained index did not handle the alias update")
				}
				for i, item := range rows.List {
					if item.Fields["Name"].Text != "after" {
						t.Fatalf("duplicate slot %d missed the alias update", i)
					}
				}
			})
		}
	}
}

func TestFlatObjectAliasIndexFreeCoercionScalarRemovalRoute(t *testing.T) {
	machine := New(nil)
	boxed := Object("ScalarToken")
	boxed.Fields["value"] = String("before")
	rows := List(boxed, cloneValuePreserveRefs(boxed))
	rows.Type = "List<Object>"
	warmAliasMutationIndex(t, machine, rows)
	if _, err := coerceAssignable("List<String>", rows); err != nil {
		t.Fatal(err)
	}
	for _, item := range rows.List {
		if item.Kind != ValueString || item.Ref != 0 || item.Text != "before" {
			t.Fatal("scalar coercion introduced an object or lost its value")
		}
	}
	updated := cloneValuePreserveRefs(boxed)
	updated.Fields["value"] = String("after")
	// Cached positions for an object removed by scalar conversion must reject
	// the fast path rather than write the old object back into the list.
	if handled, _ := machine.tryIndexedFlatObjectAliasRoot(rows, snapshotAlias(boxed), updated, nil); handled {
		t.Fatal("index accepted positions whose objects were removed")
	}
	if len(machine.flatAliasIndex) != 0 || machine.flatAliasIndexSlots != 0 {
		t.Fatal("failed position validation retained the stale index")
	}
}

func TestFlatObjectAliasIndexFreeCoercionNestedRouteFallsBack(t *testing.T) {
	row := Object("Account")
	row.Fields["Name"] = String("before")
	rows := List(List(row))
	rows.Type = "List<List<Account>>"
	if _, ok := buildFlatObjectAliasIndex(rows, nil); ok {
		t.Fatal("nested list unexpectedly eligible before coercion")
	}
	if _, err := coerceAssignable("List<List<Object>>", rows); err != nil {
		t.Fatal(err)
	}
	if _, ok := buildFlatObjectAliasIndex(rows, nil); ok {
		t.Fatal("nested list unexpectedly eligible after coercion")
	}
	if got := rows.List[0].List[0]; got.Ref != row.Ref || !reflect.DeepEqual(got.Fields, row.Fields) {
		t.Fatal("recursive coercion changed nested object identity or fields")
	}
}

func TestFlatObjectAliasIndexDOMAttributeWriterRoutesPreserveObjects(t *testing.T) {
	for _, method := range []string{"setAttribute", "setAttributeNs"} {
		for _, namespace := range []Value{Null, String("urn:after")} {
			t.Run(method+"/"+string(namespace.Kind), func(t *testing.T) {
				machine := New(nil)
				node := newDomXmlNode("ELEMENT", "root", "", "")
				attr := domAttribute("key", "before", "", "")
				attrs := typedList("List<Dom.XmlAttribute>")
				attrs.List = append(attrs.List, attr)
				node.Fields["attributes"] = attrs
				warmAliasMutationIndex(t, machine, attrs)
				key, _ := flatObjectAliasKey(attrs)
				index := machine.flatAliasIndex[key]
				args := []Value{String("key"), String("after")}
				if method == "setAttributeNs" {
					args = append(args, Null, namespace)
				}
				_, _, changed, handled, err := callDomXmlNodeMember(node, method, args)
				if err != nil || !handled || !changed {
					t.Fatalf("attribute write: handled=%v changed=%v err=%v", handled, changed, err)
				}
				got := attrs.List[0]
				if got.Ref != attr.Ref || got.Fields["value"].Text != "after" || !flatObjectAliasValueSafe(got, nil) {
					t.Fatal("attribute replacement changed identity, missed the slot, or introduced nesting")
				}
				if method == "setAttributeNs" && !reflect.DeepEqual(got.Fields["valueNamespace"], namespace) {
					t.Fatal("namespace replacement did not reach the existing slot")
				}
				if machine.flatAliasIndex[key] != index {
					t.Fatal("scalar attribute write discarded the index")
				}
				updated := cloneValuePreserveRefs(got)
				updated.Fields["value"] = String("propagated")
				if handled, changed := machine.tryIndexedFlatObjectAliasRoot(attrs, snapshotAlias(attr), updated, nil); !handled || !changed || attrs.List[0].Fields["value"].Text != "propagated" {
					t.Fatal("retained attribute index missed the alias update")
				}
			})
		}
	}
}

func TestFlatObjectAliasIndexSObjectErrorWriterRoutesInvalidate(t *testing.T) {
	for _, route := range []string{"RecordAddError", "FieldValueAddError"} {
		t.Run(route, func(t *testing.T) {
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			row := Object("Account")
			fields := []string(nil)
			if route == "FieldValueAddError" {
				fields = []string{"Name"}
			}
			addSObjectError(&row, "before", fields, true)
			errors := row.Fields[sobjectErrorsField]
			previousRef := errors.List[0].Ref
			warmAliasMutationIndex(t, machine, errors)
			var handled bool
			var err error
			if route == "FieldValueAddError" {
				_, handled, err = machine.callSObjectFieldValueAddError(&row, "Name", []Value{String("after")})
			} else {
				_, handled, err = machine.callSObjectMember(row, "addError", []Value{String("after")})
			}
			if err != nil || !handled {
				t.Fatalf("addError: handled=%v err=%v", handled, err)
			}
			if len(errors.List) != 1 || errors.List[0].Ref == previousRef || errors.List[0].Fields["message"].Text != "after" {
				t.Fatal("test did not replace an existing error through shared list backing")
			}
			if len(machine.flatAliasIndex) != 0 || machine.flatAliasIndexSlots != 0 {
				t.Fatal("error replacement retained an index with stale object identity")
			}
		})
	}
}
