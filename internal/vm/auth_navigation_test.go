package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecAuthenticationCallsUseSiteContext(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
System.assertEquals(null, Site.getSiteId());
System.assertEquals(null, Network.getNetworkId());
System.assertEquals(null, Network.forwardToAuthPage(null, null));
System.assertEquals(null, Network.forwardToAuthPage('/probe', null));
System.assertEquals(null, Site.login('unregistered@example.invalid', 'invalid', null));
System.assertEquals(null, Site.login('unregistered@example.invalid', 'invalid', '/probe'));
System.assertEquals(null, Site.login('unregistered@example.invalid', 'invalid', ''));
Aura.redirect((PageReference)null);
Aura.redirect(new PageReference('/probe'));
`, CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	for _, testContext := range []bool{false, true} {
		name := "anonymous"
		if testContext {
			name = "istest"
		}
		t.Run(name, func(t *testing.T) {
			org := storage.NewOrgState()
			// Network metadata alone leaves both public context IDs null.
			org.Objects["Network"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{APIName: "Network"},
				Records: map[storage.ID]storage.Record{
					"0DB000000000001": {ID: "0DB000000000001", Object: "Network"},
				},
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if testContext {
				machine.EnableTestContext()
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
