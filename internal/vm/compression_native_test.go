package vm

import (
	"strings"
	"testing"
)

// This is implementation equivalence with the previously used Go comparator,
// not evidence of Salesforce Unicode filename behavior.
func TestCompressionZipFoldNameMatchesEqualFold(t *testing.T) {
	names := []string{"Case.txt", "case.txt", "CASE.TXT", "K.txt", "k.txt", "K.txt", "S.txt", "s.txt", "ſ.txt", "Σ.txt", "σ.txt", "ς.txt", "I.txt", "i.txt", "İ.txt", "ı.txt", "é.txt", "É.txt", "e\u0301.txt", "\xff", "\xfe", ""}
	for _, left := range names {
		for _, right := range names {
			got := compressionZipFoldName(left) == compressionZipFoldName(right)
			if want := strings.EqualFold(left, right); got != want {
				t.Errorf("%q/%q fold equality=%v want%v", left, right, got, want)
			}
		}
	}
}

func TestOrdinaryListMembershipShortCircuitAndErrors(t *testing.T) {
	machine := New(nil)
	for _, tc := range []struct{ name, body string }{{"MembershipMatch", "return true;"}, {"MembershipError", "System.assert(false,'owned-membership-error'); return false;"}} {
		program, err := CompileAnonymous(tc.body)
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterClass(Class{Name: tc.name, Methods: map[string]Method{"equals": {Name: tc.name + ".equals", ClassName: tc.name, ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}}, Program: program}}}); err != nil {
			t.Fatal(err)
		}
	}
	needle := Object("MembershipNeedle")
	for _, method := range []string{"contains", "indexOf"} {
		result := &Result{}
		got, handled, err := machine.callListValueMember("", List(Object("MembershipMatch"), Object("MembershipError")), method, []Value{needle}, result)
		if err != nil || !handled {
			t.Fatalf("%s did not short-circuit: %#v %v", method, got, err)
		}
		if method == "contains" && (got.Kind != ValueBool || !got.Bool) {
			t.Fatalf("contains=%#v", got)
		}
		if method == "indexOf" && (got.Kind != ValueInt || got.Int != 0) {
			t.Fatalf("indexOf=%#v", got)
		}
		_, handled, err = machine.callListValueMember("", List(Object("MembershipError")), method, []Value{needle}, result)
		if !handled || err == nil || !strings.Contains(err.Error(), "owned-membership-error") {
			t.Fatalf("%s lost equality error: %v", method, err)
		}
		_, handled, err = machine.callListValueMember("", List(), method, nil, result)
		if !handled || err == nil || err.Error() != "List."+method+" expects 1 argument" {
			t.Fatalf("%s argument error: %v", method, err)
		}
	}
}

func TestNativeListAppendMaterializesOnlyInsertedString(t *testing.T) {
	for _, nativeReceiver := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "native"}[nativeReceiver], func(t *testing.T) {
			machine := New(nil)
			needle := String("stored.bin")
			needle.nativeListElement = true
			source := typedList("List<String>")
			source.nativeListMembership = true
			source.List = []Value{needle}
			clone, handled, err := machine.callListValueMember("", source, "clone", nil, &Result{})
			if err != nil || !handled {
				t.Fatalf("clone: %v", err)
			}
			clone.nativeListMembership = nativeReceiver
			machine.Globals["names"] = clone
			_, handled, err = machine.callListValueMember("names", clone, "add", []Value{needle}, &Result{})
			if err != nil || !handled {
				t.Fatalf("add: %v", err)
			}
			stored := machine.Globals["names"]
			if len(stored.List) != 2 || stored.nativeListMembership != nativeReceiver || !stored.List[0].nativeListElement || stored.List[1].nativeListElement {
				t.Fatalf("append changed backing membership: %#v", stored)
			}
			if !needle.nativeListElement || len(source.List) != 1 || !source.List[0].nativeListElement || !source.nativeListMembership || len(clone.List) != 1 || !clone.List[0].nativeListElement {
				t.Fatal("append changed source argument or preexisting clone")
			}
			got, _, err := machine.callListValueMember("names", stored, "indexOf", []Value{needle}, &Result{})
			want := int64(0)
			if nativeReceiver {
				want = 1
			}
			if err != nil || got.Kind != ValueInt || got.Int != want {
				t.Fatalf("indexOf=%#v, want %d: %v", got, want, err)
			}
			_, _, err = machine.callListValueMember("names", stored, "add", []Value{Object("NotAString")}, &Result{})
			if err == nil || len(machine.Globals["names"].List) != 2 || !source.List[0].nativeListElement {
				t.Fatal("invalid append bypassed coercion or mutated source")
			}
		})
	}
}
