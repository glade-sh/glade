package vm

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func overlayTestMethods() (base, extra []Method) {
	base = []Method{
		{Name: "Trail.mark", ClassName: "Trail", ReturnType: "String", IsStatic: true},
		{Name: "Trail.pick", ClassName: "Trail", ReturnType: "String", IsStatic: true, Params: []Param{{Name: "a", Type: "String"}}},
		{Name: "trail.PICK", ClassName: "Trail", ReturnType: "String", IsStatic: true, Params: []Param{{Name: "a", Type: "Integer"}}},
	}
	extra = []Method{
		{Name: "Trail.pick", ClassName: "Trail", ReturnType: "String", IsStatic: true, Params: []Param{{Name: "a", Type: "Decimal"}}},
		{Name: "TrailTest.run", ClassName: "TrailTest", ReturnType: "void", IsStatic: true},
		{Name: "Trail.pick", ClassName: "Trail", ReturnType: "String", IsStatic: true, Params: []Param{{Name: "a", Type: "Id"}}},
	}
	return base, extra
}

// effectiveMethodTables returns the tables as every lookup path sees them.
func effectiveMethodTables(vm *VM, names []string) (map[string]Method, map[string][]Method, map[string][]Method) {
	methods := make(map[string]Method)
	overloads := make(map[string][]Method)
	folded := make(map[string][]Method)
	for _, name := range names {
		if method, ok := vm.registeredMethod(name); ok {
			methods[name] = method
		}
		if list := vm.registeredOverloads(name); len(list) > 0 {
			overloads[name] = list
		}
		if list := vm.registeredFolded(strings.ToLower(name)); len(list) > 0 {
			folded[strings.ToLower(name)] = list
		}
	}
	return methods, overloads, folded
}

