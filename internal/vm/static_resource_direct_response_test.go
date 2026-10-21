package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These assertions describe a narrow candidate for configured resources.
// Native captures are required before accepting its direct-call parity.
func TestStaticResourceConfiguredDirectResponse(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
StaticResourceCalloutMock mock = new StaticResourceCalloutMock();
mock.setStaticResource('ReplyPayload');
mock.setStatusCode(203);
mock.setStatus('Configured reply');
mock.setHeader('Content-Type', 'application/json');
HttpRequest request = new HttpRequest();
request.setEndpoint('https://example.test/reply');
request.setMethod('GET');
Integer beforeCalls = Limits.getCallouts();
HttpResponse direct = mock.respond(request);
System.assertEquals('{"ok":true}', direct.getBody());
System.assertEquals(203, direct.getStatusCode());
System.assertEquals('Configured reply', direct.getStatus());
System.assertEquals('application/json', direct.getHeader('Content-Type'));
System.assertEquals(beforeCalls, Limits.getCallouts());
HttpCalloutMock viaInterface = mock;
HttpResponse interfaceReply = viaInterface.respond(request);
System.assertEquals(direct.getBody(), interfaceReply.getBody());
System.assertEquals(direct.getStatusCode(), interfaceReply.getStatusCode());
System.assertEquals(direct.getStatus(), interfaceReply.getStatus());
System.assertEquals(direct.getHeader('Content-Type'), interfaceReply.getHeader('Content-Type'));
System.assertEquals(beforeCalls, Limits.getCallouts());
Test.setMock(HttpCalloutMock.class, mock);
HttpResponse sent = new Http().send(request);
System.assertEquals(direct.getBody(), sent.getBody());
System.assertEquals(direct.getStatusCode(), sent.getStatusCode());
System.assertEquals(direct.getStatus(), sent.getStatus());
System.assertEquals(direct.getHeader('Content-Type'), sent.getHeader('Content-Type'));
System.assertEquals(beforeCalls + 1, Limits.getCallouts());
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			org.Metadata.StaticResources = []storage.StaticResourceMetadata{{Name: "ReplyPayload", Content: `{"ok":true}`, ContentType: "application/json"}}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
