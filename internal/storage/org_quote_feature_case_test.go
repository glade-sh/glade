package storage

import "testing"

func TestOrgQuoteFeatureCaseFold(t *testing.T) {
	for _, feature := range []string{"quotes", "qUoTeS"} {
		org := NewOrgState()
		ApplyProjectOrgShape(&org, []string{feature})
		for _, object := range []string{"Quote", "QuoteLineItem"} {
			if !org.AllowsStandardObject(object) {
				t.Errorf("%s unavailable for %s", object, feature)
			}
		}
	}
}
