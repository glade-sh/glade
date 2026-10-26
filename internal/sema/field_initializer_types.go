package sema

import (
	"fmt"
	"regexp"
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
			if d, rejected := semaStaticFieldSelfReference(typ, member, initializer); rejected {
				diagnostics = append(diagnostics, d)
				continue
			}
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

var semaStaticFieldMemberAccess = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*[A-Za-z_][A-Za-z0-9_]*$`)

// Native inner_type_shapes field_enum_collision (API 62/67): in
// "static Level level = Level.LOW" the head name binds to the field being
// declared, not to the same-spelled type, and the read is rejected.
func semaStaticFieldSelfReference(typ typesys.TypeSymbol, member typesys.MemberSymbol, initializer string) (diagnostic.Diagnostic, bool) {
	if !hasModifier(member.Modifiers, "static") {
		return diagnostic.Diagnostic{}, false
	}
	match := semaStaticFieldMemberAccess.FindStringSubmatch(initializer)
	if match == nil || !strings.EqualFold(match[1], member.Name) {
		return diagnostic.Diagnostic{}, false
	}
	d := declarationContractDiagnostic(typ, member.Range, fmt.Sprintf("static field %q reads itself in its initializer", member.Name))
	d.NativeMessage = "Illegal forward reference: " + member.Name
	return d, true
}
