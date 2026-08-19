package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecMessagingExtractInboundEmailParsesRFC822Blob(t *testing.T) {
	program, err := CompileAnonymous(`
Blob source = Blob.valueOf('From: sender@example.com\nTo: recipient@example.com\nSubject: probe\n\nbody');
Messaging.InboundEmail inbound = Messaging.extractInboundEmail(source, true);
System.assertEquals('sender@example.com', inbound.fromAddress);
System.assertEquals(1, inbound.toAddresses.size());
System.assertEquals('recipient@example.com', inbound.toAddresses[0]);
System.assertEquals('probe', inbound.subject);
System.assertEquals('body', inbound.plainTextBody);
System.assertEquals(false, inbound.plainTextBodyIsTruncated);
System.assertEquals(3, inbound.headers.size());
System.assertEquals('From', inbound.headers[0].name);
System.assertEquals('sender@example.com', inbound.headers[0].value);
System.assertEquals('To', inbound.headers[1].name);
System.assertEquals('recipient@example.com', inbound.headers[1].value);
System.assertEquals('Subject', inbound.headers[2].name);
System.assertEquals('probe', inbound.headers[2].value);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExtractInboundEmailParsesMultipartBodiesAndAttachments(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: multipart\r\n" +
		"Content-Type: multipart/mixed; boundary=outer\r\n" +
		"\r\n" +
		"--outer\r\n" +
		"Content-Type: multipart/alternative; boundary=alternative\r\n" +
		"\r\n" +
		"--alternative\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"\r\n" +
		"Hello=20plain\r\n" +
		"--alternative\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<p>Hello HTML</p>\r\n" +
		"--alternative--\r\n" +
		"--outer\r\n" +
		"Content-Type: text/plain; name=notes.txt; charset=utf-8\r\n" +
		"Content-Disposition: attachment; filename=notes.txt\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"\r\n" +
		"line=201\r\n" +
		"--outer\r\n" +
		"Content-Type: application/octet-stream\r\n" +
		"Content-Disposition: attachment; filename=bytes.bin\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"AAEC\r\n" +
		"--outer--\r\n"

	machine := New(nil)
	email, err := machine.extractInboundEmail([]Value{NewBlobValue(raw), Bool(false)})
	if err != nil {
		t.Fatal(err)
	}
	if got := stringValue(email.Fields["plainTextBody"]); got != "Hello plain" {
		t.Fatalf("plainTextBody = %q", got)
	}
	if got := stringValue(email.Fields["htmlBody"]); got != "<p>Hello HTML</p>" {
		t.Fatalf("htmlBody = %q", got)
	}
	textAttachments := email.Fields["textAttachments"]
	if len(textAttachments.List) != 1 {
		t.Fatalf("textAttachments = %#v", textAttachments)
	}
	textAttachment := textAttachments.List[0]
	if got := stringValue(textAttachment.Fields["fileName"]); got != "notes.txt" {
		t.Fatalf("text attachment fileName = %q", got)
	}
	if got := stringValue(textAttachment.Fields["body"]); got != "line 1" {
		t.Fatalf("text attachment body = %q", got)
	}
	if got := stringValue(textAttachment.Fields["charset"]); got != "utf-8" {
		t.Fatalf("text attachment charset = %q", got)
	}
	binaryAttachments := email.Fields["binaryAttachments"]
	if len(binaryAttachments.List) != 1 {
		t.Fatalf("binaryAttachments = %#v", binaryAttachments)
	}
	binaryBody, err := platformScalarText(binaryAttachments.List[0].Fields["body"], "Blob")
	if err != nil {
		t.Fatal(err)
	}
	if binaryBody != string([]byte{0, 1, 2}) {
		t.Fatalf("binary attachment body = %q", binaryBody)
	}
}

func TestExtractInboundEmailIncludesForwardedAttachmentsOnlyWhenRequested(t *testing.T) {
	raw := "Content-Type: multipart/mixed; boundary=outer\r\n\r\n" +
		"--outer\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"outer body\r\n" +
		"--outer\r\n" +
		"Content-Type: message/rfc822\r\n" +
		"Content-Disposition: attachment; filename=forwarded.eml\r\n\r\n" +
		"From: forwarded@example.com\r\n" +
		"Content-Type: multipart/mixed; boundary=inner\r\n\r\n" +
		"--inner\r\n" +
		"Content-Type: application/octet-stream\r\n" +
		"Content-Disposition: attachment; filename=inner.bin\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		"AQI=\r\n" +
		"--inner--\r\n" +
		"--outer--\r\n"

	machine := New(nil)
	without, err := machine.extractInboundEmail([]Value{NewBlobValue(raw), Bool(false)})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(without.Fields["binaryAttachments"].List); got != 0 {
		t.Fatalf("without forwarded attachments = %d", got)
	}
	with, err := machine.extractInboundEmail([]Value{NewBlobValue(raw), Bool(true)})
	if err != nil {
		t.Fatal(err)
	}
	attachments := with.Fields["binaryAttachments"]
	if len(attachments.List) != 1 {
		t.Fatalf("with forwarded attachments = %#v", attachments)
	}
	if got := stringValue(with.Fields["plainTextBody"]); got != "outer body" {
		t.Fatalf("forwarded message changed plainTextBody = %q", got)
	}
	if got := stringValue(attachments.List[0].Fields["fileName"]); got != "inner.bin" {
		t.Fatalf("forwarded fileName = %q", got)
	}
}

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

func TestExecMessagingSendEmailMissingBodyUsesSalesforceErrorContract(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.SingleEmailMessage message = new Messaging.SingleEmailMessage();
message.setToAddresses(new List<String>{'missing-body@example.test'});
List<Messaging.SendEmailResult> results = Messaging.sendEmail(
	new List<Messaging.SingleEmailMessage>{message}, false
);
System.assertEquals(1, results.size());
System.assertEquals(false, results[0].isSuccess());
System.assertEquals(1, results[0].getErrors().size());
System.assertEquals('Email body is required.', results[0].getErrors()[0].getMessage());
System.assertEquals('REQUIRED_FIELD_MISSING', String.valueOf(results[0].getErrors()[0].getStatusCode()));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMessagingMassEmailMessageSalesforceDefaults(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.MassEmailMessage mass = new Messaging.MassEmailMessage();
System.assertEquals('Mass Email (API)', mass.description);
System.assertEquals(null, mass.targetObjectIds);
System.assertEquals(null, mass.whatIds);
System.assertEquals(0, mass.getTargetObjectIds().size());
System.assertEquals(0, mass.getWhatIds().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
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
