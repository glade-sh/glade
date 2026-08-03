package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecMessagingSingleEmailMessageGettersCanonicalize15CharacterIDs(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.SingleEmailMessage message = new Messaging.SingleEmailMessage();
message.setOrgWideEmailAddressId('0D2000000000001');
message.setTargetObjectId('003000000000001');
message.setTemplateId('00X000000000001');
message.setWhatId('001000000000001');
System.assertEquals('0D2000000000001CAA', String.valueOf(message.getOrgWideEmailAddressId()));
System.assertEquals('003000000000001AAA', String.valueOf(message.getTargetObjectId()));
System.assertEquals('00X000000000001EAA', String.valueOf(message.getTemplateId()));
System.assertEquals('001000000000001AAA', String.valueOf(message.getWhatId()));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMessagingRenderStoredEmailTemplateFullLocalAttachments(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.SingleEmailMessage withBody = Messaging.renderStoredEmailTemplate(
	'00X000000000010AAA',
	'003000000000010AAA',
	'001000000000010AAA',
	Messaging.AttachmentRetrievalOption.METADATA_WITH_BODY
);
System.assertEquals('Hello Ada at TrailWorks', withBody.getSubject());
System.assertEquals('<p>Ada Trail / TrailWorks</p>', withBody.getHtmlBody());
System.assertEquals('Ada Trail / TrailWorks', withBody.getPlainTextBody());
System.assertEquals(2, withBody.getFileAttachments().size());
System.assertEquals('contract.txt', withBody.getFileAttachments()[0].getFileName());
System.assertEquals('text/plain', withBody.getFileAttachments()[0].getContentType());
System.assertEquals('contract body', withBody.getFileAttachments()[0].getBody().toString());
System.assertEquals('brief.pdf', withBody.getFileAttachments()[1].getFileName());
System.assertEquals('application/pdf', withBody.getFileAttachments()[1].getContentType());
System.assertEquals('pdf body', withBody.getFileAttachments()[1].getBody().toString());

Messaging.SingleEmailMessage metadataOnly = Messaging.renderStoredEmailTemplate(
	'00X000000000010AAA',
	'003000000000010AAA',
	'001000000000010AAA',
	Messaging.AttachmentRetrievalOption.METADATA_ONLY
);
System.assertEquals(2, metadataOnly.getFileAttachments().size());
System.assertEquals('contract.txt', metadataOnly.getFileAttachments()[0].getFileName());
System.assertEquals(null, metadataOnly.getFileAttachments()[0].getBody());

Messaging.SingleEmailMessage none = Messaging.renderStoredEmailTemplate(
	'00X000000000010AAA',
	'003000000000010AAA',
	'001000000000010AAA',
	Messaging.AttachmentRetrievalOption.NONE
);
System.assertEquals(0, none.getFileAttachments().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := fullLocalEmailOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}

	bodyOption := Object("Messaging.AttachmentRetrievalOption")
	bodyOption.Text = "BODY"
	rendered, err := machine.renderStoredEmailTemplate([]Value{
		String("00X000000000010AAA"),
		String("003000000000010AAA"),
		String("001000000000010AAA"),
	}, bodyOption)
	if err != nil {
		t.Fatal(err)
	}
	attachments := rendered.Fields["fileAttachments"]
	if attachments.Kind != ValueList || len(attachments.List) != 2 {
		t.Fatalf("BODY attachments = %#v", attachments)
	}
	if got := stringValue(attachments.List[0].Fields["body"]); got != "contract body" {
		t.Fatalf("BODY first attachment body = %q", got)
	}
}

func TestExecMessagingSendEmailFullLocalCapture(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.SendEmailOptions opts = new Messaging.SendEmailOptions();
opts.setTriggerUserEmail(true);
opts.setTriggerOtherEmail(false);
opts.setTriggerAutoResponseEmail(true);
System.assertEquals(true, opts.getTriggerUserEmail());
System.assertEquals(false, opts.getTriggerOtherEmail());
System.assertEquals(true, opts.getTriggerAutoResponseEmail());

Messaging.EmailFileAttachment file = new Messaging.EmailFileAttachment();
file.setFileName('note.txt');
file.setContentType('text/plain');
file.setBody(Blob.valueOf('note body'));
file.setInline(true);

Messaging.SingleEmailMessage single = new Messaging.SingleEmailMessage();
single.setToAddresses(new List<String>{'to@example.test'});
single.setCcAddresses(new List<String>{'cc@example.test'});
single.setBccAddresses(new List<String>{'bcc@example.test'});
single.setReplyTo('reply@example.test');
single.setSenderDisplayName('Trail Sender');
single.setSubject('Direct');
single.setPlainTextBody('Direct body');
single.setHtmlBody('<p>Direct body</p>');
single.setTemplateId('00X000000000010AAA');
single.setTargetObjectId('003000000000010AAA');
single.setWhatId('001000000000010AAA');
single.setEntityAttachments(new List<Id>{'00P000000000010AAA'});
single.setFileAttachments(new List<Messaging.EmailFileAttachment>{file});
single.setSaveAsActivity(false);
single.setBccSender(true);
single.setUseSignature(true);
single.setTreatTargetObjectAsRecipient(false);
single.setTreatBodiesAsTemplate(true);
single.setOptOutPolicy('FILTER');
single.setEmailPriority('High');

Messaging.MassEmailMessage mass = new Messaging.MassEmailMessage();
mass.setTargetObjectIds(new List<String>{'003000000000010AAA', '003000000000011AAA'});
mass.setWhatIds(new List<String>{'001000000000010AAA'});
mass.setTemplateId('00X000000000010AAA');
mass.setReplyTo('mass-reply@example.test');
mass.setSenderDisplayName('Mass Sender');
mass.setSubject('Mass subject');
mass.setBccSender(true);
mass.setUseSignature(true);
mass.setOptOutPolicy('FILTER');
mass.setEmailPriority('High');

System.assertEquals(0, Limits.getEmailInvocations());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := fullLocalEmailOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	opts := result.Vars["opts"]
	messages := List(result.Vars["single"], result.Vars["mass"])
	results, err := machine.sendEmail([]Value{messages, opts}, &result)
	if err != nil {
		t.Fatal(err)
	}
	if results.Kind != ValueList || len(results.List) != 2 {
		t.Fatalf("sendEmail results = %#v", results)
	}
	if machine.limits.EmailInvokes != 1 {
		t.Fatalf("email invocations = %d, want 1", machine.limits.EmailInvokes)
	}
	if len(machine.capturedEmails) != 2 {
		t.Fatalf("captured emails = %#v", machine.capturedEmails)
	}
	single := machine.capturedEmails[0]
	if single.ReplyTo != "reply@example.test" || single.SenderDisplayName != "Trail Sender" {
		t.Fatalf("single sender capture = %#v", single)
	}
	if single.TemplateID != "00X000000000010AAA" || single.TargetObjectID != "003000000000010AAA" || single.WhatID != "001000000000010AAA" {
		t.Fatalf("single template capture = %#v", single)
	}
	if len(single.EntityAttachments) != 1 || single.EntityAttachments[0] != "00P000000000010AAA" {
		t.Fatalf("single entity attachments = %#v", single.EntityAttachments)
	}
	if len(single.FileAttachments) != 1 || single.FileAttachments[0] != "note.txt" {
		t.Fatalf("single file attachments = %#v", single.FileAttachments)
	}
	if !single.BccSender || !single.UseSignature || !single.TreatBodiesAsTemplate || single.TreatTargetObjectAsRecipient {
		t.Fatalf("single boolean capture = %#v", single)
	}
	if single.TriggerUserEmail != true || single.TriggerOtherEmail != false || single.TriggerAutoResponseEmail != true {
		t.Fatalf("send options capture = %#v", single)
	}
	mass := machine.capturedEmails[1]
	if mass.ReplyTo != "mass-reply@example.test" || mass.SenderDisplayName != "Mass Sender" || !mass.BccSender || !mass.UseSignature {
		t.Fatalf("mass capture = %#v", mass)
	}
	if len(mass.TargetObjectIDs) != 2 || len(mass.WhatIDs) != 1 {
		t.Fatalf("mass ids = %#v", mass)
	}
}

func fullLocalEmailOrg() storage.OrgState {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	storage.EnsureStandardObject(&org, "EmailTemplate")
	storage.EnsureStandardObject(&org, "Attachment")
	storage.EnsureStandardObject(&org, "ContentDocument")
	storage.EnsureStandardObject(&org, "ContentVersion")
	storage.EnsureStandardObject(&org, "ContentDocumentLink")
	accountObject := org.Objects["Account"]
	accountObject.Records["001000000000010AAA"] = storage.Record{
		ID:     "001000000000010AAA",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Id":   storage.IDValue("001000000000010AAA"),
			"Name": storage.StringValue("TrailWorks"),
		},
	}
	org.Objects["Account"] = accountObject
	contactObject := org.Objects["Contact"]
	contactObject.Records["003000000000010AAA"] = storage.Record{
		ID:     "003000000000010AAA",
		Object: "Contact",
		Fields: map[string]storage.Value{
			"Id":        storage.IDValue("003000000000010AAA"),
			"FirstName": storage.StringValue("Ada"),
			"LastName":  storage.StringValue("Trail"),
			"Name":      storage.StringValue("Ada Trail"),
		},
	}
	contactObject.Records["003000000000011AAA"] = storage.Record{
		ID:     "003000000000011AAA",
		Object: "Contact",
		Fields: map[string]storage.Value{
			"Id":   storage.IDValue("003000000000011AAA"),
			"Name": storage.StringValue("Grace Trail"),
		},
	}
	org.Objects["Contact"] = contactObject
	templateObject := org.Objects["EmailTemplate"]
	templateObject.Records["00X000000000010AAA"] = storage.Record{
		ID:     "00X000000000010AAA",
		Object: "EmailTemplate",
		Fields: map[string]storage.Value{
			"Id":            storage.IDValue("00X000000000010AAA"),
			"DeveloperName": storage.StringValue("Full_Local"),
			"Name":          storage.StringValue("Full Local"),
			"Subject":       storage.StringValue("Hello {!Recipient.FirstName} at {!RelatedTo.Name}"),
			"HtmlValue":     storage.StringValue("<p>{!Contact.Name} / {!Account.Name}</p>"),
			"Body":          storage.StringValue("{!Contact.Name} / {!Account.Name}"),
		},
	}
	org.Objects["EmailTemplate"] = templateObject
	attachmentObject := org.Objects["Attachment"]
	attachmentObject.Records["00P000000000010AAA"] = storage.Record{
		ID:     "00P000000000010AAA",
		Object: "Attachment",
		Fields: map[string]storage.Value{
			"Id":          storage.IDValue("00P000000000010AAA"),
			"ParentId":    storage.IDValue("00X000000000010AAA"),
			"Name":        storage.StringValue("contract.txt"),
			"ContentType": storage.StringValue("text/plain"),
			"Body":        storage.BlobValue("contract body"),
		},
	}
	org.Objects["Attachment"] = attachmentObject
	contentDocumentObject := org.Objects["ContentDocument"]
	contentDocumentObject.Records["069000000000010AAA"] = storage.Record{
		ID:     "069000000000010AAA",
		Object: "ContentDocument",
		Fields: map[string]storage.Value{
			"Id":    storage.IDValue("069000000000010AAA"),
			"Title": storage.StringValue("brief"),
		},
	}
	org.Objects["ContentDocument"] = contentDocumentObject
	contentVersionObject := org.Objects["ContentVersion"]
	contentVersionObject.Records["068000000000010AAA"] = storage.Record{
		ID:     "068000000000010AAA",
		Object: "ContentVersion",
		Fields: map[string]storage.Value{
			"Id":                storage.IDValue("068000000000010AAA"),
			"ContentDocumentId": storage.IDValue("069000000000010AAA"),
			"PathOnClient":      storage.StringValue("brief.pdf"),
			"FileType":          storage.StringValue("PDF"),
			"VersionData":       storage.BlobValue("pdf body"),
			"IsLatest":          storage.BooleanValue(true),
		},
	}
	org.Objects["ContentVersion"] = contentVersionObject
	linkObject := org.Objects["ContentDocumentLink"]
	linkObject.Records["06A000000000010AAA"] = storage.Record{
		ID:     "06A000000000010AAA",
		Object: "ContentDocumentLink",
		Fields: map[string]storage.Value{
			"Id":                storage.IDValue("06A000000000010AAA"),
			"LinkedEntityId":    storage.IDValue("00X000000000010AAA"),
			"ContentDocumentId": storage.IDValue("069000000000010AAA"),
		},
	}
	org.Objects["ContentDocumentLink"] = linkObject
	return org
}
