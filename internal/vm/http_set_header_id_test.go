package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Apex converts an Id to String wherever a String parameter is expected, so
// HttpRequest.setHeader accepts Id values for the header value.
func TestHttpRequestSetHeaderAcceptsIdValue(t *testing.T) {
	program, err := CompileAnonymous(`
Id orgId = UserInfo.getOrganizationId();
Id recordId = Id.valueOf('001000000000001AAA');
HttpRequest req = new HttpRequest();
req.setHeader('X-Org', orgId);
req.setHeader('X-Record', recordId);
System.assertEquals(String.valueOf(orgId), req.getHeader('X-Org'));
System.assertEquals(String.valueOf(recordId), req.getHeader('X-Record'));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
