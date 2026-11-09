package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

type dotfixNativeProbe struct {
	Outcome    string              `json:"outcome"`
	RecordMaps []map[string]any    `json:"recordMaps"`
	Removed    map[string][]string `json:"getRemovedFields"`
	Exception  struct {
		Message string `json:"message"`
		Rows    []struct {
			Message string   `json:"getDmlMessage"`
			Fields  []string `json:"getDmlFieldNames"`
			Type    string   `json:"getDmlType"`
		} `json:"rows"`
	} `json:"exception"`
}

func dotfixRestrictedMachine(api string) *VM {
	org := testDataOrg()
	org.APIVersion = api
	storage.EnsureStandardObject(&org, "User")
	storage.EnsureStandardObject(&org, "Case")
	storage.EnsureDeterministicPlatformData(&org)
	const userID storage.ID = "005000000000990"
	const otherID storage.ID = "005000000000991"
	const psID storage.ID = "0PS000000000990"
	user := org.Objects["User"]
	user.Definition.SharingModel = "Read"
	user.Definition.Fields["SynthUserField__c"] = storage.Field{APIName: "SynthUserField__c", Type: storage.FieldString}
	for _, id := range []storage.ID{userID, otherID} {
		user.Records[id] = storage.Record{ID: id, Object: "User", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic"), "Email": storage.StringValue("synthetic@example.invalid"), "ProfileId": storage.IDValue("00e000000000002"), "SynthUserField__c": storage.StringValue("synthetic")}}
	}
	org.Objects["User"] = user
	org.Objects["SynthAccess__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "SynthAccess__c", KeyPrefix: "a00", SharingModel: "Private", Fields: map[string]storage.Field{
		"Name": {APIName: "Name", Type: storage.FieldString}, "ReadField__c": {APIName: "ReadField__c", Type: storage.FieldString}, "HiddenField__c": {APIName: "HiddenField__c", Type: storage.FieldString},
	}}, Records: map[storage.ID]storage.Record{
		"a00000000000990": {ID: "a00000000000990", Object: "SynthAccess__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic own")}, System: storage.SystemFields{OwnerID: userID}},
		"a00000000000991": {ID: "a00000000000991", Object: "SynthAccess__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic other")}, System: storage.SystemFields{OwnerID: otherID}},
	}}
	cases := org.Objects["Case"]
	cases.Records["500000000000990"] = storage.Record{ID: "500000000000990", Object: "Case", Fields: map[string]storage.Value{"Subject": storage.StringValue("Synthetic")}, System: storage.SystemFields{OwnerID: userID}}
	org.Objects["Case"] = cases
	ps := org.Objects["PermissionSet"]
	ps.Records[psID] = storage.Record{ID: psID, Object: "PermissionSet", Fields: map[string]storage.Value{"Name": storage.StringValue("SynthCaptureRestricted"), "IsOwnedByProfile": storage.BooleanValue(false)}}
	org.Objects["PermissionSet"] = ps
	assign := org.Objects["PermissionSetAssignment"]
	assign.Records["0Pa000000000990"] = storage.Record{ID: "0Pa000000000990", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"PermissionSetId": storage.IDValue(psID), "AssigneeId": storage.IDValue(userID)}}
	org.Objects["PermissionSetAssignment"] = assign
	objects := org.Objects["ObjectPermissions"]
	for i, name := range []string{"Case", "SynthAccess__c"} {
		id := storage.ID(fmt.Sprintf("11000000000099%d", i))
		objects.Records[id] = storage.Record{ID: id, Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(psID), "SobjectType": storage.StringValue(name), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true)}}
	}
	org.Objects["ObjectPermissions"] = objects
	fields := org.Objects["FieldPermissions"]
	for i, name := range []string{"Case.Subject", "SynthAccess__c.ReadField__c"} {
		object, _, _ := strings.Cut(name, ".")
		id := storage.ID(fmt.Sprintf("0FP00000000099%d", i))
		fields.Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(psID), "SobjectType": storage.StringValue(object), "Field": storage.StringValue(name), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(i == 0)}}
	}
	org.Objects["FieldPermissions"] = fields
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.CurrentUser = vmValueFromRecord(user.Records[userID])
	machine.testContext.RunAsDepth = 1
	return machine
}

