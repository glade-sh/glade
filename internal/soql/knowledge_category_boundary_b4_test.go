package soql

import "testing"

// The native capture covers Knowledge__kav; keep the previous unsupported
// behavior for unrelated objects until an observation establishes otherwise.
func TestKnowledgeCategoryOtherObjectsStayUnsupported(t *testing.T) {
	for _, object := range []string{"Account", "Contact", "Case"} {
		if _, err := Parse("SELECT Id FROM " + object + " WITH DATA CATEGORY Guides__c AT General__c"); err == nil {
			t.Errorf("uncaptured category syntax accepted for %s", object)
		}
	}
}

// Incomplete clauses return errors; this preserves the behavior of the first
// category implementation and adds supplemental parser robustness coverage.
func TestKnowledgeCategoryIncompleteClauseB4(t *testing.T) {
	for _, query := range []string{
		"SELECT Id FROM Knowledge__kav WITH DATA CATEGORY Guides__c",
		"SELECT Id FROM Knowledge__kav WITH DATA CATEGORY Guides__c AT",
	} {
		if _, err := Parse(query); err == nil {
			t.Errorf("incomplete clause accepted: %s", query)
		}
	}
}
