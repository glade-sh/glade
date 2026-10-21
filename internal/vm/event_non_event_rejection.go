package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// A single new Account with one string Name in a genuine List<SObject> is the
// captured non-event rejection. Its exception escapes Apex catch(Exception).
func (vm *VM) capturedNonEventAccountList(list Value) bool {
	if vm.Org == nil || vm.Org.Namespace != "" || list.Kind != ValueList || len(list.List) != 1 {
		return false
	}
	version := vm.currentMethod.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	if !apexversion.AtLeast(version, 62) || apexversion.AtLeast(version, 69) {
		return false
	}
	element, _ := collectionElementType(runtimeObjectType(list))
	if !strings.EqualFold(element, "SObject") {
		return false
	}
	record := list.List[0]
	if record.Kind != ValueObject || record.projectClass || !strings.EqualFold(runtimeObjectType(record), "Account") {
		return false
	}
	if _, hasID := sObjectIDFromValue(record); hasID {
		return false
	}
	name := false
	for field, value := range record.Fields {
		if isInternalSObjectField(field) || value.Kind == ValueNull && !isExplicitSObjectField(record, field) {
			continue
		}
		// Schema defaults are not additional constructor inputs. An explicit
		// write removes default provenance and must retain the prior path.
		if isDefaultedSObjectField(record, field) && !isExplicitSObjectField(record, field) {
			continue
		}
		if !strings.EqualFold(field, "Name") || value.Kind != ValueString || value.Text == "" {
			return false
		}
		name = true
	}
	return name
}
