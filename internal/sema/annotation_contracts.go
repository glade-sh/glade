package sema

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexlang"
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

var (
	invocableDecimalValue    = regexp.MustCompile(`^[+-]?(?:\d+(?:\.\d*)?|\.\d+)$`)
	invocableCapabilityValue = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*://[A-Za-z][A-Za-z0-9_]*$`)
)

func checkAnnotationCatalog(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) {
			continue
		}
		diagnostics = append(diagnostics, annotationCatalogDiagnostics(typ.File, typ.Range, typ.Annotations)...)
		for _, member := range typ.Members {
			if member.Kind == apexast.DeclarationClass || member.Kind == apexast.DeclarationInterface || member.Kind == apexast.DeclarationEnum {
				continue
			}
			diagnostics = append(diagnostics, annotationCatalogDiagnostics(typ.File, member.Range, member.Annotations)...)
			for _, parameter := range member.Parameters {
				diagnostics = append(diagnostics, annotationCatalogDiagnostics(typ.File, parameter.Range, parameter.Annotations)...)
			}
			for _, accessor := range member.Accessors {
				diagnostics = append(diagnostics, annotationCatalogDiagnostics(typ.File, accessor.Range, accessor.Annotations)...)
			}
		}
	}
	return diagnostics
}

func annotationCatalogDiagnostic(file string, r diagnostic.Range, detail string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA031", Message: detail, File: file, Range: &r}
}

func annotationCatalogDiagnostics(file string, fallback diagnostic.Range, annotations []apexast.Annotation) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, annotation := range annotations {
		spec, ok := apexlang.LookupAnnotation(annotation.Name)
		r := annotation.Range
		if r.End.Offset <= r.Start.Offset {
			r = fallback
		}
		if !ok {
			diagnostics = append(diagnostics, annotationCatalogDiagnostic(file, r, fmt.Sprintf("unknown Apex annotation @%s", annotation.Name)))
			continue
		}
		if spec.Preview {
			diagnostics = append(diagnostics, annotationCatalogDiagnostic(file, r, fmt.Sprintf("Apex annotation @%s is preview-disabled", annotation.Name)))
		}
		seenArguments := make(map[string]bool, len(annotation.Arguments))
		positionalArguments := 0
		for _, argument := range annotation.Arguments {
			if argument.Name == "" {
				positionalArguments++
				if positionalArguments > spec.MaxPositionalArguments {
					diagnostics = append(diagnostics, annotationCatalogDiagnostic(file, argument.Range, fmt.Sprintf("annotation @%s does not support positional arguments", annotation.Name)))
				} else if spec.PositionalArgumentLiteral {
					if _, ok := apexStringLiteralValue(argument.Value); !ok {
						diagnostics = append(diagnostics, annotationCatalogDiagnostic(file, argument.Range, fmt.Sprintf("annotation @%s requires a string literal argument", annotation.Name)))
					}
				}
				continue
			}
			name := strings.ToLower(argument.Name)
			if seenArguments[name] {
				diagnostics = append(diagnostics, annotationCatalogDiagnostic(file, argument.Range, fmt.Sprintf("annotation @%s repeats property %q", annotation.Name, argument.Name)))
				continue
			}
			seenArguments[name] = true
			kind, allowed := spec.Arguments[name]
			if !allowed {
				diagnostics = append(diagnostics, annotationCatalogDiagnostic(file, argument.Range, fmt.Sprintf("annotation @%s does not support property %q", annotation.Name, argument.Name)))
				continue
			}
			switch kind {
			case apexlang.AnnotationStringArgument:
				if _, ok := apexStringLiteralValue(argument.Value); !ok {
					diagnostics = append(diagnostics, annotationCatalogDiagnostic(file, argument.Range, fmt.Sprintf("annotation @%s property %q requires a string literal", annotation.Name, argument.Name)))
				}
			case apexlang.AnnotationBooleanArgument:
				if !strings.EqualFold(strings.TrimSpace(argument.Value), "true") && !strings.EqualFold(strings.TrimSpace(argument.Value), "false") {
					diagnostics = append(diagnostics, annotationCatalogDiagnostic(file, argument.Range, fmt.Sprintf("annotation @%s property %q requires a Boolean literal", annotation.Name, argument.Name)))
				}
			}
		}
	}
	return diagnostics
}

