package apextest

import (
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func resolverTestIndex() typesys.Index {
	return typesys.Index{Types: []typesys.TypeSymbol{
		{Name: "ChoiceLab", Kind: apexast.DeclarationClass, EffectiveAPIVersion: "62.0", Members: []typesys.MemberSymbol{
			{Name: "item", Kind: apexast.DeclarationField, Type: "Pebble", Modifiers: []string{"public", "static"}},
			{Name: "name", Kind: apexast.DeclarationField, Type: "String", Modifiers: []string{"public"}},
		}},
		{Name: "ChoiceLab.Pebble", Kind: apexast.DeclarationClass, EffectiveAPIVersion: "62.0"},
		{Name: "OtherLab", Kind: apexast.DeclarationClass, EffectiveAPIVersion: "62.0"},
		{Name: "OtherLab.Pebble", Kind: apexast.DeclarationClass, EffectiveAPIVersion: "62.0"},
	}}
}

func countResolverBuilds(t *testing.T) *int {
	t.Helper()
	resetExpressionResolvers()
	count := new(int)
	old := newExpressionResolver
	newExpressionResolver = func(index typesys.Index) vm.ExpressionTypeResolver {
		*count++
		return sema.RuntimeExpressionTypeResolver(index)
	}
	t.Cleanup(func() {
		newExpressionResolver = old
		resetExpressionResolvers()
	})
	return count
}

func TestRuntimeExpressionResolverBuiltOncePerGeneration(t *testing.T) {
	builds := countResolverBuilds(t)
	index := resolverTestIndex()
	for range 5 {
		runtimeExpressionResolver("generation-a", index)
	}
	if *builds != 1 {
		t.Fatalf("resolver built %d times for one generation, want 1", *builds)
	}
	runtimeExpressionResolver("generation-b", index)
	if *builds != 2 {
		t.Fatalf("a new generation key must build its own resolver, builds=%d", *builds)
	}
	for _, key := range []runtimeCacheKey{"generation-c", "generation-d", "generation-e"} {
		runtimeExpressionResolver(key, index)
	}
	before := *builds
	runtimeExpressionResolver("generation-a", index)
	if *builds != before+1 {
		t.Fatalf("an evicted generation must rebuild, builds=%d want %d", *builds, before+1)
	}
	if len(expressionResolvers) > maxExpressionResolvers {
		t.Fatalf("retained %d resolvers, bound is %d", len(expressionResolvers), maxExpressionResolvers)
	}
	InvalidateRuntimeCaches()
	runtimeExpressionResolver("generation-a", index)
	if *builds != before+2 {
		t.Fatalf("InvalidateRuntimeCaches kept a resolver, builds=%d", *builds)
	}
	runtimeExpressionResolver("", index)
	runtimeExpressionResolver("", index)
	if *builds != before+4 {
		t.Fatalf("an empty key must never be cached, builds=%d", *builds)
	}
}

func TestRuntimeExpressionResolverCachedAnswersMatchFresh(t *testing.T) {
	countResolverBuilds(t)
	index := resolverTestIndex()
	fresh := sema.RuntimeExpressionTypeResolver(index)
	runtimeExpressionResolver("generation-a", index) // warm
	cached := runtimeExpressionResolver("generation-a", index)
	variables := map[string]string{"pebbles": "List<ChoiceLab.Pebble>", "texts": "List<String>"}
	exprs := []string{
		"true ? null : JSON.serialize(pebbles)",
		"true ? null : texts.get(0)",
		"true ? null : pebbles",
		"true ? null : ChoiceLab.item",
		"true ? null : ('a' + 'b')",
		"true ? null : null",
	}
	// Repeat, in two orders, so the shared resolver's own caches are exercised.
	for round := range 2 {
		for _, owner := range []string{"ChoiceLab", "OtherLab", "Missing", ""} {
			for i := range exprs {
				text := exprs[i]
				if round == 1 {
					text = exprs[len(exprs)-1-i]
				}
				program, err := vm.CompileAnonymous("return " + text + ";")
				if err != nil {
					t.Fatal(err)
				}
				var expr ir.Expr = program.Instructions[0].Expr
				want := fresh(expr, variables, owner)
				if got := cached(expr, variables, owner); got != want {
					t.Errorf("owner %q %s: cached %q, fresh %q", owner, text, got, want)
				}
			}
		}
	}
}

func TestRuntimeExpressionResolverSeparatesIndexesSharingAKey(t *testing.T) {
	countResolverBuilds(t)
	build := func(fieldType string) typesys.Index {
		return typesys.Index{Types: []typesys.TypeSymbol{
			{Name: "SourcelessHolder", Kind: apexast.DeclarationClass, Artifact: true, Members: []typesys.MemberSymbol{
				{Name: "slot", Kind: apexast.DeclarationField, Type: fieldType, Modifiers: []string{"public", "static"}},
			}},
		}}
	}
	program, err := vm.CompileAnonymous("return true ? null : SourcelessHolder.slot;")
	if err != nil {
		t.Fatal(err)
	}
	expr := program.Instructions[0].Expr
	// Same runtime key, as for sourceless types that differ only in members.
	asString := runtimeExpressionResolver("shared-key", build("String"))
	asInteger := runtimeExpressionResolver("shared-key", build("Integer"))
	if got := asString(expr, nil, "SourcelessHolder"); got != "String" {
		t.Fatalf("string model resolved %q", got)
	}
	if got := asInteger(expr, nil, "SourcelessHolder"); got != "Integer" {
		t.Fatalf("a resolver built for another model was reused: %q, want Integer", got)
	}
	// An equal index (separate allocation) still shares.
	builds := countResolverBuilds(t)
	runtimeExpressionResolver("shared-key", build("String"))
	runtimeExpressionResolver("shared-key", build("String"))
	if *builds != 1 {
		t.Fatalf("equal indexes built %d resolvers, want 1", *builds)
	}
}

func TestRuntimeExpressionResolverSharedAcrossGoroutines(t *testing.T) {
	countResolverBuilds(t)
	index := resolverTestIndex()
	program, err := vm.CompileAnonymous("return true ? null : ChoiceLab.item;")
	if err != nil {
		t.Fatal(err)
	}
	expr := program.Instructions[0].Expr
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				if got := runtimeExpressionResolver("generation-a", index)(expr, nil, "OtherLab"); got != "ChoiceLab.Pebble" {
					t.Errorf("resolved %q", got)
				}
			}
		}()
	}
	wg.Wait()
}
