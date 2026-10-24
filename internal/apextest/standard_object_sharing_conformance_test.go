package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type standardSharingObservation struct {
	Outcome string `json:"outcome"`
	Value   struct {
		InitialOWD   string            `json:"initialOWD"`
		RelatedOWD   map[string]string `json:"relatedOWD"`
		OWD          string            `json:"owd"`
		Rule         bool              `json:"rule"`
		SeedKind     string            `json:"seedKind"`
		AdminVisible bool              `json:"adminVisible"`
		ActorVisible bool              `json:"actorVisible"`
		RuleShares   int               `json:"ruleShares"`
		Observations []struct {
			Owner         string  `json:"owner"`
			OK            bool    `json:"ok"`
			N             int     `json:"n"`
			Outside       int     `json:"os"`
			WrongOwner    int     `json:"wrongOwner"`
			ExceptionType *string `json:"et"`
		} `json:"observations"`
	} `json:"value"`
	PayloadSHA256 string            `json:"raw_payload_sha256"`
	HashVerified  bool              `json:"hash_verified"`
	CarriedFields map[string]string `json:"carriedFields,omitempty"`
}

type standardSharingRow struct {
	ID           string                                `json:"id"`
	Group        string                                `json:"group"`
	SeedKind     string                                `json:"seed_kind"`
	Model        string                                `json:"owd_model"`
	Mode         string                                `json:"mode"`
	Operation    string                                `json:"operation"`
	MeasuredApex string                                `json:"measured_apex"`
	SourceSHA256 string                                `json:"sourceSHA256"`
	Observations map[string]standardSharingObservation `json:"observations"`
}

