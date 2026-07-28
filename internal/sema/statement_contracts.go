package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
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
	scope = cloneIRStatementScope(scope)
	var diagnostics []diagnostic.Diagnostic
	var walk func([]ir.Instruction, irStatementContext, *irSemaScope)
	walk = func(items []ir.Instruction, context irStatementContext, currentScope *irSemaScope) {
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
				thrownType := a.inferIRExprType(inst.Expr, *currentScope, model, typ.Name)
				if thrownType != "" && !semaAssignableToType("Exception", thrownType, model) {
					diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "throw requires an Exception value", bodyOffset+inst.Pos, source))
				}
			case ir.OpSwitch:
				selectorType := a.inferIRExprType(inst.Expr, *currentScope, model, typ.Name)
				selectorType = semaResolveSwitchSelectorType(selectorType, typ.Name, model)
				if selectorType != "" && !semaSupportedSwitchSelector(selectorType, model) {
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
						if caseType, _, typeCase := irSwitchTypeCase(caseExpr); typeCase {
							key := "__typecase:" + normalizeName(caseType)
							if seenCaseValues[key] {
								diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "duplicate switch branch type", bodyOffset+switchCase.Pos, source))
							}
							seenCaseValues[key] = true
							if selectorType != "" && !semaSwitchTypeCaseCompatible(selectorType, caseType, model) {
								diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, fmt.Sprintf("switch type branch %s is incompatible with selector type %s", caseType, selectorType), bodyOffset+switchCase.Pos, source))
							}
							continue
						}
						if !semaSwitchValueCaseAllowed(selectorType, caseExpr, model) {
							diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "switch branch must be a literal or enum constant", bodyOffset+switchCase.Pos, source))
							continue
						}
						caseType := a.inferIRExprType(caseExpr, *currentScope, model, typ.Name)
						if selectorType != "" && caseType != "" && !semaSwitchCaseTypeCompatible(selectorType, caseType, model) {
							diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, fmt.Sprintf("switch branch type %s is incompatible with selector type %s", caseType, selectorType), bodyOffset+switchCase.Pos, source))
						}
						key := normalizeName(strings.TrimSpace(string(caseExpr.Kind) + ":" + caseExpr.Value + ":" + caseExpr.Name))
						if key != "::" && seenCaseValues[key] {
							diagnostics = append(diagnostics, statementContractDiagnostic(typ, member, "duplicate switch branch value", bodyOffset+switchCase.Pos, source))
						}
						seenCaseValues[key] = true
					}
					caseScope := *currentScope
					caseScope.push()
					for _, caseExpr := range switchCase.Exprs {
						if caseType, binding, ok := irSwitchTypeCase(caseExpr); ok {
							caseScope.declare(binding, resolveNestedTypeReference(model, typ.Name, caseType))
						}
					}
					walk(switchCase.Body, irStatementContext{loopDepth: context.loopDepth, switchDepth: context.switchDepth + 1}, &caseScope)
				}
			case ir.OpWhile, ir.OpDoWhile, ir.OpFor, ir.OpForEach:
				loopScope := *currentScope
				loopScope.push()
				if inst.Op == ir.OpFor {
					inits := inst.Inits
					if len(inits) == 0 && inst.Init != nil {
						inits = []ir.Instruction{*inst.Init}
					}
					walk(inits, context, &loopScope)
				} else if inst.Op == ir.OpForEach {
					loopScope.declare(inst.Name, resolveNestedTypeReference(model, typ.Name, inst.Type))
				}
				walk(inst.Then, irStatementContext{loopDepth: context.loopDepth + 1, switchDepth: context.switchDepth}, &loopScope)
			case ir.OpIf:
				thenScope := *currentScope
				thenScope.push()
				walk(inst.Then, context, &thenScope)
				elseScope := *currentScope
				elseScope.push()
				walk(inst.Else, context, &elseScope)
			case ir.OpTry:
				tryScope := *currentScope
				tryScope.push()
				walk(inst.Then, context, &tryScope)
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
					catchScope := *currentScope
					catchScope.push()
					if catchClause.Name != "" {
						catchType := "Exception"
						if len(catchClause.Types) > 0 {
							catchType = catchClause.Types[0]
						}
						catchScope.declare(catchClause.Name, resolveNestedTypeReference(model, typ.Name, catchType))
					}
					walk(catchClause.Body, context, &catchScope)
				}
				finallyScope := *currentScope
				finallyScope.push()
				walk(inst.Finally, context, &finallyScope)
			case ir.OpBlock:
				blockScope := *currentScope
				blockScope.push()
				walk(inst.Then, context, &blockScope)
			case ir.OpDeclGroup:
				walk(inst.Then, context, currentScope)
			}
			if inst.Op == ir.OpDeclare {
				currentScope.declare(inst.Name, resolveNestedTypeReference(model, typ.Name, inst.Type))
			}
			if inst.Op != ir.OpThrow && irInstructionTerminates(inst) {
				terminated = true
			}
		}
	}
	walk(instructions, irStatementContext{}, &scope)
	return diagnostics
}

