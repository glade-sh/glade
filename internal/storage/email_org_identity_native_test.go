package storage_test

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// carried_conflict_isolation/org.tsv:2 and :254 observe the same two
// synthetic addresses in independently provisioned plain Developer orgs.
// Both address domains differ between orgs; repeated reads and redeployment
// retain each org's domain. The capture records label lengths, not the domains.
func TestEmailServicesDomainIdentityIsOrgScoped(t *testing.T) {
	services := []project.EmailService{{
		HandlerDeclared: true,
		ApexClass:       "SyntheticHandler",
		FunctionName:    "SyntheticFunction",
		Addresses: []project.EmailServiceAddress{
			{DeveloperName: "First", LocalPart: "first", RunAsUser: "system@example.invalid"},
			{DeveloperName: "Second", LocalPart: "second", RunAsUser: "system@example.invalid"},
		},
	}}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var domains [2]map[storage.ID]string
			for i := range domains {
				org := storage.NewOrgState()
				org.OrgID = fmt.Sprintf("synthetic-org-%d", i)
				org.APIVersion = api
				storage.EnsureDeterministicPlatformData(&org)
				if err := storage.ApplyEmailServices(&org, services); err != nil {
					t.Fatal(err)
				}
				domains[i] = map[storage.ID]string{}
				for id, record := range org.Objects["EmailServicesAddress"].Records {
					domains[i][id] = record.Fields["EmailDomainName"].String
				}
				if len(domains[i]) != 2 {
					t.Fatalf("org %d: got %d address domains, want two", i, len(domains[i]))
				}
				if err := storage.ApplyEmailServices(&org, services); err != nil {
					t.Fatal(err)
				}
				for id, record := range org.Objects["EmailServicesAddress"].Records {
					if got := record.Fields["EmailDomainName"].String; got != domains[i][id] {
						t.Errorf("org %d: redeploy changed persisted domain", i)
					}
				}
			}
			for id, first := range domains[0] {
				second, ok := domains[1][id]
				if !ok {
					t.Fatalf("same metadata produced different address IDs")
				}
				if first == second {
					t.Errorf("address domain is identical across different org identities")
				}
			}
		})
	}
}
