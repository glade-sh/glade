package storage

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// sharedSetupOrg returns an org whose read-only setup objects hold rows, next
// to a business object with rows that a clone must still copy.
func sharedSetupOrg(t testing.TB, rows int) OrgState {
	t.Helper()
	org := NewOrgState()
	EnsureStandardObject(&org, "Account")
	for _, name := range runtimeSharedSetupObjectNames() {
		EnsureStandardObject(&org, name)
		object := org.Objects[name]
		for i := 0; i < rows; i++ {
			id := ID(fmt.Sprintf("%.3s%012dAAA", name, i))
			object.PutRecord(id, Record{ID: id, Object: name, Fields: map[string]Value{"Name": StringValue(fmt.Sprintf("%s %d", name, i))}})
		}
		org.Objects[name] = object
	}
	account := org.Objects["Account"]
	account.PutRecord("001000000000001AAA", Record{ID: "001000000000001AAA", Object: "Account", Fields: map[string]Value{"Name": StringValue("Acme")}})
	org.Objects["Account"] = account
	return org
}

// runtimeSharedSetupObjectNames lists the read-only setup objects whose rows
// a runtime clone shares with its template.
func runtimeSharedSetupObjectNames() []string {
	return []string{"ApexTypeImplementor", "UserLicense", "PermissionSetLicense", "OpportunityStage", "TaskStatus", "LeadStatus", "TaskPriority", "DuplicateRule", "MatchingRule"}
}

// rowsDigest renders every row of every object, including field maps, so an
// in-place change to a shared row is visible.
func rowsDigest(org OrgState) map[string]string {
	out := map[string]string{}
	for name, object := range org.Objects {
		if len(object.Records) == 0 {
			continue
		}
		out[name] = fmt.Sprintf("%v", object.Records)
	}
	return out
}

func TestCloneRuntimeFrozenSharedSharesOnlySetupRows(t *testing.T) {
	row := func(name string) map[ID]Record { return map[ID]Record{"x": {ID: "x", Object: name}} }
	org := NewOrgState()
	for _, name := range runtimeSharedSetupObjectNames() {
		org.Objects[name] = ObjectState{Definition: ObjectDefinition{APIName: name}, Records: row(name)}
		org.Objects["alias of "+name] = ObjectState{Definition: ObjectDefinition{APIName: name}, Records: row(name)}
		org.Objects["empty "+name] = ObjectState{Definition: ObjectDefinition{APIName: name}, Records: map[ID]Record{}}
	}
	for _, name := range []string{"Account", "User", "UserLogin", "Region__mdt", "Case", "AsyncApexJob", "PermissionSetAssignment", "Lead", "Task"} {
		org.Objects[name] = ObjectState{Definition: ObjectDefinition{APIName: name}, Records: row(name)}
	}
	clone := org.CloneRuntimeFrozenShared()
	for _, name := range runtimeSharedSetupObjectNames() {
		if !clone.Objects[name].RecordsShared || !clone.Objects["alias of "+name].RecordsShared {
			t.Errorf("%s rows are not shared", name)
		}
		if clone.Objects["empty "+name].RecordsShared {
			t.Errorf("empty %s must keep its own map", name)
		}
	}
	for _, name := range []string{"Account", "User", "UserLogin", "Region__mdt", "Case", "AsyncApexJob", "PermissionSetAssignment", "Lead", "Task"} {
		if clone.Objects[name].RecordsShared {
			t.Errorf("%s rows must stay copied", name)
		}
	}
}

func TestCloneRuntimeFrozenSharedSetupRowsCostIndependentOfRowCount(t *testing.T) {
	small := sharedSetupOrg(t, 2)
	large := sharedSetupOrg(t, 200)
	measure := func(org OrgState) float64 {
		return testing.AllocsPerRun(10, func() { _ = org.CloneRuntimeFrozenShared() })
	}
	smallAllocs, largeAllocs := measure(small), measure(large)
	if largeAllocs > smallAllocs {
		t.Fatalf("clone allocs = %.0f with 200 setup rows per object, %.0f with 2; cost must not grow with setup rows", largeAllocs, smallAllocs)
	}
	clone := large.CloneRuntimeFrozenShared()
	for _, name := range runtimeSharedSetupObjectNames() {
		if !clone.Objects[name].RecordsShared {
			t.Fatalf("%s rows are not shared with the clone", name)
		}
	}
	if clone.Objects["Account"].RecordsShared {
		t.Fatal("business rows must stay copied")
	}
}

