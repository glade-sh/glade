package vm

import "testing"

// A class that extends a platform class (Auth.AuthProviderPluginClass) does not
// bring the platform namespace into scope. Native Apex at API 62.0 and 67.0
// resolves an unqualified JWT to the user class (captured in a scratch org).
func TestPlatformSuperclassNamespaceDoesNotShadowUserClass(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			for _, class := range []Class{
				{Name: "JWT"},
				{Name: "Auth.JWT"},
				{Name: "ProvAuth", SuperClass: "Auth.AuthProviderPluginClass"},
			} {
				class.APIVersion = api
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			if got := machine.resolveTypeNameInClass("ProvAuth", "JWT"); got != "JWT" {
				t.Fatalf("unqualified JWT in a class extending a platform class resolved to %q, want JWT", got)
			}
		})
	}
}
