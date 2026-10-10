package sema

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/ir"
)

func TestSecurityAccessMethodGateMatchesPreviousChecks(t *testing.T) {
	receivers := []string{"AccessLevel", "Security", "Database", "TxnSecurity.EventCondition", "UserProvisioning.UserProvisioningLog", "String", "Wrapper"}
	methods := []string{"name", "ordinal", "values", "valueOf", "stripInaccessible", "ſtripInaccessible", "query", "insert", "update", "delete", "upsert", "execute", "log", "toString", "", " \t ", "普通"}
	for _, receiver := range receivers {
		for _, method := range methods {
			for _, args := range [][]ir.Expr{nil, {{Kind: ir.ExprVariable, Name: "rows"}, {Kind: ir.ExprLiteral, Value: "null"}}} {
				expr := ir.Expr{Kind: ir.ExprCall, Callee: receiver + "." + method, Args: args}
				gotAnalyzer, gotModel, gotScope := securityAccessTestContext(false)
				wantAnalyzer, wantModel, wantScope := securityAccessTestContext(false)
				got := gotAnalyzer.securityAccessCallMessage(expr, gotScope, gotModel, "Probe")
				want := legacySecurityAccessCallMessageForTest(wantAnalyzer, expr, wantScope, wantModel, "Probe")
				if got != want {
					t.Fatalf("%q with %d arguments: got %q, want %q", expr.Callee, len(args), got, want)
				}
			}
		}
	}
}

