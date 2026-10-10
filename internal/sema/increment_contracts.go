package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func semaIRIncrementCall(expr ir.Expr) bool {
	return expr.Kind == ir.ExprCall && (expr.Callee == "__prefix:++" || expr.Callee == "__prefix:--" ||
		expr.Callee == "__postfix:++" || expr.Callee == "__postfix:--")
}

// semaIncrementRule is one increment shape in the native collection-increment
// captures at API 62.0 and 67.0. Named rows are
// deployed uncalled class methods (C) and @IsTest methods (R); anonymous rows
// are execute-anonymous bodies (R). Revision r4 rows (R4- with rejection twins
// C4-) are statements on both routes. Every other increment keeps the ordinary
// call path.
//
// A shape spells the operand from its root. L is a non-final local and T a
// class name. .f is a user-class instance field in a dotted name and :f one
// after an index or call; .s and :s are the SObject equivalents; .S is a
// static field; ?f and ?s are safe-navigation fields. [lit] and [var] index a
// list with a literal or a local, and (path) is a field read on a local; .lget(...)
// and .mget(...) are List.get and Map.get. ! marks a private field the caller
// cannot see; # marks a final one.
type semaIncrementRule struct {
	anonymous, statement bool
	shape, types         string
	// Empty means all four operators.
	operators string
}

const (
	semaPostIncrementOnly = "__postfix:++"
	semaPreIncrement      = "__prefix:++"
	semaPreDecrement      = "__prefix:--"
	semaPostfixOnly       = "__postfix:++ __postfix:--"
	semaPostIncPreInc     = "__postfix:++ __prefix:++"
	semaPostIncPreDec     = "__postfix:++ __prefix:--"
	semaNotPreDecrement   = "__postfix:++ __prefix:++ __postfix:--"
)

