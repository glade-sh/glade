package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// TestConnectApiNativeConformance replays the captured ConnectApi rows: each row
// is one @IsTest(SeeAllData=false) class at its API version. The row body sets
// String o to "EX|<type>|<message>" or "OK|<value>" and the test asserts false
// with "ROW|" + o, so the native outcome is the assertion message. Kinds:
//   - row: the reported outcome matches the native one exactly;
//   - assert: the call fails the test with the native uncatchable
//     System.AssertException message (setTest result not registered);
//   - compile: the org rejected the class at that API version;
//   - gap: a recorded native difference, listed and not replayed.
func TestConnectApiNativeConformance(t *testing.T) {
	var data struct {
		Rows []struct {
			ID, API, Origin, Kind, Native, Code, Gap string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/connectapi.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	byAPI := map[string][]int{}
	gaps := 0
	for i, row := range data.Rows {
		if row.Kind == "gap" {
			gaps++
			continue
		}
		byAPI[row.API] = append(byAPI[row.API], i)
	}
	if len(byAPI["62.0"]) == 0 || len(byAPI["67.0"]) == 0 {
		t.Fatalf("connectapi rows must cover APIs 62.0 and 67.0: %v", byAPI)
	}
	apis := make([]string, 0, len(byAPI))
	for api := range byAPI {
		apis = append(apis, api)
	}
	sort.Strings(apis)
	for _, api := range apis {
		indexes := byAPI[api]
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project, _ := json.Marshal(map[string]any{"sourceApiVersion": api, "packageDirectories": []map[string]any{{"path": "force-app", "default": true}}})
			writeFile(t, filepath.Join(root, "sfdx-project.json"), string(project))
			classRow := map[string]int{}
			for n, i := range indexes {
				row := data.Rows[i]
				class := fmt.Sprintf("CapiConformance%04d", n)
				classRow[class] = i
				body := "@IsTest\nprivate class " + class + " {\n    @IsTest\n    static void t() {\n" + indent(row.Code) +
					"\n        System.assert(false, 'ROW|' + o);\n    }\n}\n"
				path := filepath.Join(root, "force-app/main/default/classes", class+".cls")
				writeFile(t, path, body)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			}
			run := Run(loadTestIndex(t, root), Options{Parallelism: 1, NoDiskCache: true})
			got := map[string]testreport.Case{}
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					got[c.ClassName] = c
				}
			}
			for class, i := range classRow {
				row := data.Rows[i]
				c, ok := got[class]
				if !ok {
					t.Errorf("%s %s: no result", api, row.ID)
					continue
				}
				message := ""
				if c.Problem != nil {
					message = c.Problem.Message
				}
				switch row.Kind {
				case "row":
					_, outcome, found := strings.Cut(message, "ROW|")
					if !found || strings.TrimSpace(outcome) != row.Native {
						t.Errorf("%s %s (%s)\n native: %s\n  glade: %s %s", api, row.ID, row.Origin, row.Native, c.Status, message)
					}
				case "assert":
					if c.Status != testreport.StatusFail || !strings.Contains(message, "Assertion Failed: "+row.Native) || strings.Contains(message, "ROW|") {
						t.Errorf("%s %s (%s)\n native: AssertException %s\n  glade: %s %s", api, row.ID, row.Origin, row.Native, c.Status, message)
					}
				case "compile":
					if c.Status != testreport.StatusCompileError {
						t.Errorf("%s %s (%s)\n native: compile error %s\n  glade: %s %s", api, row.ID, row.Origin, row.Native, c.Status, message)
					}
				default:
					t.Errorf("%s %s: unknown kind %q", api, row.ID, row.Kind)
				}
			}
		})
	}
	t.Logf("%d replayed rows, %d recorded gaps", len(data.Rows)-gaps, gaps)
}

func indent(code string) string {
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		lines[i] = "        " + line
	}
	return strings.Join(lines, "\n")
}

// Slack and DataWeave stay outside the ConnectApi surface: these controls pin
// Glade's existing local outcomes (not native rows) beside a ConnectApi class
// that now compiles in the same project.
func TestConnectApiSurfaceKeepsSlackAndDataWeaveExcluded(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	classes := map[string]string{
		"CapiInputTest": `@IsTest private class CapiInputTest {
  @IsTest static void t() {
    ConnectApi.FeedItemInput input = new ConnectApi.FeedItemInput();
    input.subjectId = 'me';
    ConnectApi.MessageBodyInput body = new ConnectApi.MessageBodyInput();
    body.messageSegments = new List<ConnectApi.MessageSegmentInput>{ new ConnectApi.TextSegmentInput() };
    input.body = body;
    System.assertEquals(1, input.body.messageSegments.size());
  }
}`,
		"SlackUse":     `public class SlackUse { public static Object app() { return Slack.App.Foo.get(); } }`,
		"SlackUseTest": `@IsTest private class SlackUseTest { @IsTest static void t() { SlackUse.app(); } }`,
		"DataWeaveUseTest": `@IsTest private class DataWeaveUseTest {
  @IsTest static void t() {
    DataWeave.Script script = DataWeave.Script.createScript('mapping');
    script.execute(new Map<String, Object>());
  }
}`,
	}
	for name, source := range classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>")
	}
	run := Run(loadTestIndex(t, root), Options{Parallelism: 1, NoDiskCache: true})
	got := map[string]testreport.Case{}
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			got[c.ClassName] = c
		}
	}
	if c := got["CapiInputTest"]; c.Status != testreport.StatusPass {
		t.Fatalf("ConnectApi input class: %s %+v", c.Status, c.Problem)
	}
	if c := got["SlackUseTest"]; c.Status != testreport.StatusCompileError || c.Problem == nil || !strings.Contains(c.Problem.Message, "Slack.App.Foo") {
		t.Fatalf("Slack control changed: %s %+v", c.Status, c.Problem)
	}
	if c := got["DataWeaveUseTest"]; c.Status != testreport.StatusFail || c.Problem == nil || !strings.Contains(c.Problem.Message, "Could not find DataWeave script mapping") {
		t.Fatalf("DataWeave control changed: %s %+v", c.Status, c.Problem)
	}
}

// Native compile probes (APIs 62.0 and 67.0): ConnectApi.BaseEndpointExtension
// is the only extendable ConnectApi class, and each of its ten hooks can be
// overridden. Every other class reports "Non-virtual and non-abstract type
// cannot be extended" or a non-visible constructor.
func TestConnectApiBaseEndpointExtensionHooksOverride(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			classes := map[string]string{
				"OrderHook": `public class OrderHook extends ConnectApi.BaseEndpointExtension {
  public override ConnectApi.EndpointExtensionRequest beforePost(ConnectApi.EndpointExtensionRequest request) { return request; }
  public override ConnectApi.EndpointExtensionResponse afterPost(ConnectApi.EndpointExtensionResponse response, ConnectApi.EndpointExtensionRequest request) { return response; }
}`,
				"OrderHookTest": `@IsTest private class OrderHookTest {
  @IsTest static void t() {
    ConnectApi.BaseEndpointExtension hook = new OrderHook();
    System.assertEquals(null, hook.beforePost(null));
  }
}`,
			}
			for name, source := range classes {
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			}
			run := Run(loadTestIndex(t, root), Options{Parallelism: 1, NoDiskCache: true})
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
				t.Fatalf("summary = %#v problem=%q", got, firstRunProblem(run))
			}
		})
	}
}
