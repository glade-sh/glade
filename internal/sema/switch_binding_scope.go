package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The IR checker already owns switch-binding validity and redeclaration checks.
// Give text-based call and assignment checks the same case-local bindings.
func collectSemaSwitchTypeBindings(typ typesys.TypeSymbol, member typesys.MemberSymbol, body string, scopes *semaScopeModel, model *semaTypeMemberView) {
	if !strings.Contains(strings.ToLower(body), "switch") {
		return
	}
	program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{PrefixStatementCandidates: semaPrefixStatementCandidates(typ, member, false)})
	if err != nil {
		return
	}
	var walk func([]ir.Instruction)
	walk = func(instructions []ir.Instruction) {
		for _, inst := range instructions {
			for _, switchCase := range inst.Cases {
				for _, expr := range switchCase.Exprs {
					caseType, binding, ok := irSwitchTypeCase(expr)
					if !ok {
						continue
					}
					start, end := semaSwitchCaseBlockBounds(body, switchCase.Pos)
					if start >= end {
						continue
					}
					scopes.invalidateFlat()
					scopes.locals = append(scopes.locals, semaLocal{
						name:       binding,
						key:        scopes.canonicalName(binding),
						typeName:   resolveNestedTypeReference(model, typ.Name, caseType),
						start:      start + 1,
						scopeStart: start,
						scopeEnd:   end,
					})
				}
				walk(switchCase.Body)
			}
			walk(inst.Then)
			walk(inst.Else)
			for _, clause := range catchClauses(inst) {
				walk(clause.Body)
			}
			walk(inst.Finally)
		}
	}
	walk(program.Instructions)
}

func semaSwitchCaseBlockBounds(body string, pos int) (int, int) {
	for i := pos; i < len(body); i++ {
		switch body[i] {
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
			}
		case '{':
			return blockBoundsAt(body, i+1)
		}
	}
	return pos, pos
}
