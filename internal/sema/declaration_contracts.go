package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

const maxApexParameters = 32

func (a *Analyzer) checkDeclarationContracts(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) {
			continue
		}
		diagnostics = append(diagnostics, declarationIdentityDiagnostics(typ)...)
		if declarationFromParsedSource(typ) {
			diagnostics = append(diagnostics, declarationModifierDiagnostics(typ)...)
			diagnostics = append(diagnostics, memberModifierDiagnostics(typ)...)
		}
		diagnostics = append(diagnostics, memberIdentityDiagnostics(typ)...)
	}
	return diagnostics
}

func declarationFromParsedSource(typ typesys.TypeSymbol) bool {
	if typ.LocalName == "" || typ.File == "" {
		return false
	}
	// Flow interview symbols are synthesized from metadata, not Apex declarations.
	return !strings.HasSuffix(strings.ToLower(typ.File), ".flow-meta.xml")
}

func declarationIdentityDiagnostics(typ typesys.TypeSymbol) []diagnostic.Diagnostic {
	if typ.LocalName == "" || typ.OwnerName == "" {
		return nil
	}
	for _, ancestor := range strings.Split(typ.OwnerName, ".") {
		if strings.EqualFold(ancestor, typ.LocalName) {
			return []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     "GLADESEMA031",
				Message:  fmt.Sprintf("inner type %q reuses ancestor type name %q", typ.Name, ancestor),
				File:     typ.File,
				Range:    &typ.Range,
			}}
		}
	}
	return nil
}

func declarationModifierDiagnostics(typ typesys.TypeSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	mods := typ.Modifiers

	if typ.NestingDepth == 0 {
		hasPrivate := hasModifier(mods, "private")
		hasProtected := hasModifier(mods, "protected")
		// Apex top-level declarations require public or global visibility. @IsTest
		// private declarations remain the one documented exception.
		if !typ.IsTest && (!hasAnyAccessModifier(mods) || hasPrivate || hasProtected) {
			diagnostics = append(diagnostics, declarationContractDiagnostic(typ, typ.Range,
				fmt.Sprintf("top-level %s %q must be public or global", typ.Kind, typ.Name)))
		}
	}
	if typ.NestingDepth > 1 && (typ.Kind == apexast.DeclarationClass || typ.Kind == apexast.DeclarationInterface) {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, typ.Range,
			fmt.Sprintf("type %q nests deeper than one inner level", typ.Name)))
	}
	if typ.Kind == apexast.DeclarationClass {
		if hasModifier(mods, "static") {
			diagnostics = append(diagnostics, declarationContractDiagnostic(typ, typ.Range,
				fmt.Sprintf("class %q cannot be declared static", typ.Name)))
		}
		if hasModifier(mods, "final") {
			diagnostics = append(diagnostics, declarationContractDiagnostic(typ, typ.Range,
				fmt.Sprintf("class %q cannot be declared final", typ.Name)))
		}
		if hasModifier(mods, "abstract") && hasModifier(mods, "virtual") {
			diagnostics = append(diagnostics, declarationContractDiagnostic(typ, typ.Range,
				fmt.Sprintf("class %q cannot be both abstract and virtual", typ.Name)))
		}
	}
	if sharingModifierCount(mods) > 1 {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, typ.Range,
			fmt.Sprintf("type %q declares mutually exclusive sharing modifiers", typ.Name)))
	}
	if len(typ.TypeParameters) > 0 {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, typ.Range,
			fmt.Sprintf("%s %q cannot declare type parameters", typ.Kind, typ.Name)))
	}
	return diagnostics
}

