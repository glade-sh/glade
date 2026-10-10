package vm

import "testing"

// The retained API67 Decimal catalog is SHA256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/2197/members/8 through /documents/2197/members/26.
// Eight selected method cases from frozen sourceabed22a0. These are
// finite family regressions, not whole-method or API62-67 parity claims.
// Existing VM regressions and owned fixtures supply the preservation cases.
// The whole-number stripTrailingZeros value/scale/precision tuple is excluded
// pending Root's API67 observation. The pow upper-bound case asserts rejection
// only: the retained source limits the exponent to 0..32767, but does not pin
// the exception type or message. Root must bind and admit this proposed suite.
func TestExecDecimalFamilyAPI67(t *testing.T) {
	cases := []struct {
		name          string
		source        string
		wantRejection bool
	}{
		{
			name: "member08_abs",
			source: `
Decimal value = Decimal.valueOf('-0.20');
System.assertEquals(0.20, value.abs());
System.assertEquals('0.20', value.abs().toPlainString());
System.assertEquals(2, value.abs().scale());
System.assertEquals('-0.20', value.toPlainString());
`,
		},
		{
			name: "member09_divide_Decimal_Integer",
			source: `
Decimal value = Decimal.valueOf('19');
Decimal quotient = value.divide(Decimal.valueOf('100'), 3);
System.assertEquals(0.190, quotient);
System.assertEquals('0.190', quotient.toPlainString());
System.assertEquals(3, quotient.scale());
System.assertEquals('2.50', Decimal.valueOf('10').divide(4, 2).toPlainString());
`,
		},
		{
			name: "member11_doubleValue",
			source: `
Decimal value = Decimal.valueOf('9007199254740993');
Double converted = value.doubleValue();
System.assertEquals(9007199254740992L, converted);
System.assertEquals(9007199254740992L, converted + 1);
System.assertEquals('9007199254740993', value.toPlainString());
`,
		},
		{
			name: "member13_intValue",
			source: `
Integer positive = Decimal.valueOf('12.5').intValue();
Integer negative = Decimal.valueOf('-12.5').intValue();
System.assertEquals(12, positive);
System.assertEquals(-12, negative);
System.assertEquals(2147483647, Decimal.valueOf('2147483647.999999999').intValue());
`,
		},
		{
			name: "member14_longValue",
			source: `
Long whole = Decimal.valueOf('9007199254740993').longValue();
System.assertEquals(9007199254740993L, whole);
System.assert(!(whole instanceof Integer));
System.assertEquals(-12L, Decimal.valueOf('-12.5').longValue());
`,
		},
		{
			name: "member15_pow_Integer",
			source: `
Decimal value = Decimal.valueOf('4.12');
System.assertEquals('16.9744', value.pow(2).toPlainString());
System.assertEquals(1, value.pow(0));
System.assertEquals(1, Decimal.valueOf('1').pow(32767));
System.assertEquals('9007199254740993', Decimal.valueOf('9007199254740993').pow(1).toPlainString());
`,
		},
		{
			name: "member17_round",
			source: `
Long positive = Decimal.valueOf('2.5').round();
Long negative = Decimal.valueOf('-2.5').round();
System.assertEquals(2L, positive);
System.assertEquals(-2L, negative);
System.assertEquals(14L, Decimal.valueOf('13.5').round());
System.assert(!(positive instanceof Integer));
`,
		},
		{
			name: "member18_round_RoundingMode",
			source: `
Decimal positive = Decimal.valueOf('12.5');
Decimal negative = Decimal.valueOf('-12.5');
System.assertEquals(13L, positive.round(RoundingMode.UP));
System.assertEquals(12L, positive.round(RoundingMode.DOWN));
System.assertEquals(13L, positive.round(RoundingMode.CEILING));
System.assertEquals(-12L, negative.round(RoundingMode.CEILING));
System.assertEquals(-13L, negative.round(RoundingMode.FLOOR));
System.assertEquals(13L, positive.round(RoundingMode.HALF_UP));
System.assertEquals(12L, positive.round(RoundingMode.HALF_DOWN));
System.assertEquals(12L, positive.round(RoundingMode.HALF_EVEN));
System.assertEquals(12L, Decimal.valueOf('12.0').round(RoundingMode.UNNECESSARY));
Long rounded = positive.round(RoundingMode.HALF_UP);
System.assert(!(rounded instanceof Integer));
`,
		},
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
			if tc.wantRejection {
				if err == nil {
					t.Fatal("Decimal.pow accepted exponent 32768 outside the documented 0..32767 range")
				}
				t.Logf("unclassified rejection; exception type/message not asserted: %v", err)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
