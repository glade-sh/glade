package vm

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/glade-sh/glade/internal/storage"
)

func newSendEmailResult() Value {
	result := Object("Messaging.SendEmailResult")
	result.Fields["success"] = Bool(true)
	result.Fields["errors"] = List()
	return result
}
func newSendEmailError(message string) Value {
	err := Object("Messaging.SendEmailError")
	err.Fields["fields"] = List()
	err.Fields["message"] = String(message)
	err.Fields["statusCode"] = Null
	err.Fields["targetObjectId"] = Null
	return err
}
func newEmailFileAttachment() Value {
	attachment := Object("Messaging.EmailFileAttachment")
	attachment.Fields["body"] = Null
	attachment.Fields["contentType"] = Null
	attachment.Fields["fileName"] = Null
	attachment.Fields["id"] = Null
	attachment.Fields["inline"] = Bool(false)
	return attachment
}
func newRenderEmailTemplateBodyResult(mergedBody string) Value {
	result := Object("Messaging.RenderEmailTemplateBodyResult")
	result.Fields["success"] = Bool(true)
	result.Fields["mergedBody"] = String(mergedBody)
	result.Fields["errors"] = List()
	return result
}
func newFailedSendEmailResult(message string) Value {
	result := Object("Messaging.SendEmailResult")
	result.Fields["success"] = Bool(false)
	result.Fields["errors"] = List(newSendEmailError(message))
	return result
}
func newSingleEmailMessage() Value {
	message := Object("Messaging.SingleEmailMessage")
	for _, field := range []string{
		"toAddresses", "ccAddresses", "bccAddresses", "fileAttachments",
		"entityAttachments", "documentAttachments", "targetObjectIds",
	} {
		message.Fields[field] = List()
	}
	for _, field := range []string{
		"subject", "plainTextBody", "htmlBody", "replyTo", "senderDisplayName",
		"charset", "inReplyTo", "references", "orgWideEmailAddressId",
		"targetObjectId", "templateId", "templateName", "whatId", "optOutPolicy",
		"emailPriority", "unsubscribeComment",
	} {
		message.Fields[field] = Null
	}
	message.Fields["unsubscribeUrls"] = List()
	for _, field := range []string{
		"saveAsActivity", "treatBodiesAsTemplate", "treatTargetObjectAsRecipient",
		"useSignature", "bccSender", "oneClickPost", "userMail",
	} {
		message.Fields[field] = Bool(false)
	}
	return message
}
func newMassEmailMessage() Value {
	message := Object("Messaging.MassEmailMessage")
	for _, field := range []string{"targetObjectIds", "whatIds"} {
		message.Fields[field] = List()
	}
	for _, field := range []string{
		"templateId", "description", "optOutPolicy", "replyTo", "senderDisplayName",
		"subject", "emailPriority",
	} {
		message.Fields[field] = Null
	}
	for _, field := range []string{"saveAsActivity", "bccSender", "useSignature"} {
		message.Fields[field] = Bool(false)
	}
	return message
}
func newInboundEmail() Value {
	email := Object("Messaging.InboundEmail")
	for _, field := range []string{"authenticationResults", "binaryAttachments", "ccAddresses", "headers", "textAttachments", "toAddresses"} {
		email.Fields[field] = List()
	}
	for _, field := range []string{
		"fromAddress", "fromName", "htmlBody", "inReplyTo", "messageId", "plainTextBody",
		"references", "replyTo", "subject",
	} {
		email.Fields[field] = Null
	}
	email.Fields["htmlBodyIsTruncated"] = Bool(false)
	email.Fields["plainTextBodyIsTruncated"] = Bool(false)
	return email
}
func newInboundEnvelope() Value {
	envelope := Object("Messaging.InboundEnvelope")
	envelope.Fields["fromAddress"] = Null
	envelope.Fields["toAddress"] = Null
	return envelope
}
func newInboundEmailResult() Value {
	result := Object("Messaging.InboundEmailResult")
	result.Fields["success"] = Bool(false)
	result.Fields["message"] = Null
	return result
}
func isLocalEmailMessage(value Value) bool {
	return value.Kind == ValueObject && (value.Type == "Messaging.SingleEmailMessage" || value.Type == "Messaging.MassEmailMessage")
}
func (vm *VM) sendEmail(args []Value, result *Result) (Value, error) {
	if len(args) == 0 {
		return Null, fmt.Errorf("Messaging.sendEmail expects messages")
	}
	if len(args) > 2 {
		return Null, unsupportedCallError("Messaging.sendEmail send options overloads")
	}
	if args[0].Kind != ValueList {
		return Null, fmt.Errorf("Messaging.sendEmail expects List")
	}
	if len(args) == 2 && args[1].Kind != ValueBool && !isSendEmailOptions(args[1]) {
		return Null, unsupportedCallError("Messaging.sendEmail send options overloads")
	}
	allOrNothing := true
	sendOptions := Null
	if len(args) == 2 && args[1].Kind == ValueBool {
		allOrNothing = args[1].Bool
	} else if len(args) == 2 {
		sendOptions = args[1]
	}
	for _, message := range args[0].List {
		if !isLocalEmailMessage(message) {
			return Null, fmt.Errorf("Messaging.sendEmail expects SingleEmailMessage or MassEmailMessage list items")
		}
	}
	validationErrors := make([]string, len(args[0].List))
	for i, message := range args[0].List {
		validationErrors[i] = localEmailValidationError(message)
		if validationErrors[i] != "" && allOrNothing {
			return Null, newExceptionError("EmailException", validationErrors[i])
		}
	}
	if err := vm.incrementLimit("emailInvocations", 1); err != nil {
		return Null, err
	}
	appendTrace(result, "apex.email.send", "apex.email", map[string]any{"messages": len(args[0].List)})
	results := make([]Value, 0, len(args[0].List))
	for i, message := range args[0].List {
		if validationErrors[i] != "" {
			results = append(results, newFailedSendEmailResult(validationErrors[i]))
			continue
		}
		captured := vm.captureEmail(message, sendOptions)
		if message.Type == "Messaging.SingleEmailMessage" && captured.TemplateID != "" {
			message.Fields["subject"] = String(captured.Subject)
			message.Fields["plainTextBody"] = String(captured.PlainTextBody)
			message.Fields["htmlBody"] = String(captured.HTMLBody)
			args[0].List[i] = message
		}
		vm.capturedEmails = append(vm.capturedEmails, captured)
		results = append(results, newSendEmailResult())
	}
	return List(results...), nil
}

