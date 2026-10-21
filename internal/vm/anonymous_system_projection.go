package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// The observed anonymous dynamic query rejects explicit SYSTEM_MODE before
// executing its identity-scoped Case projection. Named queries remain valid.
// Other query and access-level shapes retain their existing paths.
func (vm *VM) anonymousSystemProjectionDenied(query soql.Query) bool {
	major, known := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
	if !known || major != 62 && major != 67 && major != 68 ||
		!strings.EqualFold(query.Object, "Case") || len(query.Fields) != 4 || len(query.Order) != 1 ||
		query.Order[0].Desc || !strings.EqualFold(query.Order[0].Nulls, "FIRST") ||
		!strings.EqualFold(query.Order[0].Field, query.Fields[0]) ||
		!strings.EqualFold(query.Fields[1], "Id") || !vm.userModeInaccessibleExpandedIDQuery(query) {
		return false
	}
	object, ok := vm.Org.Objects["Case"]
	if !ok {
		return false
	}
	for position, fieldType := range map[int]storage.FieldType{0: storage.FieldString, 2: storage.FieldPicklist, 3: storage.FieldPicklist} {
		name, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, query.Fields[position])
		if !ok || object.Definition.Fields[name].Type != fieldType {
			return false
		}
	}
	return true
}
