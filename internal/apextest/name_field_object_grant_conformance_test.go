package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// CR-039/CD-117b observes object CRUD, field FLS and intrinsic write limits
// independently. Capture-org proof records remain evidence, not runtime output.
func TestNameFieldObjectGrantConformance(t *testing.T) {
	var data struct {
		APIVersions       []string `json:"apiVersions"`
		NativeCellCount   int      `json:"nativeCellCount"`
		AssertedCellCount int      `json:"assertedCellCount"`
		CarriedCellCount  int      `json:"carriedCellCount"`
		Captures          []struct {
			API          string `json:"api"`
			Route        string `json:"route"`
			OrgTSVSHA256 string `json:"orgTSVSHA256"`
			OrgRunSHA256 string `json:"orgRunSHA256"`
			Rows         []struct {
				ID            string            `json:"id"`
				Field         string            `json:"field"`
				Method        string            `json:"method"`
				Native        json.RawMessage   `json:"native"`
				Observed      map[string]any    `json:"observed"`
				CarryReason   string            `json:"carryReason"`
				CarriedFields map[string]string `json:"carriedFields"`
			} `json:"rows"`
		} `json:"captures"`
	}
	raw, err := os.ReadFile("testdata/conformance/name_field_object_grants.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || len(data.Captures) != 6 || data.NativeCellCount != 369 || data.AssertedCellCount != 285 || data.CarriedCellCount != 84 {
		t.Fatal("incomplete native matrix")
	}
	fixture := userModeInaccessibleFixture{Metadata: map[string]string{
		"objects/SyntheticFieldGrant__c/SyntheticFieldGrant__c.object-meta.xml": `<CustomObject><label>Synthetic grant record</label><pluralLabel>Synthetic grant records</pluralLabel><nameField><label>Record name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`,
		"objects/SyntheticFieldGrant__c/fields/Value__c.field-meta.xml":         `<CustomField><fullName>Value__c</fullName><label>Value</label><type>Text</type><length>80</length></CustomField>`,
		"objects/SyntheticSequence__c/SyntheticSequence__c.object-meta.xml":     `<CustomObject><label>Synthetic sequence</label><pluralLabel>Synthetic sequences</pluralLabel><nameField><label>Sequence</label><type>AutoNumber</type><displayFormat>S-{0000}</displayFormat></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`,
	}}
	asserted, carried, total := 0, 0, 0
	seen := map[string]bool{}
	for _, capture := range data.Captures {
		captureName := capture.API + "/" + capture.Route
		switch capture.API {
		case "62.0", "67.0", "68.0":
		default:
			t.Fatal("unexpected capture API")
		}
		wantRows := 57
		if capture.Route == "anonymous" {
			wantRows = 66
		} else if capture.Route != "istest" {
			t.Fatal("unexpected capture route")
		}
		if len(capture.Rows) != wantRows {
			t.Fatal("missing native rows")
		}
		rowIDs := map[string]bool{}
		for _, row := range capture.Rows {
			if !strings.HasPrefix(row.ID, "CR15_") || rowIDs[row.ID] {
				t.Fatal("duplicate or invalid native row")
			}
			rowIDs[row.ID] = true
		}
		if seen[captureName] || len(capture.OrgTSVSHA256) != 64 || len(capture.OrgRunSHA256) != 64 {
			t.Fatal("invalid capture provenance")
		}
		seen[captureName] = true
		t.Run(captureName, func(t *testing.T) {
			counts := newConformanceCountLog(t, "NameFieldObjectGrant", capture.API)
			observations := map[string]map[string]any{}
			var anonymousFault map[string]any
			if capture.Route == "istest" {
				for _, grant := range []string{"N", "O", "R", "W"} {
					source := nameFieldGrantSource(grant)
					index := userModeInaccessibleIndex(t, capture.API, fixture, source.Classes)
					org := orgFromIndex(index)
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org})
					_, execErr := runAsDefaultSecurityExecute(t, capture.API, capture.Route, source, index, runner)
					var runtimeErr *vm.RuntimeError
					if !errors.As(execErr, &runtimeErr) || userModeInaccessibleExceptionType(runtimeErr.Type) != "System.AssertException" {
						t.Fatalf("missing observation for %s: %v", grant, execErr)
					}
					_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "NAME_GRANT|")
					if !found {
						t.Fatalf("fixture failed for %s: %v", grant, execErr)
					}
					var values map[string]any
					if err := json.Unmarshal([]byte(payload), &values); err != nil {
						t.Fatal(err)
					}
					observations[grant] = values
				}
			} else if capture.Route == "anonymous" {
				index := userModeInaccessibleIndex(t, capture.API, fixture, nil)
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org})
				source := userModeInaccessibleSource{Anonymous: `System.runAs(new User()) { System.assert(false,'unreachable'); }`}
				_, execErr := runAsDefaultSecurityExecute(t, capture.API, capture.Route, source, index, runner)
				var runtimeErr *vm.RuntimeError
				if !errors.As(execErr, &runtimeErr) {
					t.Fatalf("missing anonymous rejection: %v", execErr)
				}
				anonymousFault = map[string]any{"exceptionType": userModeInaccessibleExceptionType(runtimeErr.Type), "message": runtimeErr.ExceptionMessage()}
			} else {
				t.Fatal("invalid route")
			}
			for _, row := range capture.Rows {
				total++
				t.Run(row.ID, func(t *testing.T) {
					if len(row.Native) == 0 {
						t.Fatal("missing native evidence")
					}
					if row.CarryReason != "" {
						if len(row.Observed) != 0 {
							t.Fatal("carried observation asserted")
						}
						carried++
						counts.TrackKind(t, capture.Route, "carried", 1)
						t.Logf("carried: %s; native=%s", row.CarryReason, row.Native)
						return
					}
					var native map[string]any
					if err := json.Unmarshal(row.Native, &native); err != nil {
						t.Fatal(err)
					}
					projection := map[string]any{}
					if capture.Route == "anonymous" {
						message := native["exceptionMessage"]
						if message == nil {
							message = native["message"]
						}
						projection = map[string]any{"exceptionType": native["exceptionType"], "message": message}
					} else {
						permissionable, ok := native["permissionable"]
						if !ok {
							permissionable = native["isPermissionable"]
						}
						crud, ok := native["crud"]
						if !ok {
							crud = native["actorCrud"]
						}
						if flags, ok := crud.(map[string]any); ok {
							crud = []any{flags["read"], flags["create"], flags["edit"], flags["delete"]}
						}
						projection = map[string]any{"value": native["value"], "permissionable": permissionable, "crud": crud}
					}
					for field := range row.CarriedFields {
						delete(projection, field)
					}
					if !reflect.DeepEqual(projection, row.Observed) {
						t.Fatalf("native projection changed: %v; stored=%v", projection, row.Observed)
					}
					asserted++
					counts.TrackKind(t, capture.Route, "partial", 1)
					if len(row.Observed) == 0 {
						t.Fatal("observation silently omitted")
					}
					got := anonymousFault
					if capture.Route == "istest" {
						parts := strings.Split(row.ID, "_")
						if len(parts) != 4 {
							t.Fatal("invalid row ID")
						}
						fields, ok := observations[parts[2]][row.Field].(map[string]any)
						if !ok {
							t.Fatalf("missing field observation: %v", observations[parts[2]])
						}
						got = map[string]any{"value": fields[row.Method], "permissionable": fields["permissionable"], "crud": fields["crud"]}
					}
					for field, reason := range row.CarriedFields {
						if field != "permissionable" || reason == "" {
							t.Fatal("invalid carried facet")
						}
						t.Logf("carried %s: %s; native=%s", field, reason, row.Native)
						delete(got, field)
					}
					if !reflect.DeepEqual(got, row.Observed) {
						t.Fatalf("native=%v; Glade=%v", row.Observed, got)
					}
				})
			}
		})
	}
	if total != data.NativeCellCount || asserted != data.AssertedCellCount || carried != data.CarriedCellCount {
		t.Fatalf("native cell coverage: total=%d asserted=%d carried=%d", total, asserted, carried)
	}
}

