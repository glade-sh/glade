package vm

import (
	"fmt"
	"strings"
)

func (vm *VM) enclosingClassLiteral(name string) (Value, bool, error) {
	if _, generic := genericTypeArgs(name); generic {
		return Null, false, nil
	}
	// Platform types need not have registered runtime classes. Their complete
	// names still take precedence over a source class sharing the prefix.
	if generatedPlatformTypeName(name) {
		return Null, false, nil
	}
	for dot := strings.LastIndexByte(name, '.'); dot > 0; dot = strings.LastIndexByte(name[:dot], '.') {
		prefix := name[:dot]
		resolved, ok := vm.resolveClassName(prefix)
		if !ok {
			continue
		}
		tokenName := resolved
		if class, ok := vm.lookupClass(resolved); ok {
			if class.PlatformType && !sObjectValueType(resolved) {
				return Null, false, nil
			}
			next, _, _ := strings.Cut(name[dot+1:], ".")
			if _, found := vm.lookupFieldInMap(class.StaticFields, next); found {
				return Null, true, fmt.Errorf("Invalid type: %s", name)
			}
			for _, constant := range class.EnumValues {
				if strings.EqualFold(constant, next) {
					return Null, true, fmt.Errorf("Invalid type: %s", name)
				}
			}
			tokenName = vm.classTypeToken(class)
		}
		token := vm.markReflectionTypeToken(Value{Kind: ValueObject, Type: "Type", Text: tokenName}, tokenName)
		return vm.lexicalClassLiteralToken(prefix, token), true, nil
	}
	return Null, false, nil
}
