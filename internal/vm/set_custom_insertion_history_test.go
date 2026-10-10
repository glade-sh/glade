package vm

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// The API67 custom-key guide and Set catalog 2215/3,8 warn that
// changing an inserted element's hash makes it unfindable. These controls
// exercise insertion history and its transport, not whole-Set/native parity.
func TestExecSetRetainsCustomInsertionHashAPI67(t *testing.T) {
	options := CompileOptions{APIVersion: "67.0"}
	compile := func(source string) ir.Program {
		t.Helper()
		program, err := CompileAnonymousWithOptions(source, options)
		if err != nil {
			t.Fatal(err)
		}
		return program
	}
	constructor := compile("this.x = initial;")
	equals := compile(`
MutableKey candidate = other instanceof MutableKey ? (MutableKey)other : null;
return candidate != null && x == candidate.x;
`)
	transport := compile("return items;")
	cases := []struct {
		name, hash, source string
	}{
		{"literalChangedAndUnchanged", "return x;", `
MutableKey key = new MutableKey(1);
Set<MutableKey> values = new Set<MutableKey>{key};
System.assert(values.contains(key));
key.x = 2;
System.assert(!values.contains(key));
System.assertEquals(1, values.size());
`},
		{"addAddAllAndNonemptyConstructor", "return x;", `
MutableKey first = new MutableKey(1);
MutableKey second = new MutableKey(2);
MutableKey third = new MutableKey(3);
Set<MutableKey> added = new Set<MutableKey>();
System.assert(added.add(first));
System.assert(added.addAll(new List<MutableKey>{second}));
Set<MutableKey> constructed = new Set<MutableKey>(new List<MutableKey>{third});
System.assert(added.contains(first));
System.assert(added.contains(second));
System.assert(constructed.contains(third));
first.x = 11; second.x = 12; third.x = 13;
System.assert(!added.contains(first));
System.assert(!added.contains(second));
System.assert(!constructed.contains(third));
`},
		{"collisionsRetainEquality", "return 7;", `
Set<MutableKey> values = new Set<MutableKey>{new MutableKey(1), new MutableKey(2)};
System.assertEquals(2, values.size());
System.assert(values.contains(new MutableKey(1)));
System.assert(values.contains(new MutableKey(2)));
System.assert(!values.contains(new MutableKey(3)));
System.assert(!values.add(new MutableKey(1)));
MutableKey subclassEqual = new DerivedKey(1);
System.assert(values.contains(subclassEqual));
System.assert(!values.add(subclassEqual));
System.assertEquals(2, values.size());
Set<MutableKey> subclassFirst = new Set<MutableKey>{new DerivedKey(1)};
System.assert(subclassFirst.contains(new MutableKey(1)));
System.assert(!subclassFirst.add(new MutableKey(1)));
`},
		{"sameObjectIndependentInsertionTimes", "return x;", `
MutableKey key = new MutableKey(1);
Set<MutableKey> oldValues = new Set<MutableKey>{key};
key.x = 2;
Set<MutableKey> newValues = new Set<MutableKey>{key};
System.assert(!oldValues.contains(key));
System.assert(newValues.contains(key));
`},
		{"typedAssignmentAndMethodReturn", "return x;", `
MutableKey key = new MutableKey(1);
Set<MutableKey> original = new Set<MutableKey>{key};
Object boxed = original;
Set<MutableKey> assigned = (Set<MutableKey>)boxed;
Set<MutableKey> returned = key.pass(assigned);
System.assert(returned.contains(key));
key.x = 2;
System.assert(!original.contains(key));
System.assert(!assigned.contains(key));
System.assert(!returned.contains(key));
`},
		{"removeAndFilterPreserveSurvivorHistory", "return x;", `
MutableKey first = new MutableKey(1);
MutableKey second = new MutableKey(2);
MutableKey third = new MutableKey(3);
Set<MutableKey> removed = new Set<MutableKey>{first, second, third};
Set<MutableKey> filtered = removed.clone();
Set<MutableKey> retained = removed.clone();
System.assert(removed.remove(first));
System.assert(filtered.removeAll(new List<MutableKey>{first, third}));
System.assert(retained.retainAll(new Set<MutableKey>{second}));
System.assert(removed.contains(second));
System.assert(filtered.contains(second));
System.assert(retained.contains(second));
second.x = 20;
System.assert(!removed.contains(second));
System.assert(!filtered.contains(second));
System.assert(!retained.contains(second));
System.assert(removed.contains(third));
`},
		{"clearReaddAndCloneIndependence", "return x;", `
MutableKey key = new MutableKey(1);
Set<MutableKey> original = new Set<MutableKey>{key};
Set<MutableKey> alias = original;
Set<MutableKey> copy = original.clone();
key.x = 2;
original.clear();
System.assert(original.add(key));
System.assert(alias.contains(key));
System.assert(!copy.contains(key));
System.assert(copy.add(new MutableKey(3)));
System.assertEquals(2, copy.size());
System.assertEquals(1, original.size());
System.assert(!original.contains(new MutableKey(3)));
`},
		{"checkedCallbackErrorsLeaveCoherentHistory", "if (x < 0) { throw new IllegalArgumentException('owned hash failure'); } return x;", `
MutableKey key = new MutableKey(1);
Set<MutableKey> values = new Set<MutableKey>{key};
Boolean rejectedAdd = false;
try { values.add(new MutableKey(-1)); } catch (Exception error) { rejectedAdd = true; }
System.assert(rejectedAdd);
System.assertEquals(1, values.size());
System.assert(values.contains(key));
Boolean rejectedAddAll = false;
try { values.addAll(new List<MutableKey>{new MutableKey(-2)}); } catch (Exception error) { rejectedAddAll = true; }
System.assert(rejectedAddAll);
System.assert(values.contains(key));
key.x = 2;
System.assert(!values.contains(key));
`},
		{"ordinaryListMembershipUnchanged", "return x;", `
MutableKey key = new MutableKey(1);
List<MutableKey> values = new List<MutableKey>{key};
key.x = 2;
System.assert(values.contains(key));
System.assertEquals(0, values.indexOf(key));
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			if err := machine.RegisterClass(Class{
				Name:   "MutableKey",
				Fields: map[string]Field{"x": {Name: "x", Type: "Integer"}},
				Constructors: []Method{{
					Name: "MutableKey.<init>", ClassName: "MutableKey",
					Params: []Param{{Name: "initial", Type: "Integer"}}, Program: constructor,
				}},
				Methods: map[string]Method{
					"hashCode": {Name: "MutableKey.hashCode", ClassName: "MutableKey", ReturnType: "Integer", Program: compile(tc.hash)},
					"equals":   {Name: "MutableKey.equals", ClassName: "MutableKey", ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}}, Program: equals},
					"pass":     {Name: "MutableKey.pass", ClassName: "MutableKey", ReturnType: "Set<MutableKey>", Params: []Param{{Name: "items", Type: "Set<MutableKey>"}}, Program: transport},
				},
			}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterClass(Class{
				Name: "DerivedKey", SuperClass: "MutableKey",
				Constructors: []Method{{
					Name: "DerivedKey.<init>", ClassName: "DerivedKey",
					Params: []Param{{Name: "initial", Type: "Integer"}}, Program: compile("super(initial);"),
				}},
			}); err != nil {
				t.Fatal(err)
			}
			program := compile(tc.source)
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Internal transports must preserve paired tags without leaking them into
// public serialization or public Set equality.
func TestSetInsertionHistoryInternalTransports(t *testing.T) {
	key := Object("MutableKey")
	key.Fields["x"] = Int(1)
	original := appendSetEntry(Set(), key, setInsertionHash{key: "owned:1", tracked: true})
	for _, copied := range []Value{cloneValue(original), cloneValuePreserveRefs(original), cloneValueDetachedPreserveRefs(original)} {
		if !sameSetInsertionHistory(original, copied) {
			t.Fatal("clone lost insertion history")
		}
		copied.setInsertionHashes[0] = setInsertionHash{key: "owned:2", tracked: true}
		if original.setInsertionHashAt(0).key != "owned:1" {
			t.Fatal("clone shares insertion history storage")
		}
	}
	changed := original
	changed.setInsertionHashes = []setInsertionHash{{key: "owned:2", tracked: true}}
	if !original.Equal(changed) {
		t.Fatal("internal history changed public Set equality")
	}
	if sameAliasValue(original, changed) || sameAliasRuntimeData(original, changed) || sameAliasRuntimeBacking(original, changed) || sameMethodReturnAliasBatchBacking(original, changed) {
		t.Fatal("internal comparison ignored changed history")
	}
	backed := methodReturnAliasBatchValueWithBacking(changed, original)
	if !sameSetInsertionHistory(backed, original) {
		t.Fatal("backing transfer lost history")
	}
	truncated := cloneValueWithSeen(original, map[uint64]bool{original.Ref: true})
	if len(truncated.Set) != 0 || len(truncated.setInsertionHashes) != 0 {
		t.Fatal("cycle truncation retained orphaned history")
	}
	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "owned:1") || strings.Contains(string(raw), "Insertion") {
		t.Fatal("internal history leaked into serialization")
	}
}