// Each mutation route a runtime can take against a clone: the record funnel
// (put, delete, clear, replace, in-place edit then put) after the privatize
// step, a rollback snapshot, an isolation journal rollback and a nested
// clone. None may reach the template or a sibling clone.
func TestCloneRuntimeFrozenSharedSetupRowsMutationRoutes(t *testing.T) {
	for _, name := range runtimeSharedSetupObjectNames() {
		t.Run(name, func(t *testing.T) {
			template := sharedSetupOrg(t, 4)
			want := rowsDigest(template)
			existing := ID(fmt.Sprintf("%.3s%012dAAA", name, 1))
			routes := map[string]func(org *OrgState){
				"put": func(org *OrgState) {
					object, _ := EnsureMutableObjectRecords(org, name)
					object.PutRecord("new", Record{ID: "new", Object: name})
				},
				"update in place": func(org *OrgState) {
					object, _ := EnsureMutableObjectRecords(org, name)
					row := object.Records[existing]
					row.Fields["Name"] = StringValue("changed")
					object.PutRecord(existing, row)
				},
				"delete": func(org *OrgState) {
					object, _ := EnsureMutableObjectRecords(org, name)
					object.DeleteRecord(existing)
				},
				"undelete": func(org *OrgState) {
					object, _ := EnsureMutableObjectRecords(org, name)
					row := object.Records[existing]
					row.System.IsDeleted = true
					object.PutRecord(existing, row)
					row.System.IsDeleted = false
					object.PutRecord(existing, row)
				},
				"clear": func(org *OrgState) {
					object, _ := EnsureMutableObjectRecords(org, name)
					object.ClearRecords()
				},
				"replace": func(org *OrgState) {
					object := org.Objects[name]
					object.SetRecords(map[ID]Record{"only": {ID: "only", Object: name}})
					org.Objects[name] = object
				},
				"rollback snapshot": func(org *OrgState) {
					snapshot := SnapshotRuntimeOrg(org)
					object, _ := EnsureMutableObjectRecords(org, name)
					object.DeleteRecord(existing)
					*org = snapshot
					object, _ = EnsureMutableObjectRecords(org, name)
					row := object.Records[existing]
					row.Fields["Name"] = StringValue("after rollback")
					object.PutRecord(existing, row)
				},
				"journal rollback": func(org *OrgState) {
					journal := NewIsolationJournal(org)
					mark := journal.Mark()
					object, _ := EnsureMutableObjectRecords(org, name)
					journal.RecordUpdate(name, existing, object.Records[existing])
					row := object.Records[existing].Clone()
					row.Fields["Name"] = StringValue("journaled")
					object.PutRecord(existing, row)
					journal.RecordInsert(name, "inserted")
					object.PutRecord("inserted", Record{ID: "inserted", Object: name})
					if err := journal.Rollback(mark); err != nil {
						t.Fatal(err)
					}
				},
				"nested clone": func(org *OrgState) {
					nested := org.CloneRuntimeFrozenShared()
					object, _ := EnsureMutableObjectRecords(&nested, name)
					object.DeleteRecord(existing)
				},
			}
			for label, route := range routes {
				clone := template.CloneRuntimeFrozenShared()
				sibling := template.CloneRuntimeFrozenShared()
				route(&clone)
				if got := rowsDigest(template); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s reached the template", label)
				}
				if got := rowsDigest(sibling); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s reached a sibling clone", label)
				}
			}
		})
	}
}

