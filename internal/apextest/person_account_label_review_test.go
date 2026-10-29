package apextest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Without a required lookup using the declared label, preserve the pre-fix
// projection. Qualified identities likewise retain their legacy projection.
func TestPersonAccountLabelProjectionBoundaries(t *testing.T) {
	for _, route := range []string{"fold", "profile"} {
		for _, tc := range []struct {
			name                                     string
			active, optional, qualified, match, want bool
		}{
			{"required", true, false, false, true, true},
			{"inactive", false, false, false, true, false},
			{"optional", true, true, false, true, false},
			{"different", true, false, false, false, false},
			{"qualified", true, false, true, true, false},
		} {
			t.Run(route+"/"+tc.name, func(t *testing.T) {
				account := storage.ObjectDefinition{APIName: "Account"}
				storage.EnsureStandardObjectFieldsForFeatures(&account, []string{"PersonAccounts"})
				if tc.qualified {
					account.RecordTypes = append(account.RecordTypes, storage.RecordTypeInfo{DeveloperName: "pkg__Other", Name: "Other"})
				}
				label := "Member"
				value := label
				if !tc.match {
					value = "Different"
				}
				org := storage.NewOrgState()
				org.Objects["Account"] = storage.ObjectState{Definition: account}
				org.Objects["PersonAccount"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "PersonAccount", RecordTypes: []storage.RecordTypeInfo{{DeveloperName: "PersonAccount", Name: label, Active: true, Available: true}}}}
				org.Objects["Probe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Probe__c", Fields: map[string]storage.Field{"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, FilteredLookupInfo: storage.FilteredLookupInfo{Active: tc.active, OptionalFilter: tc.optional, FilterItems: []storage.LookupFilterItem{{Field: "Account.RecordTypeId", Operation: "equals", Value: value}}}}}}}
				if route == "fold" {
					foldPersonAccountObjectIntoAccount(&org)
				} else if err := ApplyProjectProfileMetadata(&org, project.Project{}); err != nil {
					t.Fatal(err)
				}
				for _, rt := range org.Objects["Account"].Definition.RecordTypes {
					if rt.DeveloperName == "PersonAccount" {
						want := "Person Account"
						if tc.want {
							want = label
						}
						if rt.Name != want {
							t.Fatalf("name=%q want %q", rt.Name, want)
						}
						return
					}
				}
				t.Fatal("missing generic record type")
			})
		}
	}
}

func TestPersonAccountLabelSyntheticExecutionRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"orgName":"Synthetic","edition":"Developer","features":["PersonAccounts"]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/PersonAccount/PersonAccount.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Person Account</label><pluralLabel>Person Accounts</pluralLabel></CustomObject>`)
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/PersonAccount/recordTypes/PersonAccount.recordType-meta.xml"), `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>PersonAccount</fullName><active>true</active><label>Member</label></RecordType>`)
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Probe__c/Probe__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Probe</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Probes</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Probe__c/fields/Account__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Account__c</fullName><label>Account</label><lookupFilter><active>true</active><filterItems><field>Account.RecordTypeId</field><operation>equals</operation><value>Member</value></filterItems><isOptional>false</isOptional></lookupFilter><referenceTo>Account</referenceTo><relationshipLabel>Probes</relationshipLabel><relationshipName>Probes</relationshipName><type>Lookup</type></CustomField>`)
			body := `Id byName=Schema.SObjectType.Account.getRecordTypeInfosByName().get('Member').getRecordTypeId();
Id byDeveloper=Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('PersonAccount').getRecordTypeId();
Account a=new Account(LastName='Example',RecordTypeId=byDeveloper); insert a;
Probe__c p=new Probe__c(Name='static',Account__c=a.Id); insert p;
SObject q=Schema.getGlobalDescribe().get('Probe__c').newSObject(); q.put('Name','dynamic'); q.put('Account__c',a.Id); insert q;
System.assertEquals(byDeveloper,byName);`
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/LabelRoute.cls"), "public class LabelRoute { public static void check(){\n"+body+"\n} }")
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/LabelRoute.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/LabelRouteTest.cls"), "@IsTest private class LabelRouteTest { @IsTest static void check(){ LabelRoute.check(); } }")
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/LabelRouteTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			for _, route := range []struct{ name, source string }{{"anonymous", body}, {"named", "LabelRoute.check();"}} {
				t.Run(route.name, func(t *testing.T) {
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					program, err := vm.CompileAnonymousWithOptions(route.source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if result, err := runner.execute(program); err != nil {
						t.Fatalf("execute: %v; result=%#v", err, result)
					}
				})
			}
			t.Run("istest", func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"LabelRouteTest"}, NoDiskCache: true, Parallelism: 1})
				if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							if c.Problem != nil {
								t.Log(c.Problem.Message)
							}
						}
					}
					t.Fatalf("summary=%+v", got)
				}
			})
		})
	}
}
