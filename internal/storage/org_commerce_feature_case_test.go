package storage

import "testing"

func TestOrgCommerceFeatureCaseFold(t *testing.T) {
	for _, feature := range []string{"ordermanagement", "oRdErMaNaGeMeNt"} {
		org := NewOrgState()
		ApplyProjectOrgShape(&org, []string{feature})
		for _, object := range []string{"WebStore", "BuyerGroup", "CartItem"} {
			if !org.AllowsStandardObject(object) {
				t.Errorf("%s unavailable for %s", object, feature)
			}
		}
	}
}
