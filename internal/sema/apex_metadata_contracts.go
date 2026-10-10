package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// Metadata's generated DTO catalog also contains types that Apex cannot name.
// Owned type-admission cases and their field/clone counterparts capture this
// distinction. Check parsed references, never strings or comments in the source.
func (a *Analyzer) apexMetadataAdmissionDiagnostics(typ typesys.TypeSymbol, program ir.Program, scope irSemaScope, bodyOffset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	body := program.Source
	if body == "" {
		body = source
	}
	folded := strings.ToLower(body)
	owned := strings.Contains(folded, "metadata") || strings.Contains(folded, "visualeditor")
	if !owned {
		for _, frame := range scope.frames {
			for _, binding := range frame {
				name := strings.ToLower(binding.typ)
				owned = owned || strings.Contains(name, "metadata.") || strings.Contains(name, "visualeditor.")
			}
		}
	}
	if !owned {
		return nil
	}
	// This check resolves its own locals without mutating the ordinary IR gate.
	scope = newIRSemaScope(scope.flat())
	var diagnostics []diagnostic.Diagnostic
	appendDiagnostic := func(message string, pos int) {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error, Code: "GLADESEMA_METADATA", Message: message,
			NativeMessage: message, File: typ.File, Range: semaRange(source, bodyOffset+pos, bodyOffset+pos+1),
		})
	}
	checkType := func(typeName string, pos int) {
		for _, ref := range extractTypeNames(typeName) {
			if target, ok := model.lookupName(ref); ok && !target.dependency {
				continue
			}
			switch strings.ToLower(ref) {
			case "metadata.minilayoutitem":
				appendDiagnostic("Invalid type: Metadata.MiniLayoutItem", pos)
			case "metadata.asyncresult":
				appendDiagnostic("Invalid type: Metadata.AsyncResult", pos)
			case "metadata.customfield":
				appendDiagnostic("Invalid type: Metadata.CustomField", pos)
			case "metadata.consolecomponent":
				appendDiagnostic("Type is not visible: Metadata.ConsoleComponent", pos)
			}
		}
	}
	checkField := func(name string, pos int) {
		if _, isType := model.lookupName(name); isType {
			return
		}
		at := strings.LastIndexByte(name, '.')
		if at < 0 {
			return
		}
		receiver, field := name[:at], name[at+1:]
		receiverType := semaIRReceiverType(receiver, scope, model, typ.Name)
		if receiverType == "" && strings.HasPrefix(strings.ToLower(receiver), "metadata.") {
			// C015 (with the valid R154 control): platform members can carry
			// the local enum name, so the shared spelling gate does not resolve
			// a fully qualified Metadata enum receiver. Resolve only that exact
			// platform namespace, while preserving a local root's precedence.
			root, _, _ := strings.Cut(receiver, ".")
			_, shadowed := scope.lookup(root)
			if target, ok := model.lookupName(receiver); !shadowed && ok && target.platform && strings.EqualFold(target.namespace, "Metadata") {
				receiverType = receiver
			}
		}
		if !strings.HasPrefix(strings.ToLower(receiverType), "metadata.") || strings.EqualFold(field, "class") {
			return
		}
		if target, ok := model.lookupName(receiverType); !ok || !target.platform {
			return
		}
		if _, found := semaResolveFieldPath(model, receiverType, field); !found {
			// C015/C016: generated DTOs and enums do not admit arbitrary fields.
			appendDiagnostic("Variable does not exist: "+field, pos)
		}
	}
	var expression func(ir.Expr, int)
	expression = func(expr ir.Expr, pos int) {
		if expr.Kind == ir.ExprVariable {
			checkField(expr.Name, pos)
		}
		if strings.HasPrefix(expr.Callee, "new:") {
			name := strings.TrimPrefix(expr.Callee, "new:")
			checkType(name, pos)
			if target, ok := model.lookupName(name); ok && target.platform && (strings.HasPrefix(strings.ToLower(name), "metadata.") || strings.HasPrefix(strings.ToLower(name), "visualeditor.")) {
				// C009/C010/C019 retain the platform namespace even when the
				// lazy member view stores the symbol's local name.
				displayName := target.name
				if target.namespace != "" && !strings.HasPrefix(strings.ToLower(displayName), strings.ToLower(target.namespace)+".") {
					displayName = target.namespace + "." + displayName
				}
				switch {
				case target.kind == apexast.DeclarationInterface:
					appendDiagnostic("Type cannot be constructed: "+displayName, pos)
				case hasModifier(target.modifiers, "abstract"):
					appendDiagnostic("Abstract classes cannot be constructed: "+displayName, pos)
				case len(target.constructors) > 0:
					args := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
					if _, found, ambiguous := bestMemberByArgTypes(target.constructors, args, model); !found && !ambiguous && len(expr.NamedArgs) == 0 {
						// C019 supplies the exact missing-constructor spelling.
						appendDiagnostic("Constructor not defined: ["+displayName+"].<Constructor>("+strings.Join(args, ",")+")", pos)
					}
				}
			}
		}
		if strings.HasPrefix(expr.Callee, "__cast:") {
			checkType(strings.TrimPrefix(expr.Callee, "__cast:"), pos)
		}
		if strings.EqualFold(expr.Callee, "Metadata.Operations.checkDeployStatus") && len(expr.Args) == 2 {
			if target, ok := model.lookupName("Metadata.Operations"); !ok || target.dependency {
				// C004: the two-argument native method is absent. Retain the
				// separate local one-argument mock contract rather than widening.
				args := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
				appendDiagnostic("Method does not exist or incorrect signature: void checkDeployStatus("+strings.Join(args, ", ")+") from the type Metadata.Operations", pos)
			}
		}
		if expr.Kind == ir.ExprCall {
			receiver, method, _ := splitSemaMethodPath(expr.Callee)
			receiverType := semaIRReceiverType(receiver, scope, model, typ.Name)
			if expr.Left != nil {
				receiverType = a.inferIRExprType(*expr.Left, scope, model, typ.Name)
				if method == "" {
					method = expr.Callee
				}
			}
			if strings.HasPrefix(strings.ToLower(receiverType), "metadata.") && (strings.EqualFold(method, "clone") || strings.EqualFold(method, "addMetadata")) {
				args := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
				methods := resolveMemberMethods(model, receiverType, method)
				if _, found, ambiguous := bestResolvedMemberByArgTypes(methods, args, model); !found && !ambiguous && len(methods) > 0 {
					// C017/C018: generated dependencies retain native overload checks.
					appendDiagnostic("Method does not exist or incorrect signature: void "+method+"("+strings.Join(args, ", ")+") from the type "+receiverType, pos)
				}
			}
		}
		if expr.Operator == "instanceof" && expr.Right != nil {
			checkType(expr.Right.Name, pos)
		}
		if expr.Left != nil {
			expression(*expr.Left, pos)
		}
		if expr.Right != nil {
			expression(*expr.Right, pos)
		}
		for _, arg := range expr.Args {
			expression(arg, pos)
		}
		for _, arg := range expr.NamedArgs {
			expression(arg.Expr, pos)
		}
	}
	var instructions func([]ir.Instruction)
	scoped := func(rows []ir.Instruction) {
		scope.push()
		instructions(rows)
		scope.pop()
	}
	instructions = func(rows []ir.Instruction) {
		for _, row := range rows {
			loop := row.Op == ir.OpFor || row.Op == ir.OpForEach
			if loop {
				scope.push()
			}
			checkType(row.Type, row.Pos)
			if row.Op == ir.OpDeclare || row.Op == ir.OpForEach {
				scope.declare(row.Name, row.Type)
			}
			// C011-C014: retain the exact native assignment diagnostic for
			// Metadata targets without changing another platform's coercions.
			targetType := row.Type
			if row.Op == ir.OpAssign {
				checkField(row.Name, row.Pos)
				targetType, _ = irAssignmentTargetType(row.Name, scope, model, typ.Name)
			}
			ownedTarget := strings.Contains(strings.ToLower(targetType), "metadata.")
			if receiver, _, qualified := strings.Cut(row.Name, "."); qualified {
				ownedTarget = ownedTarget || strings.HasPrefix(strings.ToLower(semaIRReceiverType(receiver, scope, model, typ.Name)), "visualeditor.")
			}
			if (row.Op == ir.OpDeclare || row.Op == ir.OpAssign) && ownedTarget {
				valueType := a.inferIRExprType(row.Expr, scope, model, typ.Name)
				if valueType != "" && !strings.EqualFold(valueType, "null") && !semaAssignableToType(targetType, valueType, model) {
					appendDiagnostic("Illegal assignment from "+valueType+" to "+targetType, row.Pos)
				}
			}
			expression(row.Expr, row.Pos)
			if row.Init != nil {
				instructions([]ir.Instruction{*row.Init})
			}
			instructions(row.Inits)
			if row.Update != nil {
				instructions([]ir.Instruction{*row.Update})
			}
			instructions(row.Updates)
			if row.Op == ir.OpDeclGroup {
				instructions(row.Then)
			} else {
				scoped(row.Then)
			}
			scoped(row.Else)
			scoped(row.Catch)
			for _, clause := range row.Catches {
				for _, typeName := range clause.Types {
					checkType(typeName, clause.Pos)
				}
				scoped(clause.Body)
			}
			scoped(row.Finally)
			for _, clause := range row.Cases {
				for _, expr := range clause.Exprs {
					expression(expr, clause.Pos)
				}
				scoped(clause.Body)
			}
			if loop {
				scope.pop()
			}
		}
	}
	instructions(program.Instructions)
	return diagnostics
}