var semaIncrementRules = []semaIncrementRule{
	// Named result: T observed = <increment>;
	{shape: "L[lit]", types: "Integer Long Decimal Double String"},                        // C001-C032
	{shape: "L[var]", types: "Decimal String"},                                            // C033-C040
	{shape: "L.lget(lit)", types: "Decimal"},                                              // C042-C048 even
	{shape: "L", types: "Decimal Integer"},                                                // C049-C055 odd; R001-R004
	{shape: "L.mget(lit)", types: "Decimal"},                                              // C050-C056 even
	{shape: "L.s", types: "Integer Decimal String"},                                       // C057-C072; R005-R008
	{shape: "L.f.f", types: "Decimal String"},                                             // C073-C080; C121-C127 odd
	{shape: "L[lit]:f", types: "Decimal String"},                                          // C081-C088; C178-C182 even
	{shape: "L[lit]:f", types: "Integer", operators: semaNotPreDecrement},                 // C188-C192 even
	{shape: "L[lit]:#f", types: "Decimal Integer", operators: semaNotPreDecrement},        // C177-C181, C187-C191 odd
	{shape: "L[lit]:s", types: "Decimal String"},                                          // C089-C096
	{shape: "T.S", types: "Decimal String"},                                               // C097-C104
	{shape: "L.f", types: "Decimal String"},                                               // C105-C112; C113-C119, C137-C143 odd; R009-R012
	{shape: "L.f", types: "Integer", operators: semaPostIncrementOnly},                    // C196
	{shape: "L.#f", types: "Decimal Integer", operators: semaPostIncrementOnly},           // C185, C195
	{shape: "L.!f", types: "Decimal"},                                                     // C138-C144 even
	{shape: "L?f", types: "Decimal"},                                                      // C114-C120 even
	{shape: "L?f:f", types: "Decimal"},                                                    // C122-C128 even
	{shape: "L.f[lit]", types: "Decimal"},                                                 // C129-C135 odd
	{shape: "L?f[lit]", types: "Decimal"},                                                 // C130-C136 even
	{shape: "L[lit]:f:f", types: "Decimal"},                                               // C145-C151 odd
	{shape: "L[lit]:!f:f", types: "Decimal"},                                              // C146-C152 even
	{shape: "L.mget(lit):f:f", types: "Decimal"},                                          // C153-C159 odd
	{shape: "L.mget(lit):!f:f", types: "Decimal"},                                         // C154-C160 even
	{shape: "L.mget(var):f", types: "Decimal Integer", operators: semaPostIncrementOnly},  // C184, C194
	{shape: "L.mget(var):#f", types: "Decimal Integer", operators: semaPostIncrementOnly}, // C183, C193
	// Named statement: <increment>;
	{statement: true, shape: "L[lit]", types: "Integer Decimal String"},                    // C161-C176; R013-R020; C4-40, C4-43
	{statement: true, shape: "L[lit]", types: "Long Double", operators: semaPostIncPreDec}, // R4-40, R4-43
	{statement: true, shape: "L[lit]:s", types: "Decimal"},                                 // R021-R024
	// Named statement on a Map.get element's field. The r4 @IsTest rows hold
	// Map<String,Account> and Map<Id,Account> (Decimal AnnualRevenue, Integer
	// NumberOfEmployees, String Name) and Map<String,C> for a class C with
	// Long, Double and String fields; ?s and ?f are the safe-navigation twins.
	{statement: true, shape: "L.mget(lit):s", types: "Decimal Integer String", operators: semaPostIncPreInc},  // R4-01, R4-16; C4-01, C4-16
	{statement: true, shape: "L.mget(var):s", types: "Decimal Integer String", operators: semaPreDecrement},   // R4-08; C4-08
	{statement: true, shape: "L.mget(var):s", types: "Decimal Integer", operators: semaPostIncrementOnly},     // R4-13, R4-26 (missing key)
	{statement: true, shape: "L.mget(path):s", types: "Decimal Integer String", operators: semaPostfixOnly},   // R4-10, R4-22; C4-10, C4-22
	{statement: true, shape: "L.mget(var)?s", types: "Decimal Integer", operators: semaPostIncrementOnly},     // C4-13, C4-26
	{statement: true, shape: "L.mget(lit):f", types: "Long Double String", operators: semaPreIncrement},       // R4-29; C4-29
	{statement: true, shape: "L.mget(var):f", types: "Long Double String", operators: semaPreDecrement},       // R4-34; C4-34
	{statement: true, shape: "L.mget(var):f", types: "Long Double", operators: semaPostIncrementOnly},         // R4-39 (missing key)
	{statement: true, shape: "L.mget(path):f", types: "Long Double String", operators: semaPostIncrementOnly}, // R4-35; C4-35
	{statement: true, shape: "L.mget(var)?f", types: "Long Double", operators: semaPostIncrementOnly},         // C4-39
	// Anonymous result and statement.
	{anonymous: true, shape: "L", types: "Integer"},                                  // R001-R004
	{anonymous: true, shape: "L.s", types: "Integer"},                                // R005-R008
	{anonymous: true, shape: "L.f", types: "Decimal"},                                // R009-R012
	{anonymous: true, statement: true, shape: "L[lit]", types: "Integer Decimal"},    // R013-R020
	{anonymous: true, statement: true, shape: "L[lit]", types: "Long Double String"}, // R4-40-R4-43; C4-40-C4-43
	{anonymous: true, statement: true, shape: "L[lit]:s", types: "Decimal"},          // R021-R024
	// Anonymous statement on a Map.get element's field, as in the named group.
	{anonymous: true, statement: true, shape: "L.mget(lit):s", types: "Decimal Integer String"},                            // R4-01-R4-04, R4-14-R4-17; C4-01-C4-04, C4-14-C4-17
	{anonymous: true, statement: true, shape: "L.mget(var):s", types: "Decimal Integer String"},                            // R4-05-R4-08, R4-13, R4-18-R4-21, R4-26; C4-05-C4-08, C4-18-C4-21
	{anonymous: true, statement: true, shape: "L.mget(path):s", types: "Decimal Integer String"},                           // R4-09-R4-12, R4-22-R4-25; C4-09-C4-12, C4-22-C4-25
	{anonymous: true, statement: true, shape: "L.mget(var)?s", types: "Decimal Integer", operators: semaPostIncrementOnly}, // C4-13, C4-26
	{anonymous: true, statement: true, shape: "L.mget(lit):f", types: "Long Double String"},                                // R4-27-R4-30; C4-27-C4-30
	{anonymous: true, statement: true, shape: "L.mget(var):f", types: "Long Double String"},                                // R4-31-R4-34, R4-39; C4-31-C4-34
	{anonymous: true, statement: true, shape: "L.mget(path):f", types: "Long Double String"},                               // R4-35-R4-38; C4-35-C4-38
	{anonymous: true, statement: true, shape: "L.mget(var)?f", types: "Long Double", operators: semaPostIncrementOnly},     // C4-39
}

