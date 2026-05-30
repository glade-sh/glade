package vm

import (
	"math"
	"strings"
)

func (vm *VM) lookupField(typeName, fieldName string) (Field, string, bool) {
	return vm.lookupFieldForReceiver(typeName, fieldName, false)
}
func (vm *VM) lookupFieldForReceiver(typeName, fieldName string, preferDependency bool) (Field, string, bool) {
	for typeName != "" {
		class, ok := vm.lookupClass(typeName)
		if !ok {
			return Field{}, "", false
		}
		if field, ok := vm.lookupFieldInMapWithOptions(class.Fields, fieldName, preferDependency); ok {
			return field, runtimeClassName(class), true
		}
		typeName = vm.resolvedSuperClassName(class)
	}
	return Field{}, "", false
}
func (vm *VM) lookupReceiverField(typeName, fieldName string) (Field, string, bool) {
	if vm.currentClass != "" && (strings.EqualFold(typeName, vm.currentClass) || vm.isSubclass(typeName, vm.currentClass)) {
		if class, ok := vm.Classes[vm.currentClass]; ok {
			if field, ok := vm.lookupFieldInMapWithOptions(class.Fields, fieldName, vm.currentMethod.Dependency); ok {
				return field, runtimeClassName(class), true
			}
		}
	}
	return vm.lookupFieldForReceiver(typeName, fieldName, vm.currentMethod.Dependency)
}
func (vm *VM) lookupStaticField(typeName, fieldName string) (Field, string, bool) {
	return vm.lookupStaticFieldForReceiver(typeName, fieldName, false)
}
func (vm *VM) lookupStaticFieldForReceiver(typeName, fieldName string, preferDependency bool) (Field, string, bool) {
	for search := typeName; search != ""; {
		for current := search; current != ""; {
			class, ok := vm.lookupClass(current)
			if !ok {
				break
			}
			if field, ok := vm.lookupFieldInMapWithOptions(class.StaticFields, fieldName, preferDependency); ok {
				if field.Value.Kind == "" {
					field.Value = defaultValue(field.Type, field.InitialValue)
				}
				return field, runtimeClassName(class), true
			}
			current = vm.resolvedSuperClassName(class)
		}
		dot := strings.LastIndex(search, ".")
		if dot < 0 {
			break
		}
		search = search[:dot]
	}
	return Field{}, "", false
}
func (vm *VM) lookupFieldInMap(fields map[string]Field, fieldName string) (Field, bool) {
	return vm.lookupFieldInMapWithOptions(fields, fieldName, false)
}
func (vm *VM) lookupFieldInMapWithOptions(fields map[string]Field, fieldName string, preferDependency bool) (Field, bool) {
	requested := strings.TrimSpace(fieldName)
	var best Field
	bestScore := -1
	found := false
	for candidate, field := range fields {
		if !strings.EqualFold(candidate, requested) && !strings.EqualFold(strings.TrimSpace(field.Name), requested) {
			continue
		}
		if field.Name == "" {
			field.Name = candidate
		}
		field.StorageName = candidate
		score := vm.fieldProvenanceScore(field)
		if candidate == requested || field.Name == requested {
			score += 16
		}
		if dependencyPreferenceRank(fieldOrigin(field), preferDependency) == 0 {
			score += 32
		}
		if !found || score > bestScore {
			best = field
			bestScore = score
			found = true
		}
	}
	return best, found
}
func (vm *VM) fieldProvenanceScore(field Field) int {
	score := 0
	if fieldOrigin(field) == symbolOriginDependency {
		score += 8
	}
	if field.Type != "" {
		score += 2
	}
	return score
}
func staticFieldStorageName(requested string, field Field) string {
	if strings.TrimSpace(field.StorageName) != "" {
		return field.StorageName
	}
	if strings.TrimSpace(field.Name) != "" {
		return field.Name
	}
	return requested
}
func (vm *VM) staticFieldWritebackKey(owner, requested string, field Field) string {
	class, ok := vm.Classes[owner]
	if ok {
		if storageName := strings.TrimSpace(field.StorageName); storageName != "" {
			if _, exists := class.StaticFields[storageName]; exists {
				return storageName
			}
		}
		normalized := strings.TrimSpace(requested)
		for key := range class.StaticFields {
			if strings.EqualFold(strings.TrimSpace(key), normalized) {
				return key
			}
		}
	}
	return staticFieldStorageName(requested, field)
}
func builtinStaticField(typeName, fieldName string) (Value, bool) {
	if value, ok := builtinEnumStaticValue(typeName, fieldName); ok {
		return value, true
	}
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		typeName = rest
	}
	switch {
	case strings.EqualFold(typeName, "Math"):
		switch {
		case strings.EqualFold(fieldName, "E"):
			return Decimal(math.E), true
		case strings.EqualFold(fieldName, "PI"):
			return Decimal(math.Pi), true
		}
	}
	switch typeName {
	case "Math":
		switch fieldName {
		case "E":
			return Decimal(math.E), true
		case "PI":
			return Decimal(math.Pi), true
		}
	case "Integer":
		switch fieldName {
		case "MAX_VALUE":
			return Int(math.MaxInt32), true
		case "MIN_VALUE":
			return Int(math.MinInt32), true
		}
	case "Long":
		switch fieldName {
		case "MAX_VALUE":
			return Int(math.MaxInt64), true
		case "MIN_VALUE":
			return Int(math.MinInt64), true
		}
	case "Pattern":
		switch fieldName {
		case "UNIX_LINES":
			return Int(patternFlagUnixLines), true
		case "CASE_INSENSITIVE":
			return Int(patternFlagCaseInsensitive), true
		case "COMMENTS":
			return Int(patternFlagComments), true
		case "MULTILINE":
			return Int(patternFlagMultiline), true
		case "LITERAL":
			return Int(patternFlagLiteral), true
		case "DOTALL":
			return Int(patternFlagDotall), true
		case "UNICODE_CASE":
			return Int(patternFlagUnicodeCase), true
		case "CANON_EQ":
			return Int(patternFlagCanonEq), true
		case "UNICODE_CHARACTER_CLASS":
			return Int(patternFlagUnicodeCharacterClass), true
		}
	case "Dom.XmlNodeType":
		return domXmlNodeTypeValue(fieldName)
	case "Canvas.Test":
		switch fieldName {
		case "KEY_CANVAS_URL":
			return String("canvasUrl"), true
		case "KEY_DEVELOPER_NAME":
			return String("developerName"), true
		case "KEY_DISPLAY_LOCATION":
			return String("displayLocation"), true
		case "KEY_LOCATION_URL":
			return String("locationUrl"), true
		case "KEY_NAME":
			return String("name"), true
		case "KEY_NAMESPACE":
			return String("namespace"), true
		case "KEY_SUB_LOCATION":
			return String("sublocation"), true
		case "KEY_VERSION":
			return String("version"), true
		}
	}
	return Null, false
}
