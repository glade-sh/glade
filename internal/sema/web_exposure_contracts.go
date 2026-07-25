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
				if member.Kind != apexast.DeclarationMethod || !hasModifier(member.Modifiers, "global") || !hasModifier(member.Modifiers, "static") || ((verb == "get" || verb == "delete") && len(member.Parameters) != 0) {
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
	mapping := strings.Trim(annotation.Arguments[0].Value, "'\"")
	return strings.HasPrefix(mapping, "/") && len(mapping) <= 255 && (!strings.Contains(mapping, "*") || strings.HasSuffix(mapping, "/*"))
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
