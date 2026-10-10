package vm

import "testing"

func TestExecDecimalSetScaleNegativeScaleAPI67(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Decimal rounded = Decimal.valueOf('105').setScale(-1);
System.assertEquals(100, rounded);
System.assertEquals(-1, rounded.scale());

Decimal exact = Decimal.valueOf('100').setScale(-1, RoundingMode.UNNECESSARY);
System.assertEquals(100, exact);
System.assertEquals(-1, exact.scale());

Decimal zeroScale = Decimal.valueOf('105').setScale(0);
System.assertEquals(105, zeroScale);
System.assertEquals(0, zeroScale.scale());

Decimal positiveScale = Decimal.valueOf('12.345').setScale(2);
System.assertEquals(12.34, positiveScale);
System.assertEquals(2, positiveScale.scale());
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