// checkAnnotationContracts validates the rules that depend on the declaration
// carrying an annotation. The legacy modifier text remains available for
// compatibility, but new checks read only the structured annotation payload.
func checkAnnotationContracts(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) {
			continue
		}
		diagnostics = append(diagnostics, checkTypeAnnotationContracts(typ)...)
		testSetups := 0
		invocableMethods := 0
		auraMethods := map[string]int{}
		for _, member := range typ.Members {
			if member.Kind == apexast.DeclarationClass || member.Kind == apexast.DeclarationInterface || member.Kind == apexast.DeclarationEnum {
				continue
			}
			for _, annotation := range member.Annotations {
				if strings.EqualFold(annotation.Name, "TestSetup") {
					testSetups++
				}
				if strings.EqualFold(annotation.Name, "AuraEnabled") && member.Kind == apexast.DeclarationMethod {
					auraMethods[strings.ToLower(member.Name)]++
				}
				if strings.EqualFold(annotation.Name, "InvocableMethod") && member.Kind == apexast.DeclarationMethod {
					invocableMethods++
				}
			}
			diagnostics = append(diagnostics, checkMemberAnnotationContracts(typ, member)...)
			diagnostics = append(diagnostics, checkInvocableParameterConstructors(index, typ, member)...)
		}
		if testSetups > 1 {
			diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, typ.Range, "an IsTest class can declare only one TestSetup method"))
		}
		if invocableMethods > 1 {
			diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, typ.Range, "a class can declare only one InvocableMethod"))
		}
		if testSetups > 0 && annotationPropertyTrue(typ.Annotations, "IsTest", "SeeAllData") {
			diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, typ.Range, "TestSetup cannot be combined with IsTest(SeeAllData=true)"))
		}
		for name, count := range auraMethods {
			if count > 1 {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, typ.Range, fmt.Sprintf("AuraEnabled methods cannot be overloaded: %s", name)))
			}
		}
	}
	return diagnostics
}

func checkInvocableParameterConstructors(index typesys.Index, owner typesys.TypeSymbol, member typesys.MemberSymbol) []diagnostic.Diagnostic {
	if apexversion.Before(owner.EffectiveAPIVersion, 66) || !hasAnnotation(member.Annotations, "InvocableMethod") {
		return nil
	}
	typeNames := []string{member.Type}
	for _, parameter := range member.Parameters {
		typeNames = append(typeNames, parameter.Type)
	}
	seen := map[string]bool{}
	for _, typeName := range typeNames {
		arguments, ok := genericTypeArguments(typeName)
		if !ok || len(arguments) != 1 {
			continue
		}
		name := normalizeName(strings.TrimSpace(arguments[0]))
		if seen[name] {
			continue
		}
		seen[name] = true
		for _, candidate := range index.Types {
			if candidate.Kind != apexast.DeclarationClass || normalizeName(candidate.Name) != name {
				continue
			}
			if !invocableTypeHasVisibleNoArgConstructor(candidate, owner.Namespace) {
				return []diagnostic.Diagnostic{annotationContractDiagnostic(owner.File, member.Range, fmt.Sprintf("InvocableMethod parameter type %s requires a visible no-argument constructor at API version 66.0 or later", candidate.Name))}
			}
		}
	}
	return nil
}

func invocableTypeHasVisibleNoArgConstructor(typ typesys.TypeSymbol, ownerNamespace string) bool {
	visible := func(modifiers []string) bool {
		return hasModifier(modifiers, "global") || typ.Namespace == ownerNamespace && hasModifier(modifiers, "public")
	}
	hasExplicit := false
	for _, member := range typ.Members {
		if member.Kind != apexast.DeclarationConstructor {
			continue
		}
		hasExplicit = true
		if len(member.Parameters) == 0 && visible(member.Modifiers) {
			return true
		}
	}
	return !hasExplicit && visible(typ.Modifiers)
}

func checkTypeAnnotationContracts(typ typesys.TypeSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, annotation := range typ.Annotations {
		switch {
		case strings.EqualFold(annotation.Name, "IsTest"):
			if typ.Kind != apexast.DeclarationClass || typ.NestingDepth != 0 || !isTestTypeArgumentsAllowed(typ, annotation) {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "IsTest is only valid on top-level classes with supported properties"))
			}
		case strings.EqualFold(annotation.Name, "JsonAccess"):
			if typ.Kind != apexast.DeclarationClass || !jsonAccessArgumentsAllowed(annotation) {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "JsonAccess is only valid on classes with supported serialization modes"))
			}
		case strings.EqualFold(annotation.Name, "NamespaceAccessible"):
			validKind := typ.Kind == apexast.DeclarationClass || typ.Kind == apexast.DeclarationInterface || typ.Kind == apexast.DeclarationEnum
			if annotationAPIVersionAtLeast(typ, 50) && (!validKind || !hasEitherModifier(typ.Modifiers, "public", "global")) {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "NamespaceAccessible is only valid on public or global classes, interfaces, and enums at API version 50.0 or later"))
			}
		case strings.EqualFold(annotation.Name, "RestResource"):
			if typ.Kind != apexast.DeclarationClass {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "RestResource is only valid on classes"))
			}
		case strings.EqualFold(annotation.Name, "AuraEnabled"):
			diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "AuraEnabled is only valid on methods, fields, and properties"))
		}
	}
	return diagnostics
}

