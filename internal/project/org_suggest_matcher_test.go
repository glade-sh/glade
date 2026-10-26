package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetupFeatureMatcherNativeSourceRoutes(t *testing.T) {
	for _, tc := range []struct{ class, feature string }{
		{"IdentifierOnlyUse", ""}, {"LiteralOnlyUse", ""},
		{"UserTypeNameUse", ""}, {"TerritoryLabelUse", ""},
		{"ExperienceDynamicFieldUse", "Communities"},
	} {
		t.Run("sourceMatcher/"+tc.class, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("testdata/conformance/org_setup_evidence/project/force-app/main/default/classes", tc.class+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			for _, rule := range featureEvidenceRules {
				if rule.feature != "Communities" && rule.feature != "Territory2" {
					continue
				}
				if got, want := matchesSetupFeature(rule, string(source), false), rule.feature == tc.feature; got != want {
					t.Fatalf("%s source matcher: got %v, want %v", rule.feature, got, want)
				}
			}
		})
	}
}

func TestSetupFeatureMatcherLiteralAndReceiverRoutes(t *testing.T) {
	for _, tc := range []struct{ route, source, feature string }{
		{"doubleQuotedLabel", `const label = "Territory2 records IsPortalEnabled";`, ""},
		{"typedReceiverWithUnrelatedName", "class SetupUse { void a(Map<String,Object> labels) {} Object b(User u) { return u.get('IsPortalEnabled'); } }", "Communities"},
		{"doubleQuotedSchemaImport", `import portalField from "@salesforce/schema/User.IsPortalEnabled";`, "Communities"},
	} {
		t.Run("sourceMatcher/"+tc.route, func(t *testing.T) {
			for _, rule := range featureEvidenceRules {
				if rule.feature != "Communities" && rule.feature != "Territory2" {
					continue
				}
				if got, want := matchesSetupFeature(rule, tc.source, false), rule.feature == tc.feature; got != want {
					t.Fatalf("%s source matcher: got %v, want %v", rule.feature, got, want)
				}
			}
		})
	}
}
