package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/storage"
)

func recordTimeRouteFixture(zone, locale string) (*Server, storage.ObjectState) {
	org := ldsTimezoneOrg(zone, locale)
	accounts := org.Objects["Account"]
	parentID := storage.ID("001XX0000000001")
	accounts.Records[parentID] = storage.Record{ID: parentID, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Parent")}}
	org.Objects["Account"] = accounts
	contacts := org.Objects["Contact"]
	contacts.Records = map[storage.ID]storage.Record{}
	for i := 1; i <= 2; i++ {
		id := storage.ID(fmt.Sprintf("003XX000000000%d", i))
		contacts.Records[id] = storage.Record{ID: id, Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Owned"), "AccountId": storage.IDValue(parentID)}, System: storage.SystemFields{CreatedDate: "2026-10-12T02:56:03.000Z", LastModifiedDate: "2026-01-15T02:56:03.000Z"}}
	}
	org.Objects["Contact"] = contacts
	return New(&org), contacts
}

func TestRecordTimeListAndRelatedRoutes(t *testing.T) {
	for _, tc := range []struct {
		zone, locale      string
		created, modified any
	}{
		{"America/Los_Angeles", "en_US", "10/11/2026, 7:56 PM", "1/14/2026, 6:56 PM"},
		{"Asia/Kolkata", "en_US", "10/12/2026, 8:26 AM", "1/15/2026, 8:26 AM"},
		{"Invalid/OwnedZone", "en_US", nil, nil},
		{"America/Los_Angeles", "fr_FR", nil, nil},
	} {
		t.Run(tc.zone+"/"+tc.locale, func(t *testing.T) {
			s, contacts := recordTimeRouteFixture(tc.zone, tc.locale)
			config := map[string]any{"fields": []string{"Contact.CreatedDate", "Contact.LastModifiedDate"}, "parentRecordId": "001XX0000000001", "relatedListId": "Contacts"}
			for _, route := range []struct {
				name string
				call func() (map[string]any, error)
			}{
				{"list", func() (map[string]any, error) {
					data, err := s.listRecordsData(listViewMetadata{}, contacts, config)
					if err != nil {
						return nil, fmt.Errorf("%v", err)
					}
					return data, nil
				}},
				{"related", func() (map[string]any, error) {
					data, err := s.relatedListRecordsData(config)
					if err != nil {
						return nil, fmt.Errorf("%v", err)
					}
					return data, nil
				}},
			} {
				t.Run(route.name, func(t *testing.T) {
					data, err := route.call()
					if err != nil {
						t.Fatal(err)
					}
					rows := data["records"].([]any)
					if len(rows) != 2 {
						t.Fatalf("records = %d", len(rows))
					}
					for _, row := range rows {
						fields := row.(map[string]any)["fields"].(map[string]any)
						for name, want := range map[string]any{"CreatedDate": tc.created, "LastModifiedDate": tc.modified} {
							if got := fields[name].(map[string]any)["displayValue"]; got != want {
								t.Fatalf("%s display = %v; want %v", name, got, want)
							}
						}
					}
				})
			}
		})
	}
}

// Loading a named zone for every field costs several zone loads per page.
// Compare against UTC, whose lookup is allocation-free, and scale the budget
// by the host's own named-zone lookup cost instead of a fixed Go version cost.
func TestRecordTimeLocationResolutionPerPage(t *testing.T) {
	for _, zone := range []string{"America/Los_Angeles", "Asia/Kolkata"} {
		t.Run(zone, func(t *testing.T) {
			loadCost := testing.AllocsPerRun(10, func() {
				if _, err := time.LoadLocation(zone); err != nil {
					t.Fatal(err)
				}
			})
			for _, related := range []bool{false, true} {
				measure := func(name string) float64 {
					s, contacts := recordTimeRouteFixture(name, "en_US")
					config := map[string]any{"fields": []string{"Contact.CreatedDate", "Contact.LastModifiedDate"}, "parentRecordId": "001XX0000000001", "relatedListId": "Contacts"}
					return testing.AllocsPerRun(10, func() {
						var err *lwcbrowser.WireError
						if related {
							_, err = s.relatedListRecordsData(config)
						} else {
							_, err = s.listRecordsData(listViewMetadata{}, contacts, config)
						}
						if err != nil {
							t.Fatal(err)
						}
					})
				}
				utc, named := measure("UTC"), measure(zone)
				if extra, limit := named-utc, 3*loadCost+5; extra > limit {
					t.Errorf("related=%v: named-zone page adds %.0f allocations; want <= %.0f (one load costs %.0f)", related, extra, limit, loadCost)
				}
			}
		})
	}
}
