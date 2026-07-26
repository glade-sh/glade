package sema

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexlang"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func checkAnnotationCatalog(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) {
			continue
		}
		diagnostics = append(diagnostics, annotationCatalogDiagnostics(typ.File, typ.Range, typ.Annotations)...)
		for _, member := range typ.Members {
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
		for _, argument := range annotation.Arguments {
			if argument.Name == "" {
				continue
			}
			if _, allowed := spec.AllowedArguments[strings.ToLower(argument.Name)]; !allowed {
				diagnostics = append(diagnostics, annotationCatalogDiagnostic(file, argument.Range, fmt.Sprintf("annotation @%s does not support property %q", annotation.Name, argument.Name)))
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

func checkTypeAnnotationContracts(typ typesys.TypeSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, annotation := range typ.Annotations {
		switch {
		case strings.EqualFold(annotation.Name, "IsTest"):
			if typ.Kind != apexast.DeclarationClass || typ.NestingDepth != 0 || !annotationBooleanArguments(annotation, "seealldata", "isparallel", "oninstall") {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "IsTest is only valid on top-level classes with boolean properties"))
			}
		case strings.EqualFold(annotation.Name, "JsonAccess"):
			if typ.Kind != apexast.DeclarationClass || len(annotation.Arguments) == 0 || !annotationBooleanArguments(annotation, "serializable", "deserializable") {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "JsonAccess is only valid on classes with boolean properties"))
			}
		case strings.EqualFold(annotation.Name, "NamespaceAccessible"):
			if typ.Kind != apexast.DeclarationClass || !hasEitherModifier(typ.Modifiers, "public", "global") {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "NamespaceAccessible is only valid on public or global classes"))
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
			if member.Kind != apexast.DeclarationMethod || !typ.IsTest || !hasModifier(member.Modifiers, "static") || len(member.Parameters) != 0 || len(annotation.Arguments) != 0 {
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
			if member.Kind != apexast.DeclarationMethod || !hasEitherModifier(member.Modifiers, "public", "global") || !hasModifier(member.Modifiers, "static") || len(member.Parameters) != 1 || !isListType(member.Parameters[0].Type) || (!strings.EqualFold(member.Type, "void") && !isListType(member.Type)) {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "InvocableMethod must be a public or global static method with one List parameter and a void or List return type"))
			}
			for _, other := range member.Annotations {
				if !strings.EqualFold(other.Name, "InvocableMethod") && !strings.EqualFold(other.Name, "Deprecated") {
					diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "InvocableMethod can only be combined with Deprecated"))
					break
				}
			}
		case strings.EqualFold(annotation.Name, "InvocableVariable"):
			if member.Kind != apexast.DeclarationField || !hasEitherModifier(member.Modifiers, "public", "global") || hasModifier(member.Modifiers, "static") || hasModifier(member.Modifiers, "final") || strings.EqualFold(member.Type, "Object") {
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
				if version, err := strconv.ParseFloat(strings.TrimSpace(typ.EffectiveAPIVersion), 64); err == nil && version < 49 {
					valid = valid && hasAnnotation(member.Annotations, "RemoteAction")
				}
			}
			if !valid {
				diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "ReadOnly is only valid on public or global instance methods, or REST methods"))
			}
		case strings.EqualFold(annotation.Name, "JsonAccess"):
			diagnostics = append(diagnostics, annotationContractDiagnostic(typ.File, annotation.Range, "JsonAccess is only valid on classes with serialization control properties"))
		case strings.EqualFold(annotation.Name, "NamespaceAccessible"):
			if (!hasAnnotation(typ.Annotations, "NamespaceAccessible") && !hasModifier(typ.Modifiers, "global")) || hasAnnotation(member.Annotations, "AuraEnabled") || hasAnnotation(member.Annotations, "InvocableMethod") {
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
		allowed[name] = struct{}{}
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
			if version, err := strconv.ParseFloat(strings.TrimSpace(typ.EffectiveAPIVersion), 64); err == nil && version < 55 {
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