func isSendEmailOptions(value Value) bool {
	return value.Kind == ValueObject && strings.EqualFold(value.Type, "Messaging.SendEmailOptions")
}
func (vm *VM) reserveEmailCapacity(callee string, args []Value, result *Result) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueInt {
		return Null, fmt.Errorf("%s expects Integer", callee)
	}
	if args[0].Int < 0 {
		return Null, fmt.Errorf("%s expects non-negative Integer", callee)
	}
	appendTrace(result, "apex.email.reserve", "apex.email", map[string]any{"method": callee, "capacity": args[0].Int})
	return Null, nil
}
func (vm *VM) sendEmailMessage(args []Value, result *Result) (Value, error) {
	if len(args) == 0 || len(args) > 2 {
		return Null, fmt.Errorf("Messaging.sendEmailMessage expects email message Ids and optional allOrNothing")
	}
	if args[0].Kind != ValueList {
		return Null, fmt.Errorf("Messaging.sendEmailMessage expects List<Id>")
	}
	if len(args) == 2 && args[1].Kind != ValueBool {
		return Null, fmt.Errorf("Messaging.sendEmailMessage allOrNothing expects Boolean")
	}
	for _, id := range args[0].List {
		if _, ok := idValueText(id); !ok {
			return Null, fmt.Errorf("Messaging.sendEmailMessage expects List<Id>")
		}
	}
	if err := vm.incrementLimit("emailInvocations", 1); err != nil {
		return Null, err
	}
	appendTrace(result, "apex.email.message.send", "apex.email", map[string]any{"messages": len(args[0].List)})
	results := make([]Value, 0, len(args[0].List))
	for range args[0].List {
		results = append(results, newSendEmailResult())
	}
	return List(results...), nil
}
func (vm *VM) renderEmailTemplate(args []Value) (Value, error) {
	if len(args) != 3 {
		return Null, fmt.Errorf("Messaging.renderEmailTemplate expects whoId, whatId, bodies")
	}
	whoID := args[0]
	whatID := args[1]
	if args[0].Kind != ValueNull {
		if _, ok := idValueText(args[0]); !ok {
			return Null, fmt.Errorf("Messaging.renderEmailTemplate expects whoId String or Id")
		}
	}
	if args[1].Kind != ValueNull {
		if _, ok := idValueText(args[1]); !ok {
			return Null, fmt.Errorf("Messaging.renderEmailTemplate expects whatId String or Id")
		}
	}
	if args[2].Kind != ValueList {
		return Null, fmt.Errorf("Messaging.renderEmailTemplate expects List<String> bodies")
	}
	results := make([]Value, 0, len(args[2].List))
	for _, body := range args[2].List {
		if body.Kind != ValueString {
			return Null, fmt.Errorf("Messaging.renderEmailTemplate expects List<String> bodies")
		}
		results = append(results, newRenderEmailTemplateBodyResult(vm.renderEmailTemplateText(body.Text, whoID, whatID)))
	}
	return List(results...), nil
}
func (vm *VM) extractInboundEmail(args []Value) (Value, error) {
	if len(args) != 2 || args[1].Kind != ValueBool {
		return Null, fmt.Errorf("Messaging.extractInboundEmail expects source and includeForwardedAttachments Boolean")
	}
	if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Messaging.InboundEmail") {
		return args[0], nil
	}
	return newInboundEmail(), nil
}
func localEmailValidationError(message Value) string {
	if message.Type != "Messaging.SingleEmailMessage" {
		return ""
	}
	if emailFieldString(message, "plainTextBody") != "" || emailFieldString(message, "htmlBody") != "" || emailFieldString(message, "templateId") != "" {
		return ""
	}
	return "Email body or template ID is required"
}
func emailFieldString(message Value, field string) string {
	if message.Kind != ValueObject || message.Fields == nil {
		return ""
	}
	if value, ok := message.Fields[field]; ok {
		if text := stringValue(value); text != "" {
			return text
		}
	}
	normalized := strings.ToLower(field)
	for candidate, value := range message.Fields {
		if strings.EqualFold(candidate, normalized) {
			if text := stringValue(value); text != "" {
				return text
			}
		}
	}
	return ""
}
func emailFieldBool(message Value, field string) bool {
	_, value, _ := objectFieldValue(message, field)
	if boolValue(value) {
		return true
	}
	normalized := strings.ToLower(field)
	for candidate, value := range message.Fields {
		if strings.EqualFold(candidate, normalized) && boolValue(value) {
			return true
		}
	}
	return false
}
func emailFieldStrings(message Value, field string) []string {
	_, value, _ := objectFieldValue(message, field)
	values := stringsFromList(value)
	if len(values) > 0 {
		return values
	}
	normalized := strings.ToLower(field)
	for candidate, value := range message.Fields {
		if strings.EqualFold(candidate, normalized) {
			if values := stringsFromList(value); len(values) > 0 {
				return values
			}
		}
	}
	return nil
}
func (vm *VM) captureEmail(message Value, sendOptions Value) CapturedEmail {
	captured := CapturedEmail{Kind: message.Type}
	switch message.Type {
	case "Messaging.SingleEmailMessage":
		captured.ToAddresses = emailFieldStrings(message, "toAddresses")
		captured.CcAddresses = emailFieldStrings(message, "ccAddresses")
		captured.BccAddresses = emailFieldStrings(message, "bccAddresses")
		captured.FileAttachments = emailAttachmentNames(message, "fileAttachments")
		captured.EntityAttachments = emailFieldStrings(message, "entityAttachments")
		captured.DocumentAttachments = emailFieldStrings(message, "documentAttachments")
		captured.TargetObjectIDs = emailFieldStrings(message, "targetObjectIds")
		captured.Subject = emailFieldString(message, "subject")
		captured.PlainTextBody = emailFieldString(message, "plainTextBody")
		captured.HTMLBody = emailFieldString(message, "htmlBody")
		captured.TemplateID = emailFieldString(message, "templateId")
		captured.TargetObjectID = emailFieldString(message, "targetObjectId")
		captured.WhatID = emailFieldString(message, "whatId")
		captured.ReplyTo = emailFieldString(message, "replyTo")
		captured.SenderDisplayName = emailFieldString(message, "senderDisplayName")
		captured.Charset = emailFieldString(message, "charset")
		captured.OrgWideEmailAddressID = emailFieldString(message, "orgWideEmailAddressId")
		captured.OptOutPolicy = emailFieldString(message, "optOutPolicy")
		captured.EmailPriority = emailFieldString(message, "emailPriority")
		captured.SaveAsActivity = emailFieldBool(message, "saveAsActivity")
		captured.BccSender = emailFieldBool(message, "bccSender")
		captured.UseSignature = emailFieldBool(message, "useSignature")
		captured.TreatBodiesAsTemplate = emailFieldBool(message, "treatBodiesAsTemplate")
		captured.TreatTargetObjectAsRecipient = emailFieldBool(message, "treatTargetObjectAsRecipient")
		vm.renderCapturedEmailTemplate(&captured)
	case "Messaging.MassEmailMessage":
		captured.TargetObjectIDs = emailFieldStrings(message, "targetObjectIds")
		captured.WhatIDs = emailFieldStrings(message, "whatIds")
		captured.TemplateID = emailFieldString(message, "templateId")
		captured.ReplyTo = emailFieldString(message, "replyTo")
		captured.SenderDisplayName = emailFieldString(message, "senderDisplayName")
		captured.Subject = emailFieldString(message, "subject")
		captured.OptOutPolicy = emailFieldString(message, "optOutPolicy")
		captured.EmailPriority = emailFieldString(message, "emailPriority")
		captured.SaveAsActivity = emailFieldBool(message, "saveAsActivity")
		captured.BccSender = emailFieldBool(message, "bccSender")
		captured.UseSignature = emailFieldBool(message, "useSignature")
		if captured.TemplateID != "" && len(captured.TargetObjectIDs) > 0 {
			captured.TargetObjectID = captured.TargetObjectIDs[0]
			if len(captured.WhatIDs) > 0 {
				captured.WhatID = captured.WhatIDs[0]
			}
			vm.renderCapturedEmailTemplate(&captured)
		}
	}
	captureSendEmailOptions(&captured, sendOptions)
	return captured
}

