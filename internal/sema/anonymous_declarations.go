package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// WithAnonymousDeclarationContext preserves transient lexical provenance for
// semantic analysis and runtime lowering without renaming observable types.
func WithAnonymousDeclarationContext(index typesys.Index) typesys.Index {
	index.Types = append([]typesys.TypeSymbol(nil), index.Types...)
	for i := range index.Types {
		if !index.Types[i].Dependency && !hasModifier(index.Types[i].Modifiers, vm.AnonymousClassModifier) {
			index.Types[i].Modifiers = append(append([]string(nil), index.Types[i].Modifiers...), vm.AnonymousClassModifier)
		}
	}
	return index
}

// AnalyzeAnonymousDeclarations checks transient types in the implicit enclosing
// type of execute-anonymous, without changing the index used by the runtime.
// Native source observations C049/C051/C053-C055 and C095-C097/C100 cover this path.
func AnalyzeAnonymousDeclarations(index typesys.Index) Result {
	return AnalyzeAnonymousDeclarationsInContext(index, index)
}

// AnalyzeAnonymousDeclarationsInContext resolves transient declarations against
// an index that also contains the loaded project symbols and schema. Anonymous
// nesting rules and reported source diagnostics apply only to transient files.
func AnalyzeAnonymousDeclarationsInContext(index, transient typesys.Index) Result {
	if index.HasErrors() {
		diagnostics := NativeLifecycleDiagnostics(index, index.Diagnostics)
		return Result{Project: index.Project, Summary: Summary{Diagnostics: len(diagnostics)}, Diagnostics: diagnostics}
	}
	context := index
	context.Types = append([]typesys.TypeSymbol(nil), index.Types...)
	transientFiles := make(map[string]bool, len(transient.Types))
	for _, typ := range transient.Types {
		transientFiles[typ.File] = true
	}
	var diagnostics []diagnostic.Diagnostic
	projectTypes := make(map[string]bool)
	for _, typ := range index.Types {
		if !typ.Dependency && typ.HasSourceSnapshot() {
			projectTypes[normalizeName(typ.Name)] = true
		}
	}
	for i := range context.Types {
		typ := &context.Types[i]
		if !transientFiles[typ.File] || typ.Dependency || !typ.HasSourceSnapshot() {
			continue
		}
		typ.Modifiers = append([]string(nil), typ.Modifiers...)
		typ.NestingDepth++
		diagnostics = append(diagnostics, anonymousExposureDiagnostics(*typ)...)
		// Anonymous classes are inner types, so they
		// cannot implement the platform Batchable interface.
		if typ.Kind == apexast.DeclarationClass {
			for _, iface := range typ.Interfaces {
				base, _ := semaGenericBaseAndArgs(iface)
				if strings.EqualFold(base, "Database.Batchable") && !projectTypes[normalizeName(base)] && !projectTypes["database"] {
					diagnostics = append(diagnostics, declarationContractDiagnostic(*typ, typ.Range, "Only top-level classes can implement "+iface))
				}
			}
		}
		if typ.Kind == apexast.DeclarationClass {
			if hasModifier(typ.Modifiers, "protected") {
				diagnostics = append(diagnostics, declarationContractDiagnostic(*typ, typ.Range, "protected is not allowed on classes"))
			}
			if hasModifier(typ.Modifiers, "virtual") {
				diagnostics = append(diagnostics, declarationContractDiagnostic(*typ, typ.Range, "classes are by default virtual"))
			} else if !hasModifier(typ.Modifiers, "abstract") {
				typ.Modifiers = append(typ.Modifiers, "virtual")
			}
		}
		if typ.NestingDepth > 1 {
			diagnostics = append(diagnostics, declarationContractDiagnostic(*typ, typ.Range, "Inner types are not allowed to have inner types"))
		}
		for _, member := range typ.Members {
			// C048: anonymous types have a non-global implicit enclosing type.
			if member.Kind == apexast.DeclarationField && hasModifier(member.Modifiers, "global") {
				diagnostics = append(diagnostics, declarationContractDiagnostic(*typ, member.Range, "Enclosing type for global fields in apex classes must be declared as global"))
			}
			// Enum constants are synthesized static fields, not explicit static
			// declarations. C049 accepts them in anonymous source at API 62/67.
			if typ.Kind == apexast.DeclarationEnum && member.Kind == apexast.DeclarationField {
				continue
			}
			if !hasModifier(member.Modifiers, "static") {
				continue
			}
			switch member.Kind {
			case apexast.DeclarationInitializer:
				diagnostics = append(diagnostics, declarationContractDiagnostic(*typ, member.Range, "Inner types are not allowed to have static blocks"))
			case apexast.DeclarationField, apexast.DeclarationProperty:
				diagnostics = append(diagnostics, declarationContractDiagnostic(*typ, member.Range, "static can only be used on fields of a top level type"))
			case apexast.DeclarationMethod:
				diagnostics = append(diagnostics, declarationContractDiagnostic(*typ, member.Range, "static can only be used on methods of a top level type"))
			}
		}
	}
	if len(diagnostics) > 0 {
		// W5 anonymous C001-C003/C007-C012 report illegal nesting before
		// redundant virtual modifiers. Move only that competing prefix;
		// retain every diagnostic and other errors' existing precedence.
		for i, d := range diagnostics {
			if d.Message == "Inner types are not allowed to have inner types" {
				copy(diagnostics[1:i+1], diagnostics[:i])
				diagnostics[0] = d
				break
			}
			if d.Message != "classes are by default virtual" {
				break
			}
		}
		return Result{Project: index.Project, Summary: Summary{Diagnostics: len(diagnostics)}, Diagnostics: NativeLifecycleDiagnostics(context, diagnostics)}
	}
	result := Analyze(context)
	diagnostics = nil
	for _, item := range result.Diagnostics {
		if item.File == "" || transientFiles[item.File] {
			diagnostics = append(diagnostics, item)
		}
	}
	result.Diagnostics = NativeLifecycleDiagnostics(context, diagnostics)
	result.Summary.Diagnostics = len(diagnostics)
	return result
}
