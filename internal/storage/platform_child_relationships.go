package storage

import (
	"strings"
	"sync"
)

type platformChildRelationship struct {
	name, child, field string
	relationship       Relationship
}

type platformChildRelationshipCatalog struct {
	once sync.Once
	rows []platformChildRelationship
}

var platformChildRelationshipCatalogs sync.Map

// These captured child entities are newer than the embedded describe baseline.
// Their Id projection uses the common object shape; richer fields stay unknown.
var platformRelationshipChildObjectNames = []string{"ExtlRecShrStagedRecord", "FinanceBalanceSnapshot", "FinanceTransaction"}

// Captured edges supplement the portable describe baseline without rewriting it.
var platformChildRelationshipSupplements = map[string][]platformChildRelationship{
	"Account": {
		{name: "AccountCleanInfos", child: "AccountCleanInfo", field: "AccountId"},
		{name: "Emails", child: "EmailMessage", field: "RelatedToId"},
		{name: "ExtlRecShrStagedRecordAccounts", child: "ExtlRecShrStagedRecord", field: "AccountId"},
		{name: "MessagingEndUsers", child: "MessagingEndUser", field: "AccountId"},
		{name: "ParentRecords", child: "ExtlRecShrStagedRecord", field: "ParentRecordId"},
	},
	"Case": {
		{name: "DuplicateRecordItems", child: "DuplicateRecordItem", field: "RecordId"},
		{name: "Emails", child: "EmailMessage", field: "RelatedToId"},
		{name: "MessagingSessions", child: "MessagingSession", field: "CaseId"},
		{name: "ParentRecords", child: "ExtlRecShrStagedRecord", field: "ParentRecordId"},
	},
	"Contact": {
		{name: "ContactCleanInfos", child: "ContactCleanInfo", field: "ContactId"},
		{name: "MessagingEndUsers", child: "MessagingEndUser", field: "ContactId"},
		{name: "ParentRecords", child: "ExtlRecShrStagedRecord", field: "ParentRecordId"},
	},
	"Opportunity": {
		{name: "Emails", child: "EmailMessage", field: "RelatedToId"},
		{name: "Invoices", child: "Invoice", field: "ReferenceEntityId"},
		{name: "MessagingSessions", child: "MessagingSession", field: "OpportunityId"},
		{name: "Orders", child: "Order", field: "OpportunityId"},
		{name: "ParentRecords", child: "ExtlRecShrStagedRecord", field: "ParentRecordId"},
		{name: "RelatedAuthorizationFormConsents", child: "AuthorizationFormConsent", field: "RelatedRecordId"},
	},
	"User": {
		{name: "AccountCleanInfoReviewers", child: "AccountCleanInfo", field: "LastStatusChangedById"},
		{name: "Badges", child: "WorkBadge", field: "RecipientId"},
		{name: "ContactCleanInfoReviewers", child: "ContactCleanInfo", field: "LastStatusChangedById"},
		{name: "GivenThanks", child: "WorkThanks", field: "GiverId"},
		{name: "LeadCleanInfoReviewers", child: "LeadCleanInfo", field: "LastStatusChangedById"},
		{name: "MessagingEndUsers", child: "MessagingEndUser", field: "AuthenticatedEndUserId"},
	},
}

