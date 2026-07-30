package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (vm *VM) lookup(name string) (Value, error) {
	if value, ok := vm.Globals[name]; ok {
		if value.Kind == ValueNull && value.Type == "" {
			value.Type = vm.VarTypes[name]
		}
		return value, nil
	}
	if value, ok := vm.lookupTriggerGlobal(name); ok {
		return value, nil
	}
	if actual, ok := vm.lookupGlobalName(name); ok {
		value := vm.Globals[actual]
		if value.Kind == ValueNull && value.Type == "" {
			value.Type = vm.VarTypes[actual]
		}
		return value, nil
	}
	if value, ok, err := vm.lookupRestContextField(name); ok || err != nil {
		return value, err
	}
	if className, memberName, ok := vm.splitClassMember(name); ok {
		if value, ok := builtinEnumStaticValue(className, memberName); ok {
			return value, nil
		}
	}
	if value, ok := metadataDeployStatusStaticValue(name); ok {
		return value, nil
	}
	if value, ok := metadataMetadataTypeStaticValue(name); ok {
		return value, nil
	}
	if value, ok := schemaSOAPTypeStaticValue(name); ok {
		return value, nil
	}
	if value, ok := schemaDisplayTypeStaticValue(name); ok {
		return value, nil
	}
	if labelName, ok := vm.systemLabelLookupName(name); ok {
		if value, ok := vm.lookupLabel(labelName); ok {
			return value, nil
		}
	}
	if strings.HasPrefix(name, "JSONToken.") {
		tokenName := strings.TrimPrefix(name, "JSONToken.")
		for _, jsonTokenName := range jsonTokenNames {
			if tokenName == jsonTokenName {
				return jsonTokenValue(tokenName), nil
			}
		}
	}
	if value, ok := apexPagesSeverityStaticValue(name); ok {
		return value, nil
	}
	if hasSuffixFold(name, ".class") {
		className := name[:len(name)-len(".class")]
		if exceptionName := exceptionTypeName(className); isBuiltinExceptionType(exceptionName) {
			return Value{Kind: ValueObject, Type: "Type", Text: "System." + exceptionName}, nil
		}
		if resolved, ok := vm.resolveClassName(className); ok {
			if class, ok := vm.lookupClass(resolved); ok {
				return Value{Kind: ValueObject, Type: "Type", Text: vm.classTypeToken(class)}, nil
			}
			return Value{Kind: ValueObject, Type: "Type", Text: resolved}, nil
		}
		return Value{Kind: ValueObject, Type: "Type", Text: className}, nil
	}
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		if strings.EqualFold(parts[0], "super") {
			if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
				return vm.lookupPath(this, parts[1:])
			}
		}
		if strings.EqualFold(parts[0], "this") {
			if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
				return vm.lookupThisPath(this, parts[1:])
			}
		}
		if root, ok := vm.Globals[parts[0]]; ok {
			return vm.lookupPath(root, parts[1:])
		}
		if actual, ok := vm.lookupGlobalName(parts[0]); ok {
			return vm.lookupPath(vm.Globals[actual], parts[1:])
		}
		if className, memberName, ok := vm.splitClassMember(name); ok {
			if class, ok := vm.lookupClass(className); ok && len(class.EnumValues) > 0 {
				if err := vm.ensureClassInitialized(class.Name); err != nil {
					return Null, err
				}
				for _, enumValue := range class.EnumValues {
					if strings.EqualFold(enumValue, memberName) {
						return Value{Kind: ValueObject, Type: class.Name, Text: enumValue}, nil
					}
				}
			}
		}
		if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
			if root, field, owner, ok := vm.lookupNestedInstanceFieldRoot(this, parts); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+field.Name, field.Modifiers); err != nil {
					return Null, err
				}
				if field.Getter != nil {
					value, err := vm.callGetter(owner, field, this)
					if err != nil {
						return Null, err
					}
					root = value
				}
				if len(parts) == 2 {
					return root, nil
				}
				return vm.lookupPath(root, parts[2:])
			}
			if root, field, owner, ok := vm.lookupThisFieldRoot(this, parts[0]); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+field.Name, field.Modifiers); err != nil {
					return Null, err
				}
				if field.Getter != nil && !(!field.HasSetter && root.Kind != ValueNull) {
					value, err := vm.callGetter(owner, field, this)
					if err != nil {
						return Null, err
					}
					root = value
				}
				return vm.lookupPath(root, parts[1:])
			}
		}
		if vm.currentClass != "" {
			if field, owner, ok := vm.lookupStaticFieldForReceiver(vm.currentClass, parts[0], vm.currentMethod.Dependency); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+parts[0], field.Modifiers); err != nil {
					return Null, err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return Null, err
				}
				field, _, _ = vm.lookupStaticFieldForReceiver(owner, parts[0], vm.currentMethod.Dependency)
				root := field.Value
				if field.Getter != nil {
					var err error
					root, err = vm.callGetter(owner, field, Null)
					if err != nil {
						return Null, err
					}
				}
				return vm.lookupPath(root, parts[1:])
			}
		}
		if token, ok := vm.lookupSObjectTypeToken(parts); ok {
			return token, nil
		}
		if len(parts) > 2 {
			if token, ok := vm.lookupSObjectTypeToken(parts[:2]); ok {
				return vm.lookupPath(token, parts[2:])
			}
		}
		if len(parts) > 3 {
			if token, ok := vm.lookupSObjectTypeToken(parts[:3]); ok {
				return vm.lookupPath(token, parts[3:])
			}
		}
		if className, memberName, ok := vm.splitClassMember(name); ok {
			preferDependency := vm.classMemberReferenceUsesExplicitNamespace(name, className)
			if field, owner, ok := vm.lookupStaticFieldForReceiver(className, memberName, preferDependency); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+memberName, field.Modifiers); err != nil {
					return Null, err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return Null, err
				}
				field, _, _ = vm.lookupStaticFieldForReceiver(owner, memberName, preferDependency)
				if field.Getter != nil {
					return vm.callGetter(owner, field, Null)
				}
				return field.Value, nil
			}
		}
		if token, ok := vm.lookupSObjectFieldToken(parts); ok {
			return token, nil
		}
		if len(parts) == 2 {
			if strings.EqualFold(parts[0], "Page") {
				pageName := parts[1]
				if vm.pageReferences != nil {
					registered, ok := vm.pageReferences[strings.ToLower(pageName)]
					if !ok {
						return Null, fmt.Errorf("unknown Visualforce page Page.%s", pageName)
					}
					pageName = registered
				}
				return newPageTokenReference("/apex/" + pageName), nil
			}
			if value, ok := builtinStaticField(parts[0], parts[1]); ok {
				return value, nil
			}
		}
		if len(parts) > 2 {
			preferDependency := vm.currentMethod.Dependency || vm.classMemberReferenceUsesExplicitNamespace(strings.Join(parts[:2], "."), parts[0])
			if field, owner, ok := vm.lookupStaticFieldForReceiver(parts[0], parts[1], preferDependency); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+parts[1], field.Modifiers); err != nil {
					return Null, err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return Null, err
				}
				field, _, _ = vm.lookupStaticFieldForReceiver(owner, parts[1], preferDependency)
				root := field.Value
				if field.Getter != nil {
					var err error
					root, err = vm.callGetter(owner, field, Null)
					if err != nil {
						return Null, err
					}
				}
				return vm.lookupPath(root, parts[2:])
			}
		}
		if className, memberName, ok := vm.splitClassMember(name); ok {
			if value, ok := builtinStaticField(className, memberName); ok {
				return value, nil
			}
			if value, ok := vm.generatedPlatformStaticFieldValue(className, memberName); ok {
				return value, nil
			}
			if class, ok := vm.lookupClass(className); ok {
				if err := vm.ensureClassInitialized(class.Name); err != nil {
					return Null, err
				}
				for _, enumValue := range class.EnumValues {
					if strings.EqualFold(enumValue, memberName) {
						return Value{Kind: ValueObject, Type: class.Name, Text: enumValue}, nil
					}
				}
			}
			if !strings.Contains(className, ".") {
				suffix := "." + className
				for _, class := range vm.Classes {
					if !strings.HasSuffix(class.Name, suffix) || len(class.EnumValues) == 0 {
						continue
					}
					if err := vm.ensureClassInitialized(class.Name); err != nil {
						return Null, err
					}
					for _, enumValue := range class.EnumValues {
						if strings.EqualFold(enumValue, memberName) {
							return Value{Kind: ValueObject, Type: class.Name, Text: enumValue}, nil
						}
					}
				}
			}
			if dot := strings.IndexByte(memberName, '.'); dot > 0 {
				nestedEnumName := className + "." + memberName[:dot]
				nestedMemberName := memberName[dot+1:]
				nestedCandidates := []string{nestedEnumName, memberName[:dot]}
				for _, candidate := range nestedCandidates {
					if class, ok := vm.lookupClass(candidate); ok {
						if err := vm.ensureClassInitialized(class.Name); err != nil {
							return Null, err
						}
						for _, enumValue := range class.EnumValues {
							if strings.EqualFold(enumValue, nestedMemberName) {
								return Value{Kind: ValueObject, Type: class.Name, Text: enumValue}, nil
							}
						}
					}
				}
			}
		}
	}
	if len(parts) > 2 {
		for split := len(parts) - 2; split >= 1; split-- {
			className := strings.Join(parts[:split], ".")
			fieldName := parts[split]
			preferDependency := vm.currentMethod.Dependency || vm.classMemberReferenceUsesExplicitNamespace(strings.Join(parts[:split+1], "."), className)
			field, owner, ok := vm.lookupStaticFieldForReceiver(className, fieldName, preferDependency)
			if !ok {
				continue
			}
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+fieldName, field.Modifiers); err != nil {
				return Null, err
			}
			if err := vm.ensureClassInitialized(owner); err != nil {
				return Null, err
			}
			field, _, _ = vm.lookupStaticFieldForReceiver(owner, fieldName, preferDependency)
			root := field.Value
			if field.Getter != nil {
				var err error
				root, err = vm.callGetter(owner, field, Null)
				if err != nil {
					return Null, err
				}
			}
			return vm.lookupPath(root, parts[split+1:])
		}
	}
	if len(parts) > 1 {
		if value, ok := managedNestedEnumLiteralValue(parts); ok {
			return value, nil
		}
	}
	if len(parts) == 3 {
		if token, ok := vm.lookupSObjectFieldToken(parts); ok {
			return token, nil
		}
	}
	if len(parts) == 3 && apexIdentifierStartsUpper(parts[0]) && apexIdentifierStartsUpper(parts[1]) && apexIdentifierStartsUpper(parts[2]) {
		return Value{Kind: ValueObject, Type: parts[0] + "." + parts[1], Text: parts[2]}, nil
	}
	if value, ok, err := vm.lookupCurrentClassStaticField(name); ok || err != nil {
		return value, err
	}
	if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
		if value, ok := selectorBindParamValue(this, name); ok {
			return value, nil
		}
		if actualName, value, ok := objectFieldValue(this, name); ok {
			if field, owner, ok := vm.lookupReceiverField(this.Type, actualName); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+actualName, field.Modifiers); err != nil {
					return Null, err
				}
				if field.Getter != nil {
					if vm.isCurrentGetter(field.Getter) {
						return vm.currentGetterStoredValue(owner, field, value), nil
					}
					return vm.callGetter(owner, field, this)
				}
			}
			return value, nil
		}
		if field, owner, ok := vm.lookupReceiverField(this.Type, name); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name, field.Modifiers); err != nil {
				return Null, err
			}
			if field.Getter != nil {
				return vm.callGetter(owner, field, this)
			}
			return defaultValue(field.Type, field.InitialValue), nil
		}
	}
	for _, contextClass := range vm.lookupContextClasses() {
		if field, owner, ok := vm.lookupStaticFieldForReceiver(contextClass, name, vm.currentMethod.Dependency); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name, field.Modifiers); err != nil {
				return Null, err
			}
			if err := vm.ensureClassInitialized(owner); err != nil {
				return Null, err
			}
			field, _, _ = vm.lookupStaticFieldForReceiver(owner, name, vm.currentMethod.Dependency)
			if field.Getter != nil {
				return vm.callGetter(owner, field, Null)
			}
			return field.Value, nil
		}
	}
	return Null, fmt.Errorf("unknown variable %q", name)
}

