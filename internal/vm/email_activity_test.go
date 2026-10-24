package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// email_activity/org.tsv captures both APIs and anonymous/named execution.
func TestEmailActivityDirectVMRouteTwins(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, testContext := range []bool{false, true} {
			route := "anonymous"
			if testContext {
				route = "isTest"
			}
			for _, tc := range []struct{ name, setup, assertions string }{
				{"success", "", `System.assertEquals(true, sent[0].isSuccess());
System.assertEquals(0, sent[0].getErrors().size());
System.assertEquals(1, tasks.size());
System.assertEquals('Email: Synthetic activity', tasks[0].Subject);
System.assertEquals('To: recipient@example.test\nCC: \nBCC: \nAttachment: --none--\n\nSubject: Synthetic activity\nBody:\nSynthetic content', tasks[0].Description);
System.assertEquals('Completed', tasks[0].Status);
System.assertEquals('Normal', tasks[0].Priority);
System.assertEquals('Email', tasks[0].TaskSubtype);
System.assertEquals(parent.Id, tasks[0].WhatId);
System.assertEquals(target.Id, tasks[0].WhoId);
System.assertEquals(UserInfo.getUserId(),tasks[0].OwnerId);
System.assertEquals(Date.today(),tasks[0].ActivityDate);
System.assertEquals(true,tasks[0].IsClosed);`},
				{"disabled", "message.setSaveAsActivity(false);", "System.assertEquals(0,tasks.size());"},
				{"missingWhat", "message.setWhatId(null);", "System.assertEquals(1,tasks.size());System.assertEquals(null,tasks[0].WhatId);"},
				{"userWhat", "message.setWhatId(UserInfo.getUserId());", "System.assertEquals(false,sent[0].isSuccess());System.assertEquals('INVALID_ID_FIELD',String.valueOf(sent[0].getErrors()[0].getStatusCode()));System.assertEquals(0,tasks.size());"},
				{"contactWhat", "message.setWhatId(target.Id);", "System.assertEquals(false,sent[0].isSuccess());System.assertEquals('INVALID_ID_FIELD',String.valueOf(sent[0].getErrors()[0].getStatusCode()));System.assertEquals(0,tasks.size());"},
				{"missingReference", "message.setWhatId('001000000000999AAA');", "System.assertEquals(false,sent[0].isSuccess());System.assertEquals('INVALID_CROSS_REFERENCE_KEY',String.valueOf(sent[0].getErrors()[0].getStatusCode()));System.assertEquals(0,tasks.size());"},
				{"deletedWhat", "Account removed=new Account(Name='Removed');insert removed;Id removedId=removed.Id;delete removed;message.setWhatId(removedId);", "System.assertEquals(false,sent[0].isSuccess());System.assertEquals('ENTITY_IS_DELETED',String.valueOf(sent[0].getErrors()[0].getStatusCode()));System.assertEquals(0,tasks.size());"},
			} {
				t.Run(api+"/"+route+"/"+tc.name, func(t *testing.T) {
					source := fmt.Sprintf(`Account parent=new Account(Name='Activity Parent');insert parent;
Contact target=new Contact(LastName='Recipient',Email='recipient@example.test',AccountId=parent.Id);insert target;
Messaging.SingleEmailMessage message=new Messaging.SingleEmailMessage();
message.setTargetObjectId(target.Id);message.setWhatId(parent.Id);
message.setSubject('Synthetic activity');message.setPlainTextBody('Synthetic content');message.setSaveAsActivity(true);
%s
Messaging.SendEmailResult[] sent=Messaging.sendEmail(new Messaging.SingleEmailMessage[]{message},false);
List<Task> tasks=[SELECT Subject,Description,Status,Priority,TaskSubtype,WhoId,WhatId,OwnerId,ActivityDate,IsClosed FROM Task WHERE WhoId=:target.Id];
%s`, tc.setup, tc.assertions)
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := storage.NewOrgState()
					org.APIVersion = api
					storage.EnsureDeterministicPlatformData(&org)
					for _, name := range []string{"Account", "Contact", "Task"} {
						storage.EnsureStandardObject(&org, name)
					}
					machine := New(nil)
					machine.SetOrg(&org)
					if testContext {
						machine.EnableTestContext()
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
