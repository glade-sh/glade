package vm

import (
	"fmt"
	"testing"
)

// The owned utility snapshots observe these negative-epoch pairs at API
// 62/67/68 in anonymous and @IsTest contexts. Operators retain milliseconds.
func TestDatetimeNegativeEpochMethodEquality(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, pair := range []struct {
			left, right int64
			equal       bool
		}{{-1, 0, true}, {-999, 999, true}, {-1, -999, true}, {-1000, 0, false}} {
			t.Run(fmt.Sprintf("%s/%d/%d", api, pair.left, pair.right), func(t *testing.T) {
				assertion := "System.assertEquals(left,right); Assert.areEqual(left,right);"
				if !pair.equal {
					assertion = "System.assertNotEquals(left,right); Assert.areNotEqual(left,right);"
				}
				program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Datetime left=Datetime.newInstance(%dL), right=Datetime.newInstance(%dL);
System.assertEquals(%t,left.equals(right));
System.assertEquals(%t,right.equals(left));
System.assert(left!=right);
System.assertEquals(%dL,left.getTime()); System.assertEquals(%dL,right.getTime());
%s`, pair.left, pair.right, pair.equal, pair.equal, pair.left, pair.right, assertion), CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := New(nil).Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestDatetimeMethodEqualityKeepsSourceClassShadow(t *testing.T) {
	// Source-provenance values must keep their existing object comparison;
	// this control makes no claim about constructing a shadowed system type.
	for _, provenance := range []string{"project", "instance"} {
		t.Run(provenance, func(t *testing.T) {
			left, right := Object("Datetime"), Object("Datetime")
			left.Fields["value"] = String("1970-01-01T00:00:00.000Z")
			right.Fields["value"] = String("1970-01-01T00:00:00.073Z")
			left.projectClass, right.projectClass = provenance == "project", provenance == "project"
			left.classInstance, right.classInstance = provenance == "instance", provenance == "instance"
			if equal, err := New(nil).assertionEquals(left, right, &Result{}); err != nil || equal {
				t.Fatalf("source object assertion = %t, %v", equal, err)
			}
			if equal, handled, err := callObjectMember(left, "equals", []Value{right}); err != nil || !handled || equal.Bool {
				t.Fatalf("source object member comparison = %v, %t, %v", equal, handled, err)
			}
		})
	}
}
