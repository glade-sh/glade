package sema

import (
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// These reference implementations retain the flattening rules before memoizing.
func uncachedIRSemaScopeFlat(scope irSemaScope) map[string]string {
	out := make(map[string]string)
	for _, frame := range scope.frames {
		for name, binding := range frame {
			out[name] = binding.typ
		}
	}
	return out
}

func uncachedSemaScopeFlat(scope semaScopeModel, pos *int) map[string]string {
	out := make(map[string]string, len(scope.base)+len(scope.locals))
	for name, typ := range scope.base {
		out[name] = typ
	}
	starts := make(map[string]int, len(scope.locals))
	for _, local := range scope.locals {
		key := scope.localKey(local)
		if (pos == nil || *pos >= local.start && *pos <= local.scopeEnd) && local.start >= starts[key] {
			out[key] = local.typeName
			starts[key] = local.start
		}
	}
	return out
}

func requireIRSemaScopeFlat(t *testing.T, scope *irSemaScope) {
	t.Helper()
	if got, want := scope.flat(), uncachedIRSemaScopeFlat(*scope); !maps.Equal(got, want) {
		t.Fatalf("flat = %v, want %v", got, want)
	}
}

func TestIRSemaScopeFlatMemoMatchesUncached(t *testing.T) {
	for _, name := range []string{"", " ", "\t\n", "Value", " value ", "Ångström", "ÉCLAIR", "value\xff"} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			scope := newIRSemaScope(map[string]string{name: "String", "field": "Object"})
			requireIRSemaScopeFlat(t, &scope)
			before := maps.Clone(scope.flat())
			snapshot := scope.flat()
			scope.push()
			if !scope.declare("FIELD", "Integer") {
				t.Fatal("field shadow was rejected")
			}
			requireIRSemaScopeFlat(t, &scope)
			if scope.declare("field", "Long") {
				t.Fatal("duplicate local was accepted")
			}
			requireIRSemaScopeFlat(t, &scope)
			scope.declare(name, "Decimal")
			requireIRSemaScopeFlat(t, &scope)
			scope.pop()
			requireIRSemaScopeFlat(t, &scope)
			if !maps.Equal(scope.flat(), before) || !maps.Equal(snapshot, before) {
				t.Fatalf("root or retained snapshot changed: root=%v snapshot=%v want=%v", scope.flat(), snapshot, before)
			}
		})
	}
	var zero irSemaScope
	requireIRSemaScopeFlat(t, &zero)
	zero.declare("first", "String")
	requireIRSemaScopeFlat(t, &zero)
}

func TestIRSemaScopeFlatMemoCopiesAndClones(t *testing.T) {
	root := newIRSemaScope(map[string]string{"field": "Object"})
	rootSnapshot := root.flat()
	alias := root
	alias.declare("shared", "Boolean")
	requireIRSemaScopeFlat(t, &root)
	if root.flat()["shared"] != "Boolean" || rootSnapshot["shared"] != "" {
		t.Fatal("shared frame mutation did not invalidate the alias, or modified a snapshot")
	}
	left, right := root, root
	left.push()
	left.declare("local", "Integer")
	leftSnapshot := left.flat()
	right.push()
	right.declare("local", "String")
	for range 3 {
		requireIRSemaScopeFlat(t, &left)
		requireIRSemaScopeFlat(t, &right)
		if left.flat()["local"] != "Integer" || right.flat()["local"] != "String" {
			t.Fatal("same-depth sibling scopes shared the wrong flat map")
		}
	}
	if leftSnapshot["local"] != "Integer" {
		t.Fatal("sibling mutation changed a retained snapshot")
	}
	left.pop()
	requireIRSemaScopeFlat(t, &left)
	if left.flat()["local"] != "" {
		t.Fatal("popped local remained visible")
	}
	clone := cloneIRStatementScope(root)
	clone.declare("cloned", "Long")
	requireIRSemaScopeFlat(t, &clone)
	requireIRSemaScopeFlat(t, &root)
	if root.flat()["cloned"] != "" || clone.flat()["cloned"] != "Long" {
		t.Fatal("statement clone shared bindings with the original")
	}
}

