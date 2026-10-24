package vm

import "testing"

// The full declarations and the three execution routes are replayed by the
// system_type_construction native conformance test at source APIs 62 and 67.
func TestSystemTypeConstructionDirectVM(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, expression := range []string{"System.UserInfo.class", "UserInfo.class", "Type.forName('System','UserInfo')", "Type.forName('UserInfo')"} {
			t.Run(api+"/anonymous/"+expression, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions("System.assertNotEquals(null, "+expression+".newInstance());", CompileOptions{APIVersion: api})
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