func emailAttachmentNames(message Value, field string) []string {
	_, value, _ := objectFieldValue(message, field)
	if value.Kind != ValueList {
		return nil
	}
	out := make([]string, 0, len(value.List))
	for _, item := range value.List {
		if text := stringValue(item); text != "" {
			out = append(out, text)
			continue
		}
		if item.Kind == ValueObject {
			if name := emailFieldString(item, "fileName"); name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

func captureSendEmailOptions(captured *CapturedEmail, options Value) {
	if captured == nil || !isSendEmailOptions(options) {
		return
	}
	captured.TriggerUserEmail = emailFieldBool(options, "triggerUserEmail")
	captured.TriggerOtherEmail = emailFieldBool(options, "triggerOtherEmail")
	captured.TriggerAutoResponseEmail = emailFieldBool(options, "triggerAutoResponseEmail")
}
func (vm *VM) captureWorkflowEmail(alert storage.WorkflowEmailAlert, record storage.Record, result *Result) error {
	if err := vm.incrementLimit("emailInvocations", 1); err != nil {
		return err
	}
	captured := CapturedEmail{
		Kind:   "WorkflowEmailAlert",
		WhatID: string(record.ID),
	}
	captured.ToAddresses, captured.TargetObjectIDs = vm.workflowEmailRecipients(alert, record)
	if len(captured.TargetObjectIDs) > 0 {
		captured.TargetObjectID = captured.TargetObjectIDs[0]
	}
	if template, ok := vm.emailTemplateByName(alert.Template); ok {
		captured.TemplateID = string(template.ID)
		whoID := Null
		if len(captured.TargetObjectIDs) > 0 {
			whoID = String(captured.TargetObjectIDs[0])
		}
		whatID := Null
		if captured.WhatID != "" {
			whatID = String(captured.WhatID)
		}
		captured.Subject = vm.renderEmailTemplateText(storageStringField(template, "Subject"), whoID, whatID)
		captured.HTMLBody = vm.renderEmailTemplateHTML(template, whoID, whatID)
		captured.PlainTextBody = vm.renderEmailTemplateText(storageStringField(template, "Body"), whoID, whatID)
	}
	vm.capturedEmails = append(vm.capturedEmails, captured)
	appendTrace(result, "apex.email.workflow", "apex.email", map[string]any{
		"alert":      alert.Name,
		"template":   alert.Template,
		"recipients": len(captured.ToAddresses),
		"record":     string(record.ID),
	})
	return nil
}
func (vm *VM) workflowEmailRecipients(alert storage.WorkflowEmailAlert, record storage.Record) ([]string, []string) {
	addresses := make([]string, 0, len(alert.Recipients))
	targetIDs := make([]string, 0, len(alert.Recipients))
	for _, recipient := range alert.Recipients {
		if recipient.Recipient != "" {
			vm.appendWorkflowEmailRecipient(recipient.Type, recipient.Recipient, &addresses, &targetIDs)
			continue
		}
		fieldName := recipient.Field
		if fieldName == "" && strings.EqualFold(strings.TrimSpace(recipient.Type), "owner") {
			fieldName = "OwnerId"
		}
		if fieldName == "" {
			continue
		}
		if vm.Org != nil {
			if objectName, ok := vm.resolveObjectName(record.Object); ok {
				if object, ok := vm.Org.Objects[objectName]; ok {
					if resolved, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, fieldName); ok {
						fieldName = resolved
					}
				}
			}
		}
		if value, ok := record.GetField(fieldName); ok {
			vm.appendWorkflowEmailRecipient(recipient.Type, workflowEmailRecipientValue(value), &addresses, &targetIDs)
			continue
		}
		if strings.EqualFold(fieldName, "OwnerId") && record.System.OwnerID != "" {
			vm.appendWorkflowEmailRecipient(recipient.Type, string(record.System.OwnerID), &addresses, &targetIDs)
		}
	}
	return addresses, targetIDs
}
func (vm *VM) appendWorkflowEmailRecipient(recipientType, raw string, addresses, targetIDs *[]string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return
	}
	normalizedType := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(recipientType), " ", ""))
	if workflowRecipientLooksLikeID(value) || (normalizedType == "owner" && !strings.Contains(value, "@")) {
		*targetIDs = append(*targetIDs, value)
		return
	}
	*addresses = append(*addresses, value)
}
func workflowEmailRecipientValue(value storage.Value) string {
	switch value.Kind {
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueString:
		return value.String
	default:
		return ""
	}
}
func workflowRecipientLooksLikeID(value string) bool {
	if len(value) != 15 && len(value) != 18 {
		return false
	}
	for _, ch := range value {
		if ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' {
			continue
		}
		return false
	}
	return true
}
func (vm *VM) renderCapturedEmailTemplate(captured *CapturedEmail) {
	if captured == nil || captured.TemplateID == "" || vm.Org == nil {
		return
	}
	template, ok := vm.emailTemplateByID(captured.TemplateID)
	if !ok {
		return
	}
	whoID := Null
	if captured.TargetObjectID != "" {
		whoID = String(captured.TargetObjectID)
	}
	whatID := Null
	if captured.WhatID != "" {
		whatID = String(captured.WhatID)
	}
	if captured.Subject == "" {
		captured.Subject = vm.renderEmailTemplateText(storageStringField(template, "Subject"), whoID, whatID)
	}
	if captured.HTMLBody == "" {
		captured.HTMLBody = vm.renderEmailTemplateHTML(template, whoID, whatID)
	}
	if captured.PlainTextBody == "" {
		captured.PlainTextBody = vm.renderEmailTemplateText(storageStringField(template, "Body"), whoID, whatID)
	}
}
func (vm *VM) renderStoredEmailTemplate(args []Value, attachmentOption Value) (Value, error) {
	if len(args) != 3 {
		return Null, fmt.Errorf("Messaging.renderStoredEmailTemplate expects templateId, whoId, whatId")
	}
	for i, arg := range args {
		if arg.Kind == ValueNull {
			continue
		}
		if _, ok := idValueText(arg); !ok {
			names := []string{"templateId", "whoId", "whatId"}
			return Null, fmt.Errorf("Messaging.renderStoredEmailTemplate expects %s String or Id", names[i])
		}
	}
	templateID, _ := idValueText(args[0])
	if templateID == "" {
		return Null, newExceptionError("EmailException", fmt.Sprintf("Email template not found: %s", templateID))
	}
	template, ok := vm.emailTemplateByID(templateID)
	if !ok {
		return Null, newExceptionError("EmailException", fmt.Sprintf("Email template not found: %s", templateID))
	}

	message := newSingleEmailMessage()
	message.Fields["templateId"] = String(templateID)
	message.Fields["targetObjectId"] = args[1]
	message.Fields["whatId"] = args[2]
	message.Fields["subject"] = String(vm.renderEmailTemplateText(storageStringField(template, "Subject"), args[1], args[2]))
	message.Fields["htmlBody"] = String(vm.renderEmailTemplateHTML(template, args[1], args[2]))
	message.Fields["plainTextBody"] = String(vm.renderEmailTemplateText(storageStringField(template, "Body"), args[1], args[2]))
	message.Fields["fileAttachments"] = vm.renderStoredEmailTemplateAttachments(template, attachmentOption)
	return message, nil
}
func (vm *VM) renderStoredEmailTemplateAttachments(template storage.Record, option Value) Value {
	if option.Kind == ValueNull {
		return List()
	}
	optionName := strings.ToUpper(strings.TrimSpace(option.Text))
	if optionName == "" || optionName == "NONE" {
		return List()
	}
	names := localEmailTemplateAttachmentNames(template)
	if vm == nil || vm.Org == nil {
		return List()
	}
	withBody := optionName == "METADATA_WITH_BODY" || optionName == "BODY"
	attachments := make([]Value, 0, len(names))
	for _, name := range names {
		if attachment, ok := vm.localEmailAttachmentByToken(template, name, withBody); ok {
			attachments = append(attachments, attachment)
		}
	}
	attachments = append(attachments, vm.localEmailTemplateRecordAttachments(template, withBody)...)
	attachments = append(attachments, vm.localEmailTemplateContentDocumentAttachments(template, withBody)...)
	return List(attachments...)
}

