package sema

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

var staticFirstTouchMetadataPattern = regexp.MustCompile(`(?is)\bSchema\s*\.\s*(?:getGlobalDescribe|describeSObjects)\s*\(|\b[A-Za-z_][A-Za-z0-9_]*__(?:mdt|c)\s*\.\s*getAll\s*\(`)

func (a *Analyzer) checkPerformancePatterns(index typesys.Index) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	sourceCache := make(map[string]string)
	for _, typ := range index.Types {
		if skipPerformanceDiagnostics(typ) {
			continue
		}
		source, ok := readSemaSourceForType(typ, sourceCache)
		if !ok {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind == apexast.DeclarationField || member.Kind == apexast.DeclarationInitializer {
				if diagnostic, ok := staticFirstTouchDiagnostic(typ, member, source); ok {
					diagnostics = append(diagnostics, diagnostic)
				}
			}
		}
	}
	return diagnostics
}

func performanceDiagnosticsForProgram(typ typesys.TypeSymbol, program ir.Program, bodyOffset int, source string) []diagnostic.Diagnostic {
	if skipPerformanceDiagnostics(typ) {
		return nil
	}
	seen := make(map[string]bool)
	return performanceDiagnosticsForInstructions(typ, program.Instructions, 0, bodyOffset, source, seen)
}

func skipPerformanceDiagnostics(typ typesys.TypeSymbol) bool {
	return typ.Dependency || typ.Artifact || typ.IsTest || hasModifier(typ.Modifiers, "IsTest")
}

func performanceDiagnosticsForInstructions(typ typesys.TypeSymbol, instructions []ir.Instruction, loopDepth, bodyOffset int, source string, seen map[string]bool) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, inst := range instructions {
		if loopDepth > 0 {
			switch {
			case inst.Op == ir.OpDML:
				if diagnostic, ok := loopPerformanceDiagnostic(typ, inst.Pos, bodyOffset, source, "DML statement runs inside a loop; collect changed records and perform bulk DML outside the loop.", seen); ok {
					diagnostics = append(diagnostics, diagnostic)
				}
			case exprContainsDatabaseDMLCall(inst.Expr):
				if diagnostic, ok := loopPerformanceDiagnostic(typ, inst.Pos, bodyOffset, source, "Database DML call runs inside a loop; collect changed records and perform bulk DML outside the loop.", seen); ok {
					diagnostics = append(diagnostics, diagnostic)
				}
			case exprContainsSOQL(inst.Expr):
				if diagnostic, ok := loopPerformanceDiagnostic(typ, inst.Pos, bodyOffset, source, "SOQL query runs inside a loop; collect keys first and query once outside the loop.", seen); ok {
					diagnostics = append(diagnostics, diagnostic)
				}
			}
		}
		nextDepth := loopDepth
		switch inst.Op {
		case ir.OpWhile, ir.OpDoWhile, ir.OpFor, ir.OpForEach:
			nextDepth++
		}
		diagnostics = append(diagnostics, performanceDiagnosticsForInstructions(typ, inst.Then, nextDepth, bodyOffset, source, seen)...)
		diagnostics = append(diagnostics, performanceDiagnosticsForInstructions(typ, inst.Else, loopDepth, bodyOffset, source, seen)...)
		diagnostics = append(diagnostics, performanceDiagnosticsForInstructions(typ, inst.Catch, loopDepth, bodyOffset, source, seen)...)
		for _, catchClause := range inst.Catches {
			diagnostics = append(diagnostics, performanceDiagnosticsForInstructions(typ, catchClause.Body, loopDepth, bodyOffset, source, seen)...)
		}
		diagnostics = append(diagnostics, performanceDiagnosticsForInstructions(typ, inst.Finally, loopDepth, bodyOffset, source, seen)...)
		for _, switchCase := range inst.Cases {
			diagnostics = append(diagnostics, performanceDiagnosticsForInstructions(typ, switchCase.Body, loopDepth, bodyOffset, source, seen)...)
		}
		if inst.Op == ir.OpFor {
			if inst.Update != nil {
				diagnostics = append(diagnostics, performanceDiagnosticsForInstructions(typ, []ir.Instruction{*inst.Update}, loopDepth+1, bodyOffset, source, seen)...)
			}
			if len(inst.Updates) > 0 {
				diagnostics = append(diagnostics, performanceDiagnosticsForInstructions(typ, inst.Updates, loopDepth+1, bodyOffset, source, seen)...)
			}
		}
	}
	return diagnostics
}