func nameFieldGrantSource(grant string) userModeInaccessibleSource {
	grantSetup := ""
	if grant != "N" {
		grantSetup = `PermissionSet grant=new PermissionSet(Name='SyntheticObjectGrant',Label='Synthetic object grant'); insert grant;
for(String objectName:new List<String>{'Account','SyntheticFieldGrant__c','SyntheticSequence__c'}) {
 insert new ObjectPermissions(ParentId=grant.Id,SObjectType=objectName,PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=true,PermissionsDelete=true);
}
insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=grant.Id);`
		if grant == "R" || grant == "W" {
			grantSetup += fmt.Sprintf(`insert new FieldPermissions(ParentId=grant.Id,SObjectType='SyntheticFieldGrant__c',Field='SyntheticFieldGrant__c.Value__c',PermissionsRead=true,PermissionsEdit=%t);`, grant == "W")
		}
	}
	source := `@IsTest private class NameGrantObservation {
@IsTest static void observe() {
 Profile profile=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' LIMIT 1];
 User actor=new User(ProfileId=profile.Id,Alias='ngrant',LastName='Synthetic',Email='name.grant@example.invalid',Username='name.grant@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles');
 insert actor;
 ` + grantSetup + `
 Map<String,Object> out=new Map<String,Object>();
 System.runAs(actor) {
  for(String qualified:new List<String>{'Account.Name','SyntheticFieldGrant__c.Name','SyntheticFieldGrant__c.Value__c','Account.CreatedDate','SyntheticSequence__c.Name'}) {
   List<String> parts=qualified.split('\\.');
   Schema.DescribeSObjectResult objectDescribe=Schema.getGlobalDescribe().get(parts.get(0)).getDescribe();
   Schema.DescribeFieldResult field=objectDescribe.fields.getMap().get(parts.get(1)).getDescribe();
   out.put(qualified,new Map<String,Object>{'isAccessible'=>field.isAccessible(),'isCreateable'=>field.isCreateable(),'isUpdateable'=>field.isUpdateable(),'permissionable'=>field.isPermissionable(),'crud'=>new List<Boolean>{objectDescribe.isAccessible(),objectDescribe.isCreateable(),objectDescribe.isUpdateable(),objectDescribe.isDeletable()}});
  }
 }
 System.assert(false,'NAME_GRANT|'+JSON.serialize(out));
}}
`
	return userModeInaccessibleSource{Classes: map[string]string{"NameGrantObservation": source}, Tests: []string{"NameGrantObservation"}}
}
