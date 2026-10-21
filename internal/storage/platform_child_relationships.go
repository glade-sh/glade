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
