package storage

import "testing"

func TestOrgB2BFeatureCaseFold(t *testing.T) {
	org := NewOrgState()
	ApplyProjectOrgShape(&org, []string{"b2bcommerce", "commerceenabled"})
	for _, name := range []string{"B2BCommerce", "CommerceEnabled"} {
		if !org.HasFeature(name) {
			t.Errorf("%s feature should be case-insensitive", name)
		}
	}
}
