package vm

import "testing"

// API67 Math.E and Math.PI are declared Double: catalog SHA256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/2208/members/0 and /documents/2208/members/1; Math guide SHA256
// 66bd4750e7523cfffcdbf0200f472c561a80df14b21fb6e1b17cd3d8ab49f82a.
// The complete Double guide (40a6f94ac1fc4d3a7187bce569379a8565e07d53e7d7b9cbd976c6271bf17ed1)
// has no pow member; Decimal guide c3f566d5eb968c85737eb14be010ce59e529599ee4153916963bd7addc890e66
// declares pow. The admitted inference requires rejection of Decimal-only pow
// on these Double constants, without fixing a rejection phase, class, message,
// catchability, or floating-point precision rule.
func TestExecMathConstantsPreserveDoubleApplicabilityAPI67(t *testing.T) {
	cases := []struct {
		name          string
		source        string
		wantRejection bool
	}{
		{
			name:          "PIRejectsDecimalPow",
			source:        "Math.PI.pow(0);",
			wantRejection: true,
		},
		{
			name:          "ERejectsDecimalPow",
			source:        "Math.E.pow(0);",
			wantRejection: true,
		},
		{
			name:          "SystemQualifiedPIRejectsDecimalPow",
			source:        "System.Math.PI.pow(0);",
			wantRejection: true,
		},
		{
			name:          "CaseInsensitiveERejectsDecimalPow",
			source:        "system.math.e.pow(0);",
			wantRejection: true,
		},
		{
			name:          "DoubleValueOfRejectControl",
			source:        "Double.valueOf('1.25').pow(0);",
			wantRejection: true,
		},
		{
			name:   "DecimalPowRemainsApplicableControl",
			source: "System.assertEquals(1, Decimal.valueOf('1.25').pow(0));",
		},
		{
			name: "DeclaredDoubleMethodsRemainApplicableControl",
			source: `
Math.PI.intValue();
Math.E.longValue();
System.Math.PI.longValue();
system.math.e.intValue();
System.assertEquals(1, Double.valueOf('1.25').intValue());
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				if tc.wantRejection {
					return // The contract admits compile-time or execution rejection.
				}
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			_, err = New(nil).Execute(program)
			if tc.wantRejection {
				if err == nil {
					t.Fatalf("Double receiver accepted Decimal-only pow: %s", tc.source)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