func TestDotfixFollowupNativeRows(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/dotfix_fls_followups/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma, r.LazyQuotes = '\t', true
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, row := range rows[1:] {
		if row[2] != "test" || row[3] != "minimal" || (row[1] != "62.0" && row[1] != "67.0") {
			continue
		}
		var native dotfixNativeProbe
		if err := json.Unmarshal([]byte(row[6]), &native); err != nil {
			t.Fatal(err)
		}
		probe := row[0]
		source := ""
		switch {
		case strings.HasPrefix(probe, "custom.strip."):
			mode := strings.ToUpper(strings.TrimPrefix(probe, "custom.strip."))
			source = "SObjectAccessDecision d=Security.stripInaccessible(AccessType." + mode + ",new List<SObject>{new SynthAccess__c(ReadField__c='synthetic-readable',HiddenField__c='synthetic-hidden')}); Map<String,Object> f=d.getRecords()[0].getPopulatedFieldsAsMap();"
			for _, field := range []string{"ReadField__c", "HiddenField__c"} {
				_, keep := native.RecordMaps[0][field]
				removed := false
				for _, v := range native.Removed["SynthAccess__c"] {
					if v == field {
						removed = true
					}
				}
				source += fmt.Sprintf("System.assertEquals(%t,f.containsKey('%s')); System.assertEquals(%t,d.getRemovedFields().containsKey('SynthAccess__c') && d.getRemovedFields().get('SynthAccess__c').contains('%s'));", keep, field, removed, field)
			}
		case probe == "case.strip.updatable":
			_, keep := native.RecordMaps[0]["OwnerId"]
			source = fmt.Sprintf("SObjectAccessDecision d=Security.stripInaccessible(AccessType.UPDATABLE,new List<SObject>{new Case(Id='500000000000990',OwnerId='005000000000991')}); System.assertEquals(%t,d.getRecords()[0].getPopulatedFieldsAsMap().containsKey('OwnerId'));", keep)
		case strings.HasPrefix(probe, "user.select."):
			parts := strings.Split(probe, ".")
			id := "005000000000990"
			if parts[2] == "other" {
				id = "005000000000991"
			}
			field := ""
			if parts[3] == "custom" {
				field = ",SynthUserField__c"
			}
			mode := strings.ToUpper(parts[4])
			query := fmt.Sprintf("SELECT Id,Name,Email%s FROM User WHERE Id='%s' WITH %s_MODE", field, id, mode)
			message := strings.ReplaceAll(native.Exception.Message, "'", "\\'")
			source = fmt.Sprintf("Boolean failed=false; String message=''; try { Database.query('%s'); } catch (QueryException e) { failed=true; message=e.getMessage(); } System.assertEquals(%t,failed);", strings.ReplaceAll(query, "'", "\\'"), native.Outcome == "exception")
			if native.Outcome == "exception" {
				source += fmt.Sprintf("System.assertEquals('%s',message);", message)
			}
		case probe == "case.select.user":
			source = fmt.Sprintf("List<Case> c=[SELECT Id,OwnerId FROM Case WITH USER_MODE]; System.assertEquals(%t,c.size()==1 && c[0].OwnerId!=null);", native.Outcome == "success")
		case probe == "case.update.user":
			source = fmt.Sprintf("Boolean failed=false;try {update as user new Case(Id='500000000000990',OwnerId='005000000000991');} catch(Exception e) {failed=true;}System.assertEquals(%t,failed);", native.Outcome == "exception")
		case probe == "case.insert.user":
			message := strings.ReplaceAll(native.Exception.Message, "'", "\\'")
			if len(native.Exception.Rows) != 1 {
				t.Fatal("native Case insert did not contain one DML row")
			}
			dml := native.Exception.Rows[0]
			var fields []string
			for _, field := range dml.Fields {
				fields = append(fields, "'"+strings.ReplaceAll(field, "'", "\\'")+"'")
			}
			source = fmt.Sprintf("Boolean failed=false; try { insert as user new Case(Subject='Synthetic',Status='New',Origin='Web',OwnerId='005000000000990'); } catch(DmlException e) { failed=true; System.assertEquals('%s',e.getMessage()); System.assertEquals(1,e.getNumDml()); System.assertEquals(new List<String>{%s},e.getDmlFieldNames(0)); System.assertEquals('%s',e.getDmlMessage(0)); System.assertEquals('%s',String.valueOf(e.getDmlType(0))); } System.assertEquals(%t,failed);", message, strings.Join(fields, ","), strings.ReplaceAll(dml.Message, "'", "\\'"), dml.Type, native.Outcome == "exception")
		}
		if source == "" {
			continue
		}
		checked++
		t.Run(row[1]+"/"+probe, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = dotfixRestrictedMachine(row[1]).ExecuteTestInClass(program, "SynthFlsCapture"); err != nil {
				t.Fatal(err)
			}
		})
	}
	if checked != 30 {
		t.Errorf("covered native cases=%d, want 30", checked)
	}
}