func (vm *VM) lookupCurrentClassStaticField(name string) (Value, bool, error) {
	if vm.currentClass == "" {
		return Null, false, nil
	}
	class, ok := vm.lookupClass(vm.currentClass)
	if !ok {
		return Null, false, nil
	}
	directField, ok := vm.lookupFieldInMapWithOptions(class.StaticFields, name, vm.currentMethod.Dependency)
	if !ok {
		return Null, false, nil
	}
	owner := runtimeClassName(class)
	if err := vm.checkMemberAccess(owner, directField.Access, owner+"."+name, directField.Modifiers); err != nil {
		return Null, true, err
	}
	if err := vm.ensureClassInitialized(owner); err != nil {
		return Null, true, err
	}
	field, _, found := vm.lookupStaticFieldForReceiver(owner, name, vm.currentMethod.Dependency)
	if !found {
		field = directField
		if field.Value.Kind == "" {
			field.Value = defaultValue(field.Type, field.InitialValue)
		}
	}
	if field.Getter != nil {
		value, err := vm.callGetter(owner, field, Null)
		return value, true, err
	}
	return field.Value, true, nil
}

func managedNestedEnumLiteralValue(parts []string) (Value, bool) {
	if len(parts) < 3 {
		return Null, false
	}
	literal := strings.TrimSpace(parts[len(parts)-1])
	if literal == "" || !apexIdentifierStartsUpper(literal) || strings.ToUpper(literal) != literal {
		return Null, false
	}
	typeName := strings.Join(parts[:len(parts)-1], ".")
	if !looksManagedQualifiedType(strings.Join(parts[:2], ".")) {
		return Null, false
	}
	return Value{Kind: ValueObject, Type: typeName, Text: literal}, true
}

func (vm *VM) lookupContextClasses() []string {
	candidates := make([]string, 0, 6)
	appendUnique := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		for _, existing := range candidates {
			if strings.EqualFold(existing, name) {
				return
			}
		}
		candidates = append(candidates, name)
	}
	appendContext := func(name string) {
		if ns := strings.TrimSpace(vm.currentNamespace); ns != "" && !strings.Contains(name, ".") {
			appendUnique(ns + "." + name)
		}
		appendUnique(name)
	}
	appendContext(vm.currentClass)
	appendContext(vm.currentMethod.ClassName)
	return candidates
}

func (vm *VM) classMemberReferenceUsesExplicitNamespace(name, className string) bool {
	parts := strings.Split(name, ".")
	if len(parts) < 3 {
		return false
	}
	classParts := strings.Split(className, ".")
	if len(classParts) > 1 && strings.EqualFold(parts[0], classParts[0]) {
		return true
	}
	namespace := vm.classNamespace(className)
	return namespace != "" && strings.EqualFold(parts[0], namespace)
}

func systemLabelName(name string) (string, bool) {
	parts := strings.Split(name, ".")
	if len(parts) == 2 && strings.EqualFold(parts[0], "Label") {
		return "Label." + parts[1], true
	}
	if len(parts) == 3 && strings.EqualFold(parts[0], "Label") {
		return "Label." + parts[1] + "." + parts[2], true
	}
	if len(parts) == 3 && strings.EqualFold(parts[0], "System") && strings.EqualFold(parts[1], "Label") {
		return "Label." + parts[2], true
	}
	if len(parts) == 4 && strings.EqualFold(parts[0], "System") && strings.EqualFold(parts[1], "Label") {
		return "Label." + parts[2] + "." + parts[3], true
	}
	return "", false
}

func (vm *VM) systemLabelLookupName(name string) (string, bool) {
	if labelName, ok := systemLabelName(name); ok {
		return labelName, true
	}
	namespace := strings.TrimSpace(vm.currentNamespace)
	prefix := namespace + "."
	if namespace != "" && len(name) > len(prefix) && hasPrefixFold(name, prefix) {
		return systemLabelName(name[len(prefix):])
	}
	return "", false
}

func selectorBindParamValue(this Value, name string) (Value, bool) {
	_, params, ok := objectFieldValue(this, "params")
	if !ok || params.Kind != ValueMap {
		return Null, false
	}
	if value, ok := params.Map[mapKey(String(name))]; ok {
		return value, true
	}
	for _, key := range params.MapKeys {
		if key.Kind == ValueString && strings.EqualFold(key.Text, name) {
			return params.Map[mapKey(key)], true
		}
	}
	return Null, false
}

func (vm *VM) lookupThisFieldRoot(this Value, name string) (Value, Field, string, bool) {
	if actualName, root, field, owner, ok := vm.lookupDeclaredThisFieldValue(this, name); ok {
		if field.Name == "" {
			field.Name = actualName
		}
		return root, field, owner, true
	}
	actualName, root, ok := objectFieldValue(this, name)
	if !ok {
		return Null, Field{}, "", false
	}
	field, owner, ok := vm.lookupReceiverField(this.Type, actualName)
	if !ok {
		field = Field{Name: actualName, Type: root.Type}
		owner = this.Type
	}
	if field.Name == "" {
		field.Name = actualName
	}
	return root, field, owner, true
}

func (vm *VM) lookupThisPath(this Value, parts []string) (Value, error) {
	if len(parts) == 0 {
		return this, nil
	}
	actualName, value, field, owner, ok := vm.lookupDeclaredThisFieldValue(this, parts[0])
	if !ok {
		return vm.lookupPath(this, parts)
	}
	if err := vm.checkMemberAccess(owner, field.Access, owner+"."+actualName, field.Modifiers); err != nil {
		return Null, err
	}
	if field.Getter != nil {
		if field.Getter.Name == vm.currentMethod.Name {
			value = vm.currentGetterStoredValue(owner, field, value)
		} else {
			var err error
			value, err = vm.callGetter(owner, field, this)
			if err != nil {
				return Null, err
			}
		}
	}
	if len(parts) == 1 {
		return value, nil
	}
	return vm.lookupPath(value, parts[1:])
}

func (vm *VM) lookupDeclaredThisFieldValue(this Value, name string) (string, Value, Field, string, bool) {
	if this.Kind != ValueObject {
		return "", Null, Field{}, "", false
	}
	className := vm.currentMethod.ClassName
	if className == "" {
		className = vm.currentClass
	}
	if className == "" || (!strings.EqualFold(this.Type, className) && !vm.isSubclass(this.Type, className)) {
		return "", Null, Field{}, "", false
	}
	field, owner, ok := vm.lookupField(className, name)
	if !ok {
		return "", Null, Field{}, "", false
	}
	actualName := field.Name
	if actualName == "" {
		actualName = name
	}
	if field.StorageName != "" {
		actualName = field.StorageName
	}
	if this.Fields != nil {
		if value, ok := this.Fields[actualName]; ok {
			return actualName, value, field, owner, true
		}
	}
	return actualName, defaultValue(field.Type, field.InitialValue), field, owner, true
}

func (vm *VM) lookupNestedInstanceFieldRoot(this Value, parts []string) (Value, Field, string, bool) {
	if len(parts) < 2 || vm.currentClass == "" || !strings.Contains(vm.currentClass, ".") {
		return Null, Field{}, "", false
	}
	outer := vm.currentClass[:strings.IndexByte(vm.currentClass, '.')]
	if !strings.EqualFold(parts[0], outer) {
		return Null, Field{}, "", false
	}
	return vm.lookupThisFieldRoot(this, parts[1])
}

func objectFieldValue(object Value, name string) (string, Value, bool) {
	if object.Kind != ValueObject || object.Fields == nil {
		return "", Null, false
	}
	if value, ok := object.Fields[name]; ok {
		value = coerceRawRecordTypeDefaultTokenRuntimeValue(name, value)
		if !isExplicitSObjectField(object, name) {
			if aliasName, aliasValue, aliasOK := explicitAliasObjectFieldValue(object, name); aliasOK {
				return aliasName, aliasValue, true
			}
		}
		if value.Kind == ValueNull {
			if isRelationshipNull(value) {
				return name, value, true
			}
			stripped := storage.StripAnyNamespaceToken(name)
			for candidate, alternate := range object.Fields {
				if candidate != name && strings.EqualFold(storage.StripAnyNamespaceToken(candidate), stripped) && alternate.Kind != ValueNull {
					return candidate, alternate, true
				}
			}
			if isExplicitSObjectField(object, name) {
				return name, value, true
			}
			for candidate, alternate := range object.Fields {
				if candidate != name && strings.EqualFold(candidate, name) && alternate.Kind != ValueNull {
					return candidate, alternate, true
				}
			}
		}
		return name, value, true
	}
	for candidate, value := range object.Fields {
		if strings.EqualFold(candidate, name) {
			value = coerceRawRecordTypeDefaultTokenRuntimeValue(candidate, value)
			return candidate, value, true
		}
	}
	stripped := storage.StripAnyNamespaceToken(name)
	var matchedName string
	var matchedValue Value
	matched := false
	for candidate, value := range object.Fields {
		if !strings.EqualFold(storage.StripAnyNamespaceToken(candidate), stripped) {
			continue
		}
		if matched {
			return "", Null, false
		}
		matchedName = candidate
		matchedValue = coerceRawRecordTypeDefaultTokenRuntimeValue(candidate, value)
		matched = true
	}
	if matched {
		return matchedName, matchedValue, true
	}
	return "", Null, false
}

func (vm *VM) setObjectFieldValue(object *Value, name string, value Value) {
	if object == nil {
		return
	}
	vm.markCollectionRefsEscaped(value)
	vm.setGraphFieldValue(object, name, value)
	if object.Kind != ValueObject || vm.isSObjectLikeType(object.Type) {
		return
	}
	for candidate := range object.Fields {
		if candidate != name && strings.EqualFold(candidate, name) {
			if vm.distinctExactReceiverFields(object.Type, name, candidate) {
				continue
			}
			object.Fields[candidate] = value
		}
	}
}

func (vm *VM) setGraphFieldValue(object *Value, name string, value Value) {
	if object == nil {
		return
	}
	vm.advanceAliasContainmentMutation()
	if object.Fields == nil {
		object.Fields = make(map[string]Value)
	}
	object.Fields[name] = value
}

func (vm *VM) setExplicitSObjectFieldValue(object *Value, name string, value Value) {
	vm.advanceAliasContainmentMutation()
	setExplicitSObjectField(object, name, value)
}

func (vm *VM) distinctExactReceiverFields(typeName, left, right string) bool {
	leftField, leftOwner, leftOK := vm.lookupExactReceiverField(typeName, left)
	rightField, rightOwner, rightOK := vm.lookupExactReceiverField(typeName, right)
	if !leftOK || !rightOK {
		return false
	}
	leftName := leftField.Name
	if leftName == "" {
		leftName = left
	}
	rightName := rightField.Name
	if rightName == "" {
		rightName = right
	}
	if strings.EqualFold(leftName, rightName) && strings.EqualFold(leftField.Type, rightField.Type) {
		return false
	}
	return leftOwner != rightOwner || leftName != rightName
}

func (vm *VM) lookupExactReceiverField(typeName, fieldName string) (Field, string, bool) {
	for typeName != "" {
		class, ok := vm.lookupClass(typeName)
		if !ok {
			return Field{}, "", false
		}
		for candidate, field := range class.Fields {
			if candidate != fieldName && field.Name != fieldName {
				continue
			}
			if field.Name == "" {
				field.Name = candidate
			}
			field.StorageName = candidate
			return field, runtimeClassName(class), true
		}
		typeName = vm.resolvedSuperClassName(class)
	}
	return Field{}, "", false
}