type semaIncrementTarget struct {
	shape   string
	private *diagnostic.Diagnostic
	final   *resolvedMember
}

// checkIRIncrementOrExprVariables checks a declaration initializer or an
// expression statement. A captured increment there gets its storage contract;
// anything else, including an increment elsewhere, keeps the ordinary check.
func (a *Analyzer) checkIRIncrementOrExprVariables(typ typesys.TypeSymbol, member typesys.MemberSymbol, inst ir.Instruction, scope *irSemaScope, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	target, operand, ok := a.semaCapturedIncrement(typ, member, inst, *scope, bodyOffset, source, model)
	if !ok {
		if scope.uncapturedPrefix != nil && semaPrefixFallbackStatement(inst, bodyOffset, source) {
			*scope.uncapturedPrefix = true
		}
		return a.checkIRExprVariables(typ, member, inst.Expr, scope, inst.Pos, bodyOffset, source, model, constructability)
	}
	if scope.approvedPrefix != nil && semaPrefixFallbackStatement(inst, bodyOffset, source) {
		// The runtime lowers exactly the candidates a captured rule covers.
		scope.approvedPrefix[inst.Pos] = true
	}
	diagnostics := semaIncrementDiagnostics(typ, member, inst.Expr.Callee, target, operand, bodyOffset+inst.Pos, source)
	return append(diagnostics, a.checkIRExprVariables(typ, member, *inst.Expr.Left, scope, inst.Pos, bodyOffset, source, model, constructability)...)
}

func (a *Analyzer) semaCapturedIncrement(typ typesys.TypeSymbol, member typesys.MemberSymbol, inst ir.Instruction, scope irSemaScope, bodyOffset int, source string, model *semaTypeMemberView) (semaIncrementTarget, string, bool) {
	expr := inst.Expr
	if !semaIRIncrementCall(expr) || expr.Left == nil || scope.incrementHeader ||
		typ.Kind != apexast.DeclarationClass || member.Kind != apexast.DeclarationMethod || hasModifier(typ.Modifiers, vm.AnonymousClassModifier) {
		return semaIncrementTarget{}, "", false
	}
	var target semaIncrementTarget
	shape, ok := a.semaIncrementShape(*expr.Left, typ, member, scope, model, bodyOffset+inst.Pos, source, &target)
	if !ok {
		return semaIncrementTarget{}, "", false
	}
	target.shape = shape
	operand := a.inferIRExprType(*expr.Left, scope, model, typ.Name)
	statement := inst.Op == ir.OpExpr
	// Every result row declares a local of the operand's own type.
	if !statement && !strings.EqualFold(resolveNestedTypeReference(model, typ.Name, inst.Type), operand) {
		return semaIncrementTarget{}, "", false
	}
	for _, rule := range semaIncrementRules {
		if rule.anonymous == scope.anonymous && rule.statement == statement && rule.shape == shape &&
			semaWordListContains(rule.types, operand) && (rule.operators == "" || semaWordListContains(rule.operators, expr.Callee)) {
			return target, operand, true
		}
	}
	return semaIncrementTarget{}, "", false
}

