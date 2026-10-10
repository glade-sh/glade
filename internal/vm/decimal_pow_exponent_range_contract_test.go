package vm

import "testing"

// API67 Decimal.pow(Integer) limits the exponent to 0..32767.
// Retained catalog SHA256 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/2197/members/15; full guide SHA256
// c3f566d5eb968c85737eb14be010ce59e529599ee4153916963bd7addc890e66,
// lines 255-280. The guide does not specify the rejection type or message.
func TestExecDecimalPowExponentRangeAPI67(t *testing.T) {
	cases := []struct {
		name          string
		source        string
		wantRejection bool
	}{
		{
			name:          "aboveMaximumPositiveUnitBase",
			source:        "Decimal base = Decimal.valueOf('1'); base.pow(32768);",
			wantRejection: true,
		},
		{
			name:          "aboveMaximumNegativeUnitBase",
			source:        "Decimal base = Decimal.valueOf('-1'); base.pow(32768);",
			wantRejection: true,
		},
		{
			name:   "zeroExponent",
			source: "System.assertEquals(1, Decimal.valueOf('2').pow(0));",
		},
		{
			name:   "maximumExponent",
			source: "System.assertEquals(1, Decimal.valueOf('1').pow(32767));",
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
					t.Fatal("Decimal.pow accepted exponent 32768 outside the documented range")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
