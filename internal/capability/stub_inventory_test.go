package capability

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildStubInventoryReportSeparatesFeatureGatedSObjectFields(t *testing.T) {
	root := t.TempDir()
	writeStubInventoryTestFile(t, filepath.Join(root, "apex-system-stubs", "System", "String.cls"), `global class String {
    global String trim() { return null; }
}`)
	writeStubInventoryTestFile(t, filepath.Join(root, "apex-sobject-stubs", "Account.cls"), `global class Account extends SObject {
    public static SObjectFields Fields { get; private set; }
    global class SObjectFields {
        public SObjectField Name;
        public SObjectField PersonEmail;
        public SObjectField BillingCountryCode;
        public SObjectField NotAStandardField__c;
    }
}`)

	report, err := BuildStubInventoryReport(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Gaps.SObjectFieldMissingActiveCount != 3 {
		t.Fatalf("SObjectFieldMissingActiveCount = %d, want 3", report.Gaps.SObjectFieldMissingActiveCount)
	}
	if report.Gaps.SObjectFieldMissingFeatureGatedCount != 2 {
		t.Fatalf("SObjectFieldMissingFeatureGatedCount = %d, want 2", report.Gaps.SObjectFieldMissingFeatureGatedCount)
	}
	if report.Gaps.SObjectFieldMissingSupportedFeatureCount != 1 {
		t.Fatalf("SObjectFieldMissingSupportedFeatureCount = %d, want 1", report.Gaps.SObjectFieldMissingSupportedFeatureCount)
	}
	wantFeatureGated := []string{"Account.BillingCountryCode", "Account.PersonEmail"}
	for i, want := range wantFeatureGated {
		if report.Gaps.SObjectFieldMissingFeatureGatedSample[i] != want {
			t.Fatalf("feature gated sample = %#v, want prefix %#v", report.Gaps.SObjectFieldMissingFeatureGatedSample, wantFeatureGated)
		}
	}
	if len(report.Gaps.SObjectFieldMissingSupportedFeatureSample) != 1 || report.Gaps.SObjectFieldMissingSupportedFeatureSample[0] != "Account.NotAStandardField__c" {
		t.Fatalf("supported feature sample = %#v", report.Gaps.SObjectFieldMissingSupportedFeatureSample)
	}
}

func writeStubInventoryTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