// ApprovedPrefixStatements returns the source offsets of the ++/-- statements a
// class method body may lower through vm.CompileOptions.ApprovedPrefixStatements:
// the prefix candidates a captured named rule covers, decided by the same walk
// as the method's semantic check. It returns nil when no candidate is covered
// or the body keeps the name-path result.
func ApprovedPrefixStatements(index typesys.Index, typ typesys.TypeSymbol, member typesys.MemberSymbol, source string) map[int]bool {
	if !semaPrefixStatementCandidates(typ, member, false) {
		return nil
	}
	body, bodyOffset, ok := semaBodyFromRange(source, member.BodyRange)
	if !ok || !semaPrefixCandidatesLowerBody(body) {
		return nil
	}
	setup := anonymousSetupFor(index, typ.EffectiveAPIVersion)
	model := setup.typeMembers.view()
	member = semaNormalizeMemberTypes(model, typ.Name, member)
	approved := map[int]bool{}
	if _, compiled := setup.analyzer().checkBodyIRWithScopeOptions(typ, member, body, bodyOffset, source, semaBodyBaseScope(typ, member, model), model, buildConstructability(setup.index), false, approved); !compiled || len(approved) == 0 {
		return nil
	}
	offsets := make(map[int]bool, len(approved))
	for pos := range approved {
		offsets[bodyOffset+pos] = true
	}
	return offsets
}

// semaPrefixCandidatesLowerBody reports a body that only lowers with its prefix
// candidates; any other body needs no approval.
func semaPrefixCandidatesLowerBody(body string) bool {
	if _, err := vm.CompileAnonymous(body); err == nil {
		return false
	}
	_, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{PrefixStatementCandidates: true})
	return err == nil
}

// semaPrefixStatementCandidates reports a body whose VM prefix candidates are
// typed against the captured rules: an execute-anonymous body or a class
// method. Every other body keeps the name path, as the runtime does.
func semaPrefixStatementCandidates(typ typesys.TypeSymbol, member typesys.MemberSymbol, anonymous bool) bool {
	return anonymous || typ.Kind == apexast.DeclarationClass && member.Kind == apexast.DeclarationMethod && !hasModifier(typ.Modifiers, vm.AnonymousClassModifier)
}

// semaPrefixFallbackStatement reports a statement ++x...; or --x...; that the
// VM lowered as an expression: its name path stops at an index or a get call.
// A name-path prefix statement lowers to an assignment instead. Whitespace
// and comments may follow the operator.
func semaPrefixFallbackStatement(inst ir.Instruction, bodyOffset int, source string) bool {
	if inst.Op != ir.OpExpr || inst.Expr.Callee != "__prefix:++" && inst.Expr.Callee != "__prefix:--" {
		return false
	}
	start := bodyOffset + inst.Pos
	if start < 0 || start+2 >= len(source) || !strings.HasPrefix(source[start:], "++") && !strings.HasPrefix(source[start:], "--") {
		return false
	}
	next := start + 2
	for next < len(source) {
		if isWhitespace(source[next]) {
			next++
		} else if end, comment := skipSemaComment(source, next); comment {
			next = end + 1
		} else {
			break
		}
	}
	return next < len(source) && ('a' <= source[next] && source[next] <= 'z' || 'A' <= source[next] && source[next] <= 'Z' || source[next] == '_')
}

func semaWordListContains(list, word string) bool {
	for _, item := range strings.Fields(list) {
		if strings.EqualFold(item, word) {
			return true
		}
	}
	return false
}

// semaIncrementShape spells an operand as a semaIncrementRule shape. It fails
// for any receiver the rules cannot name.
func (a *Analyzer) semaIncrementShape(node ir.Expr, typ typesys.TypeSymbol, member typesys.MemberSymbol, scope irSemaScope, model *semaTypeMemberView, start int, source string, target *semaIncrementTarget) (string, bool) {
	switch node.Kind {
	case ir.ExprVariable:
		return a.semaIncrementPathShape(node.Name, typ, member, scope, model, start, source, target)
	case ir.ExprCall:
		if node.Left == nil {
			// The parser flattens a get call on a bare local: l.get(0), m.get(k).
			root, method, found := strings.Cut(node.Callee, ".")
			rootType, local := semaIncrementLocal(root, scope)
			if !found || method != "get" || !local || len(node.Args) != 1 {
				return "", false
			}
			argument, ok := semaIncrementArgument(node.Args[0], scope, model)
			switch base, _ := semaGenericBaseAndArgs(rootType); {
			case ok && strings.EqualFold(base, "List"):
				return "L.lget(" + argument + ")", true
			case ok && strings.EqualFold(base, "Map"):
				return "L.mget(" + argument + ")", true
			}
			return "", false
		}
		left, ok := a.semaIncrementShape(*node.Left, typ, member, scope, model, start, source, target)
		if !ok {
			return "", false
		}
		receiverType := a.inferIRExprType(*node.Left, scope, model, typ.Name)
		switch {
		case node.Callee == "get" && node.Operator == "[]" && len(node.Args) == 1:
			argument, ok := semaIncrementArgument(node.Args[0], scope, model)
			if base, _ := semaGenericBaseAndArgs(receiverType); !ok || !strings.EqualFold(base, "List") {
				return "", false
			}
			return left + "[" + argument + "]", true
		case strings.HasPrefix(node.Callee, "__field:"):
			step, ok := semaIncrementFieldStep(":", receiverType, strings.TrimPrefix(node.Callee, "__field:"), false, typ, member, model, start, source, target)
			return left + step, ok
		case strings.HasPrefix(node.Callee, "__safe_field:"):
			step, ok := semaIncrementFieldStep("?", receiverType, strings.TrimPrefix(node.Callee, "__safe_field:"), false, typ, member, model, start, source, target)
			return left + step, ok
		}
	}
	return "", false
}

