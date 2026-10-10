package vm

import "testing"

// API67 retained TimeZone source 90fae6deff59678e4f47adab0ebc65dddfeadc5c8ae831b00df62a71971b2ca1
// admits Java-supported IDs. These are the exact factory/getID bodies from
// timezone-phoenix-current-red.json: Phoenix and the documented Los Angeles
// control. Two cases/two assertions qualify identity only; display names,
// offsets, historical rules, UserInfo, native parity and API intervals remain open.
func TestExecTimeZoneFactoryIdentityAPI67(t *testing.T) {
	cases := []struct {
		name, source string
	}{
		{"Phoenix", `System.assertEquals('America/Phoenix',TimeZone.getTimeZone('America/Phoenix').getID());`},
		{"LAControl", `System.assertEquals('America/Los_Angeles',TimeZone.getTimeZone('America/Los_Angeles').getID());`},
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
			machine := New(nil)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