func TestIRSemaScopeFlatMemoAnonymousShadow(t *testing.T) {
	for _, anonymous := range []bool{false, true} {
		scope := newIRSemaScope(nil)
		scope.anonymous = anonymous
		scope.declare("name", "String")
		snapshot := scope.flat()
		scope.push()
		if got := scope.declare("NAME", "Integer"); got != anonymous {
			t.Fatalf("anonymous=%v: shadow accepted=%v", anonymous, got)
		}
		requireIRSemaScopeFlat(t, &scope)
		scope.pop()
		requireIRSemaScopeFlat(t, &scope)
		if scope.flat()["name"] != "String" || snapshot["name"] != "String" {
			t.Fatal("root declaration changed after shadowing")
		}
	}
}

func TestSemaScopeFlatMemoMatchesUncached(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	minInt := -maxInt - 1
	for _, name := range []string{"", " ", "\t\n", "Value", " value ", "Ångström", "ÉCLAIR", "value\xff"} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			scope := semaScopeModel{
				base: map[string]string{normalizeName(name): "Object", "param": "Id"},
				locals: []semaLocal{
					{name: "param", typeName: "String", start: -1, scopeEnd: maxInt},
					{name: name, typeName: "String", start: 2, scopeEnd: 15},
					{name: name, typeName: "Integer", start: 5, scopeEnd: 8},
					{name: "other", typeName: "Long", start: 6, scopeEnd: 30},
					{name: name, typeName: "Decimal", start: 5, scopeEnd: 8},
					{name: "edge", typeName: "Boolean", start: maxInt, scopeEnd: maxInt},
				},
			}
			if got, want := scope.flat(), uncachedSemaScopeFlat(scope, nil); !maps.Equal(got, want) {
				t.Fatalf("flat = %v, want %v", got, want)
			}
			for _, pos := range []int{minInt, -1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 15, 16, 30, 31, maxInt, 7, 3, 31} {
				if got, want := scope.flatAt(pos), uncachedSemaScopeFlat(scope, &pos); !maps.Equal(got, want) {
					t.Fatalf("flatAt(%d) = %v, want %v", pos, got, want)
				}
			}
		})
	}
}

func TestSemaScopeFlatMemoInvalidationAndCopies(t *testing.T) {
	scope := semaScopeModel{base: map[string]string{"field": "Object"}}
	before := scope.flat()
	beforeAt := scope.flatAt(5)
	declare := func(scope *semaScopeModel, name, typ string, start, end int, catch bool) {
		t.Helper()
		if catch {
			if diags := scope.declareCatchLocal(typesys.TypeSymbol{}, typesys.MemberSymbol{}, name, typ, start, start, end, 0, "", 0, 0); len(diags) != 0 {
				t.Fatalf("declareCatchLocal: %v", diags)
			}
		} else if diags := scope.declareLocal(typesys.TypeSymbol{}, typesys.MemberSymbol{}, name, typ, start, start, end, 0, "", 0, 0); len(diags) != 0 {
			t.Fatalf("declareLocal: %v", diags)
		}
		if got, want := scope.flat(), uncachedSemaScopeFlat(*scope, nil); !maps.Equal(got, want) {
			t.Fatalf("flat after declaration = %v, want %v", got, want)
		}
		for _, pos := range []int{start - 1, start, end, end + 1} {
			if got, want := scope.flatAt(pos), uncachedSemaScopeFlat(*scope, &pos); !maps.Equal(got, want) {
				t.Fatalf("flatAt(%d) after declaration = %v, want %v", pos, got, want)
			}
		}
	}
	left, right := scope, scope
	declare(&left, "name", "Integer", 2, 10, false)
	declare(&right, "name", "String", 2, 10, false)
	for range 3 {
		if left.flat()["name"] != "Integer" || right.flat()["name"] != "String" || left.flatAt(5)["name"] != "Integer" || right.flatAt(5)["name"] != "String" {
			t.Fatal("same-length scope copies reused one another's maps")
		}
	}
	declare(&scope, "field", "Boolean", 2, 10, false)
	declare(&scope, "caught", "Exception", 12, 15, true)
	member := typesys.MemberSymbol{Parameters: []apexast.Parameter{{Name: "PARAM", Type: "Id"}}}
	if diags := declareSemaParameters(typesys.TypeSymbol{}, member, strings.Repeat(" ", 20), 0, "", &scope); len(diags) != 0 {
		t.Fatalf("declareSemaParameters: %v", diags)
	}
	if got, want := scope.flat(), uncachedSemaScopeFlat(scope, nil); !maps.Equal(got, want) {
		t.Fatalf("flat after parameter = %v, want %v", got, want)
	}
	if !maps.Equal(before, map[string]string{"field": "Object"}) || !maps.Equal(beforeAt, before) {
		t.Fatal("declarations changed a retained snapshot")
	}
}

