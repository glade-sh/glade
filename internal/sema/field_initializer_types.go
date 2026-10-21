package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func (a *Analyzer) checkFieldInitializerTypes(index typesys.Index, model *semaTypeMemberView, sources *semaSources) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) {
			continue
		}
		var source string
		var loaded bool
		fieldModel := model
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationField {
				continue
			}
			if !loaded {
				source, _ = sources.normalizedForType(typ)
				loaded = true
				if semaSourceUnitHasDuplicate(model, typ) {
					fieldModel = semaModelWithCurrentType(model, typ)
				}
			}
			declaration, offset, ok := memberSource(source, member.Range)
			if !ok {
				continue
			}
			masked := maskSemaBindingText(declaration, newSemaCodeSpans(declaration))
			equals := semaTopLevelByte(masked, '=')
			if equals < 0 {
				continue
			}
			initializer := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(declaration[equals+1:]), ";"))
			program, err := vm.CompileAnonymous("return " + initializer + ";")
			if err != nil || len(program.Instructions) != 1 || program.Instructions[0].Op != ir.OpReturn {
				continue
			}
			scope := newIRSemaScope(semaBodyBaseScope(typ, member, fieldModel))
			diagnostics = append(diagnostics, a.checkIRAssignmentType(typ, member, member.Type, member.Name,
				program.Instructions[0].Expr, &scope, offset, 0, source, fieldModel, "initializes")...)
		}
	}
	return diagnostics
}
