package sema

import (
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestHTTPReceiverTraversalPreservesDiagnosticsAndTypes(t *testing.T) {
	variable := func(name string) *ir.Expr { return &ir.Expr{Kind: ir.ExprVariable, Name: name} }
	literal := func(value string) ir.Expr { return ir.Expr{Kind: ir.ExprLiteral, Value: value} }
	call := func(left *ir.Expr, method string, args ...ir.Expr) *ir.Expr {
		return &ir.Expr{Kind: ir.ExprCall, Callee: method, Left: left, Args: args}
	}
	cases := []struct {
		name   string
		expr   *ir.Expr
		shadow bool
	}{
		{name: "user fluent chain", expr: call(call(call(variable("wrapper"), "next"), "next"), "next")},
		{name: "unknown HTTP method", expr: call(call(variable("request"), "notAnHTTPMethod"), "size")},
		{name: "bad HTTP arguments", expr: call(call(variable("request"), "setTimeout", literal("'bad'")), "size")},
		{name: "HTTP return chain", expr: call(call(variable("response"), "getHeaderKeys"), "size")},
		{name: "mixed wrapper and HTTP", expr: call(call(call(call(variable("wrapper"), "next"), "response"), "getHeaderKeys"), "size")},
		{name: "cast visibility", expr: call(call(call(nil, "__cast:HttpRequest", *variable("missing")), "getBody"), "length")},
		{name: "safe call", expr: call(call(variable("request"), "__safe_call:getBody"), "length")},
		{name: "safe field", expr: call(call(call(variable("wrapper"), "__safe_field:reply"), "getBody"), "length")},
		{name: "project shadow", expr: call(call(variable("request"), "next"), "next"), shadow: true},
		{name: "explicit system constructor", expr: call(call(call(nil, "new:System.HttpResponse"), "getBody"), "length"), shadow: true},
		{name: "query receiver", expr: call(&ir.Expr{Kind: ir.ExprSOQL, Value: "SELECT Id FROM Account"}, "notAnHTTPMethod")},
		{name: "binary receiver", expr: call(&ir.Expr{Kind: ir.ExprBinary, Operator: "+", Left: &ir.Expr{Kind: ir.ExprLiteral, Value: "1"}, Right: &ir.Expr{Kind: ir.ExprLiteral, Value: "2"}}, "notAnHTTPMethod")},
		{name: "not receiver still checks left", expr: call(&ir.Expr{Kind: ir.ExprUnary, Operator: "!", Left: call(variable("request"), "notAnHTTPMethod")}, "notAnHTTPMethod")},
		{name: "string literal", expr: call(&ir.Expr{Kind: ir.ExprLiteral, Value: "'λ'"}, "notAnHTTPMethod")},
		{name: "keyword literal", expr: call(&ir.Expr{Kind: ir.ExprLiteral, Value: " true "}, "notAnHTTPMethod")},
		{name: "empty literal", expr: call(&ir.Expr{Kind: ir.ExprLiteral}, "notAnHTTPMethod")},
		{name: "literal fallback", expr: call(&ir.Expr{Kind: ir.ExprLiteral, Value: "new HttpRequest()"}, "notAnHTTPMethod")},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				typ := typesys.TypeSymbol{Kind: apexast.DeclarationClass, Name: "Probe", File: "Probe.cls", EffectiveAPIVersion: api}
				member := typesys.MemberSymbol{Kind: apexast.DeclarationMethod, Name: "run", Type: "void"}
				source := strings.Repeat(" ", 256)
				for _, inferType := range []bool{false, true} {
					gotAnalyzer, gotModel, gotScope := httpReceiverTestContext(tc.shadow)
					wantAnalyzer, wantModel, wantScope := httpReceiverTestContext(tc.shadow)
					var got []diagnostic.Diagnostic
					gotType := ""
					if inferType {
						got, gotType = gotAnalyzer.checkIRHTTPReceiverCallsWithType(typ, member, *tc.expr, gotScope, 3, 7, source, gotModel, nil, true)
					} else {
						got = gotAnalyzer.checkIRHTTPReceiverCalls(typ, member, *tc.expr, gotScope, 3, 7, source, gotModel, nil)
					}
					want := legacyHTTPReceiverCallsForTest(wantAnalyzer, typ, member, *tc.expr, wantScope, 3, 7, source, wantModel, nil)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("inferType=%v diagnostics\ngot: %#v\nwant: %#v", inferType, got, want)
					}
					if inferType {
						wantType := wantAnalyzer.inferIRExprType(*tc.expr, wantScope, wantModel, typ.Name)
						if gotType != wantType {
							t.Fatalf("inferred type = %q, want %q", gotType, wantType)
						}
					}
				}
			})
		}
	}
}

func httpReceiverTestContext(shadow bool) (*Analyzer, *semaTypeMemberView, irSemaScope) {
	members := map[string]typeMembers{
		"wrapper": {
			name: "Wrapper", kind: apexast.DeclarationClass,
			methods: map[string][]typesys.MemberSymbol{
				"next":     {{Kind: apexast.DeclarationMethod, Name: "next", Type: "Wrapper", Modifiers: []string{"public"}}},
				"response": {{Kind: apexast.DeclarationMethod, Name: "response", Type: "HttpResponse", Modifiers: []string{"public"}}},
			},
			fields: map[string]typesys.MemberSymbol{"reply": {Kind: apexast.DeclarationField, Name: "reply", Type: "HttpResponse", Modifiers: []string{"public"}}},
		},
	}
	if shadow {
		members["httprequest"] = typeMembers{name: "HttpRequest", kind: apexast.DeclarationClass, methods: map[string][]typesys.MemberSymbol{
			"next": {{Kind: apexast.DeclarationMethod, Name: "next", Type: "Wrapper", Modifiers: []string{"public"}}},
		}}
	}
	return NewAnalyzer(), semaTypeMemberViewFromMembers(members), newIRSemaScope(map[string]string{"wrapper": "Wrapper", "request": "HttpRequest", "response": "HttpResponse"})
}

// Preserve the previous walk as an independent ordering and diagnostic oracle.
func legacyHTTPReceiverCallsForTest(a *Analyzer, typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView, constructability map[string]typesys.TypeSymbol) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	if expr.Left != nil {
		diagnostics = append(diagnostics, legacyHTTPReceiverCallsForTest(a, typ, member, *expr.Left, scope, pos, bodyOffset, source, model, constructability)...)
	}
	if expr.Kind != ir.ExprCall {
		return diagnostics
	}
	if strings.HasPrefix(expr.Callee, "__cast:") {
		for _, arg := range expr.Args {
			diagnostics = append(diagnostics, a.checkIRExprVariables(typ, member, arg, &scope, pos, bodyOffset, source, model, constructability)...)
		}
		return diagnostics
	}
	if strings.HasPrefix(expr.Callee, "__") {
		return diagnostics
	}
	receiverType := ""
	if expr.Left != nil {
		receiverType = a.inferIRExprType(*expr.Left, scope, model, typ.Name)
	} else if receiver, _, ok := splitSemaMethodPath(expr.Callee); ok {
		receiverType = semaIRReceiverType(receiver, scope, model, typ.Name)
	}
	if semaHTTPReceiver(receiverType) != "" && !semaProjectTypeShadowsPlatform(model, receiverType) {
		diagnostics = append(diagnostics, a.checkIRCall(typ, member, expr, scope, pos, bodyOffset, source, model, constructability)...)
	}
	return diagnostics
}
