package vm

import (
	"testing"
	"time"
)

// RFC6238 Appendix B SHA1 values, reduced to the documented six-digit token.
// The expected constants are independent of the implementation under test.
func TestAuthTotpRFCVectors(t *testing.T) {
	for _, test := range []struct {
		seconds int64
		code    string
	}{{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"}, {1234567890, "005924"}, {2000000000, "279037"}, {20000000000, "353130"}} {
		actual, err := authTotpCode([]byte("12345678901234567890"), test.seconds)
		if err != nil || actual != test.code {
			t.Errorf("seconds=%d code=%q err=%v", test.seconds, actual, err)
		}
	}
}

// Remain inside the known vector step after Datetime.now advances one second.
// This checks explicit elapsed time without assuming adjacent-window tolerance.
func TestAuthTotpClockAdvanceRejectsPriorDayCode(t *testing.T) {
	machine := New(nil)
	machine.fakeNow = time.Unix(58, 0).UTC()
	first, err := CompileAnonymousWithOptions(`Datetime now=Datetime.now(); System.assertEquals(true,Auth.SessionManagement.validateTotpTokenForKey('GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ','287082'));`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(first); err != nil {
		t.Fatalf("RFC30–59 step positive: %v", err)
	}
	machine.AdvanceDeterministicTime(24 * time.Hour)
	second, err := CompileAnonymousWithOptions(`System.assertEquals(false,Auth.SessionManagement.validateTotpTokenForKey('GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ','287082'));`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(second); err != nil {
		t.Fatalf("prior-day token after public clock advance: %v", err)
	}
}

// The RFC code for counter1 is fixed independently of authTotpCode. The
// Salesforce-admitted window accepts it only at counters0,1,2.
func TestAuthTotpAdjacentStepWindow(t *testing.T) {
	for _, tc := range []struct {
		seconds  int64
		accepted bool
	}{
		{0, true}, {29, true}, {30, true}, {59, true}, {60, true}, {89, true}, {90, false}, {120, false},
	} {
		machine := New(nil)
		machine.fakeNow = time.Unix(tc.seconds, 0).UTC()
		got, err := machine.validateAuthTotpForKey([]Value{String("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"), String("287082")})
		if err != nil || got.Kind != ValueBool || got.Bool != tc.accepted {
			t.Errorf("seconds=%d result=%#v err=%v want=%v", tc.seconds, got, err, tc.accepted)
		}
	}
}

// Runtime clones are isolated local execution contexts. This establishes local
// state ownership, not Salesforce same-user cross-transaction reset behavior.
func TestAuthTotpRuntimeCloneIsolation(t *testing.T) {
	base := New(nil)
	base.fakeNow = time.Unix(59, 0).UTC()
	invalid := []Value{String("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"), String("")}
	valid := []Value{invalid[0], String("287082")}
	for i := 0; i < 10; i++ {
		got, err := base.validateAuthTotpForKey(invalid)
		if err != nil || got.Kind != ValueBool || got.Bool {
			t.Fatalf("failure%d: %#v %v", i, got, err)
		}
	}
	if _, err := base.validateAuthTotpForKey(valid); err == nil {
		t.Fatal("base lockout missing")
	}
	first := base.CloneRuntime(nil)
	base.FreezeClassLookup()
	if base.frozenClassLookup == nil {
		t.Fatal("base did not freeze class lookup")
	}
	second := base.CloneRuntimeFrozenShared(nil)
	if !second.sharedStaticClasses || second.frozenClassLookup != base.frozenClassLookup {
		t.Fatal("frozen clone did not use the shared template path")
	}
	for i, machine := range []*VM{first, second} {
		machine.fakeNow = time.Unix(59, 0).UTC()
		got, err := machine.validateAuthTotpForKey(valid)
		if err != nil || got.Kind != ValueBool || !got.Bool {
			t.Fatalf("clone%d inherited quota: %#v %v", i, got, err)
		}
		for attempt := 0; attempt < 10; attempt++ {
			got, err = machine.validateAuthTotpForKey(invalid)
			if err != nil || got.Kind != ValueBool || got.Bool {
				t.Fatalf("clone%d failure%d: %#v %v", i, attempt, got, err)
			}
		}
		if _, err := machine.validateAuthTotpForKey(valid); err == nil {
			t.Fatalf("clone%d lockout missing", i)
		}
	}
	if _, err := base.validateAuthTotpForKey(valid); err == nil {
		t.Fatal("clone validation reset base lockout")
	}
}
