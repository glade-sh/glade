package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type contentChildRelationship struct {
	childObject  string
	relationship storage.Relationship
}

// Ordinary custom objects share the platform child catalog. Sparse local orgs
// only expose children whose object definitions are actually loaded.
func (vm *VM) contentChildRelationships(parentObject string) []contentChildRelationship {
	if vm == nil || vm.Org == nil {
		return nil
	}
	parentName, ok := vm.resolveObjectName(parentObject)
	if !ok {
		return nil
	}
	definition := vm.Org.Objects[parentName].Definition
	if storage.IsCustomSettingDefinition(definition) || !strings.HasSuffix(strings.ToLower(definition.APIName), "__c") {
		return nil
	}
	var out []contentChildRelationship
	storage.VisitPlatformChildRelationships(definition, func(child string, relationship storage.Relationship) {
		if _, loaded := vm.Org.Objects[child]; !loaded {
			return
		}
		// Captured deletion flags belong to the parent-side describe edge.
		switch relationship.ChildRelationship {
		case "AttachedContentDocuments", "Attachments", "CombinedAttachments", "ContentDocumentLinks",
			"DuplicateRecordItems", "FeedSubscriptionsForEntity", "Notes", "NotesAndAttachments",
			"ProcessExceptions", "ProcessInstances", "RecordActions", "RecordAssociatedGroups",
			"Shares", "TopicAssignments", "UserDefinedLabelAssignments":
			relationship.CascadeDelete = true
		}
		if relationship.Field == "LinkedEntityId" {
			relationship.ParentRelationship = "LinkedEntity"
		}
		out = append(out, contentChildRelationship{childObject: child, relationship: relationship})
		// Native describe exposes both fields under the same relationship name.
		switch relationship.ChildRelationship {
		case "FinanceBalanceSnapshots", "FinanceTransactions":
			relationship.Field = "LegalEntityId"
			out = append(out, contentChildRelationship{childObject: child, relationship: relationship})
		}
	})
	return out
}

// CASE_ACCESSOR_LOWER/UPPER and CD03 capture loaded child lists only. Keep the
// existing error/lazy-load behavior for unqueried and caller-constructed rows.
func (vm *VM) loadedContentChildRelationship(receiver Value, field string) bool {
	if _, queried := receiver.Fields[sobjectQueriedFieldsField]; !queried || !vm.queriedSObjectFieldsIncludes(receiver, field) {
		return false
	}
	_, value, loaded := objectFieldValue(receiver, field)
	if !loaded || value.Kind != ValueList {
		return false
	}
	for _, child := range vm.contentChildRelationships(receiver.Type) {
		if strings.EqualFold(field, child.relationship.ChildRelationship) {
			return true
		}
	}
	return false
}

// CD06A (Account) and CD06C (ordinary custom object) collect every relationship
// name containing "content", not merely a candidate set. Suppress only the five
// catalog extras contradicted by those native rows. Query/accessor metadata stays
// intact; other parent object types keep their existing describes.
func (vm *VM) contentChildDescribeRelationshipVisible(parentObject, relationship string) bool {
	switch strings.ToLower(relationship) {
	case "attachedcontentnotes", "contentdeliveries", "contentdocumentcontext", "contentversions", "userrelatedrecordcontents":
	default:
		return true
	}
	if vm == nil || vm.Org == nil {
		return true
	}
	parentName, ok := vm.resolveObjectName(parentObject)
	if !ok {
		return true
	}
	definition := vm.Org.Objects[parentName].Definition
	return !strings.EqualFold(definition.APIName, "Account") &&
		(storage.IsCustomSettingDefinition(definition) || !strings.HasSuffix(strings.ToLower(definition.APIName), "__c"))
}
