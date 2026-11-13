package vm

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

type permissionTransportRow struct {
	api, principal, operation, success, status, message, fields, idNull string
	dmlStatements, dmlRows                                              int
}

// The fixture rows are native named @IsTest observations at APIs 62 and 67.
// The actor is a synthetic Standard User with no object grant, read-only
// object access, or an assigned read/create/edit permission set.
func TestUserModeDeniedInsertPartialErrorTransport(t *testing.T) {
	for _, row := range loadPermissionTransportRows(t) {
		t.Run(row.api+"/"+row.principal+"/"+row.operation, func(t *testing.T) {
			machine := permissionTransportVM(row.api, row.principal)
			source := ""
			fieldCount := 0
			if row.fields != "" {
				fieldCount = len(strings.Split(row.fields, ","))
			}
			dmlCounters := ""
			if row.dmlStatements >= 0 && row.dmlRows >= 0 {
				dmlCounters = "System.assertEquals(" + fmt.Sprint(row.dmlStatements) + ",Limits.getDmlStatements());System.assertEquals(" + fmt.Sprint(row.dmlRows) + ",Limits.getDmlRows());"
			}
			if row.operation == "single" {
				source = `SObject record=Schema.getGlobalDescribe().get('PermissionProbe__c').newSObject();record.put('Name','Synthetic');record.put('Summary__c','value');
Database.SaveResult result=Database.insert(record,false,AccessLevel.USER_MODE);
System.assertEquals(` + row.success + `,result.isSuccess());
System.assertEquals(` + row.idNull + `,result.getId()==null);
` + dmlCounters + `
if(result.isSuccess()){System.assertEquals(0,result.getErrors().size());}else{System.assertEquals(1,result.getErrors().size());System.assertEquals('` + row.status + `',String.valueOf(result.getErrors()[0].getStatusCode()));System.assertEquals('` + strings.ReplaceAll(row.message, "Probe463__c", "PermissionProbe__c") + `',result.getErrors()[0].getMessage());System.assertEquals(` + fmt.Sprint(fieldCount) + `,result.getErrors()[0].getFields().size());}`
			} else {
				source = `SObject first=Schema.getGlobalDescribe().get('PermissionProbe__c').newSObject();first.put('Name','One');first.put('Summary__c','value');SObject second=Schema.getGlobalDescribe().get('PermissionProbe__c').newSObject();second.put('Name','Two');second.put('Summary__c','value');
List<Database.SaveResult> results=Database.insert(new List<SObject>{first,second},false,AccessLevel.USER_MODE);
System.assertEquals(2,results.size());System.assertEquals(` + row.success + `,results[0].isSuccess());System.assertEquals(` + row.success + `,results[1].isSuccess());
` + dmlCounters + `
for(Database.SaveResult result:results){System.assertEquals(` + row.idNull + `,result.getId()==null);if(result.isSuccess()){System.assertEquals(0,result.getErrors().size());}else{System.assertEquals(1,result.getErrors().size());System.assertEquals('` + row.status + `',String.valueOf(result.getErrors()[0].getStatusCode()));System.assertEquals('` + strings.ReplaceAll(row.message, "Probe463__c", "PermissionProbe__c") + `',result.getErrors()[0].getMessage());System.assertEquals(` + fmt.Sprint(fieldCount) + `,result.getErrors()[0].getFields().size());}}`
			}
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row.api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatalf("partial USER_MODE insert: %v", err)
			}
		})
	}
}

func TestDynamicSecurityEnforcedPermissionMessages(t *testing.T) {
	f, err := os.Open("testdata/permission_error_transport/security_enforced_permission_messages.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || strings.Join(rows[0], "\t") != "api\tprincipal\tprojection\texception\tmessage" {
		t.Fatalf("invalid query transport fixture header: %#v", rows)
	}
	for _, row := range rows[1:] {
		if len(row) != 5 {
			t.Fatalf("invalid query transport row: %#v", row)
		}
		projection := strings.ReplaceAll(row[2], "Note__c", "Summary__c")
		t.Run(row[0]+"/"+row[1]+"/"+row[2], func(t *testing.T) {
			machine := permissionTransportVM(row[0], row[1])
			program, err := CompileAnonymousWithOptions(`try { Database.query('SELECT `+projection+` FROM PermissionProbe__c WITH SECURITY_ENFORCED'); System.assert(false,'expected denial'); }
catch(QueryException error) { System.assertEquals('`+row[3]+`',error.getTypeName());System.assertEquals('`+row[4]+`',error.getMessage()); }`, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatalf("dynamic SECURITY_ENFORCED query: %v", err)
			}
		})
	}
}