func TestCloneRegisterMethodOverlayMatchesOwnedTables(t *testing.T) {
	base, extra := overlayTestMethods()
	names := []string{"Trail.mark", "Trail.pick", "trail.PICK", "TrailTest.run", "Trail.missing"}

	want := New(nil)
	for _, method := range append(append([]Method(nil), base...), extra...) {
		if err := want.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}

	template := New(nil)
	for _, method := range base {
		if err := template.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	templateMethods := copyMethodMap(template.Methods)
	templateOverloads := copyMethodSliceMap(template.MethodOverloads)
	templateFolded := copyMethodSliceMap(template.MethodFolded)

	clone := template.CloneRuntime(nil)
	for _, method := range extra {
		if err := clone.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if !clone.runtimeArtifactsShared || clone.methodOverlay == nil {
		t.Fatalf("clone RegisterMethod copied the shared tables instead of using the overlay")
	}

	wantMethods, wantOverloads, wantFolded := effectiveMethodTables(want, names)
	gotMethods, gotOverloads, gotFolded := effectiveMethodTables(clone, names)
	if !reflect.DeepEqual(gotMethods, wantMethods) || !reflect.DeepEqual(gotOverloads, wantOverloads) || !reflect.DeepEqual(gotFolded, wantFolded) {
		t.Fatalf("overlay tables differ from owned tables\n got %#v\n%#v\n%#v\nwant %#v\n%#v\n%#v", gotMethods, gotOverloads, gotFolded, wantMethods, wantOverloads, wantFolded)
	}
	for _, name := range names {
		if got, want := clone.registeredMethodCandidates(name), want.registeredMethodCandidates(name); !reflect.DeepEqual(got, want) {
			t.Fatalf("registeredMethodCandidates(%q) = %#v, want %#v", name, got, want)
		}
	}

	// A clone of an overlay clone sees the same tables, and its own
	// registrations stay out of its source.
	grandchild := clone.CloneRuntime(nil)
	gotMethods, gotOverloads, gotFolded = effectiveMethodTables(grandchild, names)
	if !reflect.DeepEqual(gotMethods, wantMethods) || !reflect.DeepEqual(gotOverloads, wantOverloads) || !reflect.DeepEqual(gotFolded, wantFolded) {
		t.Fatalf("clone of overlay clone lost overlay entries")
	}
	if err := grandchild.RegisterMethod(Method{Name: "Trail.pick", ClassName: "Trail", IsStatic: true, Params: []Param{{Name: "a", Type: "Boolean"}}}); err != nil {
		t.Fatal(err)
	}
	if got := len(clone.registeredOverloads("Trail.pick")); got != len(wantOverloads["Trail.pick"]) {
		t.Fatalf("grandchild RegisterMethod changed its source overlay: %d overloads", got)
	}

	// Materializing (unregister) folds the overlay into private maps with the
	// same result as the owned path.
	removed := extra[1]
	want.unregisterMethod(removed)
	clone.unregisterMethod(removed)
	if clone.runtimeArtifactsShared || clone.methodOverlay != nil {
		t.Fatalf("unregister did not materialize private tables")
	}
	if !reflect.DeepEqual(clone.Methods, want.Methods) || !reflect.DeepEqual(clone.MethodOverloads, want.MethodOverloads) || !reflect.DeepEqual(clone.MethodFolded, want.MethodFolded) {
		t.Fatalf("materialized tables differ from owned tables")
	}

	if !reflect.DeepEqual(template.Methods, templateMethods) || !reflect.DeepEqual(template.MethodOverloads, templateOverloads) || !reflect.DeepEqual(template.MethodFolded, templateFolded) {
		t.Fatalf("clone registration mutated the template tables")
	}
}

func TestCloneRegisterTriggerMaterializesMethodOverlay(t *testing.T) {
	base, extra := overlayTestMethods()
	template := New(nil)
	for _, method := range base {
		if err := template.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	clone := template.CloneRuntime(nil)
	if err := clone.RegisterMethod(extra[0]); err != nil {
		t.Fatal(err)
	}
	wantOverloads := append([]Method(nil), clone.registeredOverloads("Trail.pick")...)
	if err := clone.RegisterTrigger(Trigger{Name: "TrailTrigger", Object: "Account", Timing: "before", Operation: "insert"}); err != nil {
		t.Fatal(err)
	}
	if clone.runtimeArtifactsShared || clone.methodOverlay != nil {
		t.Fatalf("RegisterTrigger did not materialize private tables")
	}
	if !reflect.DeepEqual(clone.MethodOverloads["Trail.pick"], wantOverloads) {
		t.Fatalf("materialized overloads = %#v, want %#v", clone.MethodOverloads["Trail.pick"], wantOverloads)
	}
	if len(template.Triggers["Account"]) != 0 || len(template.MethodOverloads["Trail.pick"]) != 1 {
		t.Fatalf("clone registration mutated the template")
	}
}

func overlayStaticMethod(t *testing.T, name, result string, params ...Param) Method {
	t.Helper()
	program, err := CompileAnonymous("return '" + result + "';")
	if err != nil {
		t.Fatal(err)
	}
	dot := strings.LastIndex(name, ".")
	return Method{Name: name, ClassName: name[:dot], ReturnType: "String", IsStatic: true, Params: params, Program: program}
}

func assertSameCallStatic(t *testing.T, got, want *VM, name string, args []Value) {
	t.Helper()
	gotValue, gotErr := got.CallStatic(name, args)
	wantValue, wantErr := want.CallStatic(name, args)
	if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) || !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("CallStatic(%s, %v) = %#v, %v; fresh machine = %#v, %v", name, args, gotValue, gotErr, wantValue, wantErr)
	}
}

func assertSameOverloadOrder(t *testing.T, got, want *VM, names ...string) {
	t.Helper()
	for _, name := range names {
		if gotList, wantList := got.registeredOverloads(name), want.registeredOverloads(name); !reflect.DeepEqual(gotList, wantList) {
			t.Fatalf("MethodOverloads[%s] = %#v, fresh machine = %#v", name, gotList, wantList)
		}
		folded := strings.ToLower(name)
		if gotList, wantList := got.registeredFolded(folded), want.registeredFolded(folded); !reflect.DeepEqual(gotList, wantList) {
			t.Fatalf("MethodFolded[%s] = %#v, fresh machine = %#v", folded, gotList, wantList)
		}
		gotMethod, gotOK := got.registeredMethod(name)
		wantMethod, wantOK := want.registeredMethod(name)
		if gotOK != wantOK || !reflect.DeepEqual(gotMethod, wantMethod) {
			t.Fatalf("Methods[%s] = %#v, fresh machine = %#v", name, gotMethod, wantMethod)
		}
	}
}

func sameOverlayMethodMultiset(got, want []Method) bool {
	if len(got) != len(want) {
		return false
	}
	matched := make([]bool, len(want))
	for _, method := range got {
		found := false
		for i, candidate := range want {
			if !matched[i] && reflect.DeepEqual(method, candidate) {
				matched[i] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func assertSameClassRegisteredMethods(t *testing.T, got, want *VM, names ...string) {
	t.Helper()
	for _, name := range names {
		gotList, wantList := got.registeredOverloads(name), want.registeredOverloads(name)
		if !sameOverlayMethodMultiset(gotList, wantList) {
			t.Fatalf("MethodOverloads[%s] = %#v, fresh machine = %#v", name, gotList, wantList)
		}
		folded := strings.ToLower(name)
		if gotList, wantList := got.registeredFolded(folded), want.registeredFolded(folded); !sameOverlayMethodMultiset(gotList, wantList) {
			t.Fatalf("MethodFolded[%s] = %#v, fresh machine = %#v", folded, gotList, wantList)
		}
		// RegisterClass ranges a method map, including distinct-file bodies
		// retained by mergeMethods. Its order is unspecified on both paths.
		// Methods must still contain each machine's last exact registration.
		for _, machine := range []*VM{got, want} {
			method, ok := machine.registeredMethod(name)
			list := machine.registeredOverloads(name)
			if ok != (len(list) > 0) || (ok && !reflect.DeepEqual(method, list[len(list)-1])) {
				t.Fatalf("Methods[%s] = %#v, present=%v; overloads = %#v", name, method, ok, list)
			}
		}
	}
}

func TestOverlayMethodMultisetComparisonPreservesDuplicatesAndBodies(t *testing.T) {
	first := overlayStaticMethod(t, "Trail.pick", "first", Param{Name: "a", Type: "String"})
	second := overlayStaticMethod(t, "Trail.pick", "second", Param{Name: "a", Type: "String"})
	want := []Method{first, first, second}
	for _, tc := range []struct {
		name  string
		got   []Method
		equal bool
	}{
		{"reordered", []Method{second, first, first}, true},
		{"missing", []Method{first, second}, false},
		{"different multiplicity", []Method{first, second, second}, false},
		{"different body", []Method{first, first, overlayStaticMethod(t, "Trail.pick", "third", Param{Name: "a", Type: "String"})}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameOverlayMethodMultiset(tc.got, want); got != tc.equal {
				t.Fatalf("method multiset comparison = %v, want %v", got, tc.equal)
			}
		})
	}
}

func TestCloneOverloadOfBaseNameMatchesFreshMachine(t *testing.T) {
	baseMethods := []Method{
		overlayStaticMethod(t, "Trail.pick", "string", Param{Name: "a", Type: "String"}),
		overlayStaticMethod(t, "Trail.pick", "integer", Param{Name: "a", Type: "Integer"}),
	}
	cloneMethods := []Method{
		overlayStaticMethod(t, "Trail.pick", "pair", Param{Name: "a", Type: "String"}, Param{Name: "b", Type: "Integer"}),
		overlayStaticMethod(t, "Trail.pick", "none"),
	}
	fresh := New(nil)
	template := New(nil)
	for _, method := range baseMethods {
		if err := fresh.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
		if err := template.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	clone := template.CloneRuntime(nil)
	for _, method := range cloneMethods {
		if err := fresh.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
		if err := clone.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if clone.methodOverlay == nil || !clone.runtimeArtifactsShared {
		t.Fatalf("clone did not use the method overlay")
	}
	assertSameOverloadOrder(t, clone, fresh, "Trail.pick")
	for _, args := range [][]Value{{String("x")}, {Int(1)}, {String("x"), Int(2)}, nil} {
		assertSameCallStatic(t, clone, fresh, "Trail.pick", args)
	}
	if got := len(template.MethodOverloads["Trail.pick"]); got != len(baseMethods) {
		t.Fatalf("template has %d Trail.pick overloads after clone registration, want %d", got, len(baseMethods))
	}
	if _, err := template.CallStatic("Trail.pick", nil); err == nil {
		t.Fatalf("template resolved a clone-only overload")
	}
}

func TestCloneRegisterClassReplacingAliasMatchesFreshMachine(t *testing.T) {
	classV1 := func() Class {
		return Class{Name: "Trail", Namespace: "pkg", Methods: map[string]Method{
			"pick": overlayStaticMethod(t, "Trail.pick", "v1", Param{Name: "a", Type: "String"}),
		}}
	}
	classV2 := func() Class {
		pick := overlayStaticMethod(t, "Trail.pick", "v2", Param{Name: "a", Type: "String"})
		pick.File = "classes/Trail.cls"
		return Class{Name: "Trail", Namespace: "pkg", Methods: map[string]Method{
			"pick":  pick,
			"other": overlayStaticMethod(t, "Trail.other", "other"),
		}}
	}
	extra := overlayStaticMethod(t, "Trail.pick", "integer", Param{Name: "a", Type: "Integer"})

	fresh := New(nil)
	template := New(nil)
	if err := fresh.RegisterClass(classV1()); err != nil {
		t.Fatal(err)
	}
	if err := template.RegisterClass(classV1()); err != nil {
		t.Fatal(err)
	}
	clone := template.CloneRuntime(nil)
	for _, machine := range []*VM{fresh, clone} {
		if err := machine.RegisterMethod(extra); err != nil {
			t.Fatal(err)
		}
	}
	if clone.methodOverlay == nil {
		t.Fatalf("clone did not use the method overlay before RegisterClass")
	}
	for _, machine := range []*VM{fresh, clone} {
		if err := machine.RegisterClass(classV2()); err != nil {
			t.Fatal(err)
		}
	}
	if clone.runtimeArtifactsShared || clone.methodOverlay != nil {
		t.Fatalf("RegisterClass merge did not materialize private tables")
	}
	assertSameClassRegisteredMethods(t, clone, fresh, "Trail.pick", "pkg.Trail.pick", "Trail.other", "pkg.Trail.other")
	// The internal cache clone must match fresh registration, without imposing
	// a Salesforce precedence contract on duplicate synthetic definitions.
	assertSameOverloadOrder(t, clone, fresh, "Trail.pick", "pkg.Trail.pick", "Trail.other", "pkg.Trail.other")
	for _, call := range []struct {
		name string
		args []Value
	}{
		{"Trail.pick", []Value{String("x")}},
		{"pkg.Trail.pick", []Value{String("x")}},
		{"Trail.pick", []Value{Int(1)}},
		{"Trail.other", nil},
		{"pkg.Trail.other", nil},
	} {
		assertSameCallStatic(t, clone, fresh, call.name, call.args)
	}
	value, err := template.CallStatic("Trail.pick", []Value{String("x")})
	if err != nil || value.Text != "v1" {
		t.Fatalf("template Trail.pick = %#v, %v after clone RegisterClass, want v1", value, err)
	}
	if _, ok := template.Methods["Trail.other"]; ok {
		t.Fatalf("clone RegisterClass mutated the template Methods")
	}
}
