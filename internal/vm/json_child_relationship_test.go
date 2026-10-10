package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native JSON R239/R240 establish QueryResult deserialization and raw-array
// rejection. TestValueEdgesNativeConformance covers the captured child envelopes.
func TestExecJSONDeserializeSObjectChildRelationship(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{name: "queryResultEnvelopeAccepted", source: `
Account source = (Account)JSON.deserialize('{"Name":"Parent","Contacts":{"totalSize":1,"done":true,"records":[{"attributes":{"type":"Contact"},"LastName":"Child"}]}}', Account.class);
System.assertEquals(1, source.Contacts.size());
System.assertEquals('Child', source.Contacts[0].LastName);
`},
		{name: "rawChildArrayRejected", source: `
Boolean rejected = false;
try {
    Account source = (Account)JSON.deserialize('{"Contacts":[{"LastName":"Child"}]}', Account.class);
} catch (JSONException error) {
    rejected = true;
    System.assertEquals('QueryResult must start with \'{\'', error.getMessage());
}
System.assertEquals(true, rejected);
`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			program, err := CompileAnonymous(test.source)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			storage.EnsureStandardObject(&org, "Account")
			storage.EnsureStandardObject(&org, "Contact")
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
