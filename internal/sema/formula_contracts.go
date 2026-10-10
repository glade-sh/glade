package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// FormulaEval is a platform namespace, not an open namespace whose existence
// makes every qualified name valid. C016/C017 reject the System exceptions here.
func semaFormulaEvalQualifiedType(name string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(name)), "formulaeval.")
}

// These names use the platform catalog's spelling in native diagnostics. The
// System formula DTOs and FormulaInstance have no public constructors (C007,
// C013/C014); FormulaBuilder's failed signatures are captured in C008-C012.
func semaFormulaPlatformType(name string) (catalogName, diagnosticName string) {
	switch normalizeName(semaCanonicalPlatformAlias(name)) {
	case "formula":
		return "Formula", "System.Formula"
	case "formularecalcresult":
		return "FormulaRecalcResult", "System.FormulaRecalcResult"
	case "formularecalcfielderror":
		return "FormulaRecalcFieldError", "System.FormulaRecalcFieldError"
	case "formulaeval.formulabuilder":
		return "formulaeval.FormulaBuilder", "formulaeval.FormulaBuilder"
	case "formulaeval.formulainstance":
		return "formulaeval.FormulaInstance", "formulaeval.FormulaInstance"
	default:
		return "", ""
	}
}

func semaFormulaMethodDiagnostic(item diagnostic.Diagnostic, receiverType, method string, argTypes []string) diagnostic.Diagnostic {
	catalogName, diagnosticName := semaFormulaPlatformType(receiverType)
	if catalogName == "" {
		return item
	}
	// Preserve unrelated methods' existing diagnostics. These are the captured
	// Formula signature failures, rather than collection operations.
	if catalogName == "Formula" {
		if !strings.EqualFold(method, "recalculateFormulas") {
			return item
		}
	} else if catalogName == "formulaeval.FormulaBuilder" {
		switch normalizeName(method) {
		case "withreturntype", "withtype", "withformula", "treatnumericnullaszero":
		default:
			return item
		}
	} else {
		return item
	}
	for _, typ := range typesys.StandardPlatformSymbolView() {
		name := typ.Name
		if typ.Namespace != "" {
			name = typ.Namespace + "." + name
		}
		if !strings.EqualFold(name, catalogName) {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind == apexast.DeclarationMethod && strings.EqualFold(member.Name, method) {
				item.Message = fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", member.Name, strings.Join(argTypes, ", "), diagnosticName)
				return item
			}
		}
	}
	return item
}

const semaCalculatedFieldModifier = "__glade_calculated_field"

func semaFormulaReadOnlyField(model *semaTypeMemberView, receiverType, field string) (string, bool) {
	target, ok := semaResolveFieldPath(model, receiverType, field)
	if !ok || !hasModifier(target.member.Modifiers, semaCalculatedFieldModifier) {
		return "", false
	}
	return target.owner + "." + target.member.Name, true
}