func memberIdentityDiagnostics(typ typesys.TypeSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	fieldNames := make(map[string]typesys.MemberSymbol)
	methodKeys := make(map[string]typesys.MemberSymbol)
	ctorKeys := make(map[string]typesys.MemberSymbol)

	for _, member := range typ.Members {
		switch member.Kind {
		case apexast.DeclarationField, apexast.DeclarationProperty:
			key := normalizeName(member.Name)
			if previous, ok := fieldNames[key]; ok {
				diagnostics = append(diagnostics, duplicateMemberDiagnostic(typ, member, previous.Kind))
				continue
			}
			fieldNames[key] = member
		case apexast.DeclarationMethod:
			key := memberSignatureKey(member)
			if previous, ok := methodKeys[key]; ok {
				diagnostics = append(diagnostics, duplicateMemberDiagnostic(typ, member, previous.Kind))
				continue
			}
			methodKeys[key] = member
		case apexast.DeclarationConstructor:
			key := constructorSignatureKey(member)
			if previous, ok := ctorKeys[key]; ok {
				diagnostics = append(diagnostics, duplicateMemberDiagnostic(typ, member, previous.Kind))
				continue
			}
			ctorKeys[key] = member
		}
	}
	return diagnostics
}

func memberModifierDiagnostics(typ typesys.TypeSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, member := range typ.Members {
		switch member.Kind {
		case apexast.DeclarationMethod:
			diagnostics = append(diagnostics, methodContractDiagnostics(typ, member)...)
		case apexast.DeclarationConstructor:
			diagnostics = append(diagnostics, constructorContractDiagnostics(typ, member)...)
		case apexast.DeclarationInitializer:
			if typ.NestingDepth > 0 && hasModifier(member.Modifiers, "static") {
				diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
					fmt.Sprintf("inner type %q cannot declare a static initializer", typ.Name)))
			}
		case apexast.DeclarationProperty:
			diagnostics = append(diagnostics, propertyAccessorDiagnostics(typ, member)...)
		}
	}
	return diagnostics
}

func methodContractDiagnostics(typ typesys.TypeSymbol, member typesys.MemberSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	mods := member.Modifiers
	abstract := hasModifier(mods, "abstract")
	virtual := hasModifier(mods, "virtual")
	override := hasModifier(mods, "override")
	static := hasModifier(mods, "static")
	final := hasModifier(mods, "final")

	if abstract && virtual {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("method %q cannot be both abstract and virtual", member.Name)))
	}
	if abstract && override {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("method %q cannot be both abstract and override", member.Name)))
	}
	if abstract && static {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("method %q cannot be both abstract and static", member.Name)))
	}
	if virtual && static {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("method %q cannot be both virtual and static", member.Name)))
	}
	if override && static {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("method %q cannot be both override and static", member.Name)))
	}
	if typ.NestingDepth > 0 && static {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("inner type %q cannot declare a static method %q", typ.Name, member.Name)))
	}
	if static && hasModifier(mods, "protected") {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("protected method %q cannot be static", member.Name)))
	}
	if hasModifier(mods, "global") && !hasModifier(typ.Modifiers, "global") {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("global method %q requires a global enclosing type", member.Name)))
	}
	if final {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("method %q cannot explicitly declare final", member.Name)))
	}
	if (abstract || override) && typeUsesAPIVersionAtLeast(typ, 65) && !hasAnyAccessModifier(mods) {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("%s method %q requires an explicit protected, public, or global access modifier in API version 65.0 or later", methodModifierKind(abstract), member.Name)))
	}
	if len(member.Parameters) > maxApexParameters {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("method %q exceeds the %d parameter limit", member.Name, maxApexParameters)))
	}

	switch typ.Kind {
	case apexast.DeclarationInterface:
		if hasModifier(typ.Modifiers, "global") && hasModifier(mods, "public") {
			diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
				fmt.Sprintf("method %q cannot explicitly declare public in a global interface", member.Name)))
		}
		if member.HasBody {
			diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
				fmt.Sprintf("interface method %q cannot have a body", member.Name)))
		}
	case apexast.DeclarationClass:
		if abstract && member.HasBody {
			diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
				fmt.Sprintf("abstract method %q cannot have a body", member.Name)))
		}
		if !abstract && !member.HasBody {
			diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
				fmt.Sprintf("method %q must have a body", member.Name)))
		}
	}
	return diagnostics
}

