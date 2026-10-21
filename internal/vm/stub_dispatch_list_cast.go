package vm

import "strings"

// A single-record concrete Account list keeps its runtime collection type
// through a List<SObject> method view. An explicit List<Contact> cast must not
// accept that view before checking its concrete type. Leave empty, generic,
// queried and matching-target lists on their existing paths.
func sObjectListViewCastRejectsUnrelatedConcrete(typeName string, value Value) bool {
	if value.Kind != ValueList || len(value.List) != 1 || collectionBase(typeName) != "List" {
		return false
	}
	target, targetOK := collectionElementType(typeName)
	source, sourceOK := collectionElementType(value.Runtime)
	view, viewOK := collectionElementType(value.Type)
	staticView, staticOK := collectionElementType(value.Static)
	if !targetOK || !sourceOK || !viewOK || !staticOK ||
		collectionBase(value.Runtime) != "List" || collectionBase(value.Type) != "List" || collectionBase(value.Static) != "List" ||
		!strings.EqualFold(target, "Contact") || !strings.EqualFold(source, "Account") ||
		!strings.EqualFold(view, "SObject") || !strings.EqualFold(staticView, "SObject") {
		return false
	}
	row := value.List[0]
	if row.Kind != ValueObject || row.classInstance || !strings.EqualFold(row.Type, "Account") {
		return false
	}
	_, queried := row.Fields[sobjectQueriedFieldsField]
	return !queried
}
