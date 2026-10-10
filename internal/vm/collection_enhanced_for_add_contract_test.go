package vm

import "testing"

// API67 ordinary nonzero-Ref List/Set enhanced-for additions: retained guide
// apex-guide/langCon_apex_collections_iterating.md SHA256
// 0e3c2ebc90e51d62a9cb4a4532ed8876f35b857f1d01c721b5a1f8e751abe648,
// lines 3-6 forbid direct addition during iteration; lines 10-13 admit adding
// staged elements after iteration. These four exact accepted receipt bodies
// establish two error-existence predicates and four after-loop assertions.
// No exception class, message, catchability or timing predicate is asserted.
// Ref0, addAll, remove, clear, Map and iterator-creation locks are unqualified.
func TestExecCollectionEnhancedForRejectsStructuralAddAPI67(t *testing.T) {
	cases := []struct {
		name      string
		source    string
		wantError bool
	}{
		{name: "ListActiveAdd", source: `List<Integer> values=new List<Integer>{1};for(Integer value:values){values.add(2);}`, wantError: true},
		{name: "SetActiveAdd", source: `Set<Integer> values=new Set<Integer>{1};for(Integer value:values){values.add(2);}`, wantError: true},
		{name: "ListPostAdd", source: `List<Integer> values=new List<Integer>{1};Integer total=0;for(Integer value:values){total+=value;}values.add(2);System.assertEquals(1,total);System.assertEquals(2,values.size());`, wantError: false},
		{name: "SetPostAdd", source: `Set<Integer> values=new Set<Integer>{1};Integer total=0;for(Integer value:values){total+=value;}values.add(2);System.assertEquals(1,total);System.assertEquals(2,values.size());`, wantError: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			_, err = New(nil).Execute(program)
			if (err != nil) != tc.wantError {
				t.Fatalf("execution error = %v, want error existence %v", err, tc.wantError)
			}
		})
	}
}

// Internal design invariants below verify identity and count lifecycle only.
// Their extra inputs do not add native Apex contract qualification.
func TestCollectionEnhancedForTraversalIdentityAndNestedCounts(t *testing.T) {
	for _, kind := range []ValueKind{ValueList, ValueSet} {
		t.Run(string(kind), func(t *testing.T) {
			machine := New(nil)
			makeCollection := func() Value {
				if kind == ValueSet {
					value := Set(Int(1))
					value.Type = "Set<Integer>"
					return value
				}
				value := List(Int(1))
				value.Type = "List<Integer>"
				return value
			}
			call := func(name string, value Value, method string, args []Value) (Value, bool, error) {
				if kind == ValueSet {
					return machine.callSetValueMember(name, value, method, args, &Result{})
				}
				return machine.callListValueMember(name, value, method, args, &Result{})
			}
			original := makeCollection()
			alias, err := machine.coerceAssignable(original.Type, original)
			if err != nil || alias.Ref != original.Ref {
				t.Fatalf("typed alias ref = %d, original = %d, error = %v", alias.Ref, original.Ref, err)
			}
			machine.Globals["alias"] = alias
			releaseOuter := machine.beginCollectionTraversal(original)
			outerReleased := false
			defer func() {
				if !outerReleased {
					releaseOuter()
				}
			}()
			if _, handled, err := call("alias", alias, "add", []Value{Int(2)}); !handled || err == nil {
				t.Fatalf("alias structural add: handled=%v error=%v", handled, err)
			}
			if kind == ValueSet {
				value, handled, err := call("alias", alias, "add", []Value{Int(1)})
				if !handled || err != nil || value.Kind != ValueBool || value.Bool {
					t.Fatalf("duplicate Set add = %#v handled=%v error=%v", value, handled, err)
				}
			}
			independent := makeCollection()
			cloned, handled, err := call("alias", alias, "clone", nil)
			if !handled || err != nil || cloned.Ref == original.Ref || cloned.Ref == 0 {
				t.Fatalf("clone identity = %d, original = %d, handled=%v error=%v", cloned.Ref, original.Ref, handled, err)
			}
			for name, value := range map[string]Value{"independent": independent, "cloned": cloned} {
				machine.Globals[name] = value
				if _, handled, err := call(name, value, "add", []Value{Int(2)}); !handled || err != nil {
					t.Fatalf("%s add: handled=%v error=%v", name, handled, err)
				}
			}
			releaseInner := machine.beginCollectionTraversal(alias)
			key := collectionTraversalKey{kind: kind, ref: original.Ref}
			if machine.activeCollectionTraversals[key] != 2 {
				t.Fatalf("nested count = %d, want 2", machine.activeCollectionTraversals[key])
			}
			releaseInner()
			if machine.activeCollectionTraversals[key] != 1 {
				t.Fatalf("outer count after inner release = %d, want 1", machine.activeCollectionTraversals[key])
			}
			if _, handled, err := call("alias", alias, "add", []Value{Int(2)}); !handled || err == nil {
				t.Fatalf("inner release unlocked outer: handled=%v error=%v", handled, err)
			}
			if err := machine.CloneRuntime(nil).rejectActiveCollectionAdd(original); err != nil {
				t.Fatalf("runtime clone inherited active traversal: %v", err)
			}
			releaseOuter()
			outerReleased = true
			if len(machine.activeCollectionTraversals) != 0 {
				t.Fatal("active traversal survived final release")
			}
			if _, handled, err := call("alias", alias, "add", []Value{Int(2)}); !handled || err != nil {
				t.Fatalf("released alias add: handled=%v error=%v", handled, err)
			}
		})
	}
}

func TestCollectionEnhancedForTraversalCleanupOnControlExit(t *testing.T) {
	for _, collectionType := range []string{"List", "Set"} {
		for _, exit := range []struct {
			name      string
			body      string
			wantError bool
		}{
			{name: "break", body: "break;"},
			{name: "return", body: "return;"},
			{name: "throw", body: "throw new DmlException('cleanup');", wantError: true},
			{name: "nestedInnerBreak", body: "for (Integer nestedValue : values) { break; } values.add(2);", wantError: true},
		} {
			t.Run(collectionType+"/"+exit.name, func(t *testing.T) {
				program, err := CompileAnonymous(collectionType + "<Integer> values = new " + collectionType + "<Integer>{1}; for (Integer item : values) {" + exit.body + "}")
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				_, err = machine.Execute(program)
				if (err != nil) != exit.wantError {
					t.Fatalf("control exit error = %v, want existence %v", err, exit.wantError)
				}
				if len(machine.activeCollectionTraversals) != 0 {
					t.Fatalf("%s left active traversals: %#v", exit.name, machine.activeCollectionTraversals)
				}
				values, ok := machine.Globals["values"]
				if !ok || values.Ref == 0 {
					t.Fatal("cleanup fixture did not create a collection identity")
				}
				if err := machine.rejectActiveCollectionAdd(values); err != nil {
					t.Fatalf("%s cleanup still rejects the collection: %v", exit.name, err)
				}
			})
		}
	}
}