// These polymorphic platform edges are present on ordinary custom objects.
var customPlatformChildRelationships = []platformChildRelationship{
	{name: "ApprovalSubmissions", child: "ApprovalSubmission", field: "RelatedRecordId"},
	{name: "ApprovalWorkItems", child: "ApprovalWorkItem", field: "RelatedRecordId"},
	{name: "AttachedContentDocuments", child: "AttachedContentDocument", field: "LinkedEntityId"},
	{name: "Attachments", child: "Attachment", field: "ParentId"},
	{name: "CombinedAttachments", child: "CombinedAttachment", field: "ParentId"},
	{name: "ContactRequests", child: "ContactRequest", field: "WhatId"},
	{name: "ContentDocumentLinks", child: "ContentDocumentLink", field: "LinkedEntityId"},
	{name: "DestinationFinanceTransactions", child: "FinanceTransaction", field: "DestinationEntityId"},
	{name: "DuplicateRecordItems", child: "DuplicateRecordItem", field: "RecordId"},
	{name: "FeedSubscriptionsForEntity", child: "EntitySubscription", field: "ParentId"},
	{name: "FinanceBalanceSnapshots", child: "FinanceBalanceSnapshot", field: "ReferenceEntityId"},
	{name: "FinanceTransactions", child: "FinanceTransaction", field: "ReferenceEntityId"},
	{name: "FlowOrchestrationWorkItems", child: "FlowOrchestrationWorkItem", field: "RelatedRecordId"},
	{name: "Notes", child: "Note", field: "ParentId"},
	{name: "NotesAndAttachments", child: "NoteAndAttachment", field: "ParentId"},
	{name: "ParentFinanceTransactions", child: "FinanceTransaction", field: "ParentReferenceEntityId"},
	{name: "ProcessExceptions", child: "ProcessException", field: "AttachedToId"},
	{name: "ProcessInstances", child: "ProcessInstance", field: "TargetObjectId"},
	{name: "ProcessSteps", child: "ProcessInstanceHistory", field: "TargetObjectId"},
	{name: "RecordActionHistories", child: "RecordActionHistory", field: "ParentRecordId"},
	{name: "RecordActions", child: "RecordAction", field: "RecordId"},
	{name: "RecordAssociatedGroups", child: "CollaborationGroupRecord", field: "RecordId"},
	{name: "SourceFinanceTransactions", child: "FinanceTransaction", field: "SourceEntityId"},
	{name: "TopicAssignments", child: "TopicAssignment", field: "EntityId"},
	{name: "UserDefinedLabelAssignments", child: "UserDefinedLabelAssignment", field: "ItemId"},
}

// Edge facets come from owned native describe rows in
// platform_child_field_describe and standard_platform_child_fields. The latter
// also records the parent-side relationship names. DefaultedOnCreate is false
// for every captured field; the remaining flags vary by field.
var platformChildReferenceTargets = []string{"CreditMemo", "CreditMemoInvApplication", "CreditMemoLine", "Invoice", "InvoiceLine", "Payment", "PaymentLineInvoice", "Refund", "RefundLinePayment"}

type platformChildEdgeField struct {
	name, relationship, childRelationship        string
	targets                                      []string
	nillable, createable, updateable, accessible bool
}

