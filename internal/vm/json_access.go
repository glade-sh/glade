package vm

import (
	"errors"
	"regexp"
	"strings"
)

var jsonAccessProperty = regexp.MustCompile(`(?i)(serializable|deserializable)='([^']*)'`)

// Runtime classes retain normalized annotation modifier text. Unannotated
// classes and platform values keep their existing serialization paths.
func (vm *VM) jsonAccessAllowed(typeName, property string) bool {
	class, ok := vm.lookupClass(typeName)
	// C235-C242 and Q001-Q006 cover unpackaged declarations and callers.
	// Installed-package and cross-namespace policies need their own capture.
	if !ok {
		return true
	}
	// Q003/Q005/Q006 preserve self calls for anonymous and nested declarations.
	// The persisted top-level native controls reject self calls at APIs 62/67.
	// Nested persisted self calls retain their existing behavior until captured.
	selfAccess := strings.EqualFold(class.Name, vm.currentClass) &&
		(class.Nested || methodHasModifier(class.Modifiers, AnonymousClassModifier))
	for _, modifier := range class.Modifiers {
		if !strings.EqualFold(baseModifierName(modifier), "JsonAccess") {
			continue
		}
		for _, match := range jsonAccessProperty.FindAllStringSubmatch(modifier, -1) {
			if !strings.EqualFold(match[1], property) {
				continue
			}
			if strings.EqualFold(match[2], "never") {
				return selfAccess
			}
			// Keep uncaptured package policies on their existing paths. The
			// explicit 'never' policy above applies to every source namespace.
			if class.Dependency || class.Namespace != "" || vm.currentExecutionNamespace() != "" || strings.EqualFold(class.Name, vm.currentClass) {
				return true
			}
			switch strings.ToLower(match[2]) {
			case "samepackage":
				// C241-C242: an unpackaged anonymous type has no package
				// identity. Installed-package exposure needs its own oracle.
				return false
			case "samenamespace":
				return strings.EqualFold(class.Namespace, vm.currentExecutionNamespace())
			default:
				return true
			}
		}
	}
	return true
}

type jsonAccessSerializationFailure struct{}

func (jsonAccessSerializationFailure) Error() string { return "Type cannot be serialized" }

func (failure jsonAccessSerializationFailure) MarshalJSON() ([]byte, error) {
	return nil, failure
}

func jsonAccessMarshalError(err error) error {
	var failure jsonAccessSerializationFailure
	if errors.As(err, &failure) {
		return failure
	}
	return err
}
