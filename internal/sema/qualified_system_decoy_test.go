package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

// SF162: an explicit System type remains platform-owned when a project defines
// an unqualified class with the same name. The short names below remain usable
// as project declarations; only System-qualified references are asserted here.
func TestQualifiedSystemTypesSurviveProjectDecoysAtAPI65(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Callable.cls":         `public class Callable {}`,
		"HttpCalloutMock.cls":  `public class HttpCalloutMock {}`,
		"Schedulable.cls":      `public class Schedulable {}`,
		"HttpResponse.cls":     `public class HttpResponse {}`,
		"HttpRequest.cls":      `public class HttpRequest {}`,
		"RestRequest.cls":      `public class RestRequest {}`,
		"RestResponse.cls":     `public class RestResponse {}`,
		"FinalizerContext.cls": `public class FinalizerContext {}`,
		"ParentJobResult.cls":  `public class ParentJobResult {}`,
		"Qualified.cls": `
global class Qualified implements System.Callable, System.Schedulable, System.HttpCalloutMock, System.Finalizer {
  global Object call(String action, Map<String, Object> arguments) { return action; }
  global void execute(System.SchedulableContext context) {}
  global System.HttpResponse respond(System.HttpRequest request) { return new System.HttpResponse(); }
  global void execute(System.FinalizerContext context) {
    switch on context.getResult() {
      when UNHANDLED_EXCEPTION {}
      when else {}
    }
  }
}`,
		"QualifiedProof.cls": `
public class QualifiedProof {
  public static void check() {
    System.HttpResponse httpResponse = new System.HttpResponse();
    httpResponse.setHeader('one', '1');
    String httpHeaders = String.join(httpResponse.getHeaderKeys(), ',');
    System.RestRequest request = new System.RestRequest();
    request.headers.put('two', '2');
    String requestHeaders = String.join(request.headers.keySet(), ',');
    System.RestResponse response = new System.RestResponse();
    response.headers.put('three', '3');
    String responseHeaders = String.join(response.headers.keySet(), ',');
  }
}`,
	}, "65.0")
	if result.HasErrors() {
		t.Fatalf("explicit System types bound to project decoys: %#v", result.Diagnostics)
	}
}

// SF162: System.HandledException is a catchable platform exception.
func TestQualifiedHandledExceptionIsCatchableAtAPI65(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Handled.cls": `
public class Handled {
  public static void check() {
    try {
      throw new System.HandledException('blocked');
    } catch (System.HandledException error) {
      System.debug(error);
    }
  }
}`,
	}, "65.0")
	if result.HasErrors() {
		t.Fatalf("System.HandledException was rejected as a catch type: %#v", result.Diagnostics)
	}
}

func TestQualifiedSystemAliasesLeaveProjectAndUserNamespacesUntouched(t *testing.T) {
	view := buildSemaTypeMemberState(typesys.Index{Types: []typesys.TypeSymbol{
		{Kind: apexast.DeclarationClass, Name: "Callable"},
		{Kind: apexast.DeclarationClass, Name: "HttpResponse"},
		{Kind: apexast.DeclarationClass, Name: "Service", Namespace: "custom", Dependency: true, Artifact: true},
	}}, nil).view()

	for _, name := range []string{"Callable", "HttpResponse"} {
		members, _, ok := semaLookupTypeMembers(view, name)
		if !ok || members.platform || members.kind != apexast.DeclarationClass {
			t.Fatalf("unqualified project decoy %s = %#v, %v", name, members, ok)
		}
	}
	for _, name := range []string{"System.Callable", "System.HttpResponse"} {
		members, _, ok := semaLookupTypeMembers(view, name)
		if !ok || !members.platform {
			t.Fatalf("qualified platform type %s = %#v, %v", name, members, ok)
		}
	}
	if got := resolveNestedTypeName(view, "Consumer", "custom.Service"); got != "custom.Service" {
		t.Fatalf("user namespace resolution = %q, want custom.Service", got)
	}
}