func (vm *VM) localEmailAttachmentByToken(template storage.Record, token string, withBody bool) (Value, bool) {
	if resource, ok := vm.staticResourceMetadata(token); ok {
		attachment := newEmailFileAttachment()
		fileName := strings.TrimSpace(resource.Description)
		if fileName == "" {
			fileName = resource.Name
		}
		attachment.Fields["fileName"] = String(fileName)
		if strings.TrimSpace(resource.ContentType) != "" {
			attachment.Fields["contentType"] = String(strings.TrimSpace(resource.ContentType))
		}
		if withBody {
			if content, ok := vm.staticResourceContent(resource.Name); ok {
				attachment.Fields["body"] = platformScalar("Blob", content)
			}
		}
		return attachment, true
	}
	if attachment, ok := vm.localAttachmentRecordByToken(template, token, withBody); ok {
		return attachment, true
	}
	if attachment, ok := vm.localContentVersionAttachmentByToken(token, withBody); ok {
		return attachment, true
	}
	return Null, false
}

func (vm *VM) localEmailTemplateRecordAttachments(template storage.Record, withBody bool) []Value {
	if vm == nil || vm.Org == nil {
		return nil
	}
	object, ok := vm.Org.Objects["Attachment"]
	if !ok {
		return nil
	}
	var records []storage.Record
	for _, record := range object.Records {
		if storageIDFromValueOrString(record, "ParentId") == template.ID {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	out := make([]Value, 0, len(records))
	for _, record := range records {
		out = append(out, emailFileAttachmentFromAttachmentRecord(record, withBody))
	}
	return out
}

func (vm *VM) localAttachmentRecordByToken(template storage.Record, token string, withBody bool) (Value, bool) {
	if vm == nil || vm.Org == nil {
		return Null, false
	}
	object, ok := vm.Org.Objects["Attachment"]
	if !ok {
		return Null, false
	}
	for _, record := range object.Records {
		if string(record.ID) == token || strings.EqualFold(storageStringField(record, "Name"), token) {
			if parentID := storageIDFromValueOrString(record, "ParentId"); parentID == "" || parentID == template.ID {
				return emailFileAttachmentFromAttachmentRecord(record, withBody), true
			}
		}
	}
	return Null, false
}

func emailFileAttachmentFromAttachmentRecord(record storage.Record, withBody bool) Value {
	attachment := newEmailFileAttachment()
	attachment.Fields["id"] = String(string(record.ID))
	if name := storageStringField(record, "Name"); name != "" {
		attachment.Fields["fileName"] = String(name)
	}
	if contentType := storageStringField(record, "ContentType"); contentType != "" {
		attachment.Fields["contentType"] = String(contentType)
	}
	if withBody {
		if body, ok := storageValueTextField(record, "Body"); ok {
			attachment.Fields["body"] = platformScalar("Blob", body)
		}
	}
	return attachment
}

func (vm *VM) localEmailTemplateContentDocumentAttachments(template storage.Record, withBody bool) []Value {
	if vm == nil || vm.Org == nil {
		return nil
	}
	links, ok := vm.Org.Objects["ContentDocumentLink"]
	if !ok {
		return nil
	}
	var documentIDs []storage.ID
	for _, link := range links.Records {
		if storageIDFromValueOrString(link, "LinkedEntityId") == template.ID {
			if documentID := storageIDFromValueOrString(link, "ContentDocumentId"); documentID != "" {
				documentIDs = append(documentIDs, documentID)
			}
		}
	}
	sort.Slice(documentIDs, func(i, j int) bool { return documentIDs[i] < documentIDs[j] })
	out := make([]Value, 0, len(documentIDs))
	for _, documentID := range documentIDs {
		if attachment, ok := vm.localContentVersionAttachmentByDocumentID(documentID, withBody); ok {
			out = append(out, attachment)
		}
	}
	return out
}

func (vm *VM) localContentVersionAttachmentByToken(token string, withBody bool) (Value, bool) {
	if vm == nil || vm.Org == nil {
		return Null, false
	}
	if attachment, ok := vm.localContentVersionAttachmentByDocumentID(storage.ID(token), withBody); ok {
		return attachment, true
	}
	versions, ok := vm.Org.Objects["ContentVersion"]
	if !ok {
		return Null, false
	}
	for _, record := range versions.Records {
		if string(record.ID) == token || strings.EqualFold(storageStringField(record, "PathOnClient"), token) || strings.EqualFold(storageStringField(record, "Title"), token) {
			return emailFileAttachmentFromContentVersion(record, withBody), true
		}
	}
	return Null, false
}

func (vm *VM) localContentVersionAttachmentByDocumentID(documentID storage.ID, withBody bool) (Value, bool) {
	if vm == nil || vm.Org == nil || documentID == "" {
		return Null, false
	}
	versions, ok := vm.Org.Objects["ContentVersion"]
	if !ok {
		return Null, false
	}
	var matches []storage.Record
	for _, record := range versions.Records {
		if storageIDFromValueOrString(record, "ContentDocumentId") == documentID {
			matches = append(matches, record)
		}
	}
	if len(matches) == 0 {
		return Null, false
	}
	sort.Slice(matches, func(i, j int) bool {
		leftLatest := storageBoolField(matches[i], "IsLatest")
		rightLatest := storageBoolField(matches[j], "IsLatest")
		if leftLatest != rightLatest {
			return leftLatest
		}
		return matches[i].ID > matches[j].ID
	})
	return emailFileAttachmentFromContentVersion(matches[0], withBody), true
}

func emailFileAttachmentFromContentVersion(record storage.Record, withBody bool) Value {
	attachment := newEmailFileAttachment()
	attachment.Fields["id"] = String(string(record.ID))
	fileName := storageStringField(record, "PathOnClient")
	if fileName == "" {
		fileName = storageStringField(record, "Title")
	}
	if fileName != "" {
		attachment.Fields["fileName"] = String(fileName)
	}
	if contentType := storageStringField(record, "ContentType"); contentType != "" {
		attachment.Fields["contentType"] = String(contentType)
	} else if fileType := storageStringField(record, "FileType"); fileType != "" {
		attachment.Fields["contentType"] = String(emailContentTypeFromFileType(fileType))
	}
	if withBody {
		if body, ok := storageValueTextField(record, "VersionData"); ok {
			attachment.Fields["body"] = platformScalar("Blob", body)
		}
	}
	return attachment
}

func emailContentTypeFromFileType(fileType string) string {
	switch strings.ToUpper(strings.TrimSpace(fileType)) {
	case "PDF":
		return "application/pdf"
	case "TXT", "TEXT":
		return "text/plain"
	case "HTML":
		return "text/html"
	default:
		return strings.ToLower(strings.TrimSpace(fileType))
	}
}

func storageIDFromValueOrString(record storage.Record, field string) storage.ID {
	value, ok := record.GetField(field)
	if !ok {
		return ""
	}
	return storageIDFromValue(value)
}

func storageBoolField(record storage.Record, field string) bool {
	value, ok := record.GetField(field)
	return ok && value.Kind == storage.ValueBoolean && value.Boolean
}

func storageValueTextField(record storage.Record, field string) (string, bool) {
	value, ok := record.GetField(field)
	if !ok {
		return "", false
	}
	switch value.Kind {
	case storage.ValueBlob:
		return value.String, true
	case storage.ValueString:
		return value.String, true
	default:
		return "", false
	}
}
func localEmailTemplateAttachmentNames(template storage.Record) []string {
	var names []string
	for _, field := range []string{"Attachment", "Attachments", "StaticResource", "StaticResources", "StaticResourceName", "StaticResourceNames"} {
		raw := strings.TrimSpace(storageStringField(template, field))
		if raw == "" {
			continue
		}
		for _, part := range strings.Split(raw, ",") {
			if name := strings.TrimSpace(part); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}
func (vm *VM) staticResourceMetadata(name string) (storage.StaticResourceMetadata, bool) {
	if vm == nil || vm.Org == nil {
		return storage.StaticResourceMetadata{}, false
	}
	for _, resource := range vm.Org.Metadata.StaticResources {
		if strings.EqualFold(resource.Name, name) {
			return resource, true
		}
	}
	return storage.StaticResourceMetadata{}, false
}
func (vm *VM) emailTemplateByID(templateID string) (storage.Record, bool) {
	if vm.Org == nil {
		return storage.Record{}, false
	}
	objectName, ok := vm.resolveObjectName("EmailTemplate")
	if !ok {
		objectName = "EmailTemplate"
	}
	object := vm.Org.Objects[objectName]
	if record, ok := object.Records[storage.ID(templateID)]; ok {
		return record, true
	}
	for _, record := range object.Records {
		if string(record.ID) == templateID {
			return record, true
		}
		if id, ok := record.GetField("Id"); ok && string(storageIDFromValue(id)) == templateID {
			return record, true
		}
	}
	return storage.Record{}, false
}
func (vm *VM) emailTemplateByName(name string) (storage.Record, bool) {
	if vm.Org == nil {
		return storage.Record{}, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return storage.Record{}, false
	}
	objectName, ok := vm.resolveObjectName("EmailTemplate")
	if !ok {
		objectName = "EmailTemplate"
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return storage.Record{}, false
	}
	for _, record := range object.Records {
		for _, field := range []string{"DeveloperName", "Name"} {
			if strings.EqualFold(storageStringField(record, field), name) {
				return record, true
			}
		}
	}
	return storage.Record{}, false
}
func (vm *VM) renderEmailTemplateText(text string, whoID, whatID Value) string {
	if text == "" || !strings.Contains(text, "{!") {
		return text
	}
	whoRecord, whoOK := vm.recordByIDValue(whoID)
	whatRecord, whatOK := vm.recordByIDValue(whatID)
	var out strings.Builder
	for {
		start := strings.Index(text, "{!")
		if start < 0 {
			out.WriteString(text)
			return out.String()
		}
		out.WriteString(text[:start])
		text = text[start+2:]
		end := strings.Index(text, "}")
		if end < 0 {
			out.WriteString("{!")
			out.WriteString(text)
			return out.String()
		}
		token := strings.TrimSpace(text[:end])
		if value, ok := vm.emailMergeTokenValue(token, whoRecord, whoOK, whatRecord, whatOK); ok {
			out.WriteString(value)
		} else {
			out.WriteString("{!")
			out.WriteString(text[:end])
			out.WriteString("}")
		}
		text = text[end+1:]
	}
}
func (vm *VM) renderEmailTemplateHTML(template storage.Record, whoID, whatID Value) string {
	html := storageStringField(template, "HtmlValue")
	if html == "" && emailTemplateLooksVisualforce(template) {
		markup := storageStringField(template, "Markup")
		if markup == "" {
			markup = storageStringField(template, "Body")
		}
		html = vm.renderVisualforceEmailTemplateMarkup(markup, whoID, whatID)
	}
	return vm.renderEmailTemplateText(html, whoID, whatID)
}
func emailTemplateLooksVisualforce(template storage.Record) bool {
	if strings.EqualFold(storageStringField(template, "TemplateType"), "visualforce") {
		return true
	}
	for _, field := range []string{"Markup", "Body"} {
		text := strings.TrimSpace(storageStringField(template, field))
		if strings.Contains(strings.ToLower(text), "<messaging:emailtemplate") {
			return true
		}
	}
	return false
}
func (vm *VM) renderVisualforceEmailTemplateMarkup(markup string, whoID, whatID Value) string {
	body := visualforceTagBody(markup, "messaging:htmlEmailBody")
	if body == "" {
		body = markup
	}
	body = vm.replaceVisualforceTemplateTags(body)
	return vm.renderEmailTemplateText(body, whoID, whatID)
}
func (vm *VM) replaceVisualforceTemplateTags(input string) string {
	var out strings.Builder
	lastHandled := false
	for i := 0; i < len(input); {
		start := strings.IndexByte(input[i:], '<')
		if start < 0 {
			out.WriteString(input[i:])
			break
		}
		start += i
		gap := input[i:start]
		nameStart := start + 1
		if nameStart >= len(input) || input[nameStart] == '/' || input[nameStart] == '!' || input[nameStart] == '?' {
			out.WriteString(gap)
			out.WriteByte(input[start])
			i = start + 1
			lastHandled = false
			continue
		}
		nameEnd := nameStart
		for nameEnd < len(input) {
			ch := input[nameEnd]
			if ch == '>' || ch == '/' || unicode.IsSpace(rune(ch)) {
				break
			}
			nameEnd++
		}
		if nameEnd == nameStart {
			out.WriteString(gap)
			out.WriteByte(input[start])
			i = start + 1
			lastHandled = false
			continue
		}
		end := visualforceTagEnd(input, nameEnd)
		if end < 0 {
			out.WriteString(gap)
			out.WriteString(input[start:])
			break
		}
		name := input[nameStart:nameEnd]
		attrs := input[nameEnd:end]
		handled := true
		replacement := ""
		switch {
		case strings.EqualFold(name, "apex:outputText"):
			replacement = visualforceOutputTextValue(attrs)
		case strings.EqualFold(visualforceLocalTagName(name), "EmailContent"):
			key := visualforceAttrValue(attrs, "key")
			replacement = vm.visualforceEmailContentValue(key)
		default:
			handled = false
			replacement = input[start : end+1]
		}
		if !handled || !lastHandled || strings.TrimSpace(gap) != "" {
			out.WriteString(gap)
		}
		out.WriteString(replacement)
		lastHandled = handled
		i = end + 1
	}
	return out.String()
}
func visualforceTagBody(markup, tagName string) string {
	lower := strings.ToLower(markup)
	openNeedle := "<" + strings.ToLower(tagName)
	open := strings.Index(lower, openNeedle)
	if open < 0 {
		return ""
	}
	openEnd := visualforceTagEnd(markup, open+len(openNeedle))
	if openEnd < 0 {
		return ""
	}
	closeNeedle := "</" + strings.ToLower(tagName) + ">"
	close := strings.Index(lower[openEnd+1:], closeNeedle)
	if close < 0 {
		return ""
	}
	close += openEnd + 1
	return markup[openEnd+1 : close]
}
func visualforceTagEnd(input string, start int) int {
	var quote byte
	escaped := false
	for i := start; i < len(input); i++ {
		ch := input[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			continue
		}
		if ch == '>' {
			return i
		}
	}
	return -1
}
func visualforceLocalTagName(name string) string {
	if idx := strings.LastIndexByte(name, ':'); idx >= 0 {
		return name[idx+1:]
	}
	return name
}
func visualforceAttrValue(attrs, name string) string {
	for i := 0; i < len(attrs); {
		for i < len(attrs) && (unicode.IsSpace(rune(attrs[i])) || attrs[i] == '/') {
			i++
		}
		start := i
		for i < len(attrs) {
			ch := attrs[i]
			if ch == '=' || unicode.IsSpace(rune(ch)) || ch == '/' {
				break
			}
			i++
		}
		attrName := strings.TrimSpace(attrs[start:i])
		for i < len(attrs) && unicode.IsSpace(rune(attrs[i])) {
			i++
		}
		if i >= len(attrs) || attrs[i] != '=' {
			for i < len(attrs) && !unicode.IsSpace(rune(attrs[i])) {
				i++
			}
			continue
		}
		i++
		for i < len(attrs) && unicode.IsSpace(rune(attrs[i])) {
			i++
		}
		if i >= len(attrs) || (attrs[i] != '"' && attrs[i] != '\'') {
			continue
		}
		quote := attrs[i]
		i++
		valueStart := i
		escaped := false
		for i < len(attrs) {
			ch := attrs[i]
			if escaped {
				escaped = false
				i++
				continue
			}
			if ch == '\\' {
				escaped = true
				i++
				continue
			}
			if ch == quote {
				value := attrs[valueStart:i]
				i++
				if strings.EqualFold(attrName, name) {
					return value
				}
				break
			}
			i++
		}
	}
	return ""
}
func visualforceOutputTextValue(attrs string) string {
	value := strings.TrimSpace(visualforceAttrValue(attrs, "value"))
	if strings.HasPrefix(value, "{!") && strings.HasSuffix(value, "}") {
		value = strings.TrimSpace(value[2 : len(value)-1])
	}
	if unquoted, ok := apexSingleQuotedTemplateLiteral(value); ok {
		return unquoted
	}
	return value
}
func apexSingleQuotedTemplateLiteral(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '\'' || value[len(value)-1] != '\'' {
		return "", false
	}
	var out strings.Builder
	escaped := false
	for i := 1; i < len(value)-1; i++ {
		ch := value[i]
		if escaped {
			switch ch {
			case 'n':
				out.WriteByte('\n')
			case 'r':
				out.WriteByte('\r')
			case 't':
				out.WriteByte('\t')
			default:
				out.WriteByte(ch)
			}
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		out.WriteByte(ch)
	}
	if escaped {
		out.WriteByte('\\')
	}
	return out.String(), true
}
func (vm *VM) visualforceEmailContentValue(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	for className, class := range vm.Classes {
		if !strings.EqualFold(className, "EmailContent") && !strings.EqualFold(class.Name, "EmailContent") && !hasSuffixFold(class.Name, ".emailcontent") {
			continue
		}
		for fieldName, field := range class.StaticFields {
			if !strings.EqualFold(fieldName, "contentMap") && !strings.EqualFold(field.Name, "contentMap") {
				continue
			}
			if value, ok := visualforceEmailContentMapValue(field.Value, key); ok {
				return value
			}
		}
	}
	return "[" + key + "]"
}
func visualforceEmailContentMapValue(content Value, key string) (string, bool) {
	if content.Kind != ValueMap {
		return "", false
	}
	encoded := mapKey(String(key))
	if value, ok := content.Map[encoded]; ok {
		return stringValue(value), true
	}
	for _, candidate := range content.MapKeys {
		if candidate.Kind == ValueString && strings.EqualFold(candidate.Text, key) {
			return stringValue(content.Map[mapKey(candidate)]), true
		}
	}
	return "", false
}
func (vm *VM) emailMergeTokenValue(token string, whoRecord storage.Record, whoOK bool, whatRecord storage.Record, whatOK bool) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", false
	}
	root := strings.TrimSpace(parts[0])
	field := strings.TrimSpace(strings.Join(parts[1:], "."))
	if root == "" || field == "" {
		return "", false
	}
	namespace := ""
	if vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	if whoOK && emailMergeRootMatches(root, whoRecord.Object, namespace, "Recipient", "Who", "TargetObject") {
		return vm.storageRecordStringField(whoRecord, field), true
	}
	if whatOK && emailMergeRootMatches(root, whatRecord.Object, namespace, "RelatedTo", "What") {
		return vm.storageRecordStringField(whatRecord, field), true
	}
	return "", false
}
func emailMergeRootMatches(root, objectName, namespace string, aliases ...string) bool {
	for _, alias := range aliases {
		if strings.EqualFold(root, alias) {
			return true
		}
	}
	if strings.EqualFold(root, objectName) {
		return true
	}
	return strings.EqualFold(root, storage.StripNamespaceToken(namespace, objectName))
}

func callEmailFileAttachmentMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "setBody":
		if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Blob" {
			return Null, receiver, false, true, fmt.Errorf("Messaging.EmailFileAttachment.setBody expects Blob")
		}
		receiver.Fields["body"] = args[0]
		return Null, receiver, true, true, nil
	case "setContentType", "setFileName":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Messaging.EmailFileAttachment.%s expects String", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setInline":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Messaging.EmailFileAttachment.setInline expects Boolean")
		}
		receiver.Fields["inline"] = args[0]
		return Null, receiver, true, true, nil
	case "getBody", "getContentType", "getFileName", "getId", "getInline":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Messaging.EmailFileAttachment.%s expects 0 arguments", method)
		}
		return receiver.Fields[emailMessageFieldName(method)], receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}
func callSingleEmailMessageMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "setToAddresses", "setCcAddresses", "setBccAddresses", "setFileAttachments", "setEntityAttachments", "setDocumentAttachments", "setTargetObjectIds":
		if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueNull) {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects List", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setSubject", "setPlainTextBody", "setHtmlBody", "setReplyTo", "setSenderDisplayName",
		"setCharset", "setInReplyTo", "setReferences", "setOrgWideEmailAddressId",
		"setTargetObjectId", "setTemplateId", "setWhatId", "setOptOutPolicy", "setEmailPriority",
		"setUnsubscribeComment":
		if len(args) != 1 || (args[0].Kind != ValueString && args[0].Kind != ValueNull && !(args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Id"))) {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects String", method)
		}
		value := args[0]
		if idText, ok := typedIDValueText(value); ok {
			value = String(idText)
		}
		receiver.Fields[emailMessageFieldName(method)] = value
		return Null, receiver, true, true, nil
	case "setSaveAsActivity", "setTreatBodiesAsTemplate", "setTreatTargetObjectAsRecipient", "setUseSignature", "setBccSender", "setOneClickPost":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects Boolean", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setUnsubscribeUrls":
		if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueNull) {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.setUnsubscribeUrls expects List")
		}
		receiver.Fields["unsubscribeUrls"] = args[0]
		return Null, receiver, true, true, nil
	case "getToAddresses", "getCcAddresses", "getBccAddresses", "getFileAttachments", "getEntityAttachments", "getDocumentAttachments", "getTargetObjectIds",
		"getSubject", "getPlainTextBody", "getHtmlBody", "getReplyTo", "getSenderDisplayName",
		"getCharset", "getInReplyTo", "getReferences", "getOrgWideEmailAddressId",
		"getTargetObjectId", "getTemplateId", "getTemplateName", "getWhatId", "getOptOutPolicy", "getEmailPriority",
		"getUnsubscribeComment", "getUnsubscribeUrls",
		"getSaveAsActivity", "getTreatBodiesAsTemplate", "getTreatTargetObjectAsRecipient", "getUseSignature", "getBccSender", "getOneClickPost":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects 0 arguments", method)
		}
		return receiver.Fields[emailMessageFieldName(method)], receiver, false, true, nil
	case "isTreatBodiesAsTemplate", "isTreatTargetObjectAsRecipient", "isUserMail":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SingleEmailMessage.%s expects 0 arguments", method)
		}
		return receiver.Fields[emailMessageFieldName(method)], receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}
func callMassEmailMessageMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "setTargetObjectIds", "setWhatIds":
		if len(args) != 1 || (args[0].Kind != ValueList && args[0].Kind != ValueNull) {
			return Null, receiver, false, true, fmt.Errorf("Messaging.MassEmailMessage.%s expects List", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "setTemplateId", "setDescription", "setOptOutPolicy", "setEmailPriority", "setReplyTo", "setSenderDisplayName", "setSubject":
		if len(args) != 1 || (args[0].Kind != ValueString && args[0].Kind != ValueNull && !(args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Id"))) {
			return Null, receiver, false, true, fmt.Errorf("Messaging.MassEmailMessage.%s expects String", method)
		}
		value := args[0]
		if idText, ok := typedIDValueText(value); ok {
			value = String(idText)
		}
		receiver.Fields[emailMessageFieldName(method)] = value
		return Null, receiver, true, true, nil
	case "setSaveAsActivity", "setBccSender", "setUseSignature":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Messaging.MassEmailMessage.%s expects Boolean", method)
		}
		receiver.Fields[emailMessageFieldName(method)] = args[0]
		return Null, receiver, true, true, nil
	case "getTargetObjectIds", "getWhatIds", "getTemplateId", "getDescription", "getOptOutPolicy",
		"getEmailPriority", "getReplyTo", "getSenderDisplayName", "getSubject",
		"getSaveAsActivity", "getBccSender", "getUseSignature":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Messaging.MassEmailMessage.%s expects 0 arguments", method)
		}
		return receiver.Fields[emailMessageFieldName(method)], receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}
