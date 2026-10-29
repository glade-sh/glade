package vm

import "strings"

// Collection element types resolve where the literal is evaluated. Native
// generic tokens also display the lexical owner of an inner element type.
// Nongeneric shadow literals keep their original display spelling.
func (vm *VM) lexicalClassLiteralToken(name string, token Value) Value {
	token.dependencyClassLiteral = true
	if platform, ok := vm.scalarPlatformClassLiteral(name); ok {
		token.Text = platform
		if token.Fields == nil {
			token.Fields = make(map[string]Value)
		}
		token.Fields[reflectionTypeIdentityField] = String(platform)
		return token
	}
	if _, generic := genericTypeArgs(name); generic && vm.currentClass != "" {
		identity := vm.resolveTypeNameInClass(vm.currentClass, name)
		if !strings.EqualFold(identity, name) {
			token.Text = vm.typeDisplayName(identity)
			if token.Fields == nil {
				token.Fields = make(map[string]Value)
			}
			token.Fields[reflectionTypeIdentityField] = String(identity)
			token.lexicalClassLiteral = true
		}
		return token
	}
	if strings.Contains(name, ".") || vm.currentClass == "" || !isCommonSObjectTypeName(name) {
		return token
	}
	identity, ok := vm.resolveNestedTypeInClassHierarchy(vm.currentClass, name)
	if !ok {
		return token
	}
	class, ok := vm.lookupClass(identity)
	if !ok || class.PlatformType {
		return token
	}
	if token.Fields == nil {
		token.Fields = make(map[string]Value)
	}
	token.Fields[reflectionTypeIdentityField] = String(runtimeClassName(class))
	token.lexicalClassLiteral = true
	return token
}

func (vm *VM) scalarPlatformClassLiteral(name string) (string, bool) {
	var platform string
	switch strings.ToLower(name) {
	case "location", "system.location":
		platform = "System.Location"
	case "selectoption", "system.selectoption":
		platform = "System.SelectOption"
	default:
		var ok bool
		platform, ok = schemaMetadataClassAlias(name)
		if !ok {
			return "", false
		}
	}
	resolved := vm.resolveTypeNameInClass(vm.currentClass, name)
	if class, ok := vm.lookupClass(resolved); ok && !class.PlatformType {
		return "", false
	}
	return platform, true
}

// Native API 62/67 rows require canonical names and Type identity for these
// aliases. Other schema types retain their existing reflection behavior.
func schemaMetadataClassAlias(name string) (string, bool) {
	switch strings.ToLower(name) {
	case "describefieldresult", "schema.describefieldresult":
		return "Schema.DescribeFieldResult", true
	case "describesobjectresult", "schema.describesobjectresult":
		return "Schema.DescribeSObjectResult", true
	case "sobjecttype", "schema.sobjecttype":
		return "Schema.SObjectType", true
	case "sobjectfield", "schema.sobjectfield":
		return "Schema.SObjectField", true
	default:
		return "", false
	}
}

// JSON uses its existing target spelling except for lexical class literals.
// Other reflection identities can contain schema qualification that the JSON
// mapper does not accept.
func jsonDeserializeTypeName(token Value) string {
	if token.lexicalClassLiteral {
		return typeValueIdentityName(token)
	}
	return typeValueName(token)
}