func explicitAliasObjectFieldValue(object Value, name string) (string, Value, bool) {
	stripped := storage.StripAnyNamespaceToken(name)
	for candidate, value := range object.Fields {
		if candidate == name || !isExplicitSObjectField(object, candidate) {
			continue
		}
		if strings.EqualFold(candidate, name) || strings.EqualFold(storage.StripAnyNamespaceToken(candidate), stripped) {
			return candidate, value, true
		}
	}
	return "", Null, false
}

const (
	safeNavigationNullRuntime      = "__glade_safe_navigation_null"
	relationshipNullRuntime        = "__glade_relationship_null"
	implicitCurrentPageNullRuntime = "__glade_implicit_current_page_null"
)

func safeNavigationNull() Value {
	value := Null
	value.Runtime = safeNavigationNullRuntime
	return value
}

func typedNull(typeName string) Value {
	value := Null
	value.Type = strings.TrimSpace(typeName)
	return value
}

func implicitCurrentPageNull() Value {
	value := typedNull("PageReference")
	value.Runtime = implicitCurrentPageNullRuntime
	return value
}

func safeNavigationNullOfType(typeName string) Value {
	value := safeNavigationNull()
	value.Type = strings.TrimSpace(typeName)
	return value
}

func storageFieldNullValue(field storage.Field) Value {
	if field.Type == storage.FieldReference {
		// A foreign-key reference field (e.g. AccountId) holds an Id value, not
		// the referenced object. The relationship field (e.g. Account) is typed
		// as the parent object through a separate path. Typing the null as Id
		// keeps overloaded calls passing a null FK from becoming ambiguous.
		return typedNull("Id")
	}
	if typeName := storageFieldTypeName(field); typeName != "" {
		return typedNull(typeName)
	}
	return Null
}

func isSafeNavigationNull(value Value) bool {
	return value.Kind == ValueNull && value.Runtime == safeNavigationNullRuntime
}

func isRelationshipNull(value Value) bool {
	return value.Kind == ValueNull && value.Runtime == relationshipNullRuntime
}

func isImplicitCurrentPageNull(value Value) bool {
	return value.Kind == ValueNull && value.Runtime == implicitCurrentPageNullRuntime
}

func plainNull(value Value) Value {
	if value.Kind == "" {
		return Null
	}
	if isSafeNavigationNull(value) {
		value.Runtime = ""
		return value
	}
	return value
}

func (vm *VM) lookupThisSimpleField(name string) (Value, bool, error) {
	this, ok := vm.Globals["this"]
	if !ok || this.Kind != ValueObject {
		return Null, false, nil
	}
	if actualName, value, field, owner, ok := vm.lookupDeclaredThisFieldValue(this, name); ok {
		if err := vm.checkMemberAccess(owner, field.Access, owner+"."+actualName, field.Modifiers); err != nil {
			return Null, true, err
		}
		if field.Getter != nil {
			if field.Getter.Name == vm.currentMethod.Name {
				return value, true, nil
			}
			value, err := vm.callGetter(owner, field, this)
			return value, true, err
		}
		return value, true, nil
	}
	if actualName, value, ok := objectFieldValue(this, name); ok {
		if relationship, isRelationship := vm.typedParentRelationshipFieldValue(this, name, value); isRelationship {
			return relationship, true, nil
		}
		if field, owner, ok := vm.lookupReceiverField(this.Type, actualName); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+actualName, field.Modifiers); err != nil {
				return Null, true, err
			}
			if field.Getter != nil {
				if field.Getter.Name == vm.currentMethod.Name {
					return value, true, nil
				}
				value, err := vm.callGetter(owner, field, this)
				return value, true, err
			}
		}
		return value, true, nil
	}
	field, owner, ok := vm.lookupReceiverField(this.Type, name)
	if !ok {
		return Null, false, nil
	}
	if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name, field.Modifiers); err != nil {
		return Null, true, err
	}
	if field.Getter != nil {
		value, err := vm.callGetter(owner, field, this)
		return value, true, err
	}
	return defaultValue(field.Type, field.InitialValue), true, nil
}

func (vm *VM) callGetter(owner string, field Field, receiver Value) (Value, error) {
	if field.Getter == nil {
		if isStubProxy(receiver) {
			value, handled, err := vm.callStubProxyGetter(receiver, field.Name, field.Type)
			if handled || err != nil {
				if err != nil || value.Kind != ValueNull || (collectionBase(field.Type) == "" && !isMapType(field.Type)) {
					return value, err
				}
			}
		}
		if _, value, ok := objectFieldValue(receiver, field.Name); ok {
			return value, nil
		}
		return field.Value, nil
	}
	fieldName := field.Name
	if fieldName == "" {
		fieldName = strings.TrimPrefix(field.Getter.Name, owner+".")
		fieldName = strings.TrimSuffix(fieldName, ".get")
	}
	if isStubProxy(receiver) {
		value, handled, err := vm.callStubProxyGetter(receiver, fieldName, field.Type)
		if handled || err != nil {
			if err != nil || value.Kind != ValueNull {
				return value, err
			}
			if vm.stubProviderRecordingMode(vm.currentStubProvider(receiver)) {
				return value, nil
			}
		}
	}
	if strings.EqualFold(fieldName, "rows") {
		if _, validated, ok := objectFieldValue(receiver, "areRowsValidated"); ok && validated.Kind == ValueBool && validated.Bool {
			if _, value, ok := objectFieldValue(receiver, fieldName); ok {
				return value, nil
			}
		}
	}
	if value, ok := vm.activeSetterStoredValue(owner, fieldName, field, receiver); ok {
		if vm.isCurrentGetter(field.Getter) {
			return value, nil
		}
	}
	key := getterCallKey(owner, fieldName, receiver)
	if vm.activeGetters[key] > 0 {
		if _, value, ok := objectFieldValue(receiver, fieldName); ok {
			return vm.currentGetterStoredValue(owner, field, value), nil
		}
		return field.Value, nil
	}
	vm.activeGetters[key]++
	defer func() {
		vm.activeGetters[key]--
		if vm.activeGetters[key] == 0 {
			delete(vm.activeGetters, key)
		}
	}()
	value, err := vm.callMethodWithReceiver(*field.Getter, receiver, nil, resultForLookup())
	if err != nil {
		return value, err
	}
	fieldType := vm.resolveTypeNameInClass(owner, field.Type)
	if value.Kind == ValueNull && vm.isSObjectLikeType(fieldType) {
		value.Type = fieldType
		value.Runtime = relationshipNullRuntime
	}
	return value, nil
}

func (vm *VM) activeSetterStoredValue(owner, fieldName string, field Field, receiver Value) (Value, bool) {
	actualName := field.Name
	if strings.TrimSpace(actualName) == "" {
		actualName = fieldName
	}
	if strings.TrimSpace(field.StorageName) != "" {
		actualName = field.StorageName
	}
	if receiver.Kind == ValueObject {
		if vm.activeSetters[activeInstanceSetterKey(owner, actualName, receiver)] == 0 {
			return Null, false
		}
		if _, value, ok := objectFieldValue(receiver, actualName); ok {
			return vm.currentGetterStoredValue(owner, field, value), true
		}
		if !strings.EqualFold(actualName, fieldName) {
			if _, value, ok := objectFieldValue(receiver, fieldName); ok {
				return vm.currentGetterStoredValue(owner, field, value), true
			}
		}
		return defaultValue(field.Type, field.InitialValue), true
	}
	if receiver.Kind == ValueNull {
		storageName := staticFieldStorageName(fieldName, field)
		if vm.activeSetters[owner+"."+storageName] == 0 {
			return Null, false
		}
		if field.Value.Kind != "" {
			return vm.currentGetterStoredValue(owner, field, field.Value), true
		}
		return defaultValue(field.Type, field.InitialValue), true
	}
	return Null, false
}

func propertyAccessorMethod(method Method) bool {
	if method.Name == "" || method.ClassName == "" {
		return false
	}
	suffix, ok := propertyAccessorSuffix(method.Name, method.ClassName)
	if !ok {
		return false
	}
	return strings.Contains(suffix, ".") &&
		(hasSuffixFold(suffix, ".get") || hasSuffixFold(suffix, ".set"))
}

func propertyAccessorSuffix(methodName, className string) (string, bool) {
	for _, owner := range []string{className, shortTypeName(className)} {
		if owner == "" {
			continue
		}
		prefix := owner + "."
		if hasPrefixFold(methodName, prefix) {
			return methodName[len(prefix):], true
		}
		qualifiedPrefix := "." + owner + "."
		index := strings.LastIndex(strings.ToLower(methodName), strings.ToLower(qualifiedPrefix))
		if index >= 0 {
			return methodName[index+len(qualifiedPrefix):], true
		}
	}
	return "", false
}

func stubProxyCanInterceptMethod(method Method) bool {
	if propertyAccessorMethod(method) {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(method.Access), "private")
}

func (vm *VM) callStubProxyGetter(receiver Value, fieldName, returnType string) (Value, bool, error) {
	provider := vm.currentStubProvider(receiver)
	resolvedReturnType := vm.resolveTypeNameInClass(receiver.Type, returnType)
	if resolvedReturnType == "" {
		resolvedReturnType = "Object"
	}
	metadataArgs := []Value{
		receiver,
		String(fieldName),
		platformScalar("Type", resolvedReturnType),
		{Kind: ValueList, Type: "List<Type>"},
		{Kind: ValueList, Type: "List<String>"},
		{Kind: ValueList, Type: "List<Object>"},
	}
	handler, ok, ambiguous := vm.resolveInstanceMethodForArgs(provider.Type, "handleMethodCall", metadataArgs)
	if ambiguous {
		return Null, true, vm.ambiguousOverloadError(provider.Type+".handleMethodCall", metadataArgs)
	}
	if !ok {
		return Null, true, fmt.Errorf("StubProvider %s must implement handleMethodCall", provider.Type)
	}
	value, err := vm.callMethodWithReceiver(handler, provider, metadataArgs, resultForLookup())
	if err != nil {
		return Null, true, err
	}
	coerced, err := vm.coerceAssignable(returnType, value)
	if err != nil {
		return Null, true, fmt.Errorf("stubbed %s.%s return: %w", receiver.Type, fieldName, err)
	}
	return coerced, true, nil
}

func (vm *VM) stubProviderRecordingMode(provider Value) bool {
	for _, name := range []string{"Stubbing", "Verifying", "verifying", "Recording"} {
		if _, value, ok := objectFieldValue(provider, name); ok && value.Kind == ValueBool && value.Bool {
			return true
		}
		value, err := vm.lookupPath(provider, []string{name})
		if err == nil && value.Kind == ValueBool && value.Bool {
			return true
		}
	}
	return false
}

func getterCallKey(owner, fieldName string, receiver Value) string {
	key := owner + "." + fieldName
	if receiver.Kind == ValueObject && receiver.Ref != 0 {
		key += fmt.Sprintf("#%d", receiver.Ref)
	}
	return key
}

func (vm *VM) lookupGlobalName(name string) (string, bool) {
	if _, ok := vm.Globals[name]; ok {
		return name, true
	}
	normalized := strings.ToLower(name)
	for candidate := range vm.Globals {
		if strings.EqualFold(candidate, normalized) {
			return candidate, true
		}
	}
	return "", false
}

func (vm *VM) lookupTriggerGlobal(name string) (Value, bool) {
	if value, ok := vm.triggerGlobals[name]; ok {
		return value, true
	}
	normalized := strings.ToLower(name)
	for candidate, value := range vm.triggerGlobals {
		if strings.EqualFold(candidate, normalized) {
			return value, true
		}
	}
	if value, ok := defaultTriggerGlobal(name); ok {
		return value, true
	}
	return Null, false
}

func defaultTriggerGlobal(name string) (Value, bool) {
	switch strings.ToLower(name) {
	case "trigger.new", "trigger.old", "trigger.newmap", "trigger.oldmap":
		return Null, true
	case "trigger.isexecuting", "trigger.isbefore", "trigger.isafter", "trigger.isinsert", "trigger.isupdate", "trigger.isdelete", "trigger.isundelete":
		return Bool(false), true
	case "trigger.operationtype":
		value := Null
		value.Type = "TriggerOperation"
		return value, true
	case "trigger.size":
		return Int(0), true
	default:
		return Null, false
	}
}

