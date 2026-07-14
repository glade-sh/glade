package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCloneRuntimeStartsWithFreshExecutionContextState(t *testing.T) {
	template := New(nil)
	template.executionUser = Object("User")
	template.currentPage = Object("PageReference")
	template.pageMessages = []Value{String("message")}
	template.testContext = &TestContext{
		HTTPMock:       Object("HttpCalloutMock"),
		WebServiceMock: Object("WebServiceMock"),
	}
	template.savepoints["sp"] = storage.OrgState{OrgID: "source"}
	template.savepointMarks["sp"] = storage.IsolationMark{}
	template.emailSavepoints["sp"] = []CapturedEmail{{}}
	template.savepointOrder["sp"] = 1
	template.nextSavepoint = 2
	template.restRequest = Object("RestRequest")
	template.restResponse = Object("RestResponse")
	template.serverBaseURL = "https://source.example"
	template.metadataDeploys["deploy"] = Object("MetadataDeploy")
	template.reportInstances["report"] = Object("ReportInstance")
	template.subMgmtTestRecords["record"] = Object("SubscriptionManagementRecord")
	template.subMgmtTestSeq = 1

	template.pageReferences = map[string]string{"Account": "/apex/Account"}
	feature := Object("FeatureValue")
	feature.Fields["Enabled"] = Bool(true)
	template.managedFeatureValues["feature"] = feature
	template.cachePut("local.session", "seed", String("cached"), 0)

	clone := template.CloneRuntime(nil)

	if clone.executionUser.Kind != "" {
		t.Fatalf("executionUser = %#v, want zero value", clone.executionUser)
	}
	if clone.currentPage.Kind != "" {
		t.Fatalf("currentPage = %#v, want zero value", clone.currentPage)
	}
	if len(clone.pageMessages) != 0 {
		t.Fatalf("pageMessages = %#v, want empty", clone.pageMessages)
	}
	if clone.testContext != nil {
		t.Fatalf("testContext = %#v, want nil so test mocks are fresh", clone.testContext)
	}
	if len(clone.savepoints) != 0 || len(clone.savepointMarks) != 0 ||
		len(clone.emailSavepoints) != 0 || len(clone.savepointOrder) != 0 || clone.nextSavepoint != 0 {
		t.Fatalf("savepoint state was carried into clone")
	}
	if clone.restRequest.Kind != "" || clone.restResponse.Kind != "" || clone.serverBaseURL != "" {
		t.Fatalf("REST request state was carried into clone")
	}
	if len(clone.metadataDeploys) != 0 || len(clone.reportInstances) != 0 ||
		len(clone.subMgmtTestRecords) != 0 || clone.subMgmtTestSeq != 0 {
		t.Fatalf("request artifacts were carried into clone")
	}

	if got := clone.pageReferences["Account"]; got != "/apex/Account" {
		t.Fatalf("page reference = %q, want copied template value", got)
	}
	clone.pageReferences["Account"] = "/apex/Changed"
	if got := template.pageReferences["Account"]; got != "/apex/Account" {
		t.Fatalf("clone page-reference mutation changed template: %q", got)
	}

	clonedFeature := clone.managedFeatureValues["feature"]
	clonedFeature.Fields["Enabled"] = Bool(false)
	if got := template.managedFeatureValues["feature"].Fields["Enabled"]; !got.Bool {
		t.Fatalf("clone managed-feature mutation changed template: %#v", got)
	}

	clone.platformCache["local.session"]["extra"] = cacheEntry{Value: String("clone")}
	if _, ok := template.platformCache["local.session"]["extra"]; ok {
		t.Fatal("clone platform-cache map mutation changed template")
	}
}

func TestCloneRuntimeDeepCopiesNestedPlatformCacheValues(t *testing.T) {
	template := New(nil)
	payload := Object("CachePayload")
	payload.Fields["Status"] = String("source")
	items := List(payload)
	nested := Map()
	nested.Map["items"] = items
	template.cachePut("local.session", "nested", nested, 0)

	clone := template.CloneRuntime(nil)
	clonedEntry := clone.platformCache["local.session"]["nested"]
	clonedEntry.Value.Map["items"].List[0].Fields["Status"] = String("clone")

	got := template.platformCache["local.session"]["nested"].Value.Map["items"].List[0].Fields["Status"]
	if got.Text != "source" {
		t.Fatalf("clone nested cache mutation changed template status to %q", got.Text)
	}
}
