package sema

import (
	"fmt"
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
