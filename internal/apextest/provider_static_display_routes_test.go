package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// Project runner (@IsTest) route twin for provider display at APIs 62/67.
// The direct provider is the captured shape of
// testdata/conformance/namespaced_type_text/org.tsv R002 (lines 5 and 19): a
// class that implements StubProvider with no superclass shows its declared
// static list beside its instance fields, in a
// namespaced project like the capture. No native row covers a provider
// that extends another class, so that shape keeps its pre-port instance-field
// text, directly and inside the generated stub. Base-preserving guard pending
// a native capture of the inherited shape.
func TestProviderStaticDisplayProjectRunnerRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			meta := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>\n"
			writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"namespace\": \"nsx\", \"sourceApiVersion\": \""+api+"\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
			classes := map[string]string{
				"GladeDisplayStubApi": "public interface GladeDisplayStubApi { String label(); }\n",
				"GladeDirectDisplayProvider": "public class GladeDirectDisplayProvider implements System.StubProvider {\n    public Boolean providerFlag = true;\n    public static List<String> observedTypes = new List<String>();\n" +
					"    public Object handleMethodCall(Object receiver, String methodName, Type returnType, List<Type> parameterTypes, List<String> parameterNames, List<Object> arguments) { return null; }\n}\n",
				"GladeBaseDisplayProvider": "public virtual class GladeBaseDisplayProvider implements System.StubProvider {\n    public static List<String> baseTypes = new List<String>();\n" +
					"    public Object handleMethodCall(Object receiver, String methodName, Type returnType, List<Type> parameterTypes, List<String> parameterNames, List<Object> arguments) { return null; }\n}\n",
				"GladeChildDisplayProvider": "public class GladeChildDisplayProvider extends GladeBaseDisplayProvider {\n    public Boolean childFlag = true;\n    public static List<String> childTypes = new List<String>();\n}\n",
				"GladeProviderDisplayTest": "@IsTest private class GladeProviderDisplayTest {\n" +
					"    @IsTest static void directProviderShowsStaticState() {\n" +
					"        GladeDirectDisplayProvider provider = new GladeDirectDisplayProvider();\n" +
					"        System.assertEquals('GladeDirectDisplayProvider:[observedTypes=(), providerFlag=true]', String.valueOf(provider));\n" +
					"        Object stub = Test.createStub(GladeDisplayStubApi.class, provider);\n" +
					"        System.assertEquals('GladeDisplayStubApi__sfdc_ApexStub:[stubProvider=GladeDirectDisplayProvider:[observedTypes=(), providerFlag=true]]', String.valueOf(stub));\n" +
					"    }\n" +
					"    @IsTest static void inheritedProviderKeepsInstanceText() {\n" +
					"        GladeChildDisplayProvider provider = new GladeChildDisplayProvider();\n" +
					"        System.assertEquals('GladeChildDisplayProvider:[childFlag=true]', String.valueOf(provider));\n" +
					"        Object stub = Test.createStub(GladeDisplayStubApi.class, provider);\n" +
					"        System.assertEquals('GladeDisplayStubApi__sfdc_ApexStub:[stubProvider=GladeChildDisplayProvider:[childFlag=true]]', String.valueOf(stub));\n" +
					"    }\n}\n",
			}
			for name, source := range classes {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), source)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), meta)
			}
			index := loadTestIndex(t, root)
			if result := sema.Analyze(index); result.HasErrors() {
				t.Fatalf("provider display semantics: %#v", result.Diagnostics)
			}
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
				data, _ := json.Marshal(run)
				t.Fatalf("provider display routes: %s", data)
			}
		})
	}
}
