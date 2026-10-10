package vm

import "testing"

// Patch3 control followed by omitted-patch null focal at API67.
// Retained Version primary7118B/d22186dc591a0af5175144cb687794429cf942470421cd7f048dcbf5ad2798b5.
// Patch3: documented2.1.3 at15 joined to supplied constructor95-96/getter189.
// Patchnull: two-part constructor76-77 joined to getter189; return type193.
// Explicit-zero construction/range/native validity stays unqualified/STOP;
// it is excluded from these two literal programs under the revised lease.
// No comparison/string/range/invalid-input/native/API-interval semantics.
func TestExecVersionOmittedPatchNullAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{name: "explicitPatchThreeTypedControl", source: `Version value=new Version(2,1,3);Integer patchValue=value.patch();System.assertEquals(3,patchValue);value=null;`},
		{name: "omittedPatchNullFocal", source: `Version value=new Version(2,1);Integer patchValue=value.patch();System.assertEquals(null,patchValue);value=null;`},
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
			org := testDataOrg()
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