// Preserve the original entry guard as an output oracle, including unusual
// preseeded depths. The caller's map is deliberately mutable in this reference.
func inferSemaArgTypeBeforeFlatMemo(arg string, scope map[string]string, model *semaTypeMemberView) string {
	if !enterSemaInference(scope) {
		return ""
	}
	defer leaveSemaInference(scope)
	arg = semaTrimSafeNavigationReceiverSuffix(strings.TrimSpace(arg))
	if scope != nil && semaInferenceTrackableArg(arg) {
		key := semaInferenceActiveKey(arg)
		if scope[key] != "" {
			return ""
		}
		scope[key] = "1"
		defer delete(scope, key)
	}
	return inferSemaArgTypeWithModelUncached(arg, scope, model)
}

func TestSemaFlatMemoInferenceGuardsPreserveInputsAndResults(t *testing.T) {
	expressions := []string{"", " \t\n", "value", " VALUE? ", "Ångström", "name\xff", "(value)", "value + other", "flag ? value : other", "value ?? other", "items[0]", "value.getName()", strings.Repeat("(", 70) + "value" + strings.Repeat(")", 70)}
	depths := []string{"", "0", "invalid", "-2", "1", "64", "65", "999999999999999999999999", "-999999999999999999999999"}
	for _, depth := range depths {
		for _, expression := range expressions {
			for _, active := range []bool{false, true} {
				bindings := map[string]string{"value": "Integer", "other": "Long", "flag": "Boolean", "items": "List<String>", normalizeName("Ångström"): "Decimal", "name\xff": "String", semaInferenceDepthScopeKey: depth}
				if active {
					bindings[semaInferenceActiveKey(semaTrimSafeNavigationReceiverSuffix(expression))] = "1"
				}
				scope := semaScopeModel{base: bindings}
				snapshot := scope.flat()
				before := maps.Clone(snapshot)
				originalMutable := maps.Clone(bindings)
				want := inferSemaArgTypeBeforeFlatMemo(expression, originalMutable, nil)
				mutable := scope.flatCopy()
				if got := inferSemaArgTypeWithModel(expression, mutable, nil); got != want {
					t.Fatalf("depth=%q active=%v expression=%q: got %q, want %q", depth, active, expression, got, want)
				}
				if !maps.Equal(mutable, originalMutable) {
					t.Fatalf("depth=%q active=%v expression=%q: inference guard side effects changed", depth, active, expression)
				}
				if !maps.Equal(snapshot, before) || !maps.Equal(scope.flat(), before) || !maps.Equal(bindings, before) {
					t.Fatalf("depth=%q active=%v expression=%q: shared scope changed", depth, active, expression)
				}
			}
		}
	}
	for _, expression := range expressions[:len(expressions)-1] {
		if got, want := inferSemaArgTypeWithModel(expression, nil, nil), inferSemaArgTypeBeforeFlatMemo(expression, nil, nil); got != want {
			t.Fatalf("nil scope, expression=%q: got %q, want %q", expression, got, want)
		}
	}
	irScope := newIRSemaScope(map[string]string{"value": "Integer"})
	snapshot := irScope.flat()
	want := maps.Clone(snapshot)
	inferSemaArgTypeWithModel("(value + value)", irScope.flatCopy(), nil)
	if !maps.Equal(irScope.flat(), want) || !maps.Equal(snapshot, want) {
		t.Fatal("inference changed the cached IR scope")
	}
}

func TestSemaScopeFlatMemoCopyIndependence(t *testing.T) {
	irScope := newIRSemaScope(map[string]string{"field": "String"})
	textScope := semaScopeModel{base: map[string]string{"field": "String"}, locals: []semaLocal{{name: "local", typeName: "Integer", start: 2, scopeEnd: 10}}}
	for _, test := range []struct {
		name string
		read func() map[string]string
		copy func() map[string]string
	}{
		{"ir", irScope.flat, irScope.flatCopy},
		{"text", textScope.flat, textScope.flatCopy},
		{"text_at", func() map[string]string { return textScope.flatAt(5) }, func() map[string]string { return textScope.flatAtCopy(5) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := test.read()
			want := maps.Clone(snapshot)
			copy := test.copy()
			copy["field"] = "Boolean"
			copy[semaCurrentTypeScopeKey] = "Owner"
			if !maps.Equal(snapshot, want) || !maps.Equal(test.read(), want) || !maps.Equal(test.copy(), want) {
				t.Fatal("mutable flat copy changed a cached snapshot")
			}
		})
	}
}