func TestSecurityAccessMethodGatePreservesReceiverDiagnostics(t *testing.T) {
	cases := []struct {
		name   string
		expr   ir.Expr
		shadow bool
	}{
		{name: "left token", expr: ir.Expr{Kind: ir.ExprCall, Callee: "ignored", Left: &ir.Expr{Kind: ir.ExprVariable, Name: "AccessLevel.MISSING"}}},
		{name: "qualified token", expr: ir.Expr{Kind: ir.ExprCall, Callee: "AccessLevel.MISSING.ignored"}},
		{name: "left call", expr: ir.Expr{Kind: ir.ExprCall, Callee: "ignored", Left: &ir.Expr{Kind: ir.ExprCall, Callee: "AccessLevel.values"}}},
		{name: "left constructor", expr: ir.Expr{Kind: ir.ExprCall, Callee: "ignored", Left: &ir.Expr{Kind: ir.ExprCall, Callee: "new:TxnSecurity.EventCondition"}}},
		{name: "constructor with argument", expr: ir.Expr{Kind: ir.ExprCall, Callee: "new:TxnSecurity.Event", Args: []ir.Expr{{Kind: ir.ExprLiteral, Value: "null"}}}},
		{name: "constructor with named argument", expr: ir.Expr{Kind: ir.ExprCall, Callee: "new:TxnSecurity.EventCondition", NamedArgs: []ir.NamedArg{{Name: "value", Expr: ir.Expr{Kind: ir.ExprLiteral, Value: "null"}}}}},
		{name: "project shadow", expr: ir.Expr{Kind: ir.ExprCall, Callee: "Security.stripInaccessible"}, shadow: true},
		{name: "Unicode fold", expr: ir.Expr{Kind: ir.ExprCall, Callee: "Security.ſtripInaccessible"}},
		{name: "unqualified whitespace method", expr: ir.Expr{Kind: ir.ExprCall, Callee: " values ", Left: &ir.Expr{Kind: ir.ExprVariable, Name: "level"}}},
		{name: "unrelated chain", expr: ir.Expr{Kind: ir.ExprCall, Callee: "ignored", Left: &ir.Expr{Kind: ir.ExprCall, Callee: "wrapper.next"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotAnalyzer, gotModel, gotScope := securityAccessTestContext(tc.shadow)
			wantAnalyzer, wantModel, wantScope := securityAccessTestContext(tc.shadow)
			got := gotAnalyzer.securityAccessCallMessage(tc.expr, gotScope, gotModel, "Probe")
			want := legacySecurityAccessCallMessageForTest(wantAnalyzer, tc.expr, wantScope, wantModel, "Probe")
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func securityAccessTestContext(shadow bool) (*Analyzer, *semaTypeMemberView, irSemaScope) {
	members := map[string]typeMembers{}
	if shadow {
		members["security"] = typeMembers{name: "Security", kind: apexast.DeclarationClass}
	}
	return NewAnalyzer(), semaTypeMemberViewFromMembers(members), newIRSemaScope(map[string]string{"rows": "List<SObject>", "level": "AccessLevel", "wrapper": "Wrapper"})
}

// The previous implementation preserves exact method spelling and dispatch rules.
func legacySecurityAccessCallMessageForTest(a *Analyzer, expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) string {
	if expr.Left != nil {
		if message := legacySecurityAccessExpressionMessageForTest(a, *expr.Left, scope, model, currentType); message != "" {
			return message
		}
	}
	if strings.HasPrefix(expr.Callee, "new:") {
		typeName := resolveNestedTypeReference(model, currentType, strings.TrimPrefix(expr.Callee, "new:"))
		if strings.EqualFold(typeName, "TxnSecurity.EventCondition") && semaDMLPlatformType(model, typeName) && len(expr.Args)+len(expr.NamedArgs) == 0 {
			return "Type cannot be constructed: TxnSecurity.EventCondition"
		}
		if strings.EqualFold(typeName, "TxnSecurity.Event") && semaDMLPlatformType(model, typeName) && len(expr.Args)+len(expr.NamedArgs) == 0 {
			return "Constructor not defined: [TxnSecurity.Event].<Constructor>()"
		}
		return ""
	}
	receiver, method, qualified := splitSemaMethodPath(expr.Callee)
	if !qualified && expr.Left != nil {
		receiver = a.inferIRExprType(*expr.Left, scope, model, currentType)
		method = expr.Callee
	} else if qualified {
		if message := a.securityAccessFieldMessage(receiver, scope, model); message != "" {
			return message
		}
		if valueType, ok := scope.lookup(receiver); ok {
			receiver = valueType
		} else if fieldType := inferSemaArgTypeWithModel(receiver, scope.flatCopy(), model); fieldType != "" &&
			!semaDMLPlatformType(model, receiver) {
			receiver = fieldType
		}
	}
	// N056-N063: a lexically declared enum takes precedence over an
	// unqualified platform spelling; explicit System qualification stays intact.
	receiver = resolveNestedTypeReference(model, currentType, receiver)
	receiver = semaCanonicalPlatformAlias(receiver)
	switch normalizeName(receiver) {
	case "accesslevel", "security", "database", "txnsecurity.eventcondition", "userprovisioning.userprovisioninglog":
	default:
		return ""
	}
	if !semaDMLPlatformType(model, receiver) {
		return ""
	}
	argTypes := irCallArgTypes(a, expr.Args, scope, model, currentType)
	rejected := false
	nativeType := receiver
	switch normalizeName(receiver) {
	case "accesslevel":
		nativeType = "System.AccessLevel"
		switch normalizeName(method) {
		case "name", "ordinal", "values", "valueof":
			rejected = true // AccessLevel is a class, not an enum (R075-R084).
		}
	case "security":
		nativeType = "System.Security"
		if strings.EqualFold(method, "stripInaccessible") {
			rejected = !semaArgsMatchAny([][]string{{"AccessType", "List<SObject>"}, {"AccessType", "List<SObject>", "Boolean"}, {"AccessType", "List<SObject>", "Boolean", "Id"}}, argTypes, model)
		}
	case "database":
		switch normalizeName(method) {
		case "query":
			rejected = !semaArgsMatchAny([][]string{{"String"}, {"String", "AccessLevel"}}, argTypes, model)
		case "insert", "update", "delete", "upsert":
			if len(argTypes) == 2 && strings.EqualFold(argTypes[1], "null") {
				return fmt.Sprintf("Ambiguous method signature: void %s(%s, NULL)", method, argTypes[0])
			}
		}
	case "txnsecurity.eventcondition":
		rejected = strings.EqualFold(method, "execute")
	case "userprovisioning.userprovisioninglog":
		if strings.EqualFold(method, "log") {
			rejected = !semaArgsMatchAny([][]string{{"Id", "String"}, {"Id", "String", "String"}}, argTypes, model)
		}
	}
	if !rejected {
		return ""
	}
	for i, argType := range argTypes {
		if strings.EqualFold(argType, "null") {
			argTypes[i] = "NULL"
		} else if strings.EqualFold(semaCanonicalPlatformAlias(argType), "AccessType") {
			argTypes[i] = "System.AccessType"
		}
	}
	return fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(argTypes, ", "), nativeType)
}

func legacySecurityAccessExpressionMessageForTest(a *Analyzer, expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, currentType string) string {
	switch expr.Kind {
	case ir.ExprVariable:
		return a.securityAccessFieldMessage(expr.Name, scope, model)
	case ir.ExprCall:
		return legacySecurityAccessCallMessageForTest(a, expr, scope, model, currentType)
	}
	return ""
}
