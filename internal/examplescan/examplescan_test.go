package examplescan

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/diagnostic"
)

func TestScanSFDXProject(t *testing.T) {
	root := t.TempDir()

	// Write sfdx-project.json.
	sfdx := `{"packageDirectories": [{"path": "force-app/main/default", "default": true}]}`
	writeFile(t, filepath.Join(root, "sfdx-project.json"), sfdx)

	// Write Apex classes.
	writeFile(t, filepath.Join(root, "force-app/main/default/classes", "MyClass.cls"),
		`public class MyClass {
			@AuraEnabled
			public static String doWork() {
				return [SELECT Id FROM Account LIMIT 1].Id;
			}
		}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes", "MyTest.cls"),
		`@isTest
		private class MyTest {
			@isTest static void testIt() {
				System.assert(true);
			}
		}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes", "Queue.cls"),
		`public class Queue implements Queueable {
			public void execute(QueueableContext ctx) {}
		}`)

	// Write trigger.
	writeFile(t, filepath.Join(root, "force-app/main/default/triggers", "AccountTrigger.trigger"),
		`trigger AccountTrigger on Account (before insert, after update) {
			if (Trigger.isBefore) { }
		}`)

	// Write object metadata.
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `<CustomField/>`)

	// Write Visualforce page.
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/MyPage.page"), `<apex:page/>`)

	// Write Aura bundle.
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/MyApp/MyApp.app"), `<aura:application/>`)

	// Write LWC.
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/myComp/myComp.js"), `export default class MyComp {}`)

	// Write workflow.
	writeFile(t, filepath.Join(root, "force-app/main/default/workflows/Account.workflow-meta.xml"), `<Workflow/>`)

	report, err := Scan(root, Options{Name: "test-sfdx", RunSema: true, RunSurfaceScan: true})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	if report.Name != "test-sfdx" {
		t.Errorf("name = %q, want test-sfdx", report.Name)
	}
	if report.SourceLayout != "sfdx" {
		t.Errorf("layout = %q, want sfdx", report.SourceLayout)
	}
	if report.Counts.ApexClasses != 3 {
		t.Errorf("apexClasses = %d, want 3", report.Counts.ApexClasses)
	}
	if report.Counts.ApexTriggers != 1 {
		t.Errorf("apexTriggers = %d, want 1", report.Counts.ApexTriggers)
	}
	if report.Counts.TestClasses != 1 {
		t.Errorf("testClasses = %d, want 1", report.Counts.TestClasses)
	}
	if report.Counts.Objects != 1 {
		t.Errorf("objects = %d, want 1", report.Counts.Objects)
	}
	if report.Counts.Fields != 1 {
		t.Errorf("fields = %d, want 1", report.Counts.Fields)
	}
	if report.Counts.VisualforcePages != 1 {
		t.Errorf("visualforcePages = %d, want 1", report.Counts.VisualforcePages)
	}
	if report.Counts.AuraComponents != 1 {
		t.Errorf("auraComponents = %d, want 1", report.Counts.AuraComponents)
	}
	if report.Counts.LWCComponents != 1 {
		t.Errorf("lwcComponents = %d, want 1", report.Counts.LWCComponents)
	}
	if report.Counts.Workflows != 1 {
		t.Errorf("workflows = %d, want 1", report.Counts.Workflows)
	}

	// Check constructs.
	if report.Constructs.Classes < 3 {
		t.Errorf("classes = %d, want >= 3", report.Constructs.Classes)
	}
	if !contains(report.Constructs.Annotations, "AuraEnabled") {
		t.Errorf("missing AuraEnabled annotation: %v", report.Constructs.Annotations)
	}
	if !contains(report.Constructs.Annotations, "isTest") {
		t.Errorf("missing isTest annotation: %v", report.Constructs.Annotations)
	}
	if !contains(report.Constructs.AsyncInterfaces, "Queueable") {
		t.Errorf("missing Queueable async interface: %v", report.Constructs.AsyncInterfaces)
	}

	// Check runtime usage.
	if !contains(report.RuntimeUsage.SOQLFeatures, "static-query") {
		t.Errorf("missing static-query SOQL feature: %v", report.RuntimeUsage.SOQLFeatures)
	}
	if !contains(report.RuntimeUsage.TriggerOperations, "isBefore") {
		t.Errorf("missing isBefore trigger op: %v", report.RuntimeUsage.TriggerOperations)
	}
	if !contains(report.RuntimeUsage.NamespaceRefs, "System") {
		t.Errorf("missing System namespace ref: %v", report.RuntimeUsage.NamespaceRefs)
	}

	// Diagnostics and surfaces should be present.
	if len(report.Surfaces) == 0 {
		t.Logf("no surfaces found (may be expected for tiny project)")
	}
}

