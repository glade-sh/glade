package vm

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestHierarchySharingConformance(t *testing.T) {
	path := filepath.Join("..", "apextest", "testdata", "conformance", "hierarchy_sharing", "org.tsv")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = 5
	if _, err := reader.Read(); err != nil {
		t.Fatal(err)
	}
	observedRows, contextCarries := 0, 0
	seen := map[string]bool{}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		name := strings.Join(row[:3], "/")
		if seen[name] {
			t.Fatalf("duplicate native row %s", name)
		}
		seen[name] = true
		var observed map[string]any
		if err := json.Unmarshal([]byte(row[4]), &observed); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if row[2] == "anonymous" {
			contextCarries++
			t.Run(name, func(t *testing.T) {
				if row[3] != "CONTEXT-CARRY" || observed["compiled"] != true || observed["exceptionMessage"] != "System.TypeException: System.runAs can only be used within a test method: " {
					t.Fatalf("unexpected native anonymous-context evidence: %#v", observed)
				}
				t.Log("Native context carry: System.runAs is unavailable outside a test; this row provides no relationship-projection observation.")
			})
			continue
		}
		if row[2] != "istest" || row[3] != "OBSERVED" {
			t.Fatalf("unexpected native context/status: %s %s", name, row[3])
		}
		observedRows++
		t.Run(name, func(t *testing.T) {
			// These native booleans record setup provenance, rather than an
			// instruction to infer grants or materialize platform users.
			for key, value := range observed {
				if flag, ok := value.(bool); ok && !flag {
					t.Errorf("native setup/unwind control %s = false", key)
				}
			}
			for _, key := range []string{"ownerIsOriginal", "readerIsStandardProfile", "readerRoleAbsent", "sourceUnchanged", "originalActorRestored"} {
				if observed[key] != true {
					t.Errorf("missing native setup control %s", key)
				}
			}
			if row[0] == "H05" {
				want := "System.DmlException:Update failed. First exception on row 0 with id <leaf>; first error: CIRCULAR_DEPENDENCY, Hierarchy Constraint Violation: [Parent__c]"
				if observed["selfSetupError"] != want {
					t.Fatalf("native self-cycle setup evidence changed: %v", observed["selfSetupError"])
				}
				t.Log("Native self-cycle setup was rejected with CIRCULAR_DEPENDENCY; only the unchanged original chain's without-sharing projection is asserted here.")
			}
			if row[0] == "H06" {
				actual := hierarchyNativeNestedActors(t, row[1]+".0")
				for key, value := range actual {
					if !reflect.DeepEqual(value, observed[key]) {
						t.Errorf("nested actor %s mismatch: actual=%#v native=%#v", key, value, observed[key])
					}
				}
				return
			}
			keys := make([]string, 0)
			for key, value := range observed {
				if _, ok := value.(map[string]any); ok {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			for _, key := range keys {
				shape := hierarchyNativeProjectionShape(t, row[0], key)
				actual := hierarchyNativeProjection(t, row[1]+".0", shape, key == "dynamic")
				if !reflect.DeepEqual(actual, observed[key]) {
					t.Errorf("%s projection mismatch\nactual: %#v\nnative: %#v", key, actual, observed[key])
				}
			}
		})
	}
	if observedRows != 18 || contextCarries != 18 {
		t.Fatalf("native row coverage observed=%d context-carry=%d, want 18 and 18", observedRows, contextCarries)
	}
}

func hierarchyNativeProjectionShape(t *testing.T, caseID, key string) string {
	t.Helper()
	switch caseID {
	case "H01", "H02":
		return "leaf_only"
	case "H03":
		return "middle_shared"
	case "H04":
		switch key {
		case "owner", "unshared":
			return key
		case "allShared":
			return "all_shared"
		}
	case "H05":
		if key == "withoutSharing" {
			return "without_sharing"
		}
	case "H06":
		switch key {
		case "ownerBefore", "innerOwner", "ownerAfter":
			return "owner"
		case "readerBefore", "readerAfter":
			return "leaf_only"
		}
	}
	t.Fatalf("unrecognized native projection %s/%s", caseID, key)
	return ""
}

func hierarchyNativeProjection(t *testing.T, api, shape string, dynamic bool) map[string]any {
	t.Helper()
	machine := hierarchyRecordVisibilityMachine(t)
	if shape != "unshared" && shape != "without_sharing" && shape != "owner" {
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
	actor := machine.currentUserID()
	sharing := "with sharing"
	if shape == "without_sharing" {
		sharing = "without sharing"
	}
	registerHierarchyVisibilityReader(t, machine, api, sharing, dynamic)
	hierarchySourceUnchanged(t, machine)
	registerHierarchyNativeObserver(t, machine, api, sharing, false)
	value, err := machine.callMethod(machine.Methods["HierarchyEntry.observe"], nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if machine.currentUserID() != actor {
		t.Fatal("projection changed the current actor")
	}
	return hierarchyNativeValue(t, value).(map[string]any)
}

func registerHierarchyNativeObserver(t *testing.T, machine *VM, api, sharing string, dynamic bool) {
	t.Helper()
	body := `
Integer beforeQueries=Limits.getQueries();
Integer beforeRows=Limits.getQueryRows();
Map<String,Object> observed=new Map<String,Object>();
observed.put('directLeaf',[SELECT COUNT() FROM Widget__c WHERE Id='a00000000000003' WITH SYSTEM_MODE]);
observed.put('directMiddle',[SELECT COUNT() FROM Widget__c WHERE Id='a00000000000002' WITH SYSTEM_MODE]);
observed.put('directRoot',[SELECT COUNT() FROM Widget__c WHERE Id='a00000000000001' WITH SYSTEM_MODE]);
List<Widget__c> rows=HierarchyReader.read();
observed.put('leafRows',rows.size());
if (!rows.isEmpty()) {
 Widget__c leaf=rows[0];
 observed.put('leafForeignKey',leaf.Parent__c);
 observed.put('parentNull',leaf.Parent__r==null);
 observed.put('getSObjectNull',leaf.getSObject('Parent__r')==null);
 observed.put('parentId',leaf.Parent__r.Id);
 observed.put('parentForeignKey',leaf.Parent__r.Parent__c);
 observed.put('rootId',leaf.Parent__r.Parent__r.Id);
 Widget__c rebound=leaf;
 observed.put('reboundParentId',rebound.Parent__r.Id);
 observed.put('reboundRootId',rebound.Parent__r.Parent__r.Id);
}
observed.put('queryDelta',Limits.getQueries()-beforeQueries);
observed.put('queryRowsDelta',Limits.getQueryRows()-beforeRows);
return observed;`
	method := "HierarchyEntry.observe"
	if dynamic {
		method += "Dynamic"
		body = strings.ReplaceAll(body, "HierarchyReader.read()", "HierarchyReader.readDynamic()")
	}
	program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: method, ClassName: "HierarchyEntry", Access: "public", IsStatic: true, APIVersion: api, SourceContextBound: true, SharingMode: sharing, Program: program}); err != nil {
		t.Fatal(err)
	}
}

func hierarchyNativeValue(t *testing.T, value Value) any {
	t.Helper()
	switch value.Kind {
	case ValueMap:
		observed := make(map[string]any, len(value.Map))
		for key, item := range value.Map {
			observed[value.MapKeys[key].Text] = hierarchyNativeValue(t, item)
		}
		return observed
	case ValueInt:
		return float64(value.Int)
	case ValueBool:
		return value.Bool
	case ValueNull:
		return "null"
	case ValueObject:
		if strings.EqualFold(value.Type, "Id") {
			if scalar, ok := value.Fields["value"]; ok {
				return hierarchyNativeValue(t, scalar)
			}
		}
		t.Fatalf("projection has unexpected object: %#v", value)
		return nil
	case ValueString:
		for id, label := range map[storage.ID]string{"a00000000000001": "root", "a00000000000002": "middle", "a00000000000003": "leaf"} {
			if storage.IDsEqual(storage.ID(value.Text), id) {
				return label
			}
		}
		return value.Text
	default:
		t.Fatalf("projection has unexpected value kind %s", value.Kind)
		return nil
	}
}

func hierarchyNativeNestedActors(t *testing.T, api string) map[string]any {
	t.Helper()
	machine := hierarchyRecordVisibilityMachine(t)
	hierarchyReadShare(machine, "a00000000000003", 1)
	hierarchySourceUnchanged(t, machine)
	registerHierarchyVisibilityReader(t, machine, api, "with sharing", false)
	registerHierarchyNativeObserver(t, machine, api, "with sharing", false)
	dynamicProgram, err := CompileAnonymousWithOptions("return Database.queryWithBinds('SELECT Id, Parent__c, Parent__r.Id, Parent__r.Parent__c, Parent__r.Parent__r.Id FROM Widget__c WHERE Id = :leafId',new Map<String,Object>{'leafId'=>(Id)'a00000000000003'},AccessLevel.SYSTEM_MODE);", CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "HierarchyReader.readDynamic", ClassName: "HierarchyReader", Access: "public", IsStatic: true, APIVersion: api, SourceContextBound: true, SharingMode: "inherited sharing", Program: dynamicProgram}); err != nil {
		t.Fatal(err)
	}
	registerHierarchyNativeObserver(t, machine, api, "with sharing", true)
	body := `
Map<String,Object> observed=new Map<String,Object>();
Id originalActor=UserInfo.getUserId();
Integer beforeQueries=Limits.getQueries();
System.runAs(new User(Id='005000000000001')) {
 observed.put('ownerBefore',HierarchyEntry.observe());
 System.runAs(new User(Id='005000000000002')) {
  observed.put('readerActor',UserInfo.getUserId()==(Id)'005000000000002');
  observed.put('readerBefore',HierarchyEntry.observeDynamic());
  try {
   System.runAs(new User(Id='005000000000001')) {
    observed.put('innerOwnerActor',UserInfo.getUserId()==(Id)'005000000000001');
    observed.put('innerOwner',HierarchyEntry.observe());
    throw new IllegalArgumentException('owned unwind');
   }
  } catch (IllegalArgumentException owned) {
   observed.put('caughtOwnedException',true);
  }
  observed.put('readerRestoredOnUnwind',UserInfo.getUserId()==(Id)'005000000000002');
  observed.put('readerAfter',HierarchyEntry.observe());
 }
 observed.put('ownerRestored',UserInfo.getUserId()==(Id)'005000000000001');
 observed.put('ownerAfter',HierarchyEntry.observeDynamic());
}
observed.put('originalActorRestored',UserInfo.getUserId()==originalActor);
observed.put('nestedQueryDelta',Limits.getQueries()-beforeQueries);
return observed;`
	program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	value, err := machine.callMethod(Method{Name: "HierarchyEntry.nested", ClassName: "HierarchyEntry", IsStatic: true, APIVersion: api, SourceContextBound: true, SharingMode: "with sharing", Program: program}, nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if machine.currentUserID() != "005000000000002" || machine.testContext.RunAsDepth != 1 {
		t.Fatal("nested exception failed to restore the original actor")
	}
	return hierarchyNativeValue(t, value).(map[string]any)
}
