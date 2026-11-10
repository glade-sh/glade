package storage_test

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// email_domain_shape/org.tsv captures the sandbox suffix for six independent
// addresses on anonymous and test routes at APIs 62.0 and 67.0.
func TestEmailDomainSandboxSuffixNative(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	services := []project.EmailService{{HandlerDeclared: true, ApexClass: "SyntheticHandler", FunctionName: "SyntheticFunction", Addresses: []project.EmailServiceAddress{
		{DeveloperName: "First", LocalPart: "x", RunAsUser: "system@example.invalid"},
		{DeveloperName: "Second", LocalPart: "longer_local", RunAsUser: "system@example.invalid"},
	}}}
	if err := storage.ApplyEmailServices(&org, services); err != nil {
		t.Fatal(err)
	}
	before := map[storage.ID]string{}
	for id, row := range org.Objects["EmailServicesAddress"].Records {
		value := row.Fields["EmailDomainName"].String
		before[id] = value
		if !strings.HasSuffix(value, ".apex.sandbox.salesforce.com") {
			t.Errorf("sandbox suffix differs: %q", value)
		}
	}
	if len(before) != 2 {
		t.Fatalf("got %d addresses", len(before))
	}
	if err := storage.ApplyEmailServices(&org, services); err != nil {
		t.Fatal(err)
	}
	for id, row := range org.Objects["EmailServicesAddress"].Records {
		if row.Fields["EmailDomainName"].String != before[id] {
			t.Error("reapplying changed persisted domain identity")
		}
		row.Fields["EmailDomainName"] = storage.StringValue("persisted.invalid")
		org.Objects["EmailServicesAddress"].Records[id] = row
	}
	if err := storage.ApplyEmailServices(&org, services); err != nil {
		t.Fatal(err)
	}
	for _, row := range org.Objects["EmailServicesAddress"].Records {
		if row.Fields["EmailDomainName"].String != "persisted.invalid" {
			t.Error("reapplying replaced an existing address domain")
		}
	}
}