func typeUsesAPIVersionAtLeast(typ typesys.TypeSymbol, minimum int) bool {
	return apexversion.AtLeast(typ.EffectiveAPIVersion, minimum)
}

func hasAnyAccessModifier(modifiers []string) bool {
	return hasModifier(modifiers, "protected") || hasModifier(modifiers, "public") || hasModifier(modifiers, "global")
}

func methodModifierKind(abstract bool) string {
	if abstract {
		return "abstract"
	}
	return "override"
}

func constructorContractDiagnostics(typ typesys.TypeSymbol, member typesys.MemberSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	if len(member.Parameters) > maxApexParameters {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("constructor for %q exceeds the %d parameter limit", typ.Name, maxApexParameters)))
	}
	if !member.HasBody {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("constructor for %q must have a body", typ.Name)))
	}
	return diagnostics
}

func propertyAccessorDiagnostics(typ typesys.TypeSymbol, member typesys.MemberSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	gets, sets := 0, 0
	propVis := declarationVisibilityRank(member.Modifiers)
	for _, accessor := range member.Accessors {
		switch strings.ToLower(accessor.Kind) {
		case "get":
			gets++
		case "set":
			sets++
		}
		if len(accessor.Modifiers) == 0 {
			continue
		}
		accVis := declarationVisibilityRank(accessor.Modifiers)
		if accVis > propVis {
			diagnostics = append(diagnostics, declarationContractDiagnostic(typ, accessor.Range,
				fmt.Sprintf("property %q accessor visibility cannot be wider than the property", member.Name)))
		}
	}
	if gets > 1 {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("property %q declares duplicate getter", member.Name)))
	}
	if sets > 1 {
		diagnostics = append(diagnostics, declarationContractDiagnostic(typ, member.Range,
			fmt.Sprintf("property %q declares duplicate setter", member.Name)))
	}
	return diagnostics
}

func duplicateMemberDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, previousKind apexast.DeclarationKind) diagnostic.Diagnostic {
	kind := string(member.Kind)
	if previousKind != member.Kind {
		kind = string(previousKind) + "/" + string(member.Kind)
	}
	name := member.Name
	if member.Kind == apexast.DeclarationConstructor {
		name = typ.LocalName
		if name == "" {
			name = typ.Name
		}
	}
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA031",
		Message:  fmt.Sprintf("type %q declares duplicate %s %q", typ.Name, kind, name),
		File:     typ.File,
		Range:    &member.Range,
	}
}

func declarationContractDiagnostic(typ typesys.TypeSymbol, r diagnostic.Range, message string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA032",
		Message:  message,
		File:     typ.File,
		Range:    &r,
	}
}

func memberSignatureKey(member typesys.MemberSymbol) string {
	return normalizeName(member.Name) + "(" + parameterTypeKey(member.Parameters) + ")"
}

func constructorSignatureKey(member typesys.MemberSymbol) string {
	return "(" + parameterTypeKey(member.Parameters) + ")"
}

func parameterTypeKey(params []apexast.Parameter) string {
	if len(params) == 0 {
		return ""
	}
	parts := make([]string, len(params))
	for i, param := range params {
		parts[i] = normalizeName(param.Type)
	}
	return strings.Join(parts, ",")
}

func hasTopLevelVisibility(modifiers []string) bool {
	return hasModifier(modifiers, "public") || hasModifier(modifiers, "global")
}

func sharingModifierCount(modifiers []string) int {
	count := 0
	for _, mod := range modifiers {
		switch strings.ToLower(strings.Join(strings.Fields(mod), " ")) {
		case "with sharing", "without sharing", "inherited sharing":
			count++
		}
	}
	return count
}

func declarationVisibilityRank(modifiers []string) int {
	switch {
	case hasModifier(modifiers, "global"):
		return 3
	case hasModifier(modifiers, "public"):
		return 2
	case hasModifier(modifiers, "protected"):
		return 1
	case hasModifier(modifiers, "private"):
		return 0
	default:
		return 0
	}
}
