package vm

import (
	"reflect"
	"testing"
)

func TestLocalListCallPreservesBindingView(t *testing.T) {
	for _, source := range []string{"items[at];", "items.get(at);"} {
		for _, storedView := range []string{"List<Object>", "List<Long>"} {
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			items := List(Int(3))
			items.Type, items.Static = "List<Integer>", storedView
			machine.Globals["items"] = items
			machine.Globals["at"] = Int(0)
			machine.VarTypes["items"] = "List<Integer>"
			expr := program.Instructions[0].Expr
			got, err := machine.eval(expr, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			receiver := items
			if expr.Left != nil {
				receiver, err = machine.lookup("items")
				if err != nil {
					t.Fatal(err)
				}
			}
			want, handled, err := machine.callValueMember("items", receiver, "get", []Value{Int(0)}, &Result{})
			if err != nil || !handled || !reflect.DeepEqual(got, want) {
				t.Fatalf("%s stored view=%s: eval=%#v shared=%#v/%v", source, storedView, got, want, err)
			}
		}
	}
}

func TestLocalListCallMatchesSharedDispatch(t *testing.T) {
	for _, source := range []string{"items.get(at);", "items.GeT(at);", "items.set(at,value);", "items.SeT(at,value);", "items[at];", "return items[at]=value;"} {
		for _, index := range []Value{Int(0), Int(-1), Int(2), Null, String("bad")} {
			for _, value := range []Value{Int(9), Null, String("bad"), List(Int(2))} {
				for _, flag := range []string{"", "__collection_readonly", "__enum_values_readonly"} {
					program, err := CompileAnonymous(source)
					if err != nil {
						t.Fatal(err)
					}
					expr := program.Instructions[0].Expr
					items := List(Int(3))
					items.Type = "List<Integer>"
					if flag != "" {
						items.Fields = map[string]Value{flag: Bool(true)}
					}
					makeVM := func() *VM {
						machine := New(nil)
						copyItems := items
						copyItems.List = append([]Value(nil), items.List...)
						machine.Globals["items"] = copyItems
						machine.Globals["alias"] = copyItems
						machine.Globals["at"] = index
						machine.Globals["value"] = value
						machine.VarTypes["items"] = items.Type
						return machine
					}
					fast, shared := makeVM(), makeVM()
					got, gotErr := fast.eval(expr, &Result{})
					method, args := "get", []Value{index}
					if len(expr.Args) == 2 {
						method, args = "set", []Value{index, value}
					}
					shared.markRootCollectionRefsEscaped(args...)
					want, handled, wantErr := shared.callValueMember("items", shared.Globals["items"], method, args, &Result{})
					if !handled {
						t.Fatal("shared dispatcher did not handle list call")
					}
					if wantErr == nil && expr.Operator == "[]=" {
						want = shared.Globals["items"].List[int(index.Int)]
					}
					if errorText(gotErr) != errorText(wantErr) || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(fast.Globals, shared.Globals) {
						t.Fatalf("%s index=%v value=%v flag=%q: eval=%#v/%v shared=%#v/%v globals equal=%v", source, index, value, flag, got, gotErr, want, wantErr, reflect.DeepEqual(fast.Globals, shared.Globals))
					}
				}
			}
		}
	}
}

func TestLocalListCallArgumentAllocation(t *testing.T) {
	for _, test := range []struct {
		source string
		budget float64
	}{
		{"items.get(at);", 1},
		{"items.set(at,value);", 6},
		{"items[at];", 3},
		{"items[at]=value;", 8},
	} {
		source := test.source
		t.Run(source, func(t *testing.T) {
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			items := List(Int(1), Int(2), Int(3), Int(4))
			items.Type = "List<Integer>"
			machine.Globals["items"] = items
			machine.Globals["at"] = Int(3)
			machine.Globals["value"] = Int(4)
			machine.VarTypes["items"] = items.Type
			machine.VarTypes["at"] = "Integer"
			machine.VarTypes["value"] = "Integer"
			result := &Result{}
			allocations := testing.AllocsPerRun(100, func() {
				got, err := machine.eval(program.Instructions[0].Expr, result)
				if err != nil {
					t.Fatal(err)
				}
				if source == "items.set(at,value);" {
					if got.Kind != ValueNull {
						t.Fatalf("set result = %v; want null", got)
					}
				} else if got.Kind != ValueInt || got.Int != 4 {
					t.Fatalf("result = %v; want 4", got)
				}
				if machine.Globals["items"].List[3].Int != 4 {
					t.Fatal("lost list contents")
				}
			})
			t.Logf("ROW local-list %s outcome=4 allocations=%.0f", source, allocations)
			// Preserve element conversion, alias publication and the bracket
			// receiver's declared-view lookup; remove argument/dispatch storage.
			if allocations > test.budget {
				t.Fatalf("local list call allocated %.0f times; want at most %.0f", allocations, test.budget)
			}
		})
	}
}
