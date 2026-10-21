package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// H01/H05 local API62/67/68 observations hide private top-level tests through
// all three overloads; H02/H04 keep public ordinary and test classes visible.
func TestTypeForNamePrivateTestEmptyNamespace(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			class Class
			want  string
		}{
			{Class{Name: "OwnedHiddenTest", Access: "private", IsTest: true}, ""},
			{Class{Name: "OwnedPublicTest", Access: "public", IsTest: true}, "OwnedPublicTest"},
			{Class{Name: "OwnedPublicClass", Access: "public"}, "OwnedPublicClass"},
		} {
			for label, args := range map[string]string{
				"single": fmt.Sprintf("'%s'", tc.class.Name),
				"empty":  fmt.Sprintf("'','%s'", tc.class.Name),
				"null":   fmt.Sprintf("null,'%s'", tc.class.Name),
			} {
				t.Run(api+"/"+tc.class.Name+"/"+label, func(t *testing.T) {
					program, err := CompileAnonymousWithOptions("Type reflected = Type.forName("+args+");", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := storage.NewOrgState()
					org.APIVersion = api
					machine := New(nil)
					machine.SetOrg(&org)
					class := tc.class
					class.APIVersion = api
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					value := result.Vars["reflected"]
					if tc.want == "" {
						if value.Kind != ValueNull {
							t.Fatalf("private top-level test resolved: %#v", value)
						}
					} else if value.Kind != ValueObject || typeValueIdentityName(value) != tc.want {
						t.Fatalf("public control = %#v, want %s", value, tc.want)
					}
				})
			}
		}
	}
}