func (vm *VM) assignmentTargetType(name string) string {
	if actual, ok := vm.lookupGlobalName(name); ok {
		return vm.VarTypes[actual]
	}
	parts := strings.Split(name, ".")
	if len(parts) <= 1 {
		if vm.currentClass != "" {
			if field, _, ok := vm.lookupStaticField(vm.currentClass, name); ok {
				return field.Type
			}
		}
		if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
			if _, field, _, ok := vm.lookupThisFieldRoot(this, name); ok {
				return field.Type
			}
		}
		return ""
	}
	if rootName, ok := vm.lookupGlobalName(parts[0]); ok {
		return vm.fieldPathTargetType(vm.VarTypes[rootName], parts[1:])
	}
	if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
		if _, field, _, ok := vm.lookupThisFieldRoot(this, parts[0]); ok {
			return vm.fieldPathTargetType(field.Type, parts[1:])
		}
	}
	if className, memberName, ok := vm.splitClassMember(name); ok {
		if field, _, ok := vm.lookupStaticField(className, memberName); ok {
			return field.Type
		}
	}
	return ""
}

func (vm *VM) fieldPathTargetType(typeName string, parts []string) string {
	for _, part := range parts {
		if typeName == "" {
			return ""
		}
		if field, _, ok := vm.lookupField(typeName, part); ok {
			typeName = field.Type
			continue
		}
		if vm.Org != nil {
			if parentType, ok := vm.parentRelationshipObjectType(typeName, part); ok {
				typeName = parentType
				continue
			}
			if objectName, ok := vm.resolveObjectName(typeName); ok {
				definition := vm.Org.Objects[objectName].Definition
				if fieldName, ok := vm.resolveFieldName(definition, part); ok {
					field := definition.Fields[fieldName]
					typeName = storageFieldTypeName(field)
					continue
				}
			}
		}
		return ""
	}
	return typeName
}

func splitFieldPath(path string) []string {
	raw := strings.Split(path, ".")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func storageFieldTypeName(field storage.Field) string {
	switch field.Type {
	case storage.FieldID:
		return "Id"
	case storage.FieldReference:
		if len(field.ReferenceTo) == 1 {
			return field.ReferenceTo[0]
		}
		return "Id"
	case storage.FieldString, storage.FieldPicklist, storage.FieldMultiPicklist:
		return "String"
	case storage.FieldBoolean:
		return "Boolean"
	case storage.FieldInteger:
		return "Integer"
	case storage.FieldDecimal:
		return "Decimal"
	case storage.FieldDate:
		return "Date"
	case storage.FieldDateTime:
		return "Datetime"
	default:
		return ""
	}
}

var roundingModeNames = []string{"UP", "DOWN", "CEILING", "FLOOR", "HALF_UP", "HALF_DOWN", "HALF_EVEN", "UNNECESSARY"}

func isDecimalRoundingModeName(name string) bool {
	_, ok := canonicalDecimalRoundingModeName(name)
	return ok
}

func canonicalDecimalRoundingModeName(name string) (string, bool) {
	for _, candidate := range roundingModeNames {
		if strings.EqualFold(name, candidate) {
			return candidate, true
		}
	}
	return "", false
}

func (vm *VM) lookupSObjectTypeToken(parts []string) (Value, bool) {
	if len(parts) < 2 {
		return Null, false
	}
	var objectName string
	switch {
	case len(parts) == 2 && strings.EqualFold(parts[1], "SObjectType"):
		objectName = parts[0]
	case len(parts) == 3 && strings.EqualFold(parts[0], "Schema") && strings.EqualFold(parts[1], "SObjectType"):
		objectName = parts[2]
	case len(parts) == 3 && strings.EqualFold(parts[0], "Schema") && strings.EqualFold(parts[2], "SObjectType"):
		objectName = parts[1]
	case len(parts) == 2 && strings.EqualFold(parts[0], "SObjectType"):
		objectName = parts[1]
	default:
		return Null, false
	}
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(objectName); ok {
			return sObjectTypeToken(canonical), true
		}
	}
	if canonical, ok := storage.ResolveKnownStandardObjectName(objectName); ok {
		return sObjectTypeToken(canonical), true
	}
	if isCustomObjectLikeName(objectName) {
		return sObjectTypeToken(objectName), true
	}
	return Null, false
}

func (vm *VM) sObjectTypeTokenForName(objectName string) (Value, bool) {
	if rest, ok := strings.CutPrefix(objectName, "Schema."); ok {
		objectName = rest
	}
	if strings.EqualFold(objectName, "SObject") {
		return sObjectTypeToken("SObject"), true
	}
	if strings.EqualFold(objectName, "AggregateResult") {
		return sObjectTypeToken("AggregateResult"), true
	}
	if vm.Org != nil {
		if canonical, ok := vm.resolveObjectName(objectName); ok {
			return sObjectTypeToken(canonical), true
		}
	}
	if canonical, ok := storage.ResolveKnownStandardObjectName(objectName); ok {
		return sObjectTypeToken(canonical), true
	}
	if isCustomObjectLikeName(objectName) {
		return sObjectTypeToken(objectName), true
	}
	return Null, false
}

func (vm *VM) callSObjectTypeStaticMember(typeName, method string, args []Value) (Value, bool, error) {
	if !strings.EqualFold(method, "getSObjectType") {
		return Null, false, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s.getSObjectType expects 0 arguments", typeName)
	}
	token, ok := vm.sObjectTypeTokenForName(typeName)
	if !ok {
		return Null, false, nil
	}
	return token, true, nil
}

func (vm *VM) lookupSObjectFieldToken(parts []string) (Value, bool) {
	if len(parts) < 2 {
		return Null, false
	}
	if len(parts) == 3 && !strings.EqualFold(parts[0], "Schema") && !strings.EqualFold(parts[1], "Fields") {
		return vm.lookupSObjectRelationshipFieldToken(parts[0], parts[1], parts[2])
	}
	objectName := parts[0]
	fieldName := ""
	switch {
	case len(parts) == 2:
		fieldName = parts[1]
	case len(parts) == 3 && strings.EqualFold(parts[0], "Schema"):
		objectName = parts[1]
		fieldName = parts[2]
	case len(parts) == 3 && strings.EqualFold(parts[1], "Fields"):
		fieldName = parts[2]
	case len(parts) == 4 && strings.EqualFold(parts[1], "SObjectType") && strings.EqualFold(parts[2], "Fields"):
		fieldName = parts[3]
	case len(parts) == 5 && strings.EqualFold(parts[0], "Schema") && strings.EqualFold(parts[2], "SObjectType") && strings.EqualFold(parts[3], "Fields"):
		objectName = parts[1]
		fieldName = parts[4]
	default:
		return Null, false
	}
	canonicalObject, definition, ok := vm.describeObjectDefinition(objectName)
	if !ok {
		return Null, false
	}
	objectName = canonicalObject
	field := storage.Field{}
	namespace := ""
	if vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	canonical, ok := vm.resolveFieldName(definition, fieldName)
	if !ok {
		standardDefinition := definition.Clone()
		storage.EnsureStandardObjectFieldsForFeatures(&standardDefinition, []string{"PersonAccounts"})
		if standardField, standardOK := storage.ResolveFieldName(standardDefinition, namespace, fieldName); standardOK {
			canonical = standardField
			field = standardDefinition.Fields[canonical]
		} else if vm.canSynthesizeSchemaField(objectName) {
			synthetic := syntheticSchemaField(fieldName)
			if synthetic.APIName == "" {
				return Null, false
			}
			canonical = synthetic.APIName
			field = synthetic
		} else if !isSObjectSystemField(fieldName) {
			return Null, false
		} else {
			canonical = fieldName
		}
	}
	if field.APIName == "" {
		field = definition.Fields[canonical]
	}
	if field.APIName == "" {
		field.APIName = canonical
	}
	return vm.sObjectFieldTokenFromField(objectName, field), true
}

func (vm *VM) lookupSObjectRelationshipFieldToken(objectName, relationshipFieldName, targetFieldName string) (Value, bool) {
	_, definition, ok := vm.describeObjectDefinition(objectName)
	if !ok {
		return Null, false
	}
	namespace := ""
	if vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	canonicalRelationshipField, ok := vm.resolveFieldName(definition, relationshipFieldName)
	if !ok {
		for name, candidate := range definition.Fields {
			apiName := candidate.APIName
			if apiName == "" {
				apiName = name
			}
			if candidate.Type != storage.FieldReference || len(candidate.ReferenceTo) == 0 {
				continue
			}
			parentRelationship := vm.parentRelationshipNameForReferenceField(definition, candidate)
			if !vmParentRelationshipNameMatches(namespace, apiName, relationshipFieldName) &&
				!vmRelationshipNameMatches(namespace, parentRelationship, relationshipFieldName) {
				continue
			}
			canonicalRelationshipField = apiName
			ok = true
			break
		}
		if !ok {
			return Null, false
		}
	}
	relationshipField := definition.Fields[canonicalRelationshipField]
	if relationshipField.APIName == "" {
		relationshipField.APIName = canonicalRelationshipField
	}
	if relationshipField.Type != storage.FieldReference || len(relationshipField.ReferenceTo) == 0 {
		return Null, false
	}
	for _, targetObjectName := range relationshipField.ReferenceTo {
		canonicalTarget, targetDefinition, targetOK := vm.describeObjectDefinition(targetObjectName)
		if !targetOK {
			continue
		}
		if targetField, ok := vm.resolveSObjectTokenField(canonicalTarget, targetDefinition, namespace, targetFieldName); ok {
			return vm.sObjectFieldTokenFromField(canonicalTarget, targetField), true
		}
	}
	return Null, false
}

func (vm *VM) resolveSObjectTokenField(objectName string, definition storage.ObjectDefinition, namespace, fieldName string) (storage.Field, bool) {
	canonical, ok := storage.ResolveFieldName(definition, namespace, fieldName)
	if !ok {
		standardDefinition := definition.Clone()
		storage.EnsureStandardObjectFieldsForFeatures(&standardDefinition, []string{"PersonAccounts"})
		if standardField, standardOK := storage.ResolveFieldName(standardDefinition, namespace, fieldName); standardOK {
			field := standardDefinition.Fields[standardField]
			if field.APIName == "" {
				field.APIName = standardField
			}
			return field, true
		}
		if vm.canSynthesizeSchemaField(objectName) {
			synthetic := syntheticSchemaField(fieldName)
			if synthetic.APIName != "" {
				return synthetic, true
			}
		}
		if !isSObjectSystemField(fieldName) {
			return storage.Field{}, false
		}
		canonical = fieldName
	}
	field := definition.Fields[canonical]
	if field.APIName == "" {
		field.APIName = canonical
	}
	return field, true
}

func (vm *VM) canSynthesizeSchemaField(objectName string) bool {
	if canonical, ok := storage.ResolveKnownStandardObjectName(objectName); ok && strings.EqualFold(canonical, objectName) {
		return false
	}
	if _, ok := storage.ResolveKnownStandardObjectName(objectName); ok {
		return false
	}
	return true
}

