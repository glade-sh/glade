package vm

import (
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestC11ImplicitChildLookupProjection(t *testing.T) {
	source, err := os.ReadFile("../apextest/testdata/conformance/c11sharing_child/anonymous.apex")
	if err != nil {
		t.Fatal(err)
	}
	checks := `
System.assertEquals(true,out.get('contact_parent_populated'));
System.assertEquals(true,out.get('opportunity_parent_populated'));
System.assertEquals(false,out.get('contact_email_populated'));
System.assertEquals(false,out.get('opportunity_description_populated'));`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			for _, name := range []string{"Account", "Contact", "Opportunity"} {
				storage.EnsureStandardObject(&org, name)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			t.Logf("lookup access: contact=%t opportunity=%t", machine.currentUserFieldPermission("Contact", "AccountId", "isAccessible"), machine.currentUserFieldPermission("Opportunity", "AccountId", "isAccessible"))
			body, err := CompileAnonymousWithOptions(string(source)+checks, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(body); err != nil {
				t.Fatal(err)
			}
		})
	}
}
