package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

// CL001-006/011-031/036-039/041/043/044/061/063, both routes at
// API 62/67/68: the captured explicit List-to-record casts reject their lists.
// Keep singleton SOQL assignment in coerceAssignable, outside this cast path.
func (vm *VM) listSObjectCastError(typeName string, value Value) error {
	sourceType := runtimeValueTypeName(value)
	// A query list can retain List<Account> behind a List<SObject> view
	// (CL061/063). Name its runtime collection, not its current list view.
	if collectionBase(value.Runtime) == "List" {
		sourceType = value.Runtime
	}
	targetType := vm.resolveAssignableTargetType(typeName)
	if canonical, ok := vm.resolveObjectName(targetType); ok {
		targetType = canonical
	}
	return newExceptionError("System.TypeException", fmt.Sprintf("Invalid conversion from runtime type %s to %s", vm.typeExceptionSourceName(sourceType), vm.typeExceptionDisplayName(targetType)))
}

// Keep the new rejection within the captured runtime type pairs. The list
// matrix covers empty, single and multiple rows without unwrapping elements.
// Map and wrong-target branches retain their separately captured cardinalities.
func (vm *VM) listSObjectCastRejectsRecord(typeName string, value Value) bool {
	if value.Kind != ValueList && value.Kind != ValueMap {
		return false
	}
	targetType := vm.resolveAssignableTargetType(typeName)
	if !vm.isSObjectLikeType(targetType) {
		return false
	}
	sourceType := value.Type
	if collectionBase(value.Runtime) == "List" || isMapType(value.Runtime) {
		sourceType = value.Runtime
	}
	if value.Kind == ValueMap {
		// CL008 and MAP_1: explicit Map<String,Integer>-to-Account is
		// not JSON construction. The singleton twin is captured via Object
		// at API 62/67/68; retain other cardinalities and views.
		key, element, ok := mapTypeArgs(sourceType)
		capturedSize := len(value.Map) == 0 || (len(value.Map) == 1 && strings.EqualFold(value.Static, "Object") && apexversion.AtLeast(vm.currentMethod.APIVersion, 62))
		return ok && capturedSize && strings.EqualFold(key, "String") && strings.EqualFold(element, "Integer") && strings.EqualFold(targetType, "Account")
	}
	element, ok := collectionElementType(sourceType)
	if !ok || collectionBase(sourceType) != "List" {
		return false
	}
	// Named reflection rows cover source helpers 53 through 55 with callers
	// 62/67/68. Primitive lists retain their label through both Object views.
	if strings.EqualFold(value.Static, "Object") && strings.EqualFold(targetType, "Account") &&
		apexversion.AtLeast(vm.currentMethod.APIVersion, 53) && !apexversion.AtLeast(vm.currentMethod.APIVersion, 56) {
		viewElement, viewOK := collectionElementType(value.Type)
		if collectionBase(value.Type) == "List" && viewOK && strings.EqualFold(viewElement, "Object") {
			for _, primitive := range []string{"Id", "Decimal", "Date", "Datetime", "Boolean"} {
				if strings.EqualFold(element, primitive) {
					return true
				}
			}
		}
	}
	// The native record-cast matrix retains runtime list identity through
	// Object parameters. Account and installed custom-object targets have
	// different captured cardinalities/views; do not unwrap their elements.
	if strings.EqualFold(value.Static, "Object") && apexversion.AtLeast(vm.currentMethod.APIVersion, 54) {
		viewElement, viewOK := collectionElementType(value.Type)
		if collectionBase(value.Type) == "List" && viewOK {
			objectView := strings.EqualFold(viewElement, "Object")
			if strings.EqualFold(element, "Opportunity") {
				typedView := strings.EqualFold(viewElement, "Opportunity")
				if strings.EqualFold(targetType, "Account") {
					return objectView || (typedView && len(value.List) != 1) ||
						(strings.EqualFold(viewElement, "SObject") && len(value.List) == 0)
				}
				// HV*_TARGET_0/2 and IL_*_TARGET_0/2: only ordinary
				// custom-object schema targets, not other custom entity kinds.
				if vm.Org != nil {
					objectName, found := vm.resolveObjectName(targetType)
					if found && strings.HasSuffix(strings.ToLower(objectName), "__c") && !storage.IsCustomSettingDefinition(vm.Org.Objects[objectName].Definition) {
						return len(value.List) != 1 && (objectView || typedView)
					}
				}
			}
			// M*_EMPTY_* captures these additional labels only through the
			// Object-list helper view, empty, with matching API 62/67/68.
			if strings.EqualFold(targetType, "Account") && objectView && len(value.List) == 0 && apexversion.AtLeast(vm.currentMethod.APIVersion, 62) {
				for _, captured := range []string{"Contact", "Long", "Decimal", "Double"} {
					if strings.EqualFold(element, captured) {
						return true
					}
				}
			}
		}
	}
	// CL044 has only the empty wrong-target twin; retain other cardinalities.
	if strings.EqualFold(element, "Account") && strings.EqualFold(targetType, "Contact") {
		return len(value.List) == 0
	}
	for _, pair := range [][2]string{
		{"String", "Account"}, {"Integer", "Account"}, {"Object", "Account"},
		{"Account", "Account"}, {"SObject", "Account"},
		{"Account", "SObject"}, {"SObject", "SObject"},
	} {
		if strings.EqualFold(element, pair[0]) && strings.EqualFold(targetType, pair[1]) {
			return true
		}
	}
	return false
}

// Object-element lists do not become String or Integer lists by inspecting
// their elements. Typed lists viewed through Object-element parameters retain
// their original runtime element type, and SObject list views stay separate.
func (vm *VM) listSObjectCastRejectsObjectElements(typeName string, value Value) bool {
	if value.Kind != ValueList {
		return false
	}
	if !strings.EqualFold(value.Static, "Object") {
		staticElement, ok := collectionElementType(value.Static)
		if !vm.collectionCastReflectionEnabled() || collectionBase(value.Static) != "List" || !ok || !strings.EqualFold(staticElement, "Object") {
			return false
		}
	}
	sourceType := value.Runtime
	if collectionBase(sourceType) != "List" {
		sourceType = value.Type
	}
	sourceElement, sourceOK := collectionElementType(sourceType)
	targetType := vm.resolveAssignableTargetType(typeName)
	targetElement, targetOK := collectionElementType(targetType)
	if collectionBase(targetType) != "List" || !sourceOK || !targetOK || !strings.EqualFold(sourceElement, "Object") {
		return false
	}
	return strings.EqualFold(targetElement, "String") || strings.EqualFold(targetElement, "Integer")
}
