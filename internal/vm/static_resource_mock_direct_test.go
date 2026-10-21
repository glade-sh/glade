package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// HttpCalloutMock.respond defines the interface contract. Compare direct
// calls with the existing Http.send path; missing-resource errors need capture.
func TestExecStaticResourceMockDirectMatchesHttpSend(t *testing.T) {
	program, err := CompileAnonymous(`
HttpRequest request = new HttpRequest();
request.setEndpoint('https://example.test/owned');request.setMethod('GET');
StaticResourceCalloutMock single = new StaticResourceCalloutMock();
single.setStaticResource('OwnedReply');single.setStatusCode(203);single.setHeader('Content-Type','application/json');
HttpCalloutMock direct = single;
HttpResponse response = direct.respond(request);
Test.setMock(HttpCalloutMock.class,single);
HttpResponse sent = new Http().send(request);
System.assertEquals(sent.getBody(),response.getBody());
System.assertEquals(sent.getStatusCode(),response.getStatusCode());
System.assertEquals(sent.getHeader('Content-Type'),response.getHeader('Content-Type'));
MultiStaticResourceCalloutMock multiple = new MultiStaticResourceCalloutMock();
multiple.setStaticResource('https://example.test/owned','OwnedReply');
HttpCalloutMock multiDirect = multiple;
Test.setMock(HttpCalloutMock.class,multiple);
System.assertEquals(new Http().send(request).getBody(),multiDirect.respond(request).getBody());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	org.Objects["StaticResource"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "StaticResource", KeyPrefix: "081", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}, "Body": {APIName: "Body", Type: storage.FieldBlob}}}, Records: map[storage.ID]storage.Record{"081000000000099": {ID: "081000000000099", Object: "StaticResource", Fields: map[string]storage.Value{"Name": storage.StringValue("OwnedReply"), "Body": storage.BlobValue(`{"reply":"owned"}`)}}}}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