// C011-C014/C020 and E035-E037 retain the native assignment text. When the
// ordinary IR gate already rejects that assignment, annotate its diagnostic
// instead of emitting a second error; its existing code and message stay intact.
func reconcileApexMetadataAssignmentDiagnostics(diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	for i := 0; i < len(diagnostics); i++ {
		metadata := diagnostics[i]
		if metadata.Code != "GLADESEMA_METADATA" || metadata.Range == nil || !strings.HasPrefix(metadata.NativeMessage, "Illegal assignment from ") {
			continue
		}
		for j := range diagnostics {
			ordinary := &diagnostics[j]
			if ordinary.Code != "GLADESEMA018" || ordinary.File != metadata.File || ordinary.Range == nil || ordinary.Range.Start.Offset != metadata.Range.Start.Offset {
				continue
			}
			ordinary.NativeMessage = metadata.NativeMessage
			copy(diagnostics[i:], diagnostics[i+1:])
			diagnostics = diagnostics[:len(diagnostics)-1]
			i--
			break
		}
	}
	return diagnostics
}

// C011/C014 and E035-E037: the named-body scanner diagnoses the initializer
// before the IR gate. Preserve the measured Metadata text on that diagnostic
// before ordinary deduplication, bounded to the same assignment statement.
func preserveApexMetadataAssignmentDiagnostics(diagnostics, irDiagnostics []diagnostic.Diagnostic, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	for _, native := range irDiagnostics {
		if native.Code != "GLADESEMA018" || native.Range == nil || !strings.HasPrefix(native.NativeMessage, "Illegal assignment from ") {
			continue
		}
		_, targetType, _ := strings.Cut(native.NativeMessage, " to ")
		ownedTarget := false
		for _, ref := range extractTypeNames(targetType) {
			if target, ok := model.lookupName(ref); ok && target.platform && strings.EqualFold(target.namespace, "Metadata") {
				ownedTarget = true
			}
		}
		if !ownedTarget {
			continue
		}
		start := native.Range.Start.Offset
		end := semaStatementEnd(source, start)
		for i := range diagnostics {
			candidate := &diagnostics[i]
			if candidate.Code == native.Code && candidate.File == native.File && candidate.Range != nil && candidate.Range.Start.Offset >= start && candidate.Range.End.Offset <= end && candidate.NativeMessage == "" {
				candidate.NativeMessage = native.NativeMessage
			}
		}
	}
	return diagnostics
}