func (vm *VM) callSchemaSObjectTypePath(callee string, args []Value, result *Result) (Value, bool, error) {
	parts := strings.Split(callee, ".")
	if len(parts) >= 4 && strings.EqualFold(parts[len(parts)-2], "fields") && strings.EqualFold(parts[len(parts)-1], "getMap") {
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
		}
		tokenParts := parts[:len(parts)-2]
		token, ok := vm.lookupSObjectTypeToken(tokenParts)
		if !ok {
			return Null, false, nil
		}
		describe, updated, mutated, handled, err := vm.callPlatformObjectMember(token, "getDescribe", nil, result)
		if err != nil || !handled {
			return describe, true, err
		}
		_ = updated
		_ = mutated
		fields, ok := describe.Fields["fields"]
		if !ok {
			return Null, true, fmt.Errorf("%s describe fields are not available", callee)
		}
		value, _, _, _, err := vm.callPlatformObjectMember(fields, "getMap", nil, result)
		return value, true, err
	}
	if len(parts) >= 4 && strings.EqualFold(parts[len(parts)-2], "fieldSets") && strings.EqualFold(parts[len(parts)-1], "getMap") {
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
		}
		tokenParts := parts[:len(parts)-2]
		token, ok := vm.lookupSObjectTypeToken(tokenParts)
		if !ok {
			return Null, false, nil
		}
		describe, _, _, handled, err := vm.callPlatformObjectMember(token, "getDescribe", nil, result)
		if err != nil || !handled {
			return describe, true, err
		}
		fieldSets, ok := describe.Fields["fieldSets"]
		if !ok {
			return Null, true, fmt.Errorf("%s describe field sets are not available", callee)
		}
		value, _, _, _, err := vm.callPlatformObjectMember(fieldSets, "getMap", nil, result)
		return value, true, err
	}
	if len(parts) >= 5 && strings.EqualFold(parts[len(parts)-3], "fieldSets") && strings.EqualFold(parts[len(parts)-1], "getFields") {
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
		}
		tokenParts := parts[:len(parts)-3]
		token, ok := vm.lookupSObjectTypeToken(tokenParts)
		if !ok {
			return Null, false, nil
		}
		describe, _, _, handled, err := vm.callPlatformObjectMember(token, "getDescribe", nil, result)
		if err != nil || !handled {
			return describe, true, err
		}
		fieldSets, ok := describe.Fields["fieldSets"]
		if !ok {
			return Null, true, fmt.Errorf("%s describe field sets are not available", callee)
		}
		fieldSet, _, _, handled, err := vm.callPlatformObjectMember(fieldSets, "get", []Value{String(parts[len(parts)-2])}, result)
		if err != nil || !handled {
			return fieldSet, true, err
		}
		if fieldSet.Kind == ValueNull {
			return Null, true, newNullDereferenceError("while accessing " + callee)
		}
		value, _, _, _, err := vm.callPlatformObjectMember(fieldSet, "getFields", nil, result)
		return value, true, err
	}
	if len(parts) >= 4 && strings.EqualFold(parts[len(parts)-2], "fields") {
		tokenParts := parts[:len(parts)-2]
		token, ok := vm.lookupSObjectTypeToken(tokenParts)
		if !ok {
			return Null, false, nil
		}
		objectValue, ok := token.Fields["object"]
		if !ok || objectValue.Kind != ValueString {
			return Null, true, fmt.Errorf("%s token missing object", callee)
		}
		fieldToken, ok := vm.lookupSObjectFieldToken([]string{objectValue.Text, parts[len(parts)-1]})
		if !ok {
			return Null, false, nil
		}
		if len(args) == 0 {
			return fieldToken, true, nil
		}
		return Null, true, fmt.Errorf("%s does not accept arguments", callee)
	}
	if len(parts) >= 5 && strings.EqualFold(parts[len(parts)-3], "fields") {
		tokenParts := parts[:len(parts)-3]
		token, ok := vm.lookupSObjectTypeToken(tokenParts)
		if !ok {
			return Null, false, nil
		}
		objectValue, ok := token.Fields["object"]
		if !ok || objectValue.Kind != ValueString {
			return Null, true, fmt.Errorf("%s token missing object", callee)
		}
		fieldToken, ok := vm.lookupSObjectFieldToken([]string{objectValue.Text, parts[len(parts)-2]})
		if !ok {
			return Null, false, nil
		}
		method := parts[len(parts)-1]
		if strings.EqualFold(method, "getDescribe") {
			value, _, _, handled, err := vm.callPlatformObjectMember(fieldToken, method, args, result)
			if err != nil || !handled {
				return value, true, err
			}
			return value, true, nil
		}
		describe, _, _, handled, err := vm.callPlatformObjectMember(fieldToken, "getDescribe", nil, result)
		if err != nil || !handled {
			return describe, true, err
		}
		value, _, _, handled, err := vm.callPlatformObjectMember(describe, method, args, result)
		if err != nil || !handled {
			return value, true, err
		}
		return value, true, nil
	}
	if len(parts) < 3 || !schemaSObjectTypeDescribeForwardMethod(parts[len(parts)-1]) {
		return Null, false, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
	}
	tokenParts := parts[:len(parts)-1]
	token, ok := vm.lookupSObjectTypeToken(tokenParts)
	if !ok {
		return Null, false, nil
	}
	describe, updated, mutated, handled, err := vm.callPlatformObjectMember(token, "getDescribe", nil, result)
	if err != nil || !handled {
		return describe, true, err
	}
	_ = updated
	_ = mutated
	value, _, _, _, err := vm.callPlatformObjectMember(describe, parts[len(parts)-1], nil, result)
	return value, true, err
}

func schemaSObjectTypeDescribeForwardMethod(method string) bool {
	for _, candidate := range []string{
		"getName", "getLabel", "getLabelPlural", "getKeyPrefix",
		"getRecordTypeInfos", "getRecordTypeInfosByName", "getRecordTypeInfosByDeveloperName", "getRecordTypeInfosById",
		"getChildRelationships", "getSObjectType",
		"isAccessible", "isCreateable", "isUpdateable", "isDeletable", "isQueryable", "isSearchable", "isCustom",
		"isCustomSetting", "isDeprecatedAndHidden", "isFeedEnabled", "isMergeable", "isMruEnabled", "isUndeletable",
	} {
		if strings.EqualFold(method, candidate) {
			return true
		}
	}
	return false
}

func (vm *VM) callDottedReceiverMember(callee string, args []Value, result *Result) (Value, bool, error) {
	dot := strings.LastIndex(callee, ".")
	if dot <= 0 || dot >= len(callee)-1 {
		return Null, false, nil
	}
	receiverName := callee[:dot]
	method := callee[dot+1:]
	if strings.EqualFold(method, "values") || strings.EqualFold(method, "valueOf") {
		if value, handled, err := vm.callEnumStaticMember(receiverName, method, args); handled || err != nil {
			return value, handled, err
		}
	}
	if typeName, fieldName, ok := splitDottedTypeMember(receiverName); ok {
		if receiver, ok := builtinStaticField(typeName, fieldName); ok {
			return vm.callValueMember(receiverName, receiver, method, args, result)
		}
	}
	receiver, err := vm.lookup(receiverName)
	if err != nil {
		if !strings.Contains(receiverName, ".") {
			if value, ok, fieldErr := vm.lookupThisSimpleField(receiverName); ok || fieldErr != nil {
				if fieldErr != nil {
					return Null, true, fieldErr
				}
				return vm.callValueMember(receiverName, value, method, args, result)
			}
		}
		return Null, false, nil
	}
	return vm.callValueMember(receiverName, receiver, method, args, result)
}

func (vm *VM) callBuiltinStaticFieldMember(callee string, args []Value, result *Result) (Value, bool, error) {
	dot := strings.LastIndex(callee, ".")
	if dot <= 0 || dot >= len(callee)-1 {
		return Null, false, nil
	}
	receiverName := callee[:dot]
	method := callee[dot+1:]
	typeName, fieldName, ok := splitDottedTypeMember(receiverName)
	if !ok {
		return Null, false, nil
	}
	receiver, ok := builtinStaticField(typeName, fieldName)
	if !ok {
		return Null, false, nil
	}
	return vm.callValueMember(receiverName, receiver, method, args, result)
}

func sObjectTypeToken(objectName string) Value {
	token := Object("Schema.SObjectType")
	token.Fields["object"] = String(objectName)
	return token
}

// sObjectTypeDescribePropertyValue resolves the legacy describe-shorthand
// property access `Schema.SObjectType.<Object>.<Property>` (e.g.
// `SObjectType.Account.Name`, `.Label`, `.KeyPrefix`). In Apex this shorthand
// yields a DescribeSObjectResult, so its scalar properties are readable as
// fields. The describe value stores these under camelCase keys, which is the
// PascalCase Apex property name with a lowercased first letter.
func (vm *VM) sObjectTypeDescribePropertyValue(objectName, property string) (Value, bool) {
	property = strings.TrimSpace(property)
	if property == "" {
		return Null, false
	}
	resolvedName, definition, ok := vm.describeObjectDefinition(objectName)
	if !ok {
		return Null, false
	}
	describe := vm.describeSObjectValue(resolvedName, definition)
	key := strings.ToLower(property[:1]) + property[1:]
	switch key {
	case "name", "label", "labelPlural", "keyPrefix", "custom", "customSetting",
		"feedEnabled", "mergeable", "mruEnabled", "undeletable", "deprecatedAndHidden":
		if value, exists := describe.Fields[key]; exists {
			return value, true
		}
	}
	return Null, false
}

func sObjectFieldToken(objectName, fieldName string) Value {
	token := Object("Schema.SObjectField")
	token.Fields["object"] = String(objectName)
	token.Fields["field"] = String(fieldName)
	token.Fields["Name"] = String(fieldName)
	token.Fields["name"] = String(fieldName)
	return token
}

func (vm *VM) sObjectFieldToken(objectName, fieldName string) Value {
	if vm == nil {
		return sObjectFieldToken(objectName, fieldName)
	}
	return sObjectFieldToken(objectName, vm.describeFieldName(fieldName))
}

func sObjectFieldTokenFromField(objectName string, field storage.Field) Value {
	fieldName := field.APIName
	token := sObjectFieldToken(objectName, fieldName)
	label := field.Label
	if label == "" {
		label = fieldName
	}
	token.Fields["label"] = String(label)
	return token
}

func (vm *VM) sObjectFieldTokenFromField(objectName string, field storage.Field) Value {
	fieldName := field.APIName
	token := vm.sObjectFieldToken(objectName, fieldName)
	label := field.Label
	if label == "" {
		label = fieldName
	}
	token.Fields["label"] = String(label)
	return token
}

