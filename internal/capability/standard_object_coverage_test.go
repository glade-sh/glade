package capability

import (
	"bytes"
	"strings"
	"testing"
)

func TestStandardObjectCoverageClassifiesShapeAndBehavior(t *testing.T) {
	report := BuildStandardObjectCoverageReport()
	entries := make(map[string]StandardObjectCoverageEntry, len(report.Objects))
	for _, entry := range report.Objects {
		entries[entry.Object] = entry
	}

	if report.Totals.ShapeObjects != report.Totals.Objects {
		t.Fatalf("shape objects = %d, want all %d standard objects", report.Totals.ShapeObjects, report.Totals.Objects)
	}
	if report.Totals.BehaviorObjects == 0 {
		t.Fatal("behavior object count was not ratcheted")
	}
	for _, objectName := range []string{
		"Account",
		"Attachment",
		"CampaignMember",
		"CampaignMemberStatus",
		"Contact",
		"ContentDistribution",
		"ContentDocument",
		"ContentDocumentLink",
		"ContentVersion",
		"Document",
		"EmailMessage",
		"EmailMessageRelation",
		"FieldPermissions",
		"Lead",
		"ObjectPermissions",
		"Opportunity",
		"OpportunityLineItem",
		"PermissionSet",
		"PermissionSetAssignment",
		"PermissionSetGroup",
		"PermissionSetGroupComponent",
		"Pricebook2",
		"PricebookEntry",
		"Product2",
		"Profile",
		"RecordType",
		"SetupEntityAccess",
		"User",
	} {
		entry, ok := entries[objectName]
		if !ok {
			t.Fatalf("missing behavior object %s", objectName)
		}
		if entry.Coverage != "behavior" {
			t.Fatalf("%s coverage = %q, want behavior", objectName, entry.Coverage)
		}
	}
	if entry := entries["AIApplication"]; entry.Coverage != "shape" {
		t.Fatalf("AIApplication coverage = %q, want shape", entry.Coverage)
	}
}

func TestWriteStandardObjectCoverageMarkdownIncludesCoverageRatchet(t *testing.T) {
	report := StandardObjectCoverageReport{
		Totals: StandardObjectCoverageTotals{Objects: 2, ShapeObjects: 2, BehaviorObjects: 1},
		Objects: []StandardObjectCoverageEntry{
			{Object: "Account", KeyPrefix: "001", Coverage: "behavior", Fields: 2},
			{Object: "AIApplication", Coverage: "shape", Fields: 1},
		},
	}
	var out bytes.Buffer
	if err := WriteStandardObjectCoverageMarkdown(&out, report); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"- Shape objects: 2",
		"- Behavior objects: 1",
		"| Object | Coverage | Key Prefix |",
		"| `Account` | `behavior` | `001` |",
		"| `AIApplication` | `shape` | `` |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown missing %q:\n%s", want, got)
		}
	}
}
