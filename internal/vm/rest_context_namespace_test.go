package vm

import "testing"

// RestContext is a System namespace class. The qualified and implicit System
// forms address the same static request/response fields (Apex Reference Guide,
// RestContext class).
func TestExecRestContextSystemNamespaceAliases(t *testing.T) {
	cases := map[string]string{
		"qualifiedRead":        `RestRequest request = new RestRequest(); request.requestURI='/owned'; RestContext.request=request; System.assertEquals('/owned',System.RestContext.request.requestURI);`,
		"qualifiedWrite":       `RestRequest request = new RestRequest(); request.httpMethod='PATCH'; System.RestContext.request=request; System.assertEquals('PATCH',RestContext.request.httpMethod);`,
		"qualifiedNestedWrite": `RestContext.response=new RestResponse(); System.RestContext.response.statusCode=202; System.assertEquals(202,RestContext.response.statusCode);`,
		"caseInsensitive":      `RestContext.request=new RestRequest(); sYsTeM.rEsTcOnTeXt.ReQuEsT.httpMethod='GET'; System.assertEquals('GET',SYSTEM.RESTCONTEXT.REQUEST.httpMethod);`,
		"nullSafe":             `RestContext.request=null; System.assertEquals(null,System.RestContext.request?.requestBody?.toString());`,
		"responseReplacement":  `RestResponse response=new RestResponse(); response.statusCode=204; System.RestContext.response=response; System.assertEquals(204,RestContext.response.statusCode);`,
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRestContextPathNamespaceBoundary(t *testing.T) {
	for _, name := range []string{"App.RestContext.request", "System.System.RestContext.request", "System.RestContextual.request", "RestContext.other"} {
		if _, ok := canonicalRestContextPath(name); ok {
			t.Fatalf("recognized unrelated path %q", name)
		}
	}
	for _, name := range []string{"RestContext.request", "RestContext.response.statusCode"} {
		if canonical, ok := canonicalRestContextPath(name); !ok || canonical != name {
			t.Fatalf("legacy path %q => %q,%v", name, canonical, ok)
		}
	}
}