func callMessagingDTOGetter(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	field := ""
	if suffix, ok := passiveAccessorSuffix(method, "get"); ok {
		field = strings.ToLower(suffix[:1]) + suffix[1:]
	} else if suffix, ok := passiveAccessorSuffix(method, "is"); ok {
		field = strings.ToLower(suffix[:1]) + suffix[1:]
	}
	if field == "" {
		return Null, receiver, false, false, nil
	}
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	if value, ok := receiver.Fields[field]; ok {
		return value, receiver, false, true, nil
	}
	if value, ok := receiver.Fields[strings.ToLower(field)]; ok {
		return value, receiver, false, true, nil
	}
	return Null, receiver, false, true, nil
}
func callMessagingActionResultMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "isSuccess", "getMessage", "getErrorCode":
		return callMessagingDTOGetter(receiver, method, args)
	default:
		return Null, receiver, false, false, nil
	}
}
func callMessagingBuilderMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	if strings.EqualFold(method, "build") {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.build expects 0 arguments", receiver.Type)
		}
		var built Value
		if strings.EqualFold(receiver.Type, "Messaging.ActionableNotification.Builder") {
			built = newActionableNotification()
		} else {
			built = newActionResult()
		}
		for field, value := range receiver.Fields {
			built.Fields[field] = value
		}
		return built, receiver, false, true, nil
	}
	if !strings.HasPrefix(method, "with") || len(method) <= len("with") {
		return Null, receiver, false, false, nil
	}
	if len(args) != 1 {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects 1 argument", receiver.Type, method)
	}
	field := strings.TrimPrefix(method, "with")
	field = strings.ToLower(field[:1]) + field[1:]
	receiver.Fields[field] = args[0]
	return receiver, receiver, true, true, nil
}
