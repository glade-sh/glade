package apextest

import (
	"path/filepath"
	"testing"
)

func TestMethodHeaderForwardingNativeConformance(t *testing.T) {
	runNativeBoundaryConformance(t, "method_header_forwarding.json", "method-header-forwarding", 1)
}

func TestParseParamsIgnoresHeaderCommentsAndStringDelimiters(t *testing.T) {
	for _, source := range []string{
		`@InvocableMethod(label='Run') /** See {@link Helper}. */ public static void run(List<Input> requests) {}`,
		`@InvocableMethod(label='Value \' { (') public static void run(List<Input> requests) {}`,
		`public void run(List<Input> requests) /* ( { ignored */ {}`,
		`public void run(/* ignored */ final List<Input> /* ignored */ requests) {}`,
		`public abstract void run(List<Input> requests);`,
		`public Container(List<Input> requests) /* { ( ignored */ {}`,
	} {
		params, err := parseParams(source)
		if err != nil {
			t.Errorf("parseParams(%q): %v", source, err)
			continue
		}
		if len(params) != 1 || params[0].Type != "List<Input>" || params[0].Name != "requests" {
			t.Errorf("parseParams(%q) = %#v", source, params)
		}
	}
}

func TestRunStaticNestedListForwardingWithMethodHeaderComments(t *testing.T) {
	for _, test := range []struct{ name, annotation, comment string }{
		{name: "plain", annotation: "@InvocableMethod(label='Dispatch Items')"},
		{name: "inline_link", annotation: "@InvocableMethod(label='Dispatch Items')", comment: "/** Delegates (after validation) to {@link ForwardRouter}. */"},
		{name: "line_comment", annotation: "@InvocableMethod(label='Dispatch Items')", comment: "// This { and ( are comments.\n"},
		{name: "annotation_string", annotation: "@InvocableMethod(label='Dispatch Items' description='Values {label} and (text)')"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			classes := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(classes, "ForwardEntry.cls"), `
public inherited sharing class ForwardEntry {
  public class Request { @InvocableVariable public String text; }
  public class Response { @InvocableVariable public String text; }
  `+test.annotation+`
  `+test.comment+`
  public static List<Response> dispatch(List<Request> requests) {
    if (requests == null || requests.isEmpty()) return new List<Response>();
    return ForwardRouter.route(requests);
  }
  public static String ping() { return 'pong'; }
}
`)
			writeFile(t, filepath.Join(classes, "ForwardRouter.cls"), `
public inherited sharing class ForwardRouter {
  public static List<ForwardEntry.Response> route(List<ForwardEntry.Request> requests) {
    List<ForwardEntry.Response> responses = new List<ForwardEntry.Response>();
    for (ForwardEntry.Request request : requests) {
      ForwardEntry.Response response = new ForwardEntry.Response();
      response.text = request.text + ':routed';
      responses.add(response);
    }
    return responses;
  }
}
`)
			writeFile(t, filepath.Join(classes, "ForwardEntryTest.cls"), `
@IsTest private class ForwardEntryTest {
  @IsTest static void forwardsNestedItems() {
    ForwardEntry.Request request = new ForwardEntry.Request();
    request.text = 'first';
    List<ForwardEntry.Request> requests = new List<ForwardEntry.Request>{request};
    List<ForwardEntry.Response> results = ForwardEntry.dispatch(requests);
    System.assertEquals(1, results.size());
    System.assertEquals('first:routed', results[0].text);
  }
  @IsTest static void emptyAndScalarControls() {
    System.assertEquals(0, ForwardEntry.dispatch(new List<ForwardEntry.Request>()).size());
    System.assertEquals(0, ForwardEntry.dispatch(null).size());
    System.assertEquals('pong', ForwardEntry.ping());
  }
}
`)
			run := Run(loadTestIndex(t, root), Options{Parallelism: 1})
			if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						if result.Problem != nil {
							t.Logf("%s: %s", result.MethodName, result.Problem.Message)
						}
					}
				}
				t.Fatalf("summary = %#v", got)
			}
		})
	}
}