func cloneIRStatementScope(scope irSemaScope) irSemaScope {
	clone := irSemaScope{frames: make([]map[string]irSemaBinding, len(scope.frames))}
	for index, frame := range scope.frames {
		clone.frames[index] = make(map[string]irSemaBinding, len(frame))
		for name, binding := range frame {
			clone.frames[index][name] = binding
		}
	}
	return clone
}

func semaResolveSwitchSelectorType(typeName, owner string, model *semaTypeMemberView) string {
	if typeName == "" {
		return typeName
	}
	resolved := resolveNestedTypeName(model, owner, typeName)
	if _, ok := model.lookup(normalizeName(resolved)); ok {
		return resolved
	}
	if _, ok := model.lookup(normalizeName(typeName)); ok {
		return typeName
	}
	return typeName
}

func semaSupportedSwitchSelector(typeName string, model *semaTypeMemberView) bool {
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "integer", "int", "long", "string":
		return true
	}
	if isSemaSObjectLike(typeName, model) {
		return true
	}
	if members, ok := model.lookup(normalizeName(typeName)); ok {
		return members.kind == apexast.DeclarationEnum
	}
	return false
}

func semaSwitchCaseTypeCompatible(selectorType, caseType string, model *semaTypeMemberView) bool {
	if strings.EqualFold(strings.TrimSpace(caseType), "null") {
		return true
	}
	selector := strings.ToLower(strings.TrimSpace(selectorType))
	branch := strings.ToLower(strings.TrimSpace(caseType))
	switch selector {
	case "integer", "int":
		return branch == "integer" || branch == "int"
	case "long":
		return branch == "integer" || branch == "int" || branch == "long"
	}
	return semaAssignableToType(selectorType, caseType, model)
}

func semaSwitchTypeCaseCompatible(selectorType, caseType string, model *semaTypeMemberView) bool {
	if !isSemaSObjectLike(selectorType, model) || !isSemaSObjectLike(caseType, model) {
		return false
	}
	return semaAssignableToType(selectorType, caseType, model) || semaAssignableToType(caseType, selectorType, model)
}

func semaSwitchValueCaseAllowed(selectorType string, expr ir.Expr, model *semaTypeMemberView) bool {
	if expr.Kind == ir.ExprLiteral {
		return true
	}
	if expr.Kind != ir.ExprVariable || selectorType == "" {
		return false
	}
	if enumType := semaEnumValuePathType(model, expr.Name); enumType != "" {
		return sameSemaSignatureType(selectorType, enumType)
	}
	members, ok := model.lookup(normalizeName(selectorType))
	if !ok || members.kind != apexast.DeclarationEnum {
		return false
	}
	field, ok := semaResolveField(model, selectorType, expr.Name, make(map[string]bool))
	return ok && field.member.Kind == apexast.DeclarationField && hasModifier(field.member.Modifiers, "static")
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
