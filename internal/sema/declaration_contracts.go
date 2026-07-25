package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func (a *Analyzer) checkDeclarationContracts(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) {
			continue
		}
		diagnostics = append(diagnostics, declarationIdentityDiagnostics(typ)...)
		diagnostics = append(diagnostics, memberIdentityDiagnostics(typ)...)
	}
	return diagnostics
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