var platformChildEdgeFields = map[string][]platformChildEdgeField{
	"FinanceBalanceSnapshot": {
		{name: "AccountId", relationship: "Account", childRelationship: "FinanceBalanceSnapshots",
			targets: []string{"Account"}, nillable: true, accessible: true},
		{name: "LegalEntityId", relationship: "LegalEntity", targets: []string{"LegalEntity"}, nillable: true, accessible: true},
		{name: "ReferenceEntityId", relationship: "ReferenceEntity", targets: platformChildReferenceTargets, nillable: true, accessible: true},
	},
	"FinanceTransaction": {
		{name: "AccountId", relationship: "Account", childRelationship: "FinanceTransactions",
			targets: []string{"Account"}, nillable: true, createable: true, accessible: true},
		{name: "DestinationEntityId", relationship: "DestinationEntity", targets: platformChildReferenceTargets, nillable: true, createable: true, accessible: true},
		{name: "LegalEntityId", relationship: "LegalEntity", targets: []string{"LegalEntity"}, nillable: true, createable: true, accessible: true},
		{name: "ParentReferenceEntityId", relationship: "ParentReferenceEntity", targets: platformChildReferenceTargets, nillable: true, createable: true, accessible: true},
		{name: "ReferenceEntityId", relationship: "ReferenceEntity", targets: platformChildReferenceTargets, nillable: true, createable: true, accessible: true},
		{name: "SourceEntityId", relationship: "SourceEntity", targets: platformChildReferenceTargets, nillable: true, createable: true, accessible: true},
	},
	"MessagingEndUser": {
		{name: "AccountId", relationship: "Account", childRelationship: "MessagingEndUsers",
			targets: []string{"Account"}, nillable: true, createable: true, updateable: true, accessible: true},
		{name: "ContactId", relationship: "Contact", childRelationship: "MessagingEndUsers",
			targets: []string{"Contact"}, nillable: true, createable: true, updateable: true, accessible: true},
		{name: "AuthenticatedEndUserId", relationship: "AuthenticatedEndUser", childRelationship: "MessagingEndUsers",
			targets: []string{"User"}, nillable: true, accessible: true},
	},
	"MessagingSession": {
		{name: "CaseId", relationship: "Case", childRelationship: "MessagingSessions",
			targets: []string{"Case"}, nillable: true, createable: true, updateable: true, accessible: true},
		{name: "OpportunityId", relationship: "Opportunity", childRelationship: "MessagingSessions",
			targets: []string{"Opportunity"}, nillable: true, createable: true, updateable: true, accessible: true},
	},
	"Order": {
		{name: "OpportunityId", relationship: "Opportunity", childRelationship: "Orders",
			targets: []string{"Opportunity"}, nillable: true},
	},
	"ExtlRecShrStagedRecord": {
		{name: "AccountId", relationship: "Account", childRelationship: "ExtlRecShrStagedRecordAccounts",
			targets: []string{"Account"}, nillable: true, createable: true, updateable: true, accessible: true},
		{name: "ParentRecordId", relationship: "ParentRecord", childRelationship: "ParentRecords",
			targets: []string{"Account", "Case", "Contact", "Lead", "Opportunity", "Product2"}, createable: true, updateable: true, accessible: true},
	},
}

func (edge platformChildEdgeField) field() Field {
	return Field{
		APIName: edge.name, Type: FieldReference, DisplayType: "REFERENCE",
		ReferenceTo: append([]string(nil), edge.targets...), RelationshipName: edge.relationship, ChildRelationshipName: edge.childRelationship,
		Nillable: BoolFlag(edge.nillable), Createable: BoolFlag(edge.createable), Updateable: BoolFlag(edge.updateable),
		DefaultedOnCreate: BoolFlag(false), Accessible: BoolFlag(edge.accessible),
	}
}

func platformChildEdgeFieldMatches(existing, want Field) bool {
	// Unset facets must be repaired even when a fallback equals the native
	// value: describe callers can use different defaults for the same flag.
	if existing.Nillable == nil || existing.Createable == nil || existing.Updateable == nil ||
		existing.DefaultedOnCreate == nil || existing.Accessible == nil {
		return false
	}
	return existing.Type == want.Type && existing.DisplayType == want.DisplayType &&
		strings.Join(existing.ReferenceTo, ",") == strings.Join(want.ReferenceTo, ",") &&
		existing.RelationshipName == want.RelationshipName && existing.ChildRelationshipName == want.ChildRelationshipName &&
		*existing.Nillable == *want.Nillable && *existing.Createable == *want.Createable &&
		*existing.Updateable == *want.Updateable && *existing.DefaultedOnCreate == *want.DefaultedOnCreate &&
		*existing.Accessible == *want.Accessible
}

// platformChildEdgeFieldsNeedRepair reports a definition persisted before these
// fields existed, or with different facets.
func platformChildEdgeFieldsNeedRepair(definition ObjectDefinition) bool {
	for _, edge := range platformChildEdgeFields[definition.APIName] {
		name, ok := ResolveFieldName(definition, "", edge.name)
		if !ok || !platformChildEdgeFieldMatches(definition.Fields[name], edge.field()) {
			return true
		}
	}
	return false
}