// The privatize step (EnsureMutableObjectRecords) is the only isolation
// between a clone and the template for shared setup rows. This test fails
// when that copy is removed: the clone would then write the template's map.
func TestCloneRuntimeFrozenSharedSetupRowsPrivatizeGuard(t *testing.T) {
	template := sharedSetupOrg(t, 3)
	clone := template.CloneRuntimeFrozenShared()
	name := "UserLicense"
	shared := clone.Objects[name]
	if !shared.RecordsShared || reflect.ValueOf(shared.Records).UnsafePointer() != reflect.ValueOf(template.Objects[name].Records).UnsafePointer() {
		t.Fatal("expected the clone to share the template's rows")
	}
	object, cloned := EnsureMutableObjectRecords(&clone, name)
	if !cloned || object.RecordsShared {
		t.Fatal("privatize step did not copy shared rows")
	}
	if reflect.ValueOf(object.Records).UnsafePointer() == reflect.ValueOf(template.Objects[name].Records).UnsafePointer() {
		t.Fatal("privatized clone still writes the template's map")
	}
	id := ID("Use000000000000AAA")
	row := object.Records[id]
	row.Fields["Name"] = StringValue("changed")
	if template.Objects[name].Records[id].Fields["Name"].String == "changed" {
		t.Fatal("privatized row still shares its field map with the template")
	}
}

// Clones of one template are made and written from parallel test classes.
// Run with -race.
func TestCloneRuntimeFrozenSharedSetupRowsParallelClasses(t *testing.T) {
	template := sharedSetupOrg(t, 20)
	want := rowsDigest(template)
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				clone := template.CloneRuntimeFrozenShared()
				for _, name := range runtimeSharedSetupObjectNames() {
					_ = len(clone.Objects[name].Records)
					if (worker+i)%2 == 0 {
						object, _ := EnsureMutableObjectRecords(&clone, name)
						id := ID(fmt.Sprintf("w%d-%d", worker, i))
						object.PutRecord(id, Record{ID: id, Object: name})
						for rowID, row := range object.Records {
							if row.Fields != nil {
								row.Fields["Name"] = StringValue("worker")
								object.PutRecord(rowID, row)
								break
							}
						}
					}
				}
			}
		}(worker)
	}
	wg.Wait()
	if got := rowsDigest(template); !reflect.DeepEqual(got, want) {
		t.Fatal("parallel clone writes reached the template")
	}
}

func TestIsRuntimeSharedSetupObjectMatchesLowerCase(t *testing.T) {
	names := []string{"", "UserLicense", "USERLICENSE", "userlicense ", "UserLicenses", "TasKStatus", "TaskKStatus", "Account",
		"PermissionSetLicense", "PermissionSetLicenseX", "MatchingRule", "matchingrulé", strings.Repeat("a", 40) + "é"}
	names = append(names, KnownStandardObjectNames()...)
	for _, name := range names {
		_, want := runtimeSharedSetupObjects[strings.ToLower(name)]
		if got := isRuntimeSharedSetupObject(name); got != want {
			t.Errorf("isRuntimeSharedSetupObject(%q) = %v, want %v", name, got, want)
		}
	}
	if allocs := testing.AllocsPerRun(100, func() {
		for _, name := range []string{"UserLicense", "Account", "PermissionSetLicenseLonger"} {
			_ = isRuntimeSharedSetupObject(name)
		}
	}); allocs != 0 {
		t.Fatalf("isRuntimeSharedSetupObject allocated %.1f times per run", allocs)
	}
}

// Re-seeding a clone must not change its template or sibling.
func TestCloneRuntimeFrozenSharedSetupReseedIsolation(t *testing.T) {
	template := sharedSetupOrg(t, 1)
	clone := template.CloneRuntimeFrozenShared()
	sibling := template.CloneRuntimeFrozenShared()
	id := ID("Use000000000000AAA")
	putSeedRecord(&clone, "UserLicense", Record{ID: id, Object: "UserLicense", Fields: map[string]Value{"Missing": StringValue("seeded")}})
	if got := clone.Objects["UserLicense"].Records[id].Fields["Missing"].String; got != "seeded" {
		t.Fatalf("clone field = %q", got)
	}
	for name, org := range map[string]OrgState{"template": template, "sibling": sibling} {
		if _, ok := org.Objects["UserLicense"].Records[id].Fields["Missing"]; ok {
			t.Fatalf("seed write reached %s", name)
		}
	}
}