func loadPermissionTransportRows(t *testing.T) []permissionTransportRow {
	t.Helper()
	f, err := os.Open("testdata/permission_error_transport/user_mode_dml_transport.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	csvReader := csv.NewReader(f)
	csvReader.Comma = '\t'
	rows, err := csvReader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || strings.Join(rows[0], "\t") != "api\tprincipal\toperation\tsuccess\tstatus\tmessage\tfields\tidNull\tdmlStatements\tdmlRows" {
		t.Fatalf("invalid native transport fixture header: %#v", rows)
	}
	out := make([]permissionTransportRow, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) != 10 {
			t.Fatalf("invalid native transport row: %#v", row)
		}
		statements, dmlRows := -1, -1
		if row[8] != "" {
			statements, err = strconv.Atoi(row[8])
			if err != nil {
				t.Fatal(err)
			}
			dmlRows, err = strconv.Atoi(row[9])
			if err != nil {
				t.Fatal(err)
			}
		}
		out = append(out, permissionTransportRow{row[0], row[1], row[2], row[3], row[4], row[5], row[6], row[7], statements, dmlRows})
	}
	return out
}

func permissionTransportVM(api, principal string) *VM {
	org := storage.NewOrgState()
	org.APIVersion = api
	org.Objects["PermissionProbe__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "PermissionProbe__c", KeyPrefix: "a00", SharingModel: "ReadWrite", Fields: map[string]storage.Field{
			"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString, DisplayType: "STRING"}, "Summary__c": {APIName: "Summary__c", Type: storage.FieldString, DisplayType: "STRING"},
		}},
		Records: map[storage.ID]storage.Record{},
	}
	const profileID storage.ID = "00e000000000002"
	const userID = "005000000000002"
	user := Object("User")
	user.Fields["Id"] = String(userID)
	user.Fields["ProfileId"] = String(string(profileID))
	profileName := "Standard User"
	read, create, fieldRead, fieldWrite := false, false, false, false
	permissionParent := profileID
	switch principal {
	case "admin":
		profileName, read, create, fieldRead, fieldWrite = "System Administrator", true, true, true, true
	case "granted":
		permissionParent = storage.ID("0PS000000000002")
		read, create, fieldRead, fieldWrite = true, true, true, true
	case "readOnly", "fieldControl":
		read = true
	case "ungranted":
	default:
		panic("unknown synthetic principal: " + principal)
	}
	org.Objects["Profile"] = storage.ObjectState{Records: map[storage.ID]storage.Record{profileID: {
		ID: profileID, Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue(profileName)},
	}}}
	if principal == "granted" {
		org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{permissionParent: {
			ID: permissionParent, Object: "PermissionSet", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe_Access")},
		}}}
		org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0Pa000000000002": {
			ID: "0Pa000000000002", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(userID), "PermissionSetId": storage.IDValue(permissionParent)},
		}}}
	}
	org.Objects["ObjectPermissions"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"110000000000001": {ID: "110000000000001", Object: "ObjectPermissions", Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(permissionParent), "SObjectType": storage.StringValue("PermissionProbe__c"),
			"PermissionsRead": storage.BooleanValue(read), "PermissionsCreate": storage.BooleanValue(create),
			"PermissionsEdit": storage.BooleanValue(create), "PermissionsDelete": storage.BooleanValue(false),
		}},
	}}
	org.Objects["FieldPermissions"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"0FP000000000001": {ID: "0FP000000000001", Object: "FieldPermissions", Fields: map[string]storage.Value{
			"ParentId": storage.IDValue(permissionParent), "SObjectType": storage.StringValue("PermissionProbe__c"),
			"Field": storage.StringValue("PermissionProbe__c.Summary__c"), "PermissionsRead": storage.BooleanValue(fieldRead),
			"PermissionsEdit": storage.BooleanValue(fieldWrite),
		}},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.CurrentUser = user
	machine.testContext.RunAsDepth = 1
	machine.currentClass = "PermissionTransportTest"
	machine.currentMethod = Method{APIVersion: api}
	return machine
}