func checkMemberAnnotationContracts(typ typesys.TypeSymbol, member typesys.MemberSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, annotation := range member.Annotations {
		switch {
		case strings.EqualFold(annotation.Name, "IsTest"):
			if member.Kind != apexast.DeclarationMethod || !typ.IsTest || !hasModifier(member.Modifiers, "static") || len(member.Parameters) != 0 || !isTestMethodArgumentsAllowed(typ, annotation) {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "IsTest methods must be static no-argument methods inside an IsTest class"))
			}
		case strings.EqualFold(annotation.Name, "TestSetup"):
			if member.Kind != apexast.DeclarationMethod || !typ.IsTest || !hasModifier(member.Modifiers, "static") || !strings.EqualFold(member.Type, "void") || len(member.Parameters) != 0 || len(annotation.Arguments) != 0 {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "TestSetup methods must be static void no-argument methods inside an IsTest class"))
			}
		case strings.EqualFold(annotation.Name, "future"):
			if member.Kind != apexast.DeclarationMethod || !hasModifier(member.Modifiers, "static") || !strings.EqualFold(member.Type, "void") || !annotationBooleanArguments(annotation, "callout") || !futureParametersAllowed(member.Parameters) {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "future methods must be static void methods with supported parameter types"))
			}
		case strings.EqualFold(annotation.Name, "AuraEnabled"):
			validTarget := member.Kind == apexast.DeclarationMethod || member.Kind == apexast.DeclarationField || member.Kind == apexast.DeclarationProperty
			if !validTarget || !auraEnabledArgumentsAllowed(typ, annotation) {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "AuraEnabled is only valid on methods, fields, and properties with supported property values"))
			}
		case strings.EqualFold(annotation.Name, "InvocableMethod"):
			if member.Kind != apexast.DeclarationMethod || !hasEitherModifier(member.Modifiers, "public", "global") || !hasModifier(member.Modifiers, "static") || (len(member.Parameters) == 1 && !isListType(member.Parameters[0].Type)) || len(member.Parameters) > 1 || (!strings.EqualFold(member.Type, "void") && !isListType(member.Type)) || !invocableMethodArgumentsAllowed(annotation) {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "InvocableMethod must be a public or global static method with zero or one List parameter and a void or List return type"))
			}
			for _, other := range member.Annotations {
				if !strings.EqualFold(other.Name, "InvocableMethod") && !strings.EqualFold(other.Name, "Deprecated") {
					diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "InvocableMethod can only be combined with Deprecated"))
					break
				}
			}
		case strings.EqualFold(annotation.Name, "InvocableVariable"):
			if member.Kind != apexast.DeclarationField || !hasEitherModifier(member.Modifiers, "public", "global") || hasModifier(member.Modifiers, "static") || hasModifier(member.Modifiers, "final") || strings.EqualFold(member.Type, "Object") || !invocableVariableArgumentsAllowed(member.Type, annotation) {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "InvocableVariable must annotate a public or global nonstatic nonfinal field"))
			}
		case strings.EqualFold(annotation.Name, "RemoteAction"):
			if member.Kind != apexast.DeclarationMethod || !hasEitherModifier(member.Modifiers, "public", "global") || !hasModifier(member.Modifiers, "static") {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "RemoteAction must annotate a public or global static method"))
			}
		case strings.EqualFold(annotation.Name, "ReadOnly"):
			valid := member.Kind == apexast.DeclarationMethod && hasEitherModifier(member.Modifiers, "public", "global")
			if valid && hasModifier(member.Modifiers, "static") {
				valid = hasAnnotation(typ.Annotations, "RestResource") && memberHasRESTVerb(member)
				if apexversion.Before(typ.EffectiveAPIVersion, 49) {
					valid = valid && hasAnnotation(member.Annotations, "RemoteAction")
				}
			}
			if !valid {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "ReadOnly is only valid on public or global instance methods, or REST methods"))
			}
		case strings.EqualFold(annotation.Name, "JsonAccess"):
			diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "JsonAccess is only valid on classes with serialization control properties"))
		case strings.EqualFold(annotation.Name, "NamespaceAccessible"):
			if annotationAPIVersionAtLeast(typ, 50) && (typ.Kind == apexast.DeclarationInterface || (!hasAnnotation(typ.Annotations, "NamespaceAccessible") && !hasModifier(typ.Modifiers, "global")) || hasAnnotation(member.Annotations, "AuraEnabled") || hasAnnotation(member.Annotations, "InvocableMethod")) {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "NamespaceAccessible methods require a NamespaceAccessible or global owner and cannot be combined with AuraEnabled or InvocableMethod"))
			}
		}
	}
	return diagnostics
}

