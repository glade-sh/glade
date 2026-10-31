package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"strings"
)

// Translate the master label produced by toLabelValue once for supported standard value sets.
func translatedPredicateLabel(org storage.OrgState, definition storage.ObjectDefinition, field string, label storage.Value, language string) storage.Value {
	if label.Kind != storage.ValueString || language == "" || !org.HasFeature("TranslationLanguage-"+language) {
		return label
	}
	// Follow only unambiguous, single-target relationships before applying
	// the supported standard value-set binding.
	if unqualified, ok := stripQualifiedCurrentObjectField(org, definition, field); ok {
		field = unqualified
	}
	parts := strings.Split(field, ".")
	current := definition
	for _, part := range parts[:len(parts)-1] {
		relations := matchingParentRelations(org.Namespace, current, part)
		if len(relations) != 1 || relations[0].Polymorphic || len(relations[0].ParentObjects) != 1 {
			return label
		}
		var canonical string
		var ok bool
		org, canonical, ok = orgWithLazyStandardObject(org, relations[0].ParentObjects[0])
		if !ok {
			return label
		}
		current = org.Objects[canonical].Definition
	}
	object := current.APIName
	leaf := parts[len(parts)-1]
	var valueSet string
	switch {
	case strings.EqualFold(object, "Account") && strings.EqualFold(leaf, "Industry"):
		valueSet = "Industry"
	case strings.EqualFold(object, "Opportunity") && strings.EqualFold(leaf, "StageName"):
		valueSet = "OpportunityStage"
	case strings.EqualFold(object, "Task") && strings.EqualFold(leaf, "Status"):
		valueSet = "TaskStatus"
	case strings.EqualFold(object, "Order") && strings.EqualFold(leaf, "Status"):
		valueSet = "OrderStatus"
	case strings.EqualFold(object, "Contract") && strings.EqualFold(leaf, "Status"):
		valueSet = "ContractStatus"
	default:
		return label
	}
	fields, ok := fieldDefinitionsForReference(org, definition, field)
	if !ok || len(fields) != 1 || fields[0].Type != storage.FieldPicklist {
		return label
	}
	for _, t := range org.Metadata.ValueSetTranslations {
		if strings.EqualFold(t.ValueSet, valueSet) && strings.EqualFold(t.Language, language) && t.MasterLabel == label.String {
			return storage.StringValue(t.Translation)
		}
	}
	return label
}
