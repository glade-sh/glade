package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

type sharingUpdateObservation struct {
	Capture      string            `json:"capture"`
	Label        string            `json:"label"`
	TransportAPI string            `json:"transportApi"`
	SourceAPI    string            `json:"sourceApi"`
	Route        string            `json:"route"`
	Model        string            `json:"model"`
	Sharing      string            `json:"sharing"`
	Patch        bool              `json:"patch"`
	Inline       bool              `json:"inline"`
	Status       string            `json:"status"`
	Native       map[string]any    `json:"native"`
	CarryFields  map[string]string `json:"carryFields"`
}

// Replay the captured record-update shape, keeping transport and source APIs
// distinct. Per-run IDs and full native DML wrapper wording remain explicit
// carries; the stored effect, exception type and status are asserted.
func TestSharingUpdateObservationsConformance(t *testing.T) {
	var data struct {
		Cells      []sharingUpdateObservation `json:"cells"`
		Unanswered []struct {
			TransportAPI   string `json:"transportApi"`
			Route          string `json:"route"`
			Model          string `json:"model"`
			Sharing        string `json:"sharing"`
			Reason         string `json:"reason"`
			NativeEvidence string `json:"nativeEvidence"`
			Native         any    `json:"native"`
		} `json:"unanswered"`
	}
	raw, err := os.ReadFile("../apextest/testdata/conformance/sharing_update_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cells) != 136 || len(data.Unanswered) != 30 {
		t.Fatal("incomplete native sharing observation matrix")
	}
	matched, partial := 0, 0
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.Capture + "/" + cell.TransportAPI + "/" + cell.Label
		if seen[key] || cell.Route != "istest" || cell.Status != "runtime" {
			t.Fatalf("invalid or repeated observation %s", key)
		}
		seen[key] = true
		if len(cell.CarryFields) == 0 {
			matched++
		} else {
			partial++
		}
		t.Run(key, func(t *testing.T) {
			machine, record := sharingUpdateObservationMachine(cell)
			user := machine.currentUserID()
			// These access facts are independent of the helper's explicit sharing.
			observed := map[string]any{
				"nonOwner":      !storage.IDsEqual(record.System.OwnerID, storage.ID(user)),
				"ownerIsAdmin":  storage.IDsEqual(record.System.OwnerID, "005000000000001"),
				"profileBypass": machine.currentUserBypassesRecordSharingForWrite(),
				"crudUpdate":    machine.currentUserObjectPermission("SharingNote__c", "isUpdateable"),
				"readAccess":    machine.userModeRecordVisible("SharingNote__c", record, user),
				"editAccess":    machine.currentUserCanWriteRecord("SharingNote__c", record, user, "update"),
			}
			body := "SharingNote__c row=new SharingNote__c(Id='a00000000000001',Memo__c='changed'"
			if !cell.Patch {
				body += ",Name='Owner seed',OwnerId='005000000000001'"
			}
			body += ");"
			entryBody := body + " update row;"
			if cell.Sharing != "entry" {
				// Reused and inline native companions share the same runtime
				// source binding. Metadata deployment framing is not replayed.
				keyword := sharingUpdateKeyword(cell.Sharing)
				program, compileErr := CompileAnonymousWithOptions("row.put('Memo__c','changed'); update row;", CompileOptions{APIVersion: cell.SourceAPI})
				if compileErr != nil {
					t.Fatal(compileErr)
				}
				if err := machine.RegisterClass(Class{Name: "SharingHelper", APIVersion: cell.SourceAPI, Modifiers: []string{keyword}}); err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterMethod(Method{Name: "SharingHelper.change", ClassName: "SharingHelper", IsStatic: true, Access: "public", Params: []Param{{Name: "row", Type: "SObject"}}, APIVersion: cell.SourceAPI, SourceContextBound: true, SharingMode: keyword, Program: program}); err != nil {
					t.Fatal(err)
				}
				entryBody = body + "SharingHelper.change(row);"
			}
			program, compileErr := CompileAnonymousWithOptions(entryBody, CompileOptions{APIVersion: cell.SourceAPI})
			if compileErr != nil {
				t.Fatal(compileErr)
			}
			if err := machine.RegisterClass(Class{Name: "SharingEntry", APIVersion: cell.SourceAPI}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{Name: "SharingEntry.observe", ClassName: "SharingEntry", IsStatic: true, Access: "public", APIVersion: cell.SourceAPI, SourceContextBound: true, Program: program}); err != nil {
				t.Fatal(err)
			}
			invoke, compileErr := CompileAnonymous("SharingEntry.observe();")
			if compileErr != nil {
				t.Fatal(compileErr)
			}
			_, execErr := machine.ExecuteTestInClass(invoke, "SharingEntry")
			fresh := machine.Org.Objects["SharingNote__c"].Records[record.ID]
			if expected, denied := cell.Native["exceptionType"]; denied {
				var runtimeErr *RuntimeError
				if !errors.As(execErr, &runtimeErr) || exceptionQualifiedTypeName(runtimeErr.Type) != expected {
					t.Fatalf("update error=%v; native type=%v", execErr, expected)
				}
				status, _ := cell.Native["dmlStatus"].(string)
				if status == "" || !strings.Contains(execErr.Error(), status) {
					t.Fatalf("update status=%v; native=%s", execErr, status)
				}
				// Local transaction control: native read-back did not complete
				// on these exception cells, so this is not a captured assertion.
				if fresh.Fields["Memo__c"].String != "before" {
					t.Fatal("denied update changed the stored record")
				}
				observed["exceptionType"] = exceptionQualifiedTypeName(runtimeErr.Type)
				observed["dmlStatus"] = status
				observed["readBackComplete"] = false
			} else {
				if execErr != nil {
					t.Fatal(execErr)
				}
				observed["updated"] = true
				observed["memo"] = fresh.Fields["Memo__c"].String
				observed["readBackComplete"] = true
			}
			observed["stage"] = "effect"
			for name, native := range cell.Native {
				if reason, carried := cell.CarryFields[name]; carried {
					if reason == "" || native == nil {
						t.Fatal("carry requires a reason and native value")
					}
					t.Logf("CARRIED %s: %s; native=%v", name, reason, native)
					continue
				}
				if !reflect.DeepEqual(observed[name], native) {
					t.Fatalf("%s: Glade=%v native=%v", name, observed[name], native)
				}
			}
		})
	}
	if matched != 84 || partial != 52 {
		t.Fatalf("unexpected matched/partial counts %d/%d", matched, partial)
	}
	for _, cell := range data.Unanswered {
		t.Run("unanswered/"+cell.TransportAPI+"/"+cell.Model+"/"+cell.Sharing, func(t *testing.T) {
			if cell.Route != "anonymous" || cell.Reason == "" || cell.NativeEvidence == "" || cell.Native != nil {
				t.Fatal("unanswered route must retain evidence and must not invent an answer")
			}
			t.Logf("UNANSWERED: %s; evidence=%s", cell.Reason, cell.NativeEvidence)
		})
	}
	t.Logf("native observations: matched=%d partial=%d unanswered=%d", matched, partial, len(data.Unanswered))
}

