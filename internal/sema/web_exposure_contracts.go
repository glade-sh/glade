package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func checkWebExposureContracts(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) {
			continue
		}
		diagnostics = append(diagnostics, checkSOAPExposureContracts(typ)...)
		rest, isRest := findAnnotation(typ.Annotations, "RestResource")
		if !isRest {
			continue
		}
		if typ.Kind != apexast.DeclarationClass || typ.NestingDepth != 0 || !hasModifier(typ.Modifiers, "global") || !validRestMapping(rest) {
			diagnostics = append(diagnostics, webExposureDiagnostic(typ.File, rest.Range, "RestResource requires a top-level global class and a valid urlMapping"))
		}
		verbs := map[string]int{}
		for _, member := range typ.Members {
			for _, annotation := range member.Annotations {
				verb := restVerb(annotation.Name)
				if verb == "" {
					continue
				}
				verbs[verb]++
				if member.Kind != apexast.DeclarationMethod || !hasModifier(member.Modifiers, "global") || !hasModifier(member.Modifiers, "static") || ((verb == "get" || verb == "delete") && len(member.Parameters) != 0) || !validRESTMethodTypes(member) {
					diagnostics = append(diagnostics, webExposureDiagnostic(typ.File, annotation.Range, fmt.Sprintf("Http%s requires a global static method%s", strings.Title(verb), map[bool]string{true: " with no parameters", false: ""}[verb == "get" || verb == "delete"])))
				}
			}
		}
		for verb, count := range verbs {
			if count > 1 {
				diagnostics = append(diagnostics, webExposureDiagnostic(typ.File, typ.Range, fmt.Sprintf("RestResource declares more than one Http%s method", strings.Title(verb))))
			}
		}
	}
	return diagnostics
}

func validRESTMethodTypes(member typesys.MemberSymbol) bool {
	if !validSOAPType(member.Type) {
		return false
	}
	for _, parameter := range member.Parameters {
		if !validSOAPType(parameter.Type) {
			return false
		}
	}
	return true
}

func checkSOAPExposureContracts(typ typesys.TypeSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	methodNames := map[string]int{}
	for _, member := range typ.Members {
		if !hasModifier(member.Modifiers, "webservice") {
			continue
		}
		methodNames[strings.ToLower(member.Name)]++
		if typ.Kind != apexast.DeclarationClass || typ.NestingDepth != 0 || !hasModifier(typ.Modifiers, "global") || member.Kind != apexast.DeclarationMethod || !hasModifier(member.Modifiers, "static") || !validSOAPType(member.Type) {
			diagnostics = append(diagnostics, webExposureDiagnostic(typ.File, member.Range, "webservice methods require a top-level global class, static method, and supported wire types"))
		}
		for _, parameter := range member.Parameters {
			if !validSOAPType(parameter.Type) {
				diagnostics = append(diagnostics, webExposureDiagnostic(typ.File, parameter.Range, "webservice parameters cannot use Map, Set, or Blob types"))
			}
		}
	}
	for name, count := range methodNames {
		if count > 1 {
			diagnostics = append(diagnostics, webExposureDiagnostic(typ.File, typ.Range, fmt.Sprintf("webservice methods cannot be overloaded: %s", name)))
		}
	}
	return diagnostics
}

func validSOAPType(typ string) bool {
	name := strings.ToLower(strings.ReplaceAll(typ, " ", ""))
	return !strings.Contains(name, "map<") && !strings.Contains(name, "set<") && !strings.Contains(name, "blob")
}

func findAnnotation(annotations []apexast.Annotation, name string) (apexast.Annotation, bool) {
	for _, annotation := range annotations {
		if strings.EqualFold(annotation.Name, name) {
			return annotation, true
		}
	}
	return apexast.Annotation{}, false
}

func validRestMapping(annotation apexast.Annotation) bool {
	if len(annotation.Arguments) != 1 || !strings.EqualFold(annotation.Arguments[0].Name, "urlMapping") {
		return false
	}
	mapping, ok := apexStringLiteralValue(annotation.Arguments[0].Value)
	if !ok || !strings.HasPrefix(mapping, "/") || len(mapping) > 255 {
		return false
	}
	for index, char := range mapping {
		if char != '*' {
			continue
		}
		if index == 0 || mapping[index-1] != '/' || index+1 < len(mapping) && mapping[index+1] != '/' {
			return false
		}
	}
	return true
}

func apexStringLiteralValue(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '\'' {
		return "", false
	}
	for index := 1; index < len(raw); index++ {
		if raw[index] == '\\' {
			index++
			continue
		}
		if raw[index] != '\'' {
			continue
		}
		if index != len(raw)-1 {
			return "", false
		}
		return raw[1:index], true
	}
	return "", false
}

func restVerb(name string) string {
	switch strings.ToLower(name) {
	case "httpget":
		return "get"
	case "httppost":
		return "post"
	case "httpput":
		return "put"
	case "httppatch":
		return "patch"
	case "httpdelete":
		return "delete"
	}
	return ""
}

func webExposureDiagnostic(file string, r diagnostic.Range, message string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA033", Message: message, File: file, Range: &r}
}
