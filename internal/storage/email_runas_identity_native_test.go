package storage_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// runas_user/org.tsv:110, :122, and :134 record unchanged domains while the
// configured user changes from administrator to standard and back. Adjacent
// rows repeat both APIs and anonymous and named-test routes.
func TestEmailServicesRunAsChangeKeepsDomain(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = api
			storage.EnsureDeterministicPlatformData(&org)
			users := org.Objects["User"]
			alternateID := storage.ID("local-user-alternate")
			alternate := users.Records[storage.ID(storage.DefaultUserID)].Clone()
			alternate.ID = alternateID
			alternate.Fields["Id"] = storage.IDValue(alternateID)
			alternate.Fields["Username"] = storage.StringValue("alternate@example.invalid")
			users.Records[alternateID] = alternate
			org.Objects["User"] = users
			services := []project.EmailService{{HandlerDeclared: true, ApexClass: "SyntheticHandler", FunctionName: "SyntheticFunction", Addresses: []project.EmailServiceAddress{
				{DeveloperName: "First", LocalPart: "first", RunAsUser: "system@example.invalid"},
				{DeveloperName: "Second", LocalPart: "second", RunAsUser: "system@example.invalid"},
			}}}
			var initial map[storage.ID]string
			for _, step := range []struct {
				username string
				userID   storage.ID
			}{{"system@example.invalid", storage.ID(storage.DefaultUserID)}, {"alternate@example.invalid", alternateID}, {"system@example.invalid", storage.ID(storage.DefaultUserID)}} {
				for i := range services[0].Addresses {
					services[0].Addresses[i].RunAsUser = step.username
				}
				if err := storage.ApplyEmailServices(&org, services); err != nil {
					t.Fatal(err)
				}
				if initial == nil {
					initial = map[storage.ID]string{}
					for id, record := range org.Objects["EmailServicesAddress"].Records {
						initial[id] = record.Fields["EmailDomainName"].String
					}
					if len(initial) != 2 {
						t.Fatalf("got %d addresses, want two", len(initial))
					}
				}
				for id, record := range org.Objects["EmailServicesAddress"].Records {
					if record.Fields["EmailDomainName"].String != initial[id] {
						t.Error("changing configured user changed an existing domain")
					}
					if !storage.IDsEqual(record.Fields["RunAsUserId"].ID, step.userID) {
						t.Error("configured user relationship did not update")
					}
				}
			}
		})
	}
}
