package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native oracle: email_activity_routes/org.tsv, enabled/noActivities/deleted/
// missing/disabled/rollback, Default/Partial, anonymous/isTest at API 62 and 67.
func TestEmailActivityCustomWhatDirectVMRouteTwins(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, context := range []string{"anonymous", "isTest"} {
			for _, overload := range []string{"Default", "Partial"} {
				for _, shape := range []string{"enabled", "noActivities", "deleted", "missing", "disabled", "rollback"} {
					t.Run(api+"/"+context+"/"+shape+overload, func(t *testing.T) {
						org := storage.NewOrgState()
						org.APIVersion = api
						storage.EnsureDeterministicPlatformData(&org)
						for _, name := range []string{"Contact", "Task"} {
							storage.EnsureStandardObject(&org, name)
						}
						for i, name := range []string{"ActivityParent__c", "PlainParent__c"} {
							enabled := "true"
							if i == 1 {
								enabled = "false"
							}
							definition := storage.ObjectDefinition{APIName: name, KeyPrefix: fmt.Sprintf("a%02d", i+40), Metadata: map[string]string{"enableActivities": enabled}, Fields: map[string]storage.Field{}}
							storage.EnsureStandardObjectFields(&definition)
							org.Objects[name] = storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
						}
						setup := ""
						wantTasks := 1
						status := ""
						switch shape {
						case "noActivities":
							setup = "PlainParent__c plain=new PlainParent__c(Name='Plain');insert plain;related=plain.Id;"
							status = "INVALID_ID_FIELD"
						case "deleted":
							setup = "delete parent;"
							status = "ENTITY_IS_DELETED"
						case "missing":
							setup = "related=String.valueOf(parent.Id).substring(0,3)+'000000000999';"
							status = "INVALID_CROSS_REFERENCE_KEY"
						case "disabled":
							setup = "message.setSaveAsActivity(false);"
							wantTasks = 0
						}
						args := ""
						if overload == "Partial" {
							args = ",false"
						}
						assertion := "System.assertEquals(true,sent[0].isSuccess());"
						catch := "System.assert(false,'unexpected email exception');"
						if status != "" {
							wantTasks = 0
							if overload == "Default" {
								assertion = "System.assert(false,'expected email exception');"
								catch = "System.assert(issue.getMessage().contains('" + status + "'));"
							} else {
								assertion = "System.assertEquals(false,sent[0].isSuccess());System.assertEquals('" + status + "',String.valueOf(sent[0].getErrors()[0].getStatusCode()));"
							}
						}
						after := fmt.Sprintf("System.assertEquals(%d,tasks.size());", wantTasks)
						if wantTasks == 1 {
							after += "System.assertEquals(related,tasks[0].WhatId);System.assertEquals(target.Id,tasks[0].WhoId);System.assertEquals('Email: Custom activity',tasks[0].Subject);"
						}
						if shape == "rollback" {
							after += "Database.rollback(point);System.assertEquals(0,[SELECT COUNT() FROM Task WHERE WhoId=:target.Id]);"
						}
						source := fmt.Sprintf(`ActivityParent__c parent=new ActivityParent__c(Name='Custom Parent');insert parent;
Contact target=new Contact(LastName='Recipient',Email=UserInfo.getUserEmail());insert target;
Id related=parent.Id;
Messaging.SingleEmailMessage message=new Messaging.SingleEmailMessage();message.setTargetObjectId(target.Id);message.setSubject('Custom activity');message.setPlainTextBody('Custom body');
%s
message.setWhatId(related);Savepoint point=Database.setSavepoint();
try { Messaging.SendEmailResult[] sent=Messaging.sendEmail(new Messaging.SingleEmailMessage[]{message}%s);%s } catch(System.EmailException issue){%s}
List<Task> tasks=[SELECT WhatId,WhoId,Subject FROM Task WHERE WhoId=:target.Id];%s`, setup, args, assertion, catch, after)
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						machine := New(nil)
						machine.SetOrg(&org)
						if context == "isTest" {
							machine.EnableTestContext()
						}
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
						// A custom object remains absent from the static standard Task catalog.
						for _, name := range org.Objects["Task"].Definition.Fields["WhatId"].ReferenceTo {
							if strings.EqualFold(name, "ActivityParent__c") {
								t.Fatal("test unexpectedly extended the standard target list")
							}
						}
					})
				}
			}
		}
	}
}