func hasAnnotation(annotations []apexast.Annotation, name string) bool {
	for _, annotation := range annotations {
		if strings.EqualFold(annotation.Name, name) {
			return true
		}
	}
	return false
}

func memberHasRESTVerb(member typesys.MemberSymbol) bool {
	for _, annotation := range member.Annotations {
		if restVerb(annotation.Name) != "" {
			return true
		}
	}
	return false
}

func annotationContractDiagnostic(file string, r diagnostic.Range, detail string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA032", Message: detail, File: file, Range: &r}
}

func annotationBooleanArguments(annotation apexast.Annotation, names ...string) bool {
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[strings.ToLower(name)] = struct{}{}
	}
	for _, argument := range annotation.Arguments {
		if argument.Name == "" {
			return false
		}
		if _, ok := allowed[strings.ToLower(argument.Name)]; !ok || (!strings.EqualFold(argument.Value, "true") && !strings.EqualFold(argument.Value, "false")) {
			return false
		}
	}
	return true
}

func jsonAccessArgumentsAllowed(annotation apexast.Annotation) bool {
	if len(annotation.Arguments) == 0 {
		return false
	}
	for _, argument := range annotation.Arguments {
		if !strings.EqualFold(argument.Name, "serializable") && !strings.EqualFold(argument.Name, "deserializable") {
			return false
		}
		value, ok := apexStringLiteralValue(argument.Value)
		if !ok {
			return false
		}
		switch strings.ToLower(value) {
		case "always", "samenamespace", "samepackage", "never":
		default:
			return false
		}
	}
	return true
}

func annotationAPIVersionAtLeast(typ typesys.TypeSymbol, minimum int) bool {
	return apexversion.AtLeast(typ.EffectiveAPIVersion, minimum)
}

func annotationAPIVersionBefore(typ typesys.TypeSymbol, minimum int) bool {
	return apexversion.Before(typ.EffectiveAPIVersion, minimum)
}

func isTestMethodArgumentsAllowed(typ typesys.TypeSymbol, annotation apexast.Annotation) bool {
	return annotationBooleanArguments(annotation, "SeeAllData") && (len(annotation.Arguments) == 0 || !annotationAPIVersionBefore(typ, 24))
}

func invocableMethodArgumentsAllowed(annotation apexast.Annotation) bool {
	for _, argument := range annotation.Arguments {
		if !strings.EqualFold(argument.Name, "capabilityType") {
			continue
		}
		value, ok := apexStringLiteralValue(argument.Value)
		if !ok || !invocableCapabilityValue.MatchString(value) {
			return false
		}
	}
	return true
}

func invocableVariableArgumentsAllowed(typeName string, annotation apexast.Annotation) bool {
	var defaultValue, placeholderText *string
	requiredPresent := false
	for _, argument := range annotation.Arguments {
		switch strings.ToLower(argument.Name) {
		case "defaultvalue":
			value, ok := apexStringLiteralValue(argument.Value)
			if !ok {
				return false
			}
			defaultValue = &value
		case "placeholdertext":
			value, ok := apexStringLiteralValue(argument.Value)
			if !ok {
				return false
			}
			placeholderText = &value
		case "required":
			requiredPresent = true
		}
	}
	if defaultValue != nil && requiredPresent {
		return false
	}
	canonicalType := semaCanonicalPlatformAlias(strings.TrimSpace(typeName))
	base, _ := semaGenericBaseAndArgs(canonicalType)
	if defaultValue != nil && !invocableVariableTextValueAllowed(base, *defaultValue) {
		return false
	}
	if placeholderText != nil && !invocableVariableTextValueAllowed(base, *placeholderText) {
		return false
	}
	return true
}

