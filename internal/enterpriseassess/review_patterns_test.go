package enterpriseassess

import "testing"

func TestReviewPatternsDetectEmptyCatchDuplicateMockIDDebugSOQLAndAPIVersionDrift(t *testing.T) {
	files := []SourceFile{
		{
			Path: "force-app/main/default/classes/ProviderSpecialtySelector.cls",
			Text: "public class ProviderSpecialtySelector { ProviderSpecialtySelector(){ try { init(); } catch (Exception e) { /* no-op */ } } }",
		},
		{
			Path: "force-app/main/default/classes/HospitalAffiliationsSelector.cls",
			Text: "public class HospitalAffiliationsSelector { void a(){ mockId('same.id'); } void b(){ mockId('same.id'); } }",
		},
		{
			Path: "force-app/main/default/classes/HospitalAffiliationUpsertSyncActionTest.cls",
			Text: "@IsTest private class HospitalAffiliationUpsertSyncActionTest { static void run(){ System.debug([SELECT Id FROM Account]); } }",
		},
		{
			Path: "sfdx-project.json",
			Text: `{"sourceApiVersion":"64.0"}`,
		},
		{
			Path: "force-app/main/default/classes/Old.cls-meta.xml",
			Text: "<ApexClass><apiVersion>60.0</apiVersion></ApexClass>",
		},
	}

	findings := ReviewPatterns(files)
	for _, id := range []string{"enterprise.review.empty_catch", "enterprise.review.duplicate_mock_id", "enterprise.review.debug_soql", "enterprise.review.api_version_drift"} {
		if !hasPatternFinding(findings, id) {
			t.Fatalf("missing finding %q in %#v", id, findings)
		}
	}
}

func hasPatternFinding(findings []PatternFinding, id string) bool {
	for _, finding := range findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}