func applyPlatformChildEdgeFields(definition *ObjectDefinition) {
	for _, edge := range platformChildEdgeFields[definition.APIName] {
		if definition.Fields == nil {
			definition.Fields = make(map[string]Field)
		}
		want := edge.field()
		name := edge.name
		if existing, ok := ResolveFieldName(*definition, "", edge.name); ok {
			name = existing
			want.APIName = existing
			if previous := definition.Fields[existing]; previous.Label != "" {
				want.Label = previous.Label
			}
		}
		definition.Fields[name] = want
	}
}

func platformChildRelationships(parent ObjectDefinition) []platformChildRelationship {
	if strings.HasSuffix(strings.ToLower(parent.APIName), "__c") {
		if IsCustomSettingDefinition(parent) {
			return nil
		}
		rows := make([]platformChildRelationship, 0, len(customPlatformChildRelationships)+1)
		rows = append(rows, customPlatformChildRelationships...)
		rows = append(rows, platformChildRelationship{name: "Shares", child: parent.APIName[:len(parent.APIName)-3] + "__Share", field: "ParentId"})
		for i := range rows {
			rows[i].relationship = Relationship{Field: rows[i].field, ParentObjects: []string{parent.APIName}, ChildRelationship: rows[i].name, Polymorphic: true}
		}
		return rows
	}
	canonical, ok := ResolveKnownStandardObjectName(parent.APIName)
	if !ok {
		return nil
	}
	loaded, _ := platformChildRelationshipCatalogs.LoadOrStore(canonical, &platformChildRelationshipCatalog{})
	cache := loaded.(*platformChildRelationshipCatalog)
	cache.once.Do(func() {
		seen := make(map[string]bool)
		VisitStandardChildRelationships(canonical, func(child string, relationship Relationship) {
			key := strings.ToLower(relationship.ChildRelationship)
			if key == "" || child == "" || seen[key] {
				return
			}
			seen[key] = true
			cache.rows = append(cache.rows, platformChildRelationship{name: relationship.ChildRelationship, child: child, field: relationship.Field, relationship: relationship})
		})
		for _, row := range platformChildRelationshipSupplements[canonical] {
			if seen[strings.ToLower(row.name)] {
				continue
			}
			row.relationship = Relationship{Field: row.field, ParentObjects: []string{canonical}, ChildRelationship: row.name}
			cache.rows = append(cache.rows, row)
		}
	})
	return cache.rows
}

// VisitPlatformChildRelationships reads only the requested parent's catalog.
// Project-declared edges should be resolved before consulting this fallback.
func VisitPlatformChildRelationships(parent ObjectDefinition, visit func(string, Relationship)) bool {
	rows := platformChildRelationships(parent)
	for _, row := range rows {
		visit(row.child, cloneRelationship(row.relationship))
	}
	return len(rows) != 0
}

// LookupPlatformChildRelationship includes children not yet loaded into an org.
func LookupPlatformChildRelationship(parent ObjectDefinition, relationship string) (string, Relationship, bool) {
	for _, row := range platformChildRelationships(parent) {
		if strings.EqualFold(row.name, relationship) {
			return row.child, cloneRelationship(row.relationship), true
		}
	}
	return "", Relationship{}, false
}

// SupportsUserRecordAccessRelationship covers the captured record-bearing
// standard parents and ordinary custom objects. User and metadata are excluded.
func SupportsUserRecordAccessRelationship(objectName string) bool {
	switch strings.ToLower(objectName) {
	case "account", "case", "contact", "opportunity":
		return true
	}
	return strings.HasSuffix(strings.ToLower(objectName), "__c")
}

// UserRecordAccessRelationshipField is the selectable pseudo-parent field set.
func UserRecordAccessRelationshipField(name string) (Field, bool) {
	for _, field := range []string{"HasReadAccess", "HasEditAccess", "HasDeleteAccess", "HasTransferAccess", "HasAllAccess", "MaxAccessLevel"} {
		if strings.EqualFold(field, name) {
			kind := FieldBoolean
			if field == "MaxAccessLevel" {
				kind = FieldString
			}
			return Field{APIName: field, Type: kind}, true
		}
	}
	return Field{}, false
}
