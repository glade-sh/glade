package vm

import (
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) sortDescribeFieldNames(objectName string, definition storage.ObjectDefinition, names []string) {
	defer prioritizeAccountCompoundName(objectName, definition, names)
	sort.Strings(names)
	if vm.sortCapturedTaskDescribeFieldNames(objectName, names) {
		return
	}
	if vm == nil || vm.Org == nil || vm.Org.Namespace == "" || !strings.EqualFold(objectName, "Account") {
		return
	}
	// PENDING-ROWS: qualification may preserve the earlier local-key traversal
	// policy for owned Account extensions. Native describe ordinals are unsettled.
	// Keep this candidate outside ordinary collections and mixed-owner metadata.
	ownedReference := false
	for key, field := range definition.Fields {
		fieldName := field.APIName
		if strings.TrimSpace(fieldName) == "" {
			fieldName = key
		}
		if fieldName != key {
			return
		}
		prefix, qualified := namespaceTokenPrefix(key)
		if !qualified {
			continue
		}
		if !strings.EqualFold(prefix, vm.Org.Namespace) {
			return
		}
		if field.Type == storage.FieldReference {
			ownedReference = true
		}
	}
	if !ownedReference {
		return
	}
	sort.SliceStable(names, func(i, j int) bool {
		return storage.StripNamespaceToken(vm.Org.Namespace, names[i]) < storage.StripNamespaceToken(vm.Org.Namespace, names[j])
	})
}

// Native Account field maps visit Name before the person-name components that
// also report isNameField. Keep all other fields in their existing relative order.
func prioritizeAccountCompoundName(objectName string, definition storage.ObjectDefinition, names []string) {
	if !strings.EqualFold(objectName, "Account") {
		return
	}
	if enabled, known := storage.StandardFieldNameFlag(definition, "FirstName"); !known || !enabled {
		return
	}
	nameIndex, firstComponent := -1, len(names)
	for i, name := range names {
		switch name {
		case "Name":
			nameIndex = i
		case "FirstName", "LastName":
			if i < firstComponent {
				firstComponent = i
			}
		}
	}
	if nameIndex < 0 || nameIndex <= firstComponent {
		return
	}
	copy(names[firstComponent+1:nameIndex+1], names[firstComponent:nameIndex])
	names[firstComponent] = "Name"
}
