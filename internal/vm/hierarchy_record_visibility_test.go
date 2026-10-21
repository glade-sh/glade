package vm

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These are local preservation controls, not native parity assertions: the
// captured matrix does not answer default-mode or non-test parent projection.
func TestPrivateSelfLookupSharingUncapturedContextControls(t *testing.T) {
	for _, shape := range []string{"default_static", "default_dynamic", "other_access_argument", "non_test"} {
		t.Run(shape, func(t *testing.T) {
			machine := hierarchyRecordVisibilityMachine(t)
			hierarchyReadShare(machine, "a00000000000003", 1)
			hierarchySourceUnchanged(t, machine)
			registerHierarchyVisibilityReader(t, machine, "62.0", "with sharing", false)
			if shape == "non_test" {
				machine.executionUser = machine.testContext.CurrentUser
				machine.testContext = nil
			} else {
				query := "SELECT Id, Parent__c, Parent__r.Id, Parent__r.Parent__c, Parent__r.Parent__r.Id FROM Widget__c WHERE Id = 'a00000000000003'"
				body := "return [" + query + "];"
				if shape == "default_dynamic" {
					body = "return Database.query('" + strings.ReplaceAll(query, "'", "\\'") + "');"
				} else if shape == "other_access_argument" {
					body = "return Database.query('" + strings.ReplaceAll(query, "'", "\\'") + "',AccessLevel.SYSTEM_MODE);"
				}
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: "62.0"})
				if err != nil {
					t.Fatal(err)
				}
				method := machine.Methods["HierarchyReader.read"]
				machine.unregisterMethod(method)
				method.Program = program
				if err := machine.RegisterMethod(method); err != nil {
					t.Fatal(err)
				}
			}
			rows, err := machine.callMethod(machine.Methods["HierarchyEntry.read"], nil, &Result{})
			if err != nil || len(rows.List) != 1 {
				t.Fatalf("unchanged query control: rows=%d error=%v", len(rows.List), err)
			}
			_, parent, ok := objectFieldValue(rows.List[0], "Parent__r")
			if !ok || parent.Kind != ValueObject {
				t.Fatal("uncaptured context changed its existing projection")
			}
		})
	}
}

// Native test-context rows preserve the leaf foreign key while projecting
// independently hidden self-lookup ancestors as null.
func TestPrivateSelfLookupSharingBoundary(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, dynamic := range []bool{false, true} {
			for _, shape := range []string{"leaf_only", "middle_shared", "all_shared", "owner", "unshared", "without_sharing", "public"} {
				t.Run(fmt.Sprintf("%s/dynamic=%t/%s", api, dynamic, shape), func(t *testing.T) {
					machine := hierarchyRecordVisibilityMachine(t)
					if shape != "unshared" {
						hierarchyReadShare(machine, "a00000000000003", 1)
					}
					if shape == "middle_shared" || shape == "all_shared" {
						hierarchyReadShare(machine, "a00000000000002", 2)
					}
					if shape == "all_shared" {
						hierarchyReadShare(machine, "a00000000000001", 3)
					}
					if shape == "owner" {
						machine.testContext.CurrentUser.Fields["Id"] = String("005000000000001")
					}
					if shape == "public" {
						state := machine.Org.Objects["Widget__c"]
						state.Definition.SharingModel = "ReadWrite"
						machine.Org.Objects["Widget__c"] = state
					}
					sharing := "with sharing"
					if shape == "without_sharing" {
						sharing = "without sharing"
					}
					registerHierarchyVisibilityReader(t, machine, api, sharing, dynamic)
					hierarchySourceUnchanged(t, machine)
					rows, err := machine.callMethod(machine.Methods["HierarchyEntry.read"], nil, &Result{})
					if err != nil {
						t.Fatal(err)
					}
					if shape == "unshared" {
						if len(rows.List) != 0 {
							t.Fatalf("unshared leaf rows=%d, want 0", len(rows.List))
						}
						return
					}
					if len(rows.List) != 1 {
						t.Fatalf("leaf rows=%d, want 1", len(rows.List))
					}
					leaf := rows.List[0]
					wantParent := "a00000000000002"
					if got := stringField(leaf, "Parent__c"); !storage.IDsEqual(storage.ID(got), storage.ID(wantParent)) {
						t.Fatalf("leaf foreign key=%q, want %s", got, wantParent)
					}
					_, parent, present := objectFieldValue(leaf, "Parent__r")
					if !present {
						t.Fatal("selected parent relationship is absent")
					}
					if shape == "leaf_only" {
						if parent.Kind != ValueNull {
							t.Fatalf("sharing-hidden parent=%s, want null", parent.Kind)
						}
						id, err := machine.lookupPath(leaf, []string{"Parent__r", "Id"})
						if err != nil || id.Kind != ValueNull {
							t.Fatalf("hidden parent path kind=%s, error=%v", id.Kind, err)
						}
						return
					}
					if parent.Kind != ValueObject || !storage.IDsEqual(sObjectIDFromFields(parent.Fields), storage.ID(wantParent)) {
						t.Fatalf("accessible parent=%#v", parent)
					}
					_, root, present := objectFieldValue(parent, "Parent__r")
					if !present {
						t.Fatal("selected nested parent relationship is absent")
					}
					if shape == "middle_shared" {
						if root.Kind != ValueNull || !storage.IDsEqual(storage.ID(stringField(parent, "Parent__c")), "a00000000000001") {
							t.Fatalf("sharing-hidden root kind=%s; middle foreign key=%s", root.Kind, stringField(parent, "Parent__c"))
						}
					} else if root.Kind != ValueObject {
						t.Fatalf("accessible nested parent=%s, want object", root.Kind)
					}
					if len(machine.Org.Objects["Widget__c"].Records) != 3 {
						t.Fatal("projection changed stored records")
					}
				})
			}
		}
	}
}

