package sema

import (
	"fmt"
	"strings"
	"testing"
)

func TestAsyncDeclaredAndInheritedCallbackContractsNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []struct{ name, iface, body string }{
			{"Queueable", "Queueable", "public %svoid execute(QueueableContext context) {}"},
			{"Schedulable", "Schedulable", "public %svoid execute(SchedulableContext context) {}"},
			{"Batchable", "Database.Batchable<SObject>", "public %sIterable<SObject> start(Database.BatchableContext context) { return new List<SObject>(); } public %svoid execute(Database.BatchableContext context, List<SObject> scope) {} public %svoid finish(Database.BatchableContext context) {}"},
		} {
			for _, static := range []bool{false, true} {
				for _, inherited := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/static=%v/inherited=%v", api, route.name, static, inherited), func(t *testing.T) {
						qualifier := ""
						if static {
							qualifier = "static "
						}
						var body string
						if route.name == "Batchable" {
							body = fmt.Sprintf(route.body, qualifier, qualifier, qualifier)
						} else {
							body = fmt.Sprintf(route.body, qualifier)
						}
						files := map[string]string{"CallbackBase.cls": "public virtual class CallbackBase implements " + route.iface + " {" + body + "}"}
						if inherited {
							files["CallbackChild.cls"] = "public class CallbackChild extends CallbackBase {}"
						}
						result := analyzeDeclarationProjectWithAPIVersion(t, files, api)
						if result.HasErrors() != (static && inherited) {
							t.Fatalf("native rejects only inherited static callbacks; diagnostics=%+v", result.Diagnostics)
						}
						if static && inherited {
							methods := []string{"execute"}
							if route.name == "Batchable" {
								methods = []string{"start", "execute", "finish"}
							}
							for _, method := range methods {
								found := false
								for _, d := range result.Diagnostics {
									if d.Code == "GLADESEMA017" && strings.HasSuffix(d.File, "CallbackChild.cls") && (strings.Contains(d.Message, "\""+method+"\"") || strings.Contains(d.Message, "."+method+"(")) {
										found = true
									}
								}
								if !found {
									t.Fatalf("native child must implement %s; diagnostics=%+v", method, result.Diagnostics)
								}
							}
						}
					})
				}
			}
		}
	}
}

// Native: testdata/platform_batch_superclass/org.tsv:6,13 accepts a complete
// implementation of the inherited hooks and finish callback.
func TestBatchableInheritedThroughPlatformSuperclassAccepted(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"PluginUser.cls": "public class PluginUser { public class Concrete extends UserProvisioning.PluginBatchable { Concrete() { super(new List<SObject>()); } public override Map<String,Object> flowInputPreprocessing(Map<String,Object> values) { return values; } public override String getEventPrefix() { return 'owned'; } public override void postBatchProcessing() {} public override void flowPostProcessing(UserProvisioning.ProvisioningProcessHandlerOutput output, SObject row) {} public void finish(Database.BatchableContext context) {} } }",
			}, api)
			if result.HasErrors() {
				t.Fatalf("native accepts the complete platform subclass; diagnostics=%+v", result.Diagnostics)
			}
		})
	}
}
