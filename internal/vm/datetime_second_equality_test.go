package vm

import (
	"fmt"
	"testing"
)

// Datetime member/assertion equality compares the observed second while
// operator equality and epoch/JSON projections retain their millisecond value.
func TestDatetimeSameSecondMemberAndAssertionEquality(t *testing.T) {
	for _, api := range []string{"62.0", "65.0", "67.0", "68.0"} {
		for _, pair := range []struct {
			name        string
			left, right int64
		}{
			{"wholeToFraction", 0, 73},
			{"fractionToWhole", 999, 0},
			{"differentFractions", 73, 999},
		} {
			for _, route := range []struct{ name, source string }{
				{"member", "System.assert(left.equals(right)); System.assert(right.equals(left));"},
				{"systemAssertion", "System.assertEquals(left,right); System.assertEquals(right,left);"},
				{"modernAssertion", "Assert.areEqual(left,right); Assert.areEqual(right,left);"},
			} {
				t.Run(api+"/"+pair.name+"/"+route.name, func(t *testing.T) {
					source := fmt.Sprintf(`
Long base=1709210096000L;
Datetime left=Datetime.newInstance(base+%dL);
Datetime right=Datetime.newInstance(base+%dL);
%s
`, pair.left, pair.right, route.source)
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
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
}

func TestDatetimeSecondEqualityPreservesMillisecondControls(t *testing.T) {
	for _, api := range []string{"62.0", "65.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Long base=1709210096000L;
Datetime whole=Datetime.newInstance(base);
Datetime first=Datetime.newInstance(base+73L);
Datetime last=Datetime.newInstance(base+999L);
Datetime nextSecond=Datetime.newInstance(base+1000L);
System.assert(whole != first);
System.assert(whole != last);
System.assert(first != last);
System.assertEquals(base,whole.getTime());
System.assertEquals(base+73L,first.getTime());
System.assertEquals(base+999L,last.getTime());
System.assertEquals(926L,last.getTime()-first.getTime());
System.assertEquals('"2024-02-29T12:34:56.000Z"',JSON.serialize(whole));
System.assertEquals('"2024-02-29T12:34:56.073Z"',JSON.serialize(first));
System.assertEquals('"2024-02-29T12:34:56.999Z"',JSON.serialize(last));
System.assert(!whole.equals(nextSecond));
System.assert(!last.equals(nextSecond));
System.assert(!nextSecond.equals(last));
System.assertNotEquals(whole,nextSecond);
System.assertNotEquals(last,nextSecond);
Assert.areNotEqual(whole,nextSecond);
Assert.areNotEqual(last,nextSecond);
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := New(nil).Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Preserve class values outside the observed platform Datetime contract.
func TestDatetimeMethodEqualityKeepsClassProvenance(t *testing.T) {
	for _, flag := range []string{"projectClass", "classInstance"} {
		for _, side := range []string{"left", "right"} {
			t.Run(flag+"/"+side, func(t *testing.T) {
				left := platformScalar("Datetime", "2024-02-29T12:34:56.073Z")
				right := platformScalar("Datetime", "2024-02-29T12:34:56.999Z")
				value := &left
				if side == "right" {
					value = &right
				}
				if flag == "projectClass" {
					value.projectClass = true
				} else {
					value.classInstance = true
				}
				equal, handled, err := callObjectMember(left, "equals", []Value{right})
				if err != nil || !handled || equal.Bool {
					t.Fatalf("class member equality changed: %v %v %v", equal, handled, err)
				}
				same, err := New(nil).assertionEquals(left, right, &Result{})
				if err != nil || same {
					t.Fatalf("class assertion equality changed: %v %v", same, err)
				}
			})
		}
	}
}