func TestPrivateSelfLookupSharingNestedActors(t *testing.T) {
	machine := hierarchyRecordVisibilityMachine(t)
	hierarchyReadShare(machine, "a00000000000003", 1)
	hierarchySourceUnchanged(t, machine)
	registerHierarchyVisibilityReader(t, machine, "67.0", "with sharing", true)
	body := `
System.runAs(new User(Id='005000000000001')) {
 List<Widget__c> owned=HierarchyReader.read();
 System.assertNotEquals(null, owned[0].Parent__r);
 System.runAs(new User(Id='005000000000002')) {
  List<Widget__c> visible=[SELECT Id FROM Widget__c WITH SYSTEM_MODE];
  System.assertEquals(1, visible.size(), 'direct sharing control');
  List<Widget__c> shared=HierarchyReader.read();
  System.assertEquals((Id)'a00000000000002', shared[0].Parent__c);
  System.assertEquals(null, shared[0].Parent__r, 'hidden ancestor projection');
  Widget__c leaf=shared[0];
  System.assertEquals(null, leaf.Parent__r.Id, 'bound hidden ancestor dot access');
  System.assertEquals(null, leaf.Parent__r.Parent__r.Id, 'bound hidden nested ancestor dot access');
  System.assertEquals(null, shared[0].getSObject('Parent__r'));
  System.assertEquals(null, shared[0].Parent__r.Id, 'hidden ancestor dot access');
  System.assertEquals(null, shared[0].Parent__r.Parent__r.Id, 'hidden nested ancestor dot access');
 }
 System.assertEquals((Id)'005000000000001', UserInfo.getUserId());
 List<Widget__c> restored=HierarchyReader.read();
 System.assertNotEquals(null, restored[0].Parent__r, 'owner context restored');
}

System.assertEquals((Id)'005000000000002', UserInfo.getUserId());`
	program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.callMethod(Method{Name: "HierarchyEntry.nested", ClassName: "HierarchyEntry", IsStatic: true, APIVersion: "67.0", SourceContextBound: true, SharingMode: "with sharing", Program: program}, nil, &Result{})
	if machine.currentUserID() != "005000000000002" || machine.testContext.RunAsDepth != 1 {
		t.Fatal("nested runAs failed to restore the original actor")
	}
	if err != nil {
		t.Fatal(err)
	}
}