func invocableVariableTextValueAllowed(typeName, value string) bool {
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "string":
		return true
	case "integer", "int":
		_, err := strconv.ParseInt(value, 10, 32)
		return err == nil
	case "double":
		if len(value) < 2 || value[len(value)-1] != 'd' && value[len(value)-1] != 'D' {
			return false
		}
		return invocableDecimalValue.MatchString(value[:len(value)-1])
	case "boolean":
		return strings.EqualFold(value, "true") || strings.EqualFold(value, "false")
	case "decimal":
		return invocableDecimalValue.MatchString(value)
	case "long":
		if len(value) < 2 || value[len(value)-1] != 'l' && value[len(value)-1] != 'L' {
			return false
		}
		_, err := strconv.ParseInt(value[:len(value)-1], 10, 64)
		return err == nil
	default:
		return false
	}
}

func isTestTypeArgumentsAllowed(typ typesys.TypeSymbol, annotation apexast.Annotation) bool {
	for _, argument := range annotation.Arguments {
		switch strings.ToLower(argument.Name) {
		case "seealldata", "isparallel", "oninstall":
			if !strings.EqualFold(argument.Value, "true") && !strings.EqualFold(argument.Value, "false") {
				return false
			}
		case "critical":
			if !strings.EqualFold(argument.Value, "true") && !strings.EqualFold(argument.Value, "false") {
				return false
			}
			if apexversion.Before(typ.EffectiveAPIVersion, 66) {
				return false
			}
		case "testfor":
			if _, ok := apexStringLiteralValue(argument.Value); !ok {
				return false
			}
			if apexversion.Before(typ.EffectiveAPIVersion, 66) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func auraEnabledArgumentsAllowed(typ typesys.TypeSymbol, annotation apexast.Annotation) bool {
	for _, argument := range annotation.Arguments {
		switch strings.ToLower(argument.Name) {
		case "cacheable":
			if !strings.EqualFold(argument.Value, "true") && !strings.EqualFold(argument.Value, "false") {
				return false
			}
		case "scope":
			if !strings.EqualFold(strings.Trim(argument.Value, "'\""), "global") {
				return false
			}
			if apexversion.Before(typ.EffectiveAPIVersion, 55) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func futureParametersAllowed(parameters []apexast.Parameter) bool {
	for _, parameter := range parameters {
		name := strings.ToLower(strings.ReplaceAll(parameter.Type, " ", ""))
		if strings.HasSuffix(name, "[]") {
			name = strings.TrimSuffix(name, "[]")
			if !futurePrimitiveType(name) {
				return false
			}
			continue
		}
		if strings.HasPrefix(name, "list<") || strings.HasPrefix(name, "set<") {
			arguments, ok := genericTypeArguments(name)
			if !ok || len(arguments) != 1 || !futurePrimitiveType(arguments[0]) {
				return false
			}
			continue
		}
		if strings.HasPrefix(name, "map<") {
			arguments, ok := genericTypeArguments(name)
			if !ok || len(arguments) != 2 || !futurePrimitiveType(arguments[0]) || !futurePrimitiveType(arguments[1]) {
				return false
			}
			continue
		}
		if !futurePrimitiveType(name) {
			return false
		}
	}
	return true
}

func futurePrimitiveType(name string) bool {
	switch name {
	case "blob", "boolean", "date", "datetime", "decimal", "double", "id", "integer", "long", "string", "time":
		return true
	default:
		return false
	}
}

func genericTypeArguments(name string) ([]string, bool) {
	open := strings.IndexByte(name, '<')
	if open < 1 || !strings.HasSuffix(name, ">") {
		return nil, false
	}
	body := name[open+1 : len(name)-1]
	if body == "" || strings.ContainsAny(body, "<>") {
		return nil, false
	}
	return strings.Split(body, ","), true
}

func isListType(name string) bool {
	return strings.HasPrefix(strings.ToLower(strings.ReplaceAll(name, " ", "")), "list<")
}

func annotationPropertyTrue(annotations []apexast.Annotation, annotationName, propertyName string) bool {
	for _, annotation := range annotations {
		if !strings.EqualFold(annotation.Name, annotationName) {
			continue
		}
		for _, argument := range annotation.Arguments {
			if strings.EqualFold(argument.Name, propertyName) && strings.EqualFold(argument.Value, "true") {
				return true
			}
		}
	}
	return false
}