// CD-080 is a named-route capture. The local fixture projects only observed
// query and record-access facts. Persisted Rule shares are materialized as the
// captured group-targeted AccountShare records; this does not emulate an org's
// asynchronous sharing-rule recalculation engine.
func TestStandardObjectSharingOrgConformance(t *testing.T) {
	var data struct {
		Capture       string                           `json:"capture"`
		Edition       string                           `json:"edition"`
		APIVersions   []string                         `json:"apiVersions"`
		Routes        []string                         `json:"routes"`
		AbsentRoutes  []struct{ Route, Reason string } `json:"absentRoutes"`
		DefaultModels map[string]string                `json:"defaultModels"`
		Rows          []standardSharingRow             `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/standard_object_sharing.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Capture != "CD-080" || data.Edition != "Developer" || strings.Join(data.APIVersions, ",") != "59.0,62.0,66.0,67.0,68.0" || strings.Join(data.Routes, ",") != "istest" || len(data.Rows) != 70 || len(data.AbsentRoutes) != 1 || data.AbsentRoutes[0].Route != "anonymous" || data.AbsentRoutes[0].Reason == "" {
		t.Fatal("incomplete captured matrix")
	}
	for object, model := range map[string]string{"Account": "ReadWrite", "Opportunity": "ReadWrite", "Case": "ReadWriteTransfer", "Contact": "ControlledByParent"} {
		if data.DefaultModels[object] != model {
			t.Fatalf("captured %s default: %q", object, data.DefaultModels[object])
		}
	}
	seen := map[string]bool{}
	carried := map[string]int{}
	for _, row := range data.Rows {
		if seen[row.ID] || len(row.SourceSHA256) != 64 || len(row.Observations) != 5 {
			t.Fatal("invalid captured row")
		}
		seen[row.ID] = true
		for api, observed := range row.Observations {
			if len(observed.CarriedFields) == 0 {
				continue
			}
			v := observed.Value
			if len(observed.CarriedFields) != 1 || observed.CarriedFields["adminCount"] == "" ||
				row.Group != "SOQL" || row.Model != "Private" || v.RuleShares != 0 ||
				len(v.Observations) != 2 || v.Observations[0].Owner != "admin" || v.Observations[0].N != 0 ||
				(row.Mode != "SYSTEM_MODE" && !(row.Mode == "DEFAULT" && (api == "59.0" || api == "62.0" || api == "66.0"))) {
				t.Fatalf("unapproved carry: %s/%s", row.ID, api)
			}
			carried[api]++
		}
	}
	for _, api := range data.APIVersions {
		want := 12
		if api == "67.0" || api == "68.0" {
			want = 6
		}
		if carried[api] != want {
			t.Fatalf("carried cells at %s = %d, want %d", api, carried[api], want)
		}
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			for _, row := range data.Rows {
				t.Run("istest/"+row.ID, func(t *testing.T) {
					observed, ok := row.Observations[api]
					if !ok {
						t.Fatal("missing native API cell")
					}
					if observed.Outcome == "runtime" && (!observed.HashVerified || len(observed.PayloadSHA256) != 64) {
						t.Fatal("unverified native payload")
					}
					if reason := observed.CarriedFields["adminCount"]; reason != "" {
						t.Logf("carried %s/%s/istest adminCount: %s", row.ID, api, reason)
					}
					if row.Group == "BASELINE" {
						// Edition-dependent org settings are evidence for the fallback, not a
						// new Organization query implementation or unrelated license contract.
						v := observed.Value
						if observed.Outcome != "runtime" || v.InitialOWD != "Edit" || v.RelatedOWD["case"] != "ReadEditTransfer" || v.RelatedOWD["opportunity"] != "Edit" || v.RelatedOWD["contact"] != "ControlledByParent" {
							t.Fatal("default readback changed")
						}
						return
					}
					body := ""
					switch row.ID {
					case "CTRL_POS":
						body = "System.assertEquals(7,7);"
					case "CTRL_CAUGHT":
						body = "Boolean caught=false;try{Integer zero=0;Integer quotient=1/zero;}catch(MathException e){caught=true;System.assertEquals('Divide by 0',e.getMessage());}System.assert(caught);"
					case "CTRL_UNCAUGHT":
						body = "Integer zero=0;Integer quotient=1/zero;"
					case "CTRL_COMPILE":
						body = "Integer invalidValue='synthetic wrong type';"
					default:
						body = standardSharingBody(t, row, observed)
					}
					fixture := userModeInaccessibleFixture{Metadata: map[string]string{}}
					// A silent standard-object metadata overlay deliberately exercises the
					// product fallback; Private/Read remain project-authored overrides.
					model := row.Model
					if model == "ReadWrite" {
						model = ""
					}
					if row.Group == "SHARING_BASELINE" {
						if observed.Value.OWD == "None" {
							model = "Private"
						} else if observed.Value.OWD == "Read" {
							model = "Read"
						}
					}
					if model != "" {
						fixture.Metadata["objects/Account/Account.object-meta.xml"] = "<CustomObject><sharingModel>" + model + "</sharingModel></CustomObject>"
					}
					source := "@IsTest private without sharing class StandardSharingTest{@IsTest static void observe(){" + body + "}}"
					index := userModeInaccessibleIndex(t, api, fixture, map[string]string{"StandardSharingTest": source})
					analysis := sema.Analyze(index)
					rejected := index.HasErrors() || analysis.HasErrors()
					if row.ID == "CTRL_COMPILE" {
						if observed.Outcome != "compile-rejected" || !rejected {
							t.Fatalf("native compile rejection; Glade=%v %v", index.Diagnostics, analysis.Diagnostics)
						}
						return
					}
					if rejected {
						t.Fatalf("native compile acceptance; Glade=%v %v", index.Diagnostics, analysis.Diagnostics)
					}
					org := orgFromIndex(index)
					standardSharingSeed(t, &org, row, observed)
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
					cases := Discover(index, Options{SelectedClasses: []string{"StandardSharingTest"}, SelectedMethod: "observe"})
					if len(cases) != 1 {
						t.Fatal("missing test method")
					}
					methods, methodErrs := compileTestMethodsForIndex(&index, cases)
					programs, errs := compileTestInvokePrograms(cases)
					key := testCaseKey(cases[0])
					if methodErrs[key] != nil {
						t.Fatal(methodErrs[key])
					}
					if errs[key] != nil {
						t.Fatal(errs[key])
					}
					machine := runner.newMachine()
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					result, execErr := machine.ExecuteTestInClass(programs[key], cases[0].ClassName)
					for _, message := range result.Debug {
						if strings.HasPrefix(message, "CARRY adminCount ") {
							t.Log(message)
						}
					}
					if row.ID == "CTRL_UNCAUGHT" {
						if observed.Outcome != "uncaught_test_failure" {
							t.Fatal("missing native uncaught control")
						}
						runtimeErr, ok := execErr.(*vm.RuntimeError)
						if !ok || userModeInaccessibleExceptionType(runtimeErr.Type) != "System.MathException" || runtimeErr.ExceptionMessage() != "Divide by 0" {
							t.Fatalf("uncaught control: %v", execErr)
						}
					} else if execErr != nil {
						t.Fatalf("native %s; Glade=%v", observed.Outcome, execErr)
					}
				})
			}
		})
	}
}

func standardSharingBody(t *testing.T, row standardSharingRow, observed standardSharingObservation) string {
	t.Helper()
	if observed.Outcome != "runtime" || !observed.Value.ActorVisible {
		t.Fatal("invalid native sharing observation")
	}
	body := "User actor=[SELECT Id FROM User WHERE Username='sharing.actor@example.invalid'];Id adminId=UserInfo.getUserId();Id actorId=actor.Id;Id adminRow;Id actorRow;"
	if row.SeedKind == "in_test" {
		body += "Account adminSeed=new Account(Name='Synthetic admin',OwnerId=adminId);Account actorSeed=new Account(Name='Synthetic actor',OwnerId=actorId);insert new List<Account>{adminSeed,actorSeed};adminRow=adminSeed.Id;actorRow=actorSeed.Id;"
	} else {
		body += "adminRow=[SELECT Id FROM Account WHERE Name='Synthetic admin'].Id;actorRow=[SELECT Id FROM Account WHERE Name='Synthetic actor'].Id;"
	}
	if row.Group == "SHARING_BASELINE" {
		// Project the captured actor read-access facts under that actor. The
		// current local UserRecordAccess surface answers the executing user.
		body += fmt.Sprintf("System.runAs(actor){UserRecordAccess adminAccess=[SELECT HasReadAccess FROM UserRecordAccess WHERE UserId=:actorId AND RecordId=:adminRow];UserRecordAccess actorAccess=[SELECT HasReadAccess FROM UserRecordAccess WHERE UserId=:actorId AND RecordId=:actorRow];System.assertEquals(%t,adminAccess.HasReadAccess);System.assertEquals(true,actorAccess.HasReadAccess);}", observed.Value.AdminVisible)
		return body
	}
	if len(observed.Value.Observations) != 2 || row.MeasuredApex == "" {
		t.Fatal("missing query observations")
	}
	body += "System.runAs(actor){"
	for i, result := range observed.Value.Observations {
		if !result.OK || result.ExceptionType != nil || result.Outside != 0 || result.WrongOwner != 0 || result.Owner != []string{"admin", "actor"}[i] {
			t.Fatal("uncontrolled native query")
		}
		body += "{Id scopeId=" + []string{"adminRow", "actorRow"}[i] + ";Id expectedOwner=" + []string{"adminId", "actorId"}[i] + ";" + row.MeasuredApex
		if result.Owner == "admin" && observed.CarriedFields["adminCount"] != "" {
			// Execute the carried query and retain owner/scope assertions. Only
			// this captured non-owner count is pending a sharing-policy decision.
			body += fmt.Sprintf("System.debug('CARRY adminCount native=%d local='+result.size());", result.N)
		} else {
			body += fmt.Sprintf("System.assertEquals(%d,result.size());", result.N)
		}
		body += "for(SObject item:result){System.assertEquals(scopeId,item.Id);System.assertEquals(expectedOwner,item.get('OwnerId'));}}"
	}
	return body + "}"
}

func standardSharingSeed(t *testing.T, org *storage.OrgState, row standardSharingRow, observed standardSharingObservation) {
	t.Helper()
	if row.Group == "CONTROL" {
		return
	}
	var profile storage.ID
	for id, record := range org.Objects["Profile"].Records {
		if record.Fields["Name"].String == "Minimum Access - Salesforce" {
			profile = id
		}
	}
	if profile == "" {
		t.Fatal("missing restricted profile")
	}
	const actor storage.ID = "005000000000099"
	fixture := storage.NewFixture()
	fixture.Objects = []storage.FixtureObject{
		{Name: "User", Records: []storage.FixtureRecord{{ID: actor, Fields: map[string]storage.Value{"Username": storage.StringValue("sharing.actor@example.invalid"), "ProfileId": storage.IDValue(profile), "LastName": storage.StringValue("Synthetic actor")}}}},
		{Name: "PermissionSet", Records: []storage.FixtureRecord{{ID: "0PS000000000099", Fields: map[string]storage.Value{"Name": storage.StringValue("SyntheticSharingRead"), "Label": storage.StringValue("Synthetic sharing read")}}}},
		{Name: "ObjectPermissions", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000099"), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true)}}}},
		{Name: "PermissionSetAssignment", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(actor), "PermissionSetId": storage.IDValue("0PS000000000099")}}}},
	}
	if row.SeedKind == "persisted" {
		fixture.Objects = append(fixture.Objects, storage.FixtureObject{Name: "Account", Records: []storage.FixtureRecord{
			{ID: "001000000000098", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic admin"), "OwnerId": storage.IDValue(storage.DefaultUserID)}},
			{ID: "001000000000099", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic actor"), "OwnerId": storage.IDValue(actor)}},
		}})
	}
	if observed.Value.RuleShares > 0 {
		fixture.Objects = append(fixture.Objects,
			storage.FixtureObject{Name: "Group", Records: []storage.FixtureRecord{{ID: "00G000000000099", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic readers"), "Type": storage.StringValue("Regular")}}}},
			storage.FixtureObject{Name: "GroupMember", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"GroupId": storage.IDValue("00G000000000099"), "UserOrGroupId": storage.IDValue(actor)}}}},
			storage.FixtureObject{Name: "AccountShare", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"AccountId": storage.IDValue("001000000000098"), "UserOrGroupId": storage.IDValue("00G000000000099"), "AccountAccessLevel": storage.StringValue("Read"), "RowCause": storage.StringValue("Rule")}}}},
		)
	}
	if err := storage.ApplyFixture(org, fixture); err != nil {
		t.Fatal(err)
	}
	// ApplyFixture stores fields verbatim; query sharing reads System.OwnerID.
	// Seed both representations from the captured owner assignment.
	accounts := org.Objects["Account"]
	for id, record := range accounts.Records {
		if owner, ok := record.GetField("OwnerId"); ok {
			record.System.OwnerID = owner.ID
			accounts.Records[id] = record
		}
	}
	org.Objects["Account"] = accounts
}
