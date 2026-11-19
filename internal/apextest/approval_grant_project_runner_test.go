package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApprovalGrantProjectRunnerAndWarm(t *testing.T) {
	setup, err := os.ReadFile("testdata/conformance/approval_grant_activity/project/setup.apex")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, state := range []string{"muted", "direct", "revoked", "expired", "global_object_muted", "global_muted", "global_both_muted", "global_direct", "global_revoked"} {
			t.Run(api+"/"+state, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"name":"GrantRunner","sourceApiVersion":"`+api+`"}`)
				writeFile(t, filepath.Join(root, "force-app/main/default/profiles/Approval Test User.profile-meta.xml"), `<Profile xmlns="http://soap.sforce.com/2006/04/metadata"><objectPermissions><object>Account</object><allowCreate>true</allowCreate><allowRead>true</allowRead><allowEdit>true</allowEdit><allowDelete>true</allowDelete><viewAllRecords>false</viewAllRecords><modifyAllRecords>false</modifyAllRecords></objectPermissions></Profile>`)
				body := strings.ReplaceAll(string(setup), "System.debug('APPROVAL_GRANT_SETUP_FAIL|'+e.getMessage());", "System.assert(false,e.getMessage());")
				if strings.HasPrefix(state, "global_") {
					mute := "true"
					if state == "global_object_muted" {
						mute = "false"
					}
					body = `PermissionSet ps=new PermissionSet(Name='ApprovalGrant',Label='Approval Grant',PermissionsModifyAllData=true);insert ps;insert new ObjectPermissions(ParentId=ps.Id,SObjectType='Account',PermissionsModifyAllRecords=true);PermissionSetGroup pg=new PermissionSetGroup(DeveloperName='ApprovalGroup',MasterLabel='Approval Group');insert pg;insert new PermissionSetGroupComponent(PermissionSetGroupId=pg.Id,PermissionSetId=ps.Id);MutingPermissionSet m=new MutingPermissionSet(DeveloperName='ApprovalMute',MasterLabel='Approval Mute',PermissionsModifyAllData=` + mute + `);insert m;`
					if state != "global_muted" {
						body += `insert new ObjectPermissions(ParentId=m.Id,SObjectType='Account',PermissionsModifyAllRecords=true);`
					}
					body += `insert new PermissionSetGroupComponent(PermissionSetGroupId=pg.Id,PermissionSetId=m.Id);`
				}
				body += `Profile p=[SELECT Id FROM Profile WHERE Name='Approval Test User' LIMIT 1]; User u=new User(Alias='probe',Email='probe@example.invalid',EmailEncodingKey='UTF-8',LastName='Probe',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',Username='approval-runner@example.invalid'); insert u; PermissionSetGroup g=[SELECT Id FROM PermissionSetGroup WHERE DeveloperName='ApprovalGroup']; insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetGroupId=g.Id);`
				if state == "direct" || state == "revoked" || state == "expired" || state == "global_direct" || state == "global_revoked" {
					body += `PermissionSet grant=[SELECT Id FROM PermissionSet WHERE Name='ApprovalGrant']; PermissionSetAssignment a=new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=grant.Id);`
					if state == "expired" {
						body += `a.ExpirationDate=System.now().addDays(-1);`
					}
					body += `insert a;`
					if state == "revoked" || state == "global_revoked" {
						body += `a.IsRevoked=true; update a;`
					}
				}
				want := "false:ENTITY_IS_LOCKED"
				if state == "direct" || state == "global_object_muted" || state == "global_muted" || state == "global_direct" {
					want = "true:NONE"
				}
				body += `System.runAs(u){Account account=new Account(Name='Probe');insert account;Approval.lock(account);account.Name='Changed';Database.SaveResult result=Database.update(account,false);String value=String.valueOf(result.isSuccess())+':'+(result.isSuccess()?'NONE':String.valueOf(result.getErrors()[0].getStatusCode()));System.assertEquals('` + want + `',value);}`
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/ApprovalGrantRunner.cls"), `@IsTest private class ApprovalGrantRunner { @IsTest static void capture(){`+body+`} }`)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/ApprovalGrantRunner.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				index := loadTestIndex(t, root)
				for _, phase := range []string{"cold", "warm"} {
					report := Run(index, Options{SelectedClasses: []string{"ApprovalGrantRunner"}, SelectedMethod: "capture", RuntimeRESTAPIVersion: "67.0", Parallelism: 1})
					if summary := report.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
						for _, suite := range report.Suites {
							for _, row := range suite.Cases {
								if row.Problem != nil {
									t.Logf("%s %s: %s", phase, row.Problem.Type, row.Problem.Message)
								}
							}
						}
						t.Fatalf("%s project runner: %+v", phase, summary)
					}
				}
			})
		}
	}
}