func loopPerformanceDiagnostic(typ typesys.TypeSymbol, pos, bodyOffset int, source, message string, seen map[string]bool) (diagnostic.Diagnostic, bool) {
	start := bodyOffset + pos
	key := typ.File + ":GLADEPERF001:" + message + ":" + strconv.Itoa(start)
	if seen[key] {
		return diagnostic.Diagnostic{}, false
	}
	seen[key] = true
	return diagnostic.Diagnostic{
		Severity: diagnostic.Warning,
		Code:     "GLADEPERF001",
		Message:  message,
		File:     typ.File,
		Range:    semaRange(source, start, start+1),
	}, true
}

func exprContainsSOQL(expr ir.Expr) bool {
	if expr.Kind == ir.ExprSOQL {
		return true
	}
	if expr.Left != nil && exprContainsSOQL(*expr.Left) {
		return true
	}
	if expr.Right != nil && exprContainsSOQL(*expr.Right) {
		return true
	}
	for _, arg := range expr.Args {
		if exprContainsSOQL(arg) {
			return true
		}
	}
	for _, arg := range expr.NamedArgs {
		if exprContainsSOQL(arg.Expr) {
			return true
		}
	}
	return false
}

func exprContainsDatabaseDMLCall(expr ir.Expr) bool {
	if expr.Kind == ir.ExprCall && isDatabaseDMLCallee(expr.Callee) {
		return true
	}
	if expr.Left != nil && exprContainsDatabaseDMLCall(*expr.Left) {
		return true
	}
	if expr.Right != nil && exprContainsDatabaseDMLCall(*expr.Right) {
		return true
	}
	for _, arg := range expr.Args {
		if exprContainsDatabaseDMLCall(arg) {
			return true
		}
	}
	for _, arg := range expr.NamedArgs {
		if exprContainsDatabaseDMLCall(arg.Expr) {
			return true
		}
	}
	return false
}

func isDatabaseDMLCallee(callee string) bool {
	callee = strings.TrimSpace(callee)
	receiver, method, ok := splitSemaMethodPath(callee)
	if !ok || !strings.EqualFold(receiver, "Database") {
		return false
	}
	switch strings.ToLower(method) {
	case "insert", "update", "upsert", "delete", "undelete", "merge":
		return true
	default:
		return false
	}
}

func staticFirstTouchDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, source string) (diagnostic.Diagnostic, bool) {
	if member.Kind == apexast.DeclarationField && !hasModifier(member.Modifiers, "static") {
		return diagnostic.Diagnostic{}, false
	}
	text, start, ok := memberSource(source, member.Range)
	if !ok {
		return diagnostic.Diagnostic{}, false
	}
	if member.Kind == apexast.DeclarationInitializer && !strings.HasPrefix(strings.TrimSpace(text), "static") {
		return diagnostic.Diagnostic{}, false
	}
	match := staticFirstTouchMetadataPattern.FindStringIndex(text)
	if match == nil {
		return diagnostic.Diagnostic{}, false
	}
	return diagnostic.Diagnostic{
		Severity: diagnostic.Warning,
		Code:     "GLADEPERF002",
		Message:  "static initialization performs mass metadata/config work; split cheap constants from lazy describe/config providers.",
		File:     typ.File,
		Range:    semaRange(source, start+match[0], start+match[1]),
	}, true
}

func memberSource(source string, r diagnostic.Range) (string, int, bool) {
	start := r.Start.Offset
	end := r.End.Offset
	if start < 0 || end <= start || start >= len(source) || end > len(source) {
		return "", 0, false
	}
	return source[start:end], start, true
}
