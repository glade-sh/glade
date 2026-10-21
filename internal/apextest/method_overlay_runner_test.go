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

type methodTableSnapshot struct {
	typ   reflect.Type
	value any
}

// snapshotMethodTable records every field in the acyclic method/IR tree, including
// unexported fields. Kind-specific readers avoid Interface/Set restrictions on
// those fields, without unsafe access or retaining mutable aliases. DeepEqual
// compares these structural snapshots, preserving types and nil/empty distinctions.
func snapshotMethodTable(value reflect.Value) methodTableSnapshot {
	out := methodTableSnapshot{typ: value.Type()}
	switch value.Kind() {
	case reflect.Map:
		// Method tables and nested IR maps have string keys. Reject other shapes
		// rather than silently omitting newly introduced state.
		if value.Type().Key().Kind() != reflect.String {
			panic(fmt.Sprintf("unsupported method table map key: %s", value.Type().Key()))
		}
		if value.IsNil() {
			break
		}
		entriesBefore := make(map[string]methodTableSnapshot, value.Len())
		entries := value.MapRange()
		for entries.Next() {
			entriesBefore[entries.Key().String()] = snapshotMethodTable(entries.Value())
		}
		out.value = entriesBefore
	case reflect.Slice, reflect.Array, reflect.Struct:
		if value.Kind() == reflect.Slice && value.IsNil() {
			break
		}
		if value.Kind() == reflect.Struct {
			fields := make([]methodTableSnapshot, value.NumField())
			for i := range fields {
				fields[i] = snapshotMethodTable(value.Field(i))
			}
			out.value = fields
		} else {
			elements := make([]methodTableSnapshot, value.Len())
			for i := range elements {
				elements[i] = snapshotMethodTable(value.Index(i))
			}
			out.value = elements
		}
	case reflect.Pointer, reflect.Interface:
		if !value.IsNil() {
			out.value = snapshotMethodTable(value.Elem())
		}
	case reflect.Bool:
		out.value = value.Bool()
	case reflect.String:
		out.value = value.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		out.value = value.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		out.value = value.Uint()
	case reflect.Float32, reflect.Float64:
		out.value = value.Float()
	case reflect.Complex64, reflect.Complex128:
		out.value = value.Complex()
	default:
		panic(fmt.Sprintf("unsupported method table snapshot type: %s", value.Type()))
	}
	return out
}

// runCase registers the test method on a per-case clone. The frozen base's
// method tables must keep their identity and contents afterwards.
func TestRunCaseLeavesBaseMethodTablesIdentical(t *testing.T) {
	base := methodOverlayBaseMachine(t, 64)
	methodsPtr := mapPointer(base.Methods)
	overloadsPtr := mapPointer(base.MethodOverloads)
	foldedPtr := mapPointer(base.MethodFolded)
	methodsBefore := snapshotMethodTable(reflect.ValueOf(base.Methods))
	overloadsBefore := snapshotMethodTable(reflect.ValueOf(base.MethodOverloads))
	foldedBefore := snapshotMethodTable(reflect.ValueOf(base.MethodFolded))

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
	if !reflect.DeepEqual(snapshotMethodTable(reflect.ValueOf(base.Methods)), methodsBefore) {
		t.Fatal("runCase changed the base Methods snapshot")
	}
	if !reflect.DeepEqual(snapshotMethodTable(reflect.ValueOf(base.MethodOverloads)), overloadsBefore) {
		t.Fatal("runCase changed the base MethodOverloads snapshot")
	}
	if !reflect.DeepEqual(snapshotMethodTable(reflect.ValueOf(base.MethodFolded)), foldedBefore) {
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