// A dotted name starts at a non-final local or, for a static field, a class.
func (a *Analyzer) semaIncrementPathShape(name string, typ typesys.TypeSymbol, member typesys.MemberSymbol, scope irSemaScope, model *semaTypeMemberView, start int, source string, target *semaIncrementTarget) (string, bool) {
	if strings.Contains(name, "?") {
		return "", false
	}
	parts := strings.Split(name, ".")
	shape := "L"
	current, local := semaIncrementLocal(parts[0], scope)
	if !local {
		if _, bound := scope.binding(parts[0]); bound || len(parts) != 2 {
			return "", false
		}
		members, ok := model.lookup(normalizeName(parts[0]))
		if !ok || members.sobject || semaIRReceiverType(parts[0], scope, model, typ.Name) == "" {
			return "", false
		}
		shape, current = "T", parts[0]
	}
	for i := 1; i < len(parts); i++ {
		step, ok := semaIncrementFieldStep(".", current, parts[i], shape == "T", typ, member, model, start, source, target)
		if !ok {
			return "", false
		}
		shape += step
		current = a.inferIRExprType(ir.Expr{Kind: ir.ExprVariable, Name: strings.Join(parts[:i+1], ".")}, scope, model, typ.Name)
	}
	return shape, true
}

func semaIncrementLocal(name string, scope irSemaScope) (string, bool) {
	binding, ok := scope.binding(name)
	return binding.typ, ok && binding.origin == irSemaOriginLocal && name != "" && !strings.Contains(name, ".")
}

// semaIncrementArgument names an index or key: a literal, a local, or (path)
// one SObject field read on a local, as acc.Name and acc.Id in R4-09-R4-12,
// R4-22-R4-25 and R4-35-R4-38. A property or method key is not captured.
func semaIncrementArgument(arg ir.Expr, scope irSemaScope, model *semaTypeMemberView) (string, bool) {
	if arg.Kind == ir.ExprLiteral {
		return "lit", true
	}
	if arg.Kind != ir.ExprVariable || strings.Contains(arg.Name, "?") {
		return "", false
	}
	root, field, dotted := strings.Cut(arg.Name, ".")
	rootType, local := semaIncrementLocal(root, scope)
	if !local {
		return "", false
	}
	if !dotted {
		return "var", true
	}
	if strings.Contains(field, ".") || !isSemaSObjectLike(rootType, model) {
		return "", false
	}
	if resolved, ok := semaResolveFieldPath(model, rootType, field); !ok || resolved.member.Kind != apexast.DeclarationField {
		return "", false
	}
	return "path", true
}