func (vm *VM) lookupPath(root Value, parts []string) (Value, error) {
	current := root
	for i, part := range parts {
		if current.Kind == ValueNull {
			if vm.isSObjectLikeType(current.Type) {
				if defaultValue, ok := vm.defaultNullSObjectAccessValue(current.Type); ok {
					current = defaultValue
				} else if isRelationshipNull(current) || i > 0 {
					if value, ok := vm.relationshipNullFieldAccessValue(current.Type, part); ok {
						current = value
						continue
					}
					if isRelationshipNull(current) {
						return Null, nil
					}
					return Null, newNullDereferenceError("while accessing " + strings.Join(parts[:i+1], "."))
				} else {
					return Null, newNullDereferenceError("while accessing " + strings.Join(parts[:i+1], "."))
				}
			} else {
				return Null, newNullDereferenceError("while accessing " + strings.Join(parts[:i+1], "."))
			}
		}
		if current.Kind == ValueList {
			if len(current.List) == 0 {
				return Null, newExceptionError("QueryException", "List has no rows for assignment to SObject")
			}
			if len(current.List) > 1 {
				return Null, fmt.Errorf("cannot access %s on list", part)
			}
			current = current.List[0]
		}
		if current.Kind != ValueObject {
			if current.Kind == ValueMap {
				switch part {
				case "values":
					out := List()
					for _, key := range orderedValueMapKeys(current) {
						out.List = append(out.List, current.Map[key])
					}
					if _, valueType, ok := mapTypeArgs(current.Type); ok {
						out.Type = "List<" + valueType + ">"
					}
					current = out
					continue
				case "keySet":
					out := Set()
					for _, rawKey := range orderedValueMapKeys(current) {
						out.Set = append(out.Set, mapStoredKey(current, rawKey))
					}
					if keyType, _, ok := mapTypeArgs(current.Type); ok {
						out.Type = "Set<" + keyType + ">"
					}
					current = out
					continue
				}
			}
			return Null, fmt.Errorf("cannot access %s on %s", part, current.Kind)
		}
		switch current.Type {
		case "Schema.SObjectType":
			objectValue, ok := current.Fields["object"]
			if !ok || objectValue.Kind != ValueString {
				return Null, fmt.Errorf("Schema.SObjectType token missing object")
			}
			if strings.EqualFold(part, "SObjectType") {
				continue
			}
			if strings.EqualFold(part, "fields") {
				objectName := objectValue.Text
				objectName, definition, ok := vm.describeObjectDefinition(objectName)
				if !ok {
					return Null, fmt.Errorf("Schema.SObjectType.fields unknown object %s", objectName)
				}
				describe := vm.describeSObjectValue(objectName, definition)
				current = describe.Fields["fields"]
				continue
			}
			if strings.EqualFold(part, "fieldSets") {
				objectName := objectValue.Text
				objectName, definition, ok := vm.describeObjectDefinition(objectName)
				if !ok {
					return Null, fmt.Errorf("Schema.SObjectType.fieldSets unknown object %s", objectName)
				}
				describe := vm.describeSObjectValue(objectName, definition)
				current = describe.Fields["fieldSets"]
				continue
			}
			if value, ok := vm.sObjectTypeDescribePropertyValue(objectValue.Text, part); ok {
				current = value
				continue
			}
		case "Schema.SObjectFieldMap":
			mapValue, ok := current.Fields["map"]
			if !ok || mapValue.Kind != ValueMap {
				return Null, fmt.Errorf("Schema.SObjectFieldMap is missing map")
			}
			if value, ok := mapValue.Map[mapKey(String(part))]; ok {
				current = value
				continue
			}
			if value, ok := mapValue.Map[mapKey(String(strings.ToLower(part)))]; ok {
				current = value
				continue
			}
			current = Null
			continue
		case "Schema.FieldSetMap":
			mapValue, ok := current.Fields["map"]
			if !ok || mapValue.Kind != ValueMap {
				return Null, fmt.Errorf("Schema.FieldSetMap is missing map")
			}
			if value, ok := mapValue.Map[mapKey(String(part))]; ok {
				current = value
				continue
			}
			if value, ok := mapValue.Map[mapKey(String(strings.ToLower(part)))]; ok {
				current = value
				continue
			}
			current = Null
			continue
		}
		currentType := runtimeObjectType(current)
		if current.Text != "" {
			switch {
			case strings.EqualFold(part, "name"):
				if value, handled, err := vm.callEnumMember(current, "name", nil); handled || err != nil {
					current = value
					continue
				}
			case strings.EqualFold(part, "ordinal"):
				if value, handled, err := vm.callEnumMember(current, "ordinal", nil); handled || err != nil {
					current = value
					continue
				}
			}
		}
		if field, owner, ok := vm.lookupReceiverField(currentType, part); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+part, field.Modifiers); err != nil {
				return Null, err
			}
			if field.Getter != nil && vm.isCurrentGetter(field.Getter) {
				if _, value, ok := objectFieldValue(current, field.Name); ok {
					current = vm.currentGetterStoredValue(owner, field, value)
					continue
				}
				if _, value, ok := objectFieldValue(current, part); ok {
					current = vm.currentGetterStoredValue(owner, field, value)
					continue
				}
				current = Null
				continue
			}
			if field.Getter != nil && !field.HasSetter {
				if _, value, ok := objectFieldValue(current, field.Name); ok && value.Kind != ValueNull {
					current = value
					continue
				}
			}
			if field.Getter != nil {
				value, err := vm.callGetter(owner, field, current)
				if err != nil {
					return Null, err
				}
				current = value
				continue
			}
			if _, value, ok := objectFieldValue(current, field.Name); ok {
				if relationship, isRelationship := vm.typedParentRelationshipFieldValue(current, field.Name, value); isRelationship {
					current = relationship
					continue
				}
				value = coerceRawRecordTypeDefaultTokenRuntimeValue(field.Name, value)
				if _, fieldDef, exists := vm.sObjectFieldDefinition(current.Type, field.Name); exists {
					value = coerceReadSObjectFieldRuntimeValue(value, fieldDef)
				}
				current = value
				continue
			}
			if _, value, ok := objectFieldValue(current, part); ok {
				if relationship, isRelationship := vm.typedParentRelationshipFieldValue(current, part, value); isRelationship {
					current = relationship
					continue
				}
				value = coerceRawRecordTypeDefaultTokenRuntimeValue(part, value)
				if _, fieldDef, exists := vm.sObjectFieldDefinition(current.Type, part); exists {
					value = coerceReadSObjectFieldRuntimeValue(value, fieldDef)
				}
				current = value
				continue
			}
			if canonical := vm.resolveSObjectFieldName(current.Type, field.Name); canonical != field.Name {
				if _, value, ok := objectFieldValue(current, canonical); ok {
					if relationship, isRelationship := vm.typedParentRelationshipFieldValue(current, canonical, value); isRelationship {
						current = relationship
						continue
					}
					value = coerceRawRecordTypeDefaultTokenRuntimeValue(canonical, value)
					if _, fieldDef, exists := vm.sObjectFieldDefinition(current.Type, canonical); exists {
						value = coerceReadSObjectFieldRuntimeValue(value, fieldDef)
					}
					current = value
					continue
				}
			}
			if vm.isSObjectLikeType(current.Type) {
				canonical := vm.resolveSObjectFieldName(current.Type, part)
				if value, ok := vm.missingSObjectFieldValue(current, canonical); ok {
					if value.Kind == ValueNull && i < len(parts)-1 {
						if shell, hasShell := vm.parentRelationshipValueFromLookupID(current, canonical); hasShell && shell.Kind == ValueObject {
							value = shell
						} else if shell, hasShell := vm.parentRelationshipShell(current, canonical); hasShell {
							value = shell
						}
					}
					current = value
					continue
				}
			}
			current = Null
			continue
		}
		if value, ok := vm.generatedPlatformInstanceField(current, part); ok {
			current = value
			continue
		}
		canonicalPart := vm.resolveSObjectFieldName(current.Type, part)
		_, value, ok := objectFieldValue(current, canonicalPart)
		if actualName, childValue, hasChild := vm.loadedChildRelationshipValue(current, canonicalPart); hasChild {
			canonicalPart = actualName
			value = childValue
			ok = true
		} else if !ok && canonicalPart != part {
			_, value, ok = objectFieldValue(current, part)
		}
		if !ok || (!isSObjectSystemField(canonicalPart) && !isParentProjectionSObject(current)) {
			if err := vm.unqueriedSObjectFieldError(current, canonicalPart, false); err != nil {
				return Null, err
			}
		}
		if ok {
			if relationship, isRelationship := vm.typedParentRelationshipFieldValue(current, canonicalPart, value); isRelationship {
				current = relationship
				continue
			}
			value = coerceRawRecordTypeDefaultTokenRuntimeValue(canonicalPart, value)
			if value.Kind == ValueNull {
				if relationshipType, hasChildRelationship := vm.jsonSObjectChildRelationshipType(current.Type, canonicalPart); hasChildRelationship && !vm.sObjectParentRelationshipField(current.Type, canonicalPart) {
					children := List()
					children.Type = relationshipType
					value = children
					current = value
					continue
				}
			}
			if value.Kind != ValueList && vm.sObjectChildRelationshipField(current.Type, canonicalPart) && !vm.sObjectParentRelationshipField(current.Type, canonicalPart) {
				if relationshipType, hasChildRelationship := vm.jsonSObjectChildRelationshipType(current.Type, canonicalPart); hasChildRelationship {
					children := List()
					children.Type = relationshipType
					current = children
					continue
				}
			}
			if value.Kind == ValueList && vm.sObjectChildRelationshipField(current.Type, canonicalPart) {
				value = shallowCopyListValue(value)
			}
			if addressValue, hasAddress := vm.sObjectCompoundAddressValue(current, canonicalPart); hasAddress {
				value = addressValue
				current = value
				continue
			}
			if _, fieldDef, exists := vm.sObjectFieldDefinition(current.Type, canonicalPart); exists {
				value = coerceReadSObjectFieldRuntimeValue(value, fieldDef)
				if fieldDef.Type == storage.FieldSummary && !isExplicitSObjectField(current, canonicalPart) && !vm.queriedSObjectFieldsIncludes(current, canonicalPart) {
					if summaryValue, hasSummary := vm.evaluateSummaryField(current, fieldDef); hasSummary {
						value = vmValueFromStorage(summaryValue)
						current = value
						continue
					}
				}
			}
		}
		if ok && value.Kind == ValueNull {
			if i < len(parts)-1 {
				if shell, hasShell := vm.parentRelationshipValueFromLookupID(current, canonicalPart); hasShell && shell.Kind == ValueObject {
					current = shell
					continue
				}
				if shell, hasShell := vm.parentRelationshipShell(current, canonicalPart); hasShell && shell.Kind == ValueObject {
					current = shell
					continue
				}
			}
			if isExplicitSObjectField(current, canonicalPart) {
				if _, fieldDef, exists := vm.sObjectFieldDefinition(current.Type, canonicalPart); exists && fieldDef.Type == storage.FieldBoolean {
					if defaultValue, ok := storage.DefaultValueForField(fieldDef); ok {
						current = vmValueFromStorage(defaultValue)
					} else {
						current = Bool(false)
					}
					continue
				}
				if typedNull, hasTypedNull := vm.explicitParentRelationshipNullValue(current, canonicalPart); hasTypedNull {
					current = typedNull
					continue
				}
				current = value
				continue
			}
			if definition, fieldDef, exists := vm.sObjectFieldDefinition(current.Type, canonicalPart); exists && fieldDef.Type == storage.FieldReference {
				if lookupID, hasLookupID := vm.lookupIDFromLoadedParentRelationship(current, definition, canonicalPart); hasLookupID {
					value = lookupID
					current = value
					continue
				}
			}
			if relationshipValue, hasRelationship := vm.parentRelationshipValue(current, canonicalPart); hasRelationship {
				value = relationshipValue
				if value.Kind == ValueNull && i < len(parts)-1 {
					if shell, ok := vm.parentRelationshipShell(current, canonicalPart); ok {
						value = shell
					}
				}
			}
		}
		if !ok {
			if value, hasComponentExpression := componentApexExpressionValue(current, part); hasComponentExpression {
				current = value
				continue
			}
			if value, ok := vm.missingSObjectFieldValue(current, canonicalPart); ok {
				if value.Kind == ValueNull && i < len(parts)-1 {
					if shell, hasShell := vm.parentRelationshipValueFromLookupID(current, canonicalPart); hasShell && shell.Kind == ValueObject {
						value = shell
					} else if shell, hasShell := vm.parentRelationshipShell(current, canonicalPart); hasShell {
						value = shell
					}
				}
				current = value
				continue
			}
			if isCustomFieldOrRelationshipType(current.Type) {
				current = Null
				continue
			}
			if looksManagedQualifiedType(current.Type) {
				value = managedPassiveFieldDefault(part)
				if current.Fields == nil {
					current.Fields = make(map[string]Value)
				}
				current.Fields[part] = value
				current = value
				continue
			}
			if componentApexRuntimeType(current.Type) {
				current = Null
				continue
			}
			return Null, fmt.Errorf("unknown field %q on %s", part, currentType)
		}
		current = value
	}
	return current, nil
}

func componentApexExpressionValue(value Value, field string) (Value, bool) {
	if value.Kind != ValueObject || !componentApexRuntimeType(value.Type) {
		return Null, false
	}
	expressions, ok := value.Fields["expressions"]
	if !ok || expressions.Kind != ValueObject {
		return Null, false
	}
	_, out, ok := objectFieldValue(expressions, field)
	return out, ok
}

func shallowCopyListValue(value Value) Value {
	out := value
	out.Ref = newValueRef()
	if value.List != nil {
		out.List = append([]Value(nil), value.List...)
	}
	return out
}

func (vm *VM) sObjectChildRelationshipField(typeName, field string) bool {
	_, ok := vm.jsonSObjectChildRelationshipType(typeName, field)
	return ok
}

