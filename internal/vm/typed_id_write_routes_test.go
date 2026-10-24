package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestTypedIDConstructorAndDirectWriteRoutesRejectMalformedString(t *testing.T) {
	for _, route := range []string{"constructor_reference", "direct_reference", "constructor_id", "direct_id"} {
		t.Run(route, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Contact")
			machine := New(nil)
			machine.SetOrg(&org)
			source := "Contact row = new Contact(AccountId='not-a-reference-value');"
			switch route {
			case "direct_reference":
				source = "Contact row = new Contact(); row.AccountId='not-a-reference-value';"
			case "constructor_id":
				source = "Contact row = new Contact(Id='not-a-reference-value');"
			case "direct_id":
				source = "Contact row = new Contact(); row.Id='not-a-reference-value';"
			}
			program, err := CompileAnonymous("String observed; try { " + source + " observed='accepted'; } catch(Exception e) { observed=e.getTypeName()+'|'+e.getMessage(); } System.assertEquals('System.StringException|Invalid id: not-a-reference-value',observed);")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
