package vm

import "testing"

func TestFinalExceptionCatchabilityRespectsSourceIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, declared, thrown string
		platform, catchable    bool
	}{
		{"platformShort", "FinalException", "FinalException", true, false},
		{"platformQualified", "System.FinalException", "System.FinalException", true, false},
		{"source", "FinalException", "FinalException", false, true},
		{"qualifiedPlatformWithSourceShadow", "FinalException", "System.FinalException", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			if err := machine.RegisterClass(Class{Name: tc.declared, SuperClass: "Exception", PlatformType: tc.platform}); err != nil {
				t.Fatal(err)
			}
			for _, catchType := range []string{"Exception", tc.thrown} {
				if got := machine.exceptionMatches(catchType, Object(tc.thrown)); got != tc.catchable {
					t.Fatalf("catch %s matches %s = %v; want %v", catchType, tc.thrown, got, tc.catchable)
				}
			}
		})
	}
}
