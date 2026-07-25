package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

type irStatementContext struct {
	loopDepth   int
	switchDepth int
}

func checkCustomExceptionNames(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, typ := range index.Types {
		if skipProjectDiagnosticType(typ) || string(typ.Kind) != "class" || !strings.EqualFold(typ.SuperClass, "Exception") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(typ.Name), "exception") {
			continue
		}
		diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA030", Message: fmt.Sprintf("custom exception class %q must end with Exception", typ.Name), File: typ.File, Range: &typ.Range})
	}
	return diagnostics
}

func (a *Analyzer) checkIRStatementContracts(typ typesys.TypeSymbol, member typesys.MemberSymbol, instructions []ir.Instruction, scope irSemaScope, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	var walk func([]ir.Instruction, irStatementContext)
	walk = func(items []ir.Instruction, context irStatementContext) {
		terminated := false
		for _, inst := range items {
			if terminated {
				diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "instruction is unreachable", bodyOffset+inst.Pos, source))
				continue
			}
			switch inst.Op {
			case ir.OpBreak:
				if context.loopDepth == 0 && context.switchDepth == 0 {
					diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "break is only valid inside a loop or switch", bodyOffset+inst.Pos, source))
				}
			case ir.OpContinue:
				if context.loopDepth == 0 {
					diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "continue is only valid inside a loop", bodyOffset+inst.Pos, source))
				}
			case ir.OpThrow:
				thrownType := a.inferIRExprType(inst.Expr, scope, model, typ.Name)
				if thrownType != "" && !semaAssignableToType("Exception", thrownType, model) {
					diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "throw requires an Exception value", bodyOffset+inst.Pos, source))
				}
			case ir.OpSwitch:
				selectorType := a.inferIRExprType(inst.Expr, scope, model, typ.Name)
				if strings.EqualFold(selectorType, "Boolean") || strings.EqualFold(selectorType, "Decimal") || strings.EqualFold(selectorType, "Date") {
					diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, fmt.Sprintf("switch does not support %s selectors", selectorType), bodyOffset+inst.Pos, source))
				}
				seenCaseValues := make(map[string]bool)
				seenElse := false
				for _, switchCase := range inst.Cases {
					if seenElse {
						diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "when else must be the final switch branch", bodyOffset+switchCase.Pos, source))
					}
					if switchCase.Else {
						seenElse = true
					}
					for _, caseExpr := range switchCase.Exprs {
						if _, _, typeCase := irSwitchTypeCase(caseExpr); typeCase {
							continue
						}
						key := normalizeName(strings.TrimSpace(string(caseExpr.Kind) + ":" + caseExpr.Value + ":" + caseExpr.Name))
						if key != "::" && seenCaseValues[key] {
							diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "duplicate switch branch value", bodyOffset+switchCase.Pos, source))
						}
						seenCaseValues[key] = true
					}
					walk(switchCase.Body, irStatementContext{loopDepth: context.loopDepth, switchDepth: context.switchDepth + 1})
				}
			case ir.OpWhile, ir.OpDoWhile, ir.OpFor, ir.OpForEach:
				walk(inst.Then, irStatementContext{loopDepth: context.loopDepth + 1, switchDepth: context.switchDepth})
			case ir.OpIf:
				walk(inst.Then, context)
				walk(inst.Else, context)
			case ir.OpTry:
				walk(inst.Then, context)
				seenCatchTypes := make(map[string]bool)
				var priorCatchTypes []string
				for _, catchClause := range catchClauses(inst) {
					for _, catchType := range catchClause.Types {
						key := normalizeName(catchType)
						if seenCatchTypes[key] {
							diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "duplicate catch type", bodyOffset+catchClause.Pos, source))
						}
						seenCatchTypes[key] = true
						if !semaAssignableToType("Exception", catchType, model) {
							diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "catch requires an Exception type", bodyOffset+catchClause.Pos, source))
						}
						for _, prior := range priorCatchTypes {
							if semaAssignableToType(prior, catchType, model) {
								diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "catch type is already covered by an earlier catch", bodyOffset+catchClause.Pos, source))
								break
							}
						}
						priorCatchTypes = append(priorCatchTypes, catchType)
					}
					walk(catchClause.Body, context)
				}
				walk(inst.Finally, context)
			case ir.OpBlock, ir.OpDeclGroup:
				walk(inst.Then, context)
			}
			if irInstructionTerminates(inst) {
				terminated = true
			}
		}
	}
	walk(instructions, irStatementContext{})
	return diagnostics
}

func statementContractDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, detail string, start int, source string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESEMA030",
		Message:  fmt.Sprintf("%s %q has invalid statement: %s", member.Kind, member.Name, detail),
		File:     typ.File,
		Range:    semaRange(source, start, start+1),
	}
}
