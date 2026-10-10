package apextest

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

func methodOverlayBaseMachine(t *testing.T, methods int) *vm.VM {
	t.Helper()
	program, err := vm.CompileAnonymous("return 1;")
	if err != nil {
		t.Fatal(err)
	}
	base := vm.New(nil)
	for i := 0; i < methods; i++ {
		name := fmt.Sprintf("Lib%d.call", i/4)
		params := make([]vm.Param, i%4)
		for p := range params {
			params[p] = vm.Param{Name: fmt.Sprintf("p%d", p), Type: "Integer"}
		}
		if err := base.RegisterMethod(vm.Method{Name: name, ClassName: fmt.Sprintf("Lib%d", i/4), ReturnType: "Integer", IsStatic: true, Params: params, Program: program}); err != nil {
			t.Fatal(err)
		}
	}
	base.FreezeClassLookup()
	return base
}

func mapPointer(value any) uintptr {
	return reflect.ValueOf(value).Pointer()
}

// snapshotMethodTable copies every map entry and nested mutable value, including
// overload slices, parameters, modifiers and the method program's IR tree.
// Immutable scalar values can be retained without aliasing the live tables.
func snapshotMethodTable(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Map:
		if value.IsNil() {
			return value
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		entries := value.MapRange()
		for entries.Next() {
			out.SetMapIndex(entries.Key(), snapshotMethodTable(entries.Value()))
		}
		return out
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			out.Index(i).Set(snapshotMethodTable(value.Index(i)))
		}
		return out
	case reflect.Pointer:
		if value.IsNil() {
			return value
		}
		out := reflect.New(value.Type().Elem())
		out.Elem().Set(snapshotMethodTable(value.Elem()))
		return out
	case reflect.Struct:
		out := reflect.New(value.Type()).Elem()
		for i := 0; i < value.NumField(); i++ {
			out.Field(i).Set(snapshotMethodTable(value.Field(i)))
		}
		return out
	default:
		return value
	}
}

// runCase registers the test method on a per-case clone. The frozen base's
// method tables must keep their identity and contents afterwards.
func TestRunCaseLeavesBaseMethodTablesIdentical(t *testing.T) {
	base := methodOverlayBaseMachine(t, 64)
	methodsPtr := mapPointer(base.Methods)
	overloadsPtr := mapPointer(base.MethodOverloads)
	foldedPtr := mapPointer(base.MethodFolded)
	methodsBefore := snapshotMethodTable(reflect.ValueOf(base.Methods)).Interface()
	overloadsBefore := snapshotMethodTable(reflect.ValueOf(base.MethodOverloads)).Interface()
	foldedBefore := snapshotMethodTable(reflect.ValueOf(base.MethodFolded)).Interface()

	body, err := vm.CompileAnonymous("Integer value = Lib0.call(); System.assertEquals(1, value);")
	if err != nil {
		t.Fatal(err)
	}
	invoke, err := vm.CompileAnonymous("OverlayTest.run();")
	if err != nil {
		t.Fatal(err)
	}
	testMethod := vm.Method{Name: "OverlayTest.run", ClassName: "OverlayTest", ReturnType: "void", IsStatic: true, Program: body}
	testCase := TestCase{ClassName: "OverlayTest", MethodName: "run"}
	for i := 0; i < 3; i++ {
		out := runCase(context.Background(), testCase, testMethod, nil, nil, invoke, nil, base, nil, nil, nil, storage.OrgState{}, 0, Options{}, false, nil, nil)
		if out.Status != testreport.StatusPass {
			t.Fatalf("runCase %d status = %s (%s)", i, out.Status, out.Reason)
		}
	}
	if mapPointer(base.Methods) != methodsPtr || mapPointer(base.MethodOverloads) != overloadsPtr || mapPointer(base.MethodFolded) != foldedPtr {
		t.Fatalf("runCase replaced the base method maps")
	}
	if !reflect.DeepEqual(base.Methods, methodsBefore) {
		t.Fatal("runCase changed the base Methods snapshot")
	}
	if !reflect.DeepEqual(base.MethodOverloads, overloadsBefore) {
		t.Fatal("runCase changed the base MethodOverloads snapshot")
	}
	if !reflect.DeepEqual(base.MethodFolded, foldedBefore) {
		t.Fatal("runCase changed the base MethodFolded snapshot")
	}
}

// Registering one test method on a clone must not copy the shared method
// tables. With 4000 base methods the old copy cost thousands of allocations.
func TestRegisterTestRuntimeOnCloneDoesNotCopyMethodTables(t *testing.T) {
	base := methodOverlayBaseMachine(t, 4000)
	method := vm.Method{Name: "OverlayTest.run", ClassName: "OverlayTest", ReturnType: "void", IsStatic: true}
	const runs = 20
	clones := make([]*vm.VM, runs+1)
	for i := range clones {
		clones[i] = base.CloneRuntimeFrozenShared(nil)
	}
	next := 0
	allocs := testing.AllocsPerRun(runs, func() {
		machine := clones[next]
		next++
		if err := registerTestRuntime(machine, []vm.Method{method}); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 32 {
		t.Fatalf("registerTestRuntime of one method on a clone allocated %.0f times; the shared method tables were copied", allocs)
	}
}
