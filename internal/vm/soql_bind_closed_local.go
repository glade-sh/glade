package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// dynamic_bind_closed_scope_native.json retains departed if-block String
// locals for dynamic SOQL. dynamic_bind_closed_id_set_native.json also observes
// non-null Set<Id> locals for countQuery at source APIs 60/62/67/68 in plain
// methods. Other types, contexts and routes keep their existing lookup.
func (vm *VM) rememberClosedSOQLLocal(name, typeName string, value Value) {
	typ := canonicalRuntimePlatformType(typeName)
	retain := strings.EqualFold(typ, "String") && (value.Kind == ValueString || value.Kind == ValueNull)
	if !retain && strings.EqualFold(typ, "Set<Id>") && value.Kind == ValueSet &&
		!vm.currentTrigger && vm.currentCallerNamespace() == "" && (vm.Org == nil || vm.Org.Namespace == "") {
		api, _ := apexversion.Major(vm.currentSourceExecutionPolicy().APIVersion)
		switch api {
		case 60, 62, 67, 68:
			// Preserve an earlier retained String binding on uncaptured routes.
			_, exists := vm.closedSOQLLocals[strings.ToLower(name)]
			retain = !exists
		}
	}
	if !retain {
		return
	}
	if vm.closedSOQLLocals == nil {
		vm.closedSOQLLocals = make(map[string]Value)
	}
	vm.closedSOQLLocals[strings.ToLower(name)] = value
}

func (vm *VM) lookupSOQLBind(name string) (Value, error) {
	value, err := vm.lookup(name)
	if err == nil || vm.dynamicSOQLBindRoute == "" || !strings.HasPrefix(err.Error(), "unknown variable ") {
		return value, err
	}
	if departed, ok := vm.closedSOQLLocals[strings.ToLower(name)]; ok {
		if departed.Kind != ValueSet || vm.dynamicSOQLBindRoute == "Database.countQuery" {
			return departed, nil
		}
	}
	return value, err
}