func sharingUpdateKeyword(sharing string) string {
	switch sharing {
	case "with":
		return "with sharing"
	case "without":
		return "without sharing"
	case "inherited":
		return "inherited sharing"
	case "omitted":
		return ""
	default:
		panic(fmt.Sprintf("unrecognized captured sharing %q", sharing))
	}
}

func sharingUpdateObservationMachine(cell sharingUpdateObservation) (*VM, storage.Record) {
	org := stripInaccessibleTestOrg()
	org.APIVersion = cell.TransportAPI
	users := org.Objects["User"]
	if users.Records == nil {
		users.Records = map[storage.ID]storage.Record{}
	}
	for _, id := range []storage.ID{"005000000000001", "005000000000002"} {
		users.Records[id] = storage.Record{ID: id, Object: "User", Fields: map[string]storage.Value{"LastName": storage.StringValue("Fixture user")}}
	}
	org.Objects["User"] = users
	// The captured fixture grants CRUD/FLS without ViewAll or ModifyAll.
	profile := org.Objects["Profile"]
	actorProfile := profile.Records["00e000000000002"]
	actorProfile.Fields["Name"] = storage.StringValue("Standard User")
	profile.Records[actorProfile.ID] = actorProfile
	org.Objects["Profile"] = profile
	permissions := org.Objects["ObjectPermissions"]
	for id, row := range permissions.Records {
		row.Fields["SObjectType"] = storage.StringValue("SharingNote__c")
		row.Fields["PermissionsDelete"] = storage.BooleanValue(true)
		permissions.Records[id] = row
	}
	org.Objects["ObjectPermissions"] = permissions
	fields := org.Objects["FieldPermissions"]
	fields.Records = map[storage.ID]storage.Record{}
	for i, name := range []string{"Name", "Memo__c", "OwnerId"} {
		id := storage.ID(fmt.Sprintf("0FP00000000000%d", i+1))
		fields.Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("00e000000000002"), "SObjectType": storage.StringValue("SharingNote__c"), "Field": storage.StringValue("SharingNote__c." + name), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true)}}
	}
	org.Objects["FieldPermissions"] = fields
	record := storage.Record{ID: "a00000000000001", Object: "SharingNote__c", System: storage.SystemFields{OwnerID: "005000000000001"}, Fields: map[string]storage.Value{"Name": storage.StringValue("Owner seed"), "Memo__c": storage.StringValue("before")}}
	org.Objects["SharingNote__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "SharingNote__c", KeyPrefix: "a00", SharingModel: cell.Model, Fields: map[string]storage.Field{
		"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString}, "Memo__c": {APIName: "Memo__c", Type: storage.FieldString}, "OwnerId": {APIName: "OwnerId", Type: storage.FieldReference, ReferenceTo: []string{"Group", "User"}, RelationshipName: "Owner"},
	}}, Records: map[storage.ID]storage.Record{record.ID: record}}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.CurrentUser = stripInaccessibleTestUser()
	machine.testContext.RunAsDepth = 1
	return machine, record
}

