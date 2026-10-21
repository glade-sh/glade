package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native page tabs expose the page's 15-character identity; web tabs expose
// the all-zero sentinel. Unobserved non-object kinds retain their old path.
func TestDescribeTabSObjectNameForPageAndWebTabs(t *testing.T) {
	for _, tc := range []struct {
		name string
		tab  storage.TabMetadata
		want Value
	}{
		{"page", storage.TabMetadata{Name: "PanelView", Page: "PanelView"}, String("066000000000001")},
		{"page with long identity", storage.TabMetadata{Name: "PanelLong", Page: "PanelLong"}, String("066000000000002")},
		{"web", storage.TabMetadata{Name: "PanelLink", URL: "https://example.invalid"}, String("000000000000000")},
		{"standard object", storage.TabMetadata{Name: "Account", SObjectName: "Account"}, String("Account")},
		{"custom object", storage.TabMetadata{Name: "Panel__c", Custom: true}, String("Panel__c")},
		{"unknown non-object", storage.TabMetadata{Name: "Utility"}, Null},
		{"component", storage.TabMetadata{Name: "PanelComponent", NavigationIdentity: "component-id"}, Null},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			org := storage.NewOrgState()
			org.Objects["ApexPage"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
				"066000000000001":    {ID: "066000000000001", Fields: map[string]storage.Value{"Name": storage.StringValue("PanelView")}},
				"066000000000002AAA": {ID: "066000000000002AAA", Fields: map[string]storage.Value{"Name": storage.StringValue("PanelLong")}},
			}}
			machine.SetOrg(&org)
			got := machine.describeTabValue(tc.tab).Fields["sObjectName"]
			if got.Kind != tc.want.Kind || got.Text != tc.want.Text {
				t.Fatalf("getSObjectName backing value = %#v, want %#v", got, tc.want)
			}
		})
	}
}
