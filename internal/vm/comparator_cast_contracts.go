package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// Concrete comparator casts preserve the implementing class and its ordinary
// compare method. Additional Object-view pairs stay within a bounded API range.
func (vm *VM) comparatorClassCastPair(class Class, from, to string) bool {
	version := vm.currentMethod.APIVersion
	if version == "" && vm.Org != nil {
		version = vm.Org.APIVersion
	}
	pair := strings.ToLower(from + ":" + to)
	initializerPair := false
	switch pair {
	case "datetime:object", "datetime:integer", "long:object", "double:object",
		"decimal:object", "time:object", "integer:object", "boolean:object":
		initializerPair = true
	case "sobject:object":
		// Named API61 rows extend the existing API62..68 SObject view.
		initializerPair = true
	case "opportunity:object":
		// Only the API61 concrete-record view is captured. Preserve the
		// existing rejection at unobserved newer source versions.
		initializerPair = !apexversion.AtLeast(version, 62)
	}
	minimumAPI := 62
	if initializerPair {
		// Assignment stays separate; captured instanceof shares these guards.
		// Boolean at 61 is bounded by the uniformly accepted primitive casts
		// and the exact Boolean/Object-parameter contract captured at 62..68.
		minimumAPI = 61
	}
	if !apexversion.AtLeast(version, minimumAPI) || apexversion.AtLeast(version, 69) ||
		class.Namespace != "" || strings.Count(class.Name, ".") > 1 ||
		class.IsInterface || class.IsAbstract || vm.resolvedSuperClassName(class) != "" {
		return false
	}
	if strings.EqualFold(from, "Boolean") && !apexversion.AtLeast(version, 62) {
		// The inferred API61 shape uses Object parameters. Do not extend that
		// inference to a Boolean-parameter implementation without native rows.
		widened := false
		for _, method := range class.Methods {
			if !method.IsStatic && !methodHasModifier(method.Modifiers, "abstract") &&
				strings.EqualFold(apexMethodMemberName(method.Name), "compare") && len(method.Params) == 2 &&
				strings.EqualFold(canonicalRuntimePlatformType(vm.resolveTypeNameInClass(class.Name, method.Params[0].Type)), "Object") &&
				strings.EqualFold(canonicalRuntimePlatformType(vm.resolveTypeNameInClass(class.Name, method.Params[1].Type)), "Object") {
				widened = true
				break
			}
		}
		if !widened {
			return false
		}
	}
	argument := vm.resolveTypeNameInClass(class.Name, from)
	if subject, found := vm.lookupClass(argument); found && !subject.PlatformType {
		return apexversion.AtLeast(version, 62) && strings.EqualFold(to, "Object") && comparatorEmptySiblingArgument(class, subject)
	}
	if initializerPair {
		return true
	}
	return false
}

// User-class arguments are limited to empty immediate siblings of the implementor.
func comparatorEmptySiblingArgument(implementor, argument Class) bool {
	owner, _, inner := strings.Cut(implementor.Name, ".")
	argumentOwner, _, sibling := strings.Cut(argument.Name, ".")
	return inner && sibling && strings.EqualFold(owner, argumentOwner) &&
		strings.Count(argument.Name, ".") == 1 && argument.Namespace == "" &&
		!argument.IsInterface && !argument.IsAbstract && argument.SuperClass == "" &&
		len(argument.Interfaces) == 0 && len(argument.Fields) == 0 && len(argument.StaticFields) == 0 &&
		len(argument.Methods) == 0 && len(argument.Constructors) == 0 &&
		len(argument.StaticInitializers) == 0 && len(argument.InstanceInitializers) == 0 && len(argument.EnumValues) == 0
}
