package vm

import "testing"

func TestExecDecimalStripTrailingZerosPreservesNegativeScaleAPI67(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Decimal whole = Decimal.valueOf('100.00').stripTrailingZeros();
System.assertEquals(100, whole);
System.assertEquals(-2, whole.scale());
System.assertEquals(1, whole.precision());
`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if program.APIVersion != "67.0" {
		t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
