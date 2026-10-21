package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

// Native C04-01..04/11..12 at API 62/67/68 on both routes distinguish
// forwarded concrete empty/populated singletons and populated pairs from
// freshly allocated List<SObject> values. Preserve that allocation provenance.
func (vm *VM) capturedImmediateEventList(list Value) bool {
	if vm.Org == nil || vm.Org.Namespace != "" || list.Kind != ValueList || (len(list.List) < 1 || len(list.List) > 2) {
		return false
	}
	version := vm.currentMethod.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	if !apexversion.AtLeast(version, 62) || apexversion.AtLeast(version, 69) {
		return false
	}
	declared, _ := collectionElementType(list.Type)
	runtime, ok := collectionElementType(list.Runtime)
	if !strings.EqualFold(declared, "SObject") || !ok || strings.EqualFold(runtime, "SObject") {
		return false
	}
	name, found := storage.ResolveObjectName(*vm.Org, runtime)
	if !found || !vm.hasPlatformEventMetadata(name) || !strings.EqualFold(vm.Org.Objects[name].Definition.Metadata["publishBehavior"], "PublishImmediately") {
		return false
	}
	for _, record := range list.List {
		if record.Kind != ValueObject || record.projectClass || record.classInstance {
			return false
		}
		if _, hasID := sObjectIDFromValue(record); hasID {
			return false
		}
		actual, actualFound := storage.ResolveObjectName(*vm.Org, runtimeObjectType(record))
		if !actualFound || name != actual {
			return false
		}
	}
	return true
}

// Native C04-10 at API 62/67/68 on both routes charges one immediate publish
// before rejecting a direct concrete pair containing the same empty event.
// Other duplicate shapes retain their existing rejection/accounting boundary.
func (vm *VM) capturedDuplicateImmediateEventPair(list Value) bool {
	if vm.Org == nil || vm.Org.Namespace != "" || list.Kind != ValueList || len(list.List) != 2 {
		return false
	}
	version := vm.currentMethod.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	major, ok := apexversion.Major(version)
	if !ok || (major != 62 && major != 67 && major != 68) {
		return false
	}
	element, ok := collectionElementType(list.Type)
	if !ok || strings.EqualFold(element, "SObject") || strings.EqualFold(element, "Object") {
		return false
	}
	name, found := storage.ResolveObjectName(*vm.Org, element)
	if !found || !vm.hasPlatformEventMetadata(name) ||
		!strings.EqualFold(vm.Org.Objects[name].Definition.Metadata["publishBehavior"], "PublishImmediately") {
		return false
	}
	if list.List[0].Ref == 0 || list.List[0].Ref != list.List[1].Ref {
		return false
	}
	for _, record := range list.List {
		if record.Kind != ValueObject || record.projectClass || record.classInstance {
			return false
		}
		if _, hasID := sObjectIDFromValue(record); hasID {
			return false
		}
		actual, found := storage.ResolveObjectName(*vm.Org, runtimeObjectType(record))
		if !found || name != actual {
			return false
		}
		for field, value := range record.Fields {
			if !isInternalSObjectField(field) && (value.Kind != ValueNull || isExplicitSObjectField(record, field)) {
				return false
			}
		}
	}
	return true
}
