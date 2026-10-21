package server

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/storage"
)

func ldsTimezoneOrg(zone, locale string) storage.OrgState {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	users := org.Objects["User"]
	user := users.Records[defaultLocalUserID]
	user.Fields["TimeZoneSidKey"] = storage.StringValue(zone)
	user.Fields["LocaleSidKey"] = storage.StringValue(locale)
	users.Records[user.ID] = user
	org.Objects["User"] = users
	return org
}

func TestLDSDateTimeUsesLoadedLocation(t *testing.T) {
	org := ldsTimezoneOrg("America/Los_Angeles", "en_US")
	zone := ldsRecordLocation(&org)
	if zone == time.UTC || zone.String() != "America/Los_Angeles" {
		t.Fatalf("LDS fell back to %v", zone)
	}
	// Existing L09 API59 ctrl_querySeed_original paired timestamp/display.
	// API68 ctrl_querySeed_original records the same zone and date layout.
	stamp := "2026-10-12T02:56:03.000Z"
	const display = "10/11/2026, 7:56 PM"
	account := org.Objects["Account"]
	account.Definition.Fields["ParentId"] = storage.Field{APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent"}
	account.Records["001XX0000000001"] = storage.Record{
		ID: "001XX0000000001", Object: "Account",
		System: storage.SystemFields{CreatedDate: stamp, LastModifiedDate: stamp},
		Fields: map[string]storage.Value{"ParentId": storage.IDValue("001XX0000000002")},
	}
	account.Records["001XX0000000002"] = storage.Record{
		ID: "001XX0000000002", Object: "Account",
		System: storage.SystemFields{CreatedDate: stamp, LastModifiedDate: stamp},
		Fields: map[string]storage.Value{"Name": storage.StringValue("Owned Parent")},
	}
	org.Objects["Account"] = account
	fields := []string{"Account.CreatedDate", "Account.LastModifiedDate", "Account.Parent.CreatedDate"}
	data, wireErr := getRecordWireData(&org, "001XX0000000001", fields, nil)
	if wireErr != nil {
		t.Fatal(wireErr)
	}
	check := func(record map[string]any, name string) {
		t.Helper()
		payload := record["fields"].(map[string]any)[name].(map[string]any)
		if payload["value"] != stamp || payload["displayValue"] != display {
			t.Fatalf("%s payload = %#v", name, payload)
		}
	}
	check(data, "CreatedDate")
	check(data, "LastModifiedDate")
	parent := data["fields"].(map[string]any)["Parent"].(map[string]any)["value"].(map[string]any)
	check(parent, "CreatedDate")
	batch := getRecordsWireData(&org, lwcbrowser.WireGetRecordsRequest{Records: []lwcbrowser.WireGetRecordsItemRequest{{RecordIDs: []string{"001XX0000000001", "001XX0000000002"}, Fields: []string{"Account.CreatedDate"}}}})
	results := batch["results"].([]map[string]any)
	if len(results) != 2 {
		t.Fatalf("batch returned %d records; want 2", len(results))
	}
	for _, result := range results {
		if result["statusCode"] != 200 {
			t.Fatalf("batch result = %#v", result)
		}
		check(result["result"].(map[string]any), "CreatedDate")
	}
	if got := (&Server{Org: &org}).listDateTimeDisplay(stamp); got != display {
		t.Fatalf("list display = %v; want %s", got, display)
	}
}

// These preserve existing local fallbacks; they do not establish native locale
// or invalid-zone semantics for either transport.
func TestLDSDateTimePreservesFallbackAndFieldBoundaries(t *testing.T) {
	for _, name := range []string{"", "UTC", "Invalid/OwnedZone"} {
		org := ldsTimezoneOrg(name, "en_US")
		if zone := ldsRecordLocation(&org); zone != time.UTC {
			t.Fatalf("zone %q resolved to %v; want UTC", name, zone)
		}
	}
	for _, tc := range []struct {
		field   storage.Field
		value   any
		display any
	}{
		{storage.Field{Type: storage.FieldDateTime}, nil, nil},
		{storage.Field{Type: storage.FieldDateTime}, "invalid timestamp", "invalid timestamp"},
		{storage.Field{Type: storage.FieldString}, "2026-10-12T02:56:03.000Z", nil},
		{storage.Field{Type: storage.FieldDate}, "2026-10-12", "2026-10-12"},
		{storage.Field{Type: storage.FieldDecimal, DisplayType: "CURRENCY", Scale: 2}, "12.5", "$12.50"},
	} {
		payload := ldsRecordFieldPayload(tc.field, tc.value, time.UTC)
		if payload["value"] != tc.value || payload["displayValue"] != tc.display {
			t.Fatalf("field %#v payload = %#v", tc.field, payload)
		}
	}
	for _, tc := range []struct{ zone, locale string }{
		{"America/Los_Angeles", "fr_FR"},
		{"Invalid/OwnedZone", "en_US"},
		{"", "en_US"},
	} {
		org := ldsTimezoneOrg(tc.zone, tc.locale)
		if got := (&Server{Org: &org}).listDateTimeDisplay("2026-10-12T02:56:03.000Z"); got != nil {
			t.Fatalf("list display for %+v = %v; want nil", tc, got)
		}
		want := "10/12/2026, 2:56 AM"
		if tc.zone == "America/Los_Angeles" {
			want = "10/11/2026, 7:56 PM"
		}
		payload := ldsRecordFieldPayload(storage.Field{Type: storage.FieldDateTime}, "2026-10-12T02:56:03.000Z", ldsRecordLocation(&org))
		if payload["displayValue"] != want {
			t.Fatalf("LDS display for %+v = %v; want %s", tc, payload["displayValue"], want)
		}
	}
}