func (vm *VM) sObjectParentRelationshipField(typeName, field string) bool {
	if vm == nil || vm.Org == nil || strings.TrimSpace(typeName) == "" || strings.TrimSpace(field) == "" {
		return false
	}
	_, ok := vm.parentRelationshipField(typeName, field)
	return ok
}

func (vm *VM) defaultNullSObjectAccessValue(typeName string) (Value, bool) {
	if vm == nil || vm.Org == nil {
		return Null, false
	}
	objectName, ok := vm.resolveObjectName(typeName)
	if !ok {
		return Null, false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return Null, false
	}
	if storage.IsCustomMetadataDefinition(object.Definition) {
		value := vm.readOnlyCustomDataDefaultValue(objectName, "custom metadata")
		value.Type = typeName
		return value, true
	}
	if storage.IsCustomSettingDefinition(object.Definition) {
		value := vm.readOnlyCustomDataDefaultValue(objectName, "custom setting")
		value.Type = typeName
		return value, true
	}
	return Null, false
}

func (vm *VM) relationshipNullFieldAccessValue(typeName, field string) (Value, bool) {
	if relation, ok := vm.parentRelationshipForName(Object(typeName), field); ok {
		return vm.parentRelationshipTypedNull(relation), true
	}
	_, fieldDef, ok := vm.sObjectFieldDefinition(typeName, field)
	if !ok {
		return Null, false
	}
	if defaultValue, ok := storage.DefaultValueForField(fieldDef); ok {
		return vmValueFromStorage(defaultValue), true
	}
	if fieldDef.Type == storage.FieldBoolean {
		return Bool(false), true
	}
	return storageFieldNullValue(fieldDef), true
}

func (vm *VM) explicitParentRelationshipNullValue(receiver Value, relationshipName string) (Value, bool) {
	if _, ok := vm.parentRelationshipForName(receiver, relationshipName); !ok {
		return Null, false
	}
	value, ok := vm.parentRelationshipValue(receiver, relationshipName)
	if !ok || value.Kind != ValueNull {
		return Null, false
	}
	return value, true
}

func (vm *VM) addQueriedRelationshipFieldToPopulatedMap(out *Value, receiver Value, fieldPath string) {
	if out == nil || out.Kind != ValueMap || strings.TrimSpace(fieldPath) == "" {
		return
	}
	parts := strings.Split(fieldPath, ".")
	if len(parts) < 2 {
		return
	}
	relationshipName := strings.TrimSpace(parts[0])
	if relationshipName == "" || strings.EqualFold(relationshipName, "object") || isInternalSObjectField(relationshipName) {
		return
	}
	encodedRelationship := mapKey(String(relationshipName))
	relationshipValue := Null
	if existing, ok := out.Map[encodedRelationship]; ok && existing.Kind == ValueObject {
		relationshipValue = existing
	} else if _, loaded, ok := objectFieldValue(receiver, relationshipName); ok && loaded.Kind == ValueObject {
		relationshipValue = loaded
	} else if shell, ok := vm.parentRelationshipShell(receiver, relationshipName); ok {
		relationshipValue = shell
	}
	if relationshipValue.Kind != ValueObject {
		return
	}
	current := relationshipValue
	for i := 1; i < len(parts); i++ {
		segment := strings.TrimSpace(parts[i])
		if segment == "" {
			return
		}
		if i == len(parts)-1 {
			if _, _, ok := objectFieldValue(current, segment); !ok {
				current.Fields[segment] = Null
			}
			break
		}
		_, child, ok := objectFieldValue(current, segment)
		if !ok || child.Kind != ValueObject {
			child = Object(segment)
			current.Fields[segment] = child
		}
		current = child
	}
	out.Map[encodedRelationship] = relationshipValue
	if out.MapKeys == nil {
		out.MapKeys = make(map[string]Value)
	}
	out.MapKeys[encodedRelationship] = String(relationshipName)
	if !containsString(out.MapOrder, encodedRelationship) {
		out.MapOrder = append(out.MapOrder, encodedRelationship)
	}
}

func (vm *VM) isCurrentGetter(getter *Method) bool {
	if getter == nil || vm.currentMethod.Name == "" {
		return false
	}
	if strings.EqualFold(getter.Name, vm.currentMethod.Name) {
		return true
	}
	return hasSuffixFold(vm.currentMethod.Name, "."+strings.ToLower(getter.Name))
}

func (vm *VM) currentGetterStoredValue(owner string, field Field, value Value) Value {
	fieldType := vm.resolveTypeNameInClass(owner, field.Type)
	if strings.TrimSpace(fieldType) == "" || value.Kind == ValueNull {
		return value
	}
	if _, err := vm.coerceAssignable(fieldType, value); err == nil {
		return value
	}
	return defaultValue(field.Type, field.InitialValue)
}

func (vm *VM) assign(name string, value Value) error {
	value = plainNull(value)
	if actual, ok := vm.lookupGlobalName(name); ok {
		if typeName := vm.VarTypes[actual]; typeName != "" {
			coerced, err := vm.coerceAssignable(typeName, value)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			value = coerced
			value.Static = typeName
		}
		vm.Globals[actual] = value
		return nil
	}
	if ok, err := vm.assignRestContextField(name, value); ok || err != nil {
		return err
	}
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		if strings.EqualFold(parts[0], "super") {
			if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
				if len(parts) == 2 {
					if class, ok := vm.lookupClass(this.Type); ok && class.SuperClass != "" {
						if field, owner, ok := vm.lookupField(class.SuperClass, parts[1]); ok {
							actualName := field.Name
							if actualName == "" {
								actualName = parts[1]
							}
							if err := vm.checkMemberAccess(owner, field.Access, owner+"."+actualName, field.Modifiers); err != nil {
								return err
							}
							fieldType := vm.resolveTypeNameInClass(owner, field.Type)
							coerced, err := vm.coerceAssignable(fieldType, value)
							if err != nil {
								return fmt.Errorf("%s.%s: %w", owner, actualName, err)
							}
							vm.setObjectFieldValue(&this, actualName, coerced)
							vm.Globals["this"] = this
							return nil
						}
					}
				}
				return vm.assignPath(this, parts[1:], value)
			}
		}
		if rootName, ok := vm.lookupGlobalName(parts[0]); ok {
			return vm.assignPath(vm.Globals[rootName], parts[1:], value)
		}
		if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
			if root, field, owner, ok := vm.lookupThisFieldRoot(this, parts[0]); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+field.Name, field.Modifiers); err != nil {
					return err
				}
				if field.Getter != nil {
					var err error
					root, err = vm.callGetter(owner, field, this)
					if err != nil {
						return err
					}
				}
				return vm.assignPath(root, parts[1:], value)
			}
		}
		if vm.currentClass != "" {
			if field, owner, ok := vm.lookupStaticField(vm.currentClass, parts[0]); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+parts[0], field.Modifiers); err != nil {
					return err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return err
				}
				field, _, _ = vm.lookupStaticField(owner, parts[0])
				root := field.Value
				if field.Getter != nil {
					var err error
					root, err = vm.callGetter(owner, field, Null)
					if err != nil {
						return err
					}
				}
				return vm.assignPath(root, parts[1:], value)
			}
		}
		if len(parts) > 2 {
			preferDependency := vm.currentMethod.Dependency || vm.classMemberReferenceUsesExplicitNamespace(strings.Join(parts[:2], "."), parts[0])
			if field, owner, ok := vm.lookupStaticFieldForReceiver(parts[0], parts[1], preferDependency); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+parts[1], field.Modifiers); err != nil {
					return err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return err
				}
				field, _, _ = vm.lookupStaticFieldForReceiver(owner, parts[1], preferDependency)
				root := field.Value
				if field.Getter != nil {
					var err error
					root, err = vm.callGetter(owner, field, Null)
					if err != nil {
						return err
					}
				}
				return vm.assignPath(root, parts[2:], value)
			}
		}
		if className, memberName, ok := vm.splitClassMember(name); ok {
			preferDependency := vm.classMemberReferenceUsesExplicitNamespace(name, className)
			if field, owner, ok := vm.lookupStaticFieldForReceiver(className, memberName, preferDependency); ok {
				if err := vm.checkMemberAccess(owner, field.Access, owner+"."+memberName, field.Modifiers); err != nil {
					return err
				}
				if err := vm.ensureClassInitialized(owner); err != nil {
					return err
				}
				field, _, _ = vm.lookupStaticFieldForReceiver(owner, memberName, preferDependency)
				fieldType := vm.resolveTypeNameInClass(owner, field.Type)
				coerced, err := vm.coerceAssignable(fieldType, value)
				if err != nil {
					return fmt.Errorf("%s.%s: %w", owner, memberName, err)
				}
				value = coerced
				if field.Setter != nil {
					storageName := staticFieldStorageName(memberName, field)
					key := owner + "." + storageName
					if vm.activeSetters[key] > 0 {
						class := vm.Classes[owner]
						vm.writeStaticFieldValue(owner, memberName, class, field, value)
						return nil
					}
					vm.activeSetters[key]++
					defer func() {
						vm.activeSetters[key]--
						if vm.activeSetters[key] == 0 {
							delete(vm.activeSetters, key)
						}
					}()
					_, err := vm.callMethod(*field.Setter, []Value{value}, resultForLookup())
					if err != nil {
						return err
					}
					if vm.isCurrentGetter(field.Getter) {
						class := vm.Classes[owner]
						vm.writeStaticFieldValue(owner, memberName, class, field, value)
					}
					return nil
				}
				class := vm.Classes[owner]
				vm.writeStaticFieldValue(owner, memberName, class, field, value)
				return nil
			}
		}
	}
	if this, ok := vm.Globals["this"]; ok && this.Kind == ValueObject {
		if this.Fields == nil {
			this.Fields = make(map[string]Value)
		}
		if actualName, field, ok := objectFieldValue(this, name); ok {
			if def, owner, ok := vm.lookupReceiverField(this.Type, actualName); ok {
				if err := vm.checkMemberAccess(owner, def.Access, owner+"."+name, def.Modifiers); err != nil {
					return err
				}
				fieldType := vm.resolveTypeNameInClass(owner, def.Type)
				coerced, err := vm.coerceAssignable(fieldType, value)
				if err != nil {
					return fmt.Errorf("%s.%s: %w", this.Type, name, err)
				}
				value = coerced
				if def.Setter != nil {
					if vm.isCurrentGetter(def.Getter) {
						vm.setObjectFieldValue(&this, actualName, value)
						vm.Globals["this"] = this
						return nil
					}
					key := activeInstanceSetterKey(owner, actualName, this)
					if vm.activeSetters[key] > 0 {
						vm.setObjectFieldValue(&this, actualName, value)
						vm.Globals["this"] = this
						return nil
					}
					vm.activeSetters[key]++
					defer func() {
						vm.activeSetters[key]--
						if vm.activeSetters[key] == 0 {
							delete(vm.activeSetters, key)
						}
					}()
					_, err := vm.callMethodWithReceiver(*def.Setter, this, []Value{value}, resultForLookup())
					if err != nil {
						return err
					}
					if vm.isCurrentGetter(def.Getter) {
						if updated, ok := vm.Globals["this"]; ok && updated.Kind == ValueObject {
							this = updated
						}
						vm.setObjectFieldValue(&this, actualName, value)
						vm.Globals["this"] = this
					}
					return nil
				}
			}
			_ = field
			vm.setObjectFieldValue(&this, actualName, value)
			vm.Globals["this"] = this
			return nil
		}
		if def, owner, ok := vm.lookupReceiverField(this.Type, name); ok {
			actualName := def.Name
			if actualName == "" {
				actualName = name
			}
			if err := vm.checkMemberAccess(owner, def.Access, owner+"."+actualName, def.Modifiers); err != nil {
				return err
			}
			fieldType := vm.resolveTypeNameInClass(owner, def.Type)
			coerced, err := vm.coerceAssignable(fieldType, value)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", this.Type, name, err)
			}
			value = coerced
			if def.Setter != nil {
				if vm.isCurrentGetter(def.Getter) {
					vm.setObjectFieldValue(&this, actualName, value)
					vm.Globals["this"] = this
					return nil
				}
				key := activeInstanceSetterKey(owner, actualName, this)
				if vm.activeSetters[key] > 0 {
					vm.setObjectFieldValue(&this, actualName, value)
					vm.Globals["this"] = this
					return nil
				}
				vm.activeSetters[key]++
				defer func() {
					vm.activeSetters[key]--
					if vm.activeSetters[key] == 0 {
						delete(vm.activeSetters, key)
					}
				}()
				_, err := vm.callMethodWithReceiver(*def.Setter, this, []Value{value}, resultForLookup())
				if err != nil {
					return err
				}
				if vm.isCurrentGetter(def.Getter) {
					if updated, ok := vm.Globals["this"]; ok && updated.Kind == ValueObject {
						this = updated
					}
					vm.setObjectFieldValue(&this, actualName, value)
					vm.Globals["this"] = this
				}
				return nil
			}
			vm.setObjectFieldValue(&this, actualName, value)
			vm.Globals["this"] = this
			return nil
		}
	}
	if vm.currentClass != "" {
		if field, owner, ok := vm.lookupStaticFieldForReceiver(vm.currentClass, name, vm.currentMethod.Dependency); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+name, field.Modifiers); err != nil {
				return err
			}
			if err := vm.ensureClassInitialized(owner); err != nil {
				return err
			}
			field, _, _ = vm.lookupStaticFieldForReceiver(owner, name, vm.currentMethod.Dependency)
			fieldType := vm.resolveTypeNameInClass(owner, field.Type)
			coerced, err := vm.coerceAssignable(fieldType, value)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", owner, name, err)
			}
			value = coerced
			if field.Setter != nil {
				storageName := staticFieldStorageName(name, field)
				key := owner + "." + storageName
				if vm.activeSetters[key] > 0 {
					class := vm.Classes[owner]
					vm.writeStaticFieldValue(owner, name, class, field, value)
					return nil
				}
				vm.activeSetters[key]++
				defer func() {
					vm.activeSetters[key]--
					if vm.activeSetters[key] == 0 {
						delete(vm.activeSetters, key)
					}
				}()
				_, err := vm.callMethod(*field.Setter, []Value{value}, resultForLookup())
				if err != nil {
					return err
				}
				if vm.isCurrentGetter(field.Getter) {
					class := vm.Classes[owner]
					vm.writeStaticFieldValue(owner, name, class, field, value)
				}
				return nil
			}
			class := vm.Classes[owner]
			vm.writeStaticFieldValue(owner, name, class, field, value)
			return nil
		}
	}
	return fmt.Errorf("unknown variable %q", name)
}