// semaIncrementFieldStep names one field of the operand. User-class fields
// must be public, or private and invisible to the caller; SObject fields must
// be writable.
func semaIncrementFieldStep(separator, receiverType, field string, static bool, typ typesys.TypeSymbol, member typesys.MemberSymbol, model *semaTypeMemberView, start int, source string, target *semaIncrementTarget) (string, bool) {
	resolved, ok := semaResolveFieldPath(model, receiverType, field)
	if !ok || resolved.member.Kind != apexast.DeclarationField {
		return "", false
	}
	// Standard SObject field members carry a static marker; SObject fields
	// are only reached through a record.
	if isSemaSObjectLike(receiverType, model) {
		if static || !semaIncrementSObjectFieldWritable(model, receiverType, field, resolved) {
			return "", false
		}
		return separator + "s", true
	}
	if hasModifier(resolved.member.Modifiers, "static") != static {
		return "", false
	}
	// Every captured user-class field is declared on the receiver's own class.
	if normalizeName(resolved.owner) != normalizeName(receiverType) {
		return "", false
	}
	marker := ""
	switch accessModifier(resolved.member.Modifiers) {
	case "public":
	case "private":
		d, blocked := checkSemaMemberAccess(typ, member, field, resolved, start, start+1, source, model)
		if !blocked || target.private != nil {
			return "", false
		}
		d.NativeMessage = "Variable is not visible: " + resolved.owner + "." + resolved.member.Name
		target.private = &d
		marker = "!"
	default:
		return "", false
	}
	if hasModifier(resolved.member.Modifiers, "final") {
		if marker != "" || target.final != nil {
			return "", false
		}
		target.final = &resolved
		marker = "#"
	}
	if static {
		return separator + marker + "S", true
	}
	return separator + marker + "f", true
}

func semaIncrementSObjectFieldWritable(model *semaTypeMemberView, receiverType, field string, resolved resolvedMember) bool {
	if _, readOnly := semaFormulaReadOnlyField(model, receiverType, field); readOnly {
		return false
	}
	if semaStandardFieldAssignmentReadOnly(model, receiverType, field) || semaPlatformEventWriteMessage(receiverType, field, model) != "" {
		return false
	}
	// A standard field described as neither createable nor updateable, such
	// as a calculated one, is not a captured target either.
	if definition, known := storage.StandardObjectDefinition(resolved.owner); known {
		for name, described := range definition.Fields {
			if strings.EqualFold(name, resolved.member.Name) && !storage.FieldFlagValue(described.Createable, true) && !storage.FieldFlagValue(described.Updateable, true) {
				return false
			}
		}
	}
	return semaProjectTypeShadowsPlatform(model, receiverType) || !semaAPI67ReadOnlyPlatformField(resolved.owner+"."+resolved.member.Name)
}

// semaIncrementDiagnostics reports the native verdict for a captured shape.
func semaIncrementDiagnostics(typ typesys.TypeSymbol, member typesys.MemberSymbol, callee string, target semaIncrementTarget, operand string, start int, source string) []diagnostic.Diagnostic {
	switch {
	case strings.Contains(target.shape, "?") || strings.HasSuffix(target.shape, ")"):
		// C114-C136 even: safe navigation. C042-C056 even: get() results.
		return []diagnostic.Diagnostic{typeContractNativeDiagnostic(typ, "Expression cannot be assigned", start, start+1, source)}
	case target.private != nil:
		// C138-C160 even.
		return []diagnostic.Diagnostic{*target.private}
	case target.final != nil:
		// C177-C195 odd.
		d, _ := semaInstanceFinalFieldAssignmentDiagnostic(typ, member, *target.final, false, start, start+1, source)
		return []diagnostic.Diagnostic{d}
	case strings.EqualFold(operand, "String"):
		operator := "postfix increment/decrement"
		if callee == "__prefix:++" {
			operator = "prefix increment"
		} else if callee == "__prefix:--" {
			operator = "prefix decrement"
		}
		return []diagnostic.Diagnostic{typeContractNativeDiagnostic(typ, "Unary "+operator+" can only be applied to numeric expressions: String", start, start+1, source)}
	}
	return nil
}

// semaIRDeclaredFinal reports a local declaration written final. The VM
// records the declaration at its type, so the modifier comes just before it.
func semaIRDeclaredFinal(source string, typeStart int) bool {
	end := min(max(typeStart, 0), len(source))
	for end > 0 && strings.IndexByte(" \t\r\n", source[end-1]) >= 0 {
		end--
	}
	begin := end - len("final")
	return begin >= 0 && strings.EqualFold(source[begin:end], "final") && (begin == 0 || !isSemaIdentifierChar(source[begin-1]))
}
