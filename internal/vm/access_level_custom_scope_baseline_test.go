package vm

import "testing"

// This is a baseline-preservation control for the review-requested revert.
// Feature-enabled custom-scope native equality remains uncaptured.
func TestAccessLevelCustomScopeEqualityPreservesBaseDirectVMControl(t *testing.T) {
	first := accessLevelValue("USER_MODE")
	first.Fields["permissionSetId"] = String("0PS000000000001")
	second := accessLevelClone(first)
	second.Fields["permissionSetId"] = String("0PS000000000002")
	machine := New(nil)
	for _, pair := range []struct {
		name        string
		left, right Value
	}{
		{"differentScopes", first, second},
		{"customAndConstant", first, accessLevelValue("USER_MODE")},
		{"constantAndCustom", accessLevelValue("USER_MODE"), second},
	} {
		t.Run(pair.name, func(t *testing.T) {
			for name, equal := range map[string]bool{
				"valueEqual":    pair.left.Equal(pair.right),
				"identityEqual": valueIdentityEqual(pair.left, pair.right),
				"enumEqual":     enumValuesEqual(pair.left, pair.right),
			} {
				if !equal {
					t.Errorf("%s changed the pre-existing custom-scope comparison", name)
				}
			}
			equal, handled := machine.resolvedEnumValuesEqual(pair.left, pair.right)
			if !handled || !equal {
				t.Errorf("resolved enum comparison = (%t,%t); base=(true,true)", equal, handled)
			}
		})
	}
}