// These are local scope controls for paths not established by the capture.
func TestDefaultRunAsPrivateTextUpdateScope(t *testing.T) {
	for _, variant := range []string{"captured", "hydrated caller", "explicit system field", "older source", "outside runAs", "package runAs", "trigger", "list", "other operation", "public model", "owner", "standard object", "numeric field", "unknown field", "other lookup", "no text", "null text", "numeric text", "other owner lookup", "owner transfer", "group owner"} {
		t.Run(variant, func(t *testing.T) {
			machine, _ := sharingUpdateObservationMachine(sharingUpdateObservation{TransportAPI: "68.0", SourceAPI: "67.0", Model: "Private"})
			machine.currentMethod = Method{APIVersion: "67.0"}
			value := Object("SharingNote__c")
			value.Fields["Id"] = String("a00000000000001")
			value.Fields["Memo__c"] = String("changed")
			op := "update"
			switch variant {
			case "hydrated caller", "explicit system field":
				markDMLAccessibleFields(&value)
				putSystemFields(value, storage.SystemFields{CreatedByID: "005000000000001", LastModifiedByID: "005000000000001", OwnerID: "005000000000001"})
				if variant == "explicit system field" {
					setExplicitSObjectField(&value, "CreatedById", String("005000000000001"))
				}
			case "owner transfer":
				value.Fields["OwnerId"] = String("005000000000002")
			case "group owner":
				state := machine.Org.Objects["SharingNote__c"]
				stored := state.Records["a00000000000001"]
				stored.System.OwnerID = "00G000000000001"
				state.Records[stored.ID] = stored
				machine.Org.Objects["SharingNote__c"] = state
			case "no text":
				delete(value.Fields, "Memo__c")
			case "null text":
				value.Fields["Memo__c"] = Null
			case "numeric text":
				value.Fields["Memo__c"] = Int(1)
			case "other owner lookup":
				state := machine.Org.Objects["SharingNote__c"]
				state.Definition.Fields["OtherOwner__c"] = storage.Field{APIName: "OtherOwner__c", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Owner"}
				machine.Org.Objects["SharingNote__c"] = state
				value.Fields["OtherOwner__c"] = String("005000000000001")
			case "older source":
				machine.currentMethod.APIVersion = "66.0"
			case "outside runAs":
				machine.testContext.RunAsDepth = 0
			case "package runAs":
				machine.testContext.PackageRunAsDepth = 1
			case "trigger":
				machine.currentTrigger = true
			case "list":
				value = List(value)
			case "other operation":
				op = "delete"
			case "public model":
				state := machine.Org.Objects["SharingNote__c"]
				state.Definition.SharingModel = "ReadWrite"
				machine.Org.Objects["SharingNote__c"] = state
			case "owner":
				machine.testContext.CurrentUser.Fields["Id"] = String("005000000000001")
			case "standard object":
				value = Object("Account")
				value.Fields["Id"] = String("001000000000001")
				value.Fields["Name"] = String("changed")
			case "numeric field":
				state := machine.Org.Objects["SharingNote__c"]
				state.Definition.Fields["Amount__c"] = storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal}
				machine.Org.Objects["SharingNote__c"] = state
				value.Fields["Amount__c"] = Decimal(1)
			case "unknown field":
				value.Fields["Unknown__c"] = String("changed")
			case "other lookup":
				state := machine.Org.Objects["SharingNote__c"]
				state.Definition.Fields["Assignee__c"] = storage.Field{APIName: "Assignee__c", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Assignee__r"}
				machine.Org.Objects["SharingNote__c"] = state
				value.Fields["Assignee__c"] = String("005000000000001")
			}
			if got := machine.defaultRunAsPrivateTextUpdate(op, value); got != (variant == "captured" || variant == "hydrated caller") {
				t.Fatalf("special default route=%t", got)
			}
		})
	}
}

func TestDefaultPrivateTextUpdatePreservesExplicitAccessPaths(t *testing.T) {
	for _, body := range []string{
		"SharingNote__c row=new SharingNote__c(Id='a00000000000001',Memo__c='changed');update as user row;",
		"SharingNote__c row=new SharingNote__c(Id='a00000000000001',Memo__c='changed');Database.update(row);",
		"List<SharingNote__c> rows=new List<SharingNote__c>{new SharingNote__c(Id='a00000000000001',Memo__c='changed')};update rows;",
	} {
		t.Run(body, func(t *testing.T) {
			machine, _ := sharingUpdateObservationMachine(sharingUpdateObservation{TransportAPI: "68.0", SourceAPI: "67.0", Model: "Private"})
			program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterClass(Class{Name: "UpdateScope", APIVersion: "67.0", Modifiers: []string{"without sharing"}}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{Name: "UpdateScope.check", ClassName: "UpdateScope", IsStatic: true, Access: "public", APIVersion: "67.0", SourceContextBound: true, SharingMode: "without sharing", Program: program}); err != nil {
				t.Fatal(err)
			}
			invoke, err := CompileAnonymous("UpdateScope.check();")
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.ExecuteTestInClass(invoke, "UpdateScope")
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || exceptionQualifiedTypeName(runtimeErr.Type) != "System.SecurityException" {
				t.Fatalf("existing access path exception=%v", err)
			}
		})
	}
}
