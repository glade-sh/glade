package storage

import "testing"

func TestOrgKnowledgeAvailabilityFeatureCaseFold(t *testing.T) {
	for _, feature := range []string{"knowledgeenabled", "kNoWlEdGeEnAbLeD"} {
		t.Run(feature, func(t *testing.T) {
			org := NewOrgState()
			ApplyProjectOrgShape(&org, []string{feature})
			for _, object := range []string{"Knowledge__ka", "Knowledge__kav"} {
				if !org.AllowsStandardObject(object) {
					t.Errorf("%s should be available for %s", object, feature)
				}
			}
		})
	}
}