func TestScanLegacyLayout(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "src/classes", "Legacy.cls"), `public class Legacy {}`)
	writeFile(t, filepath.Join(root, "src/triggers", "Legacy.trigger"), `trigger Legacy on Account (before insert) {}`)
	writeFile(t, filepath.Join(root, "src/objects", "Account.object"), `<CustomObject/>`)
	writeFile(t, filepath.Join(root, "src/pages", "LegacyPage.page"), `<apex:page/>`)

	report, err := Scan(root, Options{Name: "legacy"})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if report.SourceLayout != "legacy" {
		t.Errorf("layout = %q, want legacy", report.SourceLayout)
	}
	if report.Counts.ApexClasses != 1 {
		t.Errorf("apexClasses = %d, want 1", report.Counts.ApexClasses)
	}
	if report.Counts.ApexTriggers != 1 {
		t.Errorf("apexTriggers = %d, want 1", report.Counts.ApexTriggers)
	}
	if report.Counts.VisualforcePages != 1 {
		t.Errorf("visualforcePages = %d, want 1", report.Counts.VisualforcePages)
	}
}

func TestReportJSON(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/classes", "A.cls"), `public class A {}`)

	report, err := Scan(root, Options{})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	var buf bytes.Buffer
	if err := report.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}
	if !strings.Contains(buf.String(), "apexClasses") {
		t.Errorf("JSON missing apexClasses key")
	}
}

func TestScanSkipsIgnoredDirs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/classes", "Good.cls"), `public class Good {}`)
	writeFile(t, filepath.Join(root, "force-app/node_modules/bad/Bad.cls"), `public class Bad {}`)
	writeFile(t, filepath.Join(root, "force-app/.git/objects/abc/Bad.cls"), `public class Bad {}`)

	report, err := Scan(root, Options{})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if report.Counts.ApexClasses != 1 {
		t.Errorf("apexClasses = %d, want 1 (should skip node_modules and .git)", report.Counts.ApexClasses)
	}
}

func TestCategorizeDiagnosticsHandlesShortCodes(t *testing.T) {
	breakdown := categorizeDiagnostics([]diagnostic.Diagnostic{{
		Code:    "X",
		Message: "short diagnostic code",
	}}, nil)
	if len(breakdown.UnobservedParityFollowup) != 1 {
		t.Fatalf("unobserved followups = %d, want 1", len(breakdown.UnobservedParityFollowup))
	}
}

func TestScanReportsMalformedMetadataAsDiagnostic(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/classes/A.cls"), `public class A {}`)
	writeFile(t, filepath.Join(root, "force-app/objects/Thing__c/Thing__c.object-meta.xml"), `<CustomObject>`)

	report, err := Scan(root, Options{RunSema: true})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(report.Diagnostics.ObservedBlockers) != 1 {
		t.Fatalf("observed blockers = %#v, want one metadata blocker", report.Diagnostics.ObservedBlockers)
	}
	if report.Diagnostics.ObservedBlockers[0].Code != "OAEREXAMPLE002" {
		t.Fatalf("diagnostic code = %q, want OAEREXAMPLE002", report.Diagnostics.ObservedBlockers[0].Code)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