func hierarchySourceUnchanged(t *testing.T, machine *VM) {
	t.Helper()
	before := make(map[string]map[storage.ID]storage.Record)
	for _, name := range []string{"Widget__c", "Widget__Share"} {
		before[name] = make(map[storage.ID]storage.Record)
		for id, row := range machine.Org.Objects[name].Records {
			before[name][id] = row.Clone()
		}
	}
	t.Cleanup(func() {
		for name, rows := range before {
			if !reflect.DeepEqual(rows, machine.Org.Objects[name].Records) {
				t.Errorf("projection changed stored %s rows", name)
			}
		}
	})
}

func hierarchyRecordVisibilityMachine(t *testing.T) *VM {
	t.Helper()
	machine := recordSharingControlMachine(t, "Private")
	state := machine.Org.Objects["Widget__c"]
	state.Definition.Fields["Parent__c"] = storage.Field{APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Widget__c"}, RelationshipName: "Parent__r"}
	state.Definition.Relations = []storage.Relationship{{Field: "Parent__c", ParentObjects: []string{"Widget__c"}, ParentRelationship: "Parent__r"}}
	state.Records = make(map[storage.ID]storage.Record)
	for i := 1; i <= 3; i++ {
		id := storage.ID(fmt.Sprintf("a0000000000000%d", i))
		fields := map[string]storage.Value{"Name": storage.StringValue(fmt.Sprintf("Node %d", i))}
		if i > 1 {
			fields["Parent__c"] = storage.IDValue(storage.ID(fmt.Sprintf("a0000000000000%d", i-1)))
		}
		state.Records[id] = storage.Record{ID: id, Object: "Widget__c", Fields: fields, System: storage.SystemFields{OwnerID: "005000000000001"}}
	}
	machine.Org.Objects["Widget__c"] = state
	machine.Org.Objects["Widget__Share"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Widget__Share"}, Records: make(map[storage.ID]storage.Record)}
	machine.SetOrg(machine.Org)
	return machine
}

func hierarchyReadShare(machine *VM, parent storage.ID, n int) {
	state := machine.Org.Objects["Widget__Share"]
	id := storage.ID(fmt.Sprintf("a0100000000000%d", n))
	state.Records[id] = storage.Record{ID: id, Object: "Widget__Share", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent), "UserOrGroupId": storage.IDValue("005000000000002"), "AccessLevel": storage.StringValue("Read"), "RowCause": storage.StringValue("Manual")}}
	machine.Org.Objects["Widget__Share"] = state
}

func registerHierarchyVisibilityReader(t *testing.T, machine *VM, api, sharing string, dynamic bool) {
	t.Helper()
	query := "SELECT Id, Parent__c, Parent__r.Id, Parent__r.Parent__c, Parent__r.Parent__r.Id FROM Widget__c WHERE Id = 'a00000000000003' WITH SYSTEM_MODE"
	body := "List<Widget__c> rows=[" + query + "]; return rows;"
	if dynamic {
		body = "return Database.queryWithBinds('SELECT Id, Parent__c, Parent__r.Id, Parent__r.Parent__c, Parent__r.Parent__r.Id FROM Widget__c WHERE Id = :leafId',new Map<String,Object>{'leafId'=>(Id)'a00000000000003'},AccessLevel.SYSTEM_MODE);"
	}
	for _, spec := range []struct{ name, mode, body string }{{"HierarchyReader", "inherited sharing", body}, {"HierarchyEntry", sharing, "return HierarchyReader.read();"}} {
		program, err := CompileAnonymousWithOptions(spec.body, CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterClass(Class{Name: spec.name, APIVersion: api, Modifiers: []string{spec.mode}}); err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(Method{Name: spec.name + ".read", ClassName: spec.name, IsStatic: true, Access: "public", APIVersion: api, SourceContextBound: true, SharingMode: spec.mode, Program: program}); err != nil {
			t.Fatal(err)
		}
	}
}