func (vm *VM) writeStaticFieldValue(owner, memberName string, class Class, field Field, value Value) {
	if vm.sharedStaticClasses {
		if mutable, ok := vm.ensureMutableClass(owner); ok {
			class = mutable
			if current, found := vm.lookupFieldInMap(class.StaticFields, memberName); found {
				field = current
			}
		}
	}
	previous := field.Value
	sameStaticCollection := sameStaticCollectionWriteback(previous, value)
	if !sameStaticCollection {
		vm.markCollectionRefsEscaped(value)
	}
	field.Value = value
	if class.StaticFields == nil {
		class.StaticFields = make(map[string]Field)
	}
	fieldKey := vm.staticFieldWritebackKey(owner, memberName, field)
	class.StaticFields[fieldKey] = field
	vm.storeClassValue(class)
	location := canonicalStaticFieldLocationForClass(class, owner, fieldKey)
	vm.replaceStaticValueRefsInField(previous, value, location)
}

func (vm *VM) assignPath(root Value, parts []string, value Value) error {
	if len(parts) == 0 {
		return fmt.Errorf("empty assignment target")
	}
	value = plainNull(value)
	previousRoot := snapshotAlias(root)
	current := root
	previousLeaf := aliasSnapshot{}
	type pathParent struct {
		object Value
		field  string
	}
	parents := make([]pathParent, 0, len(parts)-1)
	propagate := func(updated Value) {
		if previousLeaf.valid() {
			vm.propagateAliasSnapshotToScope(vm.Globals, previousLeaf, updated)
			vm.propagateAliasSnapshotToStatics(previousLeaf, updated)
		}
		for i := len(parents) - 1; i >= 0; i-- {
			parent := parents[i]
			if parent.object.Kind == ValueObject && parent.object.Fields != nil {
				vm.setGraphFieldValue(&parent.object, parent.field, updated)
			}
			updated = parent.object
		}
		vm.propagateAliasSnapshotToScope(vm.Globals, previousRoot, updated)
		vm.propagateAliasSnapshotToStatics(previousRoot, updated)
	}
	for i, part := range parts[:len(parts)-1] {
		if current.Kind == ValueNull {
			return newNullDereferenceError("while assigning " + strings.Join(parts[:i+1], "."))
		}
		if current.Kind != ValueObject {
			return fmt.Errorf("cannot assign field %s on %s", part, current.Kind)
		}
		if field, owner, ok := vm.lookupReceiverField(current.Type, part); ok {
			if err := vm.checkMemberAccess(owner, field.Access, owner+"."+part, field.Modifiers); err != nil {
				return err
			}
			if field.Getter != nil {
				next, err := vm.callGetter(owner, field, current)
				if err != nil {
					return err
				}
				if current.Ref != 0 {
					if refreshed, ok := vm.findValueByRef(current.Ref); ok {
						current = refreshed
					}
				}
				if next.Kind != ValueObject {
					return fmt.Errorf("unknown field %q on %s", part, current.Type)
				}
				actualName := field.Name
				if actualName == "" {
					actualName = part
				}
				parents = append(parents, pathParent{object: current, field: actualName})
				current = next
				continue
			}
		}
		actualName := vm.resolveSObjectFieldName(current.Type, part)
		next, ok := current.Fields[actualName]
		if !ok {
			var resolved string
			resolved, next, ok = objectFieldValue(current, part)
			if ok {
				actualName = resolved
			}
		}
		if !ok || next.Kind != ValueObject {
			if generated, generatedOK := vm.generatedPlatformInstanceField(current, part); generatedOK && generated.Kind == ValueObject {
				vm.setGraphFieldValue(&current, part, generated)
				parents = append(parents, pathParent{object: current, field: part})
				current = generated
				continue
			}
			return fmt.Errorf("unknown field %q on %s", part, current.Type)
		}
		parents = append(parents, pathParent{object: current, field: actualName})
		current = next
	}
	fieldName := parts[len(parts)-1]
	if current.Kind == ValueNull {
		return newNullDereferenceError("while assigning " + strings.Join(parts, "."))
	}
	if current.Kind != ValueObject {
		return fmt.Errorf("cannot assign field %s on %s", fieldName, current.Kind)
	}
	previousLeaf = snapshotAlias(current)
	if reason, ok := sobjectReadOnlyReason(current); ok {
		return fmt.Errorf("cannot modify read-only %s", reason)
	}
	if vm.isSObjectLikeType(current.Type) && vm.sObjectParentRelationshipField(current.Type, fieldName) {
		vm.markCollectionRefsEscaped(value)
		vm.setExplicitSObjectFieldValue(&current, fieldName, value)
		markSetSObjectField(&current, fieldName)
		markUserSetSObjectField(&current, fieldName)
		markQueriedSObjectField(&current, fieldName)
		propagate(current)
		return nil
	}
	if vm.isSObjectLikeType(current.Type) && value.Kind == ValueObject && vm.isSObjectLikeType(value.Type) {
		if definition, fieldDef, exists := vm.sObjectFieldDefinition(current.Type, fieldName); exists && fieldDef.Type == storage.FieldReference {
			relationshipName := vm.parentRelationshipNameForReferenceField(definition, fieldDef)
			if relationshipName != "" {
				vm.markCollectionRefsEscaped(value)
				vm.setExplicitSObjectFieldValue(&current, relationshipName, value)
				markSetSObjectField(&current, relationshipName)
				markUserSetSObjectField(&current, relationshipName)
				markQueriedSObjectField(&current, relationshipName)
				propagate(current)
				return nil
			}
		}
	}
	if def, owner, ok := vm.lookupReceiverField(current.Type, fieldName); ok {
		actualName := def.Name
		if actualName == "" {
			actualName = fieldName
		}
		if vm.Org != nil {
			if objectName, ok := vm.resolveObjectName(current.Type); ok {
				definition := vm.Org.Objects[objectName].Definition
				if canonical, ok := vm.resolveFieldName(definition, actualName); ok {
					actualName = canonical
					value = coerceSObjectFieldRuntimeValue(value, definition.Fields[canonical])
				}
			}
		}
		if err := vm.checkMemberAccess(owner, def.Access, owner+"."+actualName, def.Modifiers); err != nil {
			return err
		}
		fieldType := vm.resolveTypeNameInClass(owner, def.Type)
		coerced, err := vm.coerceAssignable(fieldType, value)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", current.Type, fieldName, err)
		}
		value = coerced
		if def.Setter != nil {
			key := activeInstanceSetterKey(owner, actualName, current)
			if vm.activeSetters[key] > 0 {
				if vm.isSObjectLikeType(current.Type) {
					vm.markCollectionRefsEscaped(value)
					vm.setExplicitSObjectFieldValue(&current, actualName, value)
					markSetSObjectField(&current, actualName)
					markUserSetSObjectField(&current, actualName)
					markQueriedSObjectField(&current, actualName)
				} else {
					vm.setObjectFieldValue(&current, actualName, value)
				}
				propagate(current)
				return nil
			}
			vm.activeSetters[key]++
			defer func() {
				vm.activeSetters[key]--
				if vm.activeSetters[key] == 0 {
					delete(vm.activeSetters, key)
				}
			}()
			_, err := vm.callMethodWithReceiver(*def.Setter, current, []Value{value}, resultForLookup())
			if err == nil {
				if current.Ref != 0 {
					if refreshed, ok := vm.findValueByRef(current.Ref); ok {
						current = refreshed
					}
				}
				propagate(current)
			}
			return err
		}
		if vm.isSObjectLikeType(current.Type) {
			vm.markCollectionRefsEscaped(value)
			vm.setExplicitSObjectFieldValue(&current, actualName, value)
			markSetSObjectField(&current, actualName)
			markUserSetSObjectField(&current, actualName)
			markQueriedSObjectField(&current, actualName)
		} else {
			vm.setObjectFieldValue(&current, actualName, value)
		}
		propagate(current)
		return nil
	}
	if def, _, ok := vm.generatedPlatformField(current.Type, fieldName, false); ok {
		actualName := def.Name
		if actualName == "" {
			actualName = fieldName
		}
		coerced, err := vm.coerceAssignable(def.Type, value)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", current.Type, fieldName, err)
		}
		vm.markCollectionRefsEscaped(coerced)
		vm.setGraphFieldValue(&current, actualName, coerced)
		syncDatabaseOptionAliasField(&current, actualName, coerced)
		propagate(current)
		return nil
	}
	resolvedField := vm.resolveSObjectFieldName(current.Type, fieldName)
	if actualName, _, ok := objectFieldValue(current, resolvedField); ok {
		resolvedField = actualName
	}
	if vm.Org != nil {
		if objectName, ok := vm.resolveObjectName(current.Type); ok {
			definition := vm.Org.Objects[objectName].Definition
			if canonical, ok := vm.resolveFieldName(definition, resolvedField); ok {
				resolvedField = canonical
				value = coerceSObjectFieldRuntimeValue(value, definition.Fields[canonical])
			}
		}
	}
	vm.markCollectionRefsEscaped(value)
	vm.setExplicitSObjectFieldValue(&current, resolvedField, value)
	markSetSObjectField(&current, resolvedField)
	markUserSetSObjectField(&current, resolvedField)
	markQueriedSObjectField(&current, resolvedField)
	propagate(current)
	return nil
}