func TestInferFlattenedIRCallTypeNoCallMatchesHelpers(t *testing.T) {
	a := &Analyzer{}
	scope := newIRSemaScope(map[string]string{"field": "String"})
	for _, callee := range []string{"", " ", "method", "owner.method", "owner.method)", "name.getDescribe", "Ångström.ÉCLAIR", "name\xff"} {
		bindings := uncachedIRSemaScopeFlat(scope)
		want := inferSemaDescribeFieldChainType(callee, bindings, nil)
		if want == "" {
			want = inferSemaMethodCallType(callee, bindings, nil)
		}
		if got := a.inferFlattenedIRCallType(ir.Expr{Kind: ir.ExprCall, Callee: callee}, scope, nil, ""); got != want {
			t.Fatalf("callee=%q: got %q, want %q", callee, got, want)
		}
	}
}

var scopeFlatMemoSink map[string]string
var scopeInferenceMemoSink string

func TestSemaScopeFlatMemoWarmAllocations(t *testing.T) {
	irScope := newIRSemaScope(map[string]string{"name": "String", "other": "Integer"})
	textScope := semaScopeModel{base: map[string]string{"name": "String"}, locals: []semaLocal{{name: "other", typeName: "Integer", start: 2, scopeEnd: 20}}}
	irScope.flat()
	textScope.flat()
	textScope.flatAt(5)
	for _, test := range []struct {
		name string
		read func() map[string]string
	}{
		{"ir", irScope.flat},
		{"text", textScope.flat},
		{"text_interval", func() map[string]string { textScope.flatAt(4); return textScope.flatAt(10) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := testing.AllocsPerRun(100, func() { scopeFlatMemoSink = test.read() }); got != 0 {
				t.Fatalf("warm flat lookup allocated %g times", got)
			}
		})
	}
}

func BenchmarkIRSemaScopeFlatMemo(b *testing.B) {
	base := make(map[string]string, 128)
	for i := range 128 {
		base[fmt.Sprintf("binding%d", i)] = "String"
	}
	scope := newIRSemaScope(base)
	for _, memoized := range []bool{false, true} {
		b.Run(fmt.Sprintf("memoized=%v", memoized), func(b *testing.B) {
			b.ReportAllocs()
			scope.flat()
			b.ResetTimer()
			for b.Loop() {
				if memoized {
					scopeFlatMemoSink = scope.flat()
				} else {
					scopeFlatMemoSink = uncachedIRSemaScopeFlat(scope)
				}
			}
		})
	}
}

func BenchmarkSemaScopeFlatAtMemo(b *testing.B) {
	scope := semaScopeModel{base: map[string]string{"field": "Object"}}
	for i := range 32 {
		scope.locals = append(scope.locals, semaLocal{name: fmt.Sprintf("local%d", i), typeName: "String", start: i, scopeEnd: 100})
	}
	pos := 50
	for _, memoized := range []bool{false, true} {
		b.Run(fmt.Sprintf("memoized=%v", memoized), func(b *testing.B) {
			b.ReportAllocs()
			scope.flatAt(pos)
			b.ResetTimer()
			for b.Loop() {
				if memoized {
					scopeFlatMemoSink = scope.flatAt(pos)
				} else {
					scopeFlatMemoSink = uncachedSemaScopeFlat(scope, &pos)
				}
			}
		})
	}
}

func BenchmarkSemaFlatMemoRecursiveInference(b *testing.B) {
	scope := map[string]string{"value": "Integer"}
	for i := range 64 {
		scope[fmt.Sprintf("field%d", i)] = "String"
	}
	for _, copyScope := range []bool{false, true} {
		b.Run(fmt.Sprintf("copy_scope=%v", copyScope), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if copyScope {
					scopeInferenceMemoSink = inferSemaArgTypeWithModel("(value + value) * (value + value)", maps.Clone(scope), nil)
				} else {
					scopeInferenceMemoSink = inferSemaArgTypeWithModel("(value + value) * (value + value)", scope, nil)
				}
			}
		})
	}
}
