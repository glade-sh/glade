package dml

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestStandardEmailMessageInputNativeConstructorSetterAndNullDMLRoutes(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/standard_emailmessage_input_fields/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Case, Route, Message string
		Compiled, Success         bool
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 30 {
		t.Fatalf("native matrix=%d, want 30", len(rows))
	}
	accepted, carries := 0, 0
	for _, row := range rows {
		if !row.Compiled {
			continue
		}
		if !row.Success {
			if row.API != "67.0" || !strings.Contains(row.Message, "FIELD_INTEGRITY_EXCEPTION") {
				t.Fatal("unexpected native DML carry")
			}
			carries++
			continue
		}
		accepted++
		nullRoutes := []string{"fields-map"}
		if row.Case == "toids-null-insert" {
			nullRoutes = append(nullRoutes, "explicit-null-map")
		}
		for _, nullRoute := range nullRoutes {
			t.Run(row.API+"/"+row.Route+"/"+row.Case+"/"+nullRoute, func(t *testing.T) {
				org := storage.NewOrgState()
				storage.EnsureDeterministicPlatformData(&org)
				storage.EnsureStandardObject(&org, "EmailMessage")
				fields := map[string]storage.Value{"Subject": storage.StringValue("owned"), "FromAddress": storage.StringValue("sender@example.invalid"), "ToAddress": storage.StringValue("system@example.invalid"), "Incoming": storage.BooleanValue(true), "Status": storage.StringValue("3"), "IsClientManaged": storage.BooleanValue(true), "MessageDate": storage.DateTimeValue("2026-10-22T12:00:00Z")}
				record := storage.Record{Object: "EmailMessage", Fields: fields}
				if strings.HasPrefix(row.Case, "toids-") {
					if row.Case == "toids-null-insert" {
						if nullRoute == "explicit-null-map" {
							record.ExplicitNulls = map[string]bool{"ToIds": true}
						} else {
							record.Fields["ToIds"] = storage.NullValue()
						}
					} else {
						record.Fields["ToIds"] = storage.ListValue(storage.StringValue(string(storage.DefaultUserID)))
					}
				}
				engine := NewEngine(&org)
				result := engine.Insert([]storage.Record{record})
				if len(result) != 1 || !result[0].Success {
					t.Fatalf("native input accepted; Glade %+v", result)
				}
			})
		}
	}
	if accepted != 20 || carries != 4 {
		t.Fatalf("native accepted=%d carries=%d", accepted, carries)
	}
}

// Upsert-create shares insert validation; update and matched upsert retain the
// unprobed update surface. These are engine routing controls, not native rows.
func TestStandardEmailMessageInputInsertUpsertUpdateRouteTwins(t *testing.T) {
	for _, route := range []string{"insert", "upsert-create", "update", "upsert-match"} {
		for _, nullRoute := range []string{"fields-map", "explicit-null-map"} {
			t.Run(route+"/"+nullRoute, func(t *testing.T) {
				org := storage.NewOrgState()
				storage.EnsureDeterministicPlatformData(&org)
				storage.EnsureStandardObject(&org, "EmailMessage")
				engine := NewEngine(&org)
				record := storage.Record{Object: "EmailMessage", Fields: map[string]storage.Value{"Subject": storage.StringValue("owned")}}
				if route == "update" || route == "upsert-match" {
					seeded := engine.Insert([]storage.Record{record})
					if len(seeded) != 1 || !seeded[0].Success {
						t.Fatalf("seed: %+v", seeded)
					}
					record.ID = seeded[0].ID
				}
				if nullRoute == "fields-map" {
					record.Fields["ToIds"] = storage.NullValue()
				} else {
					record.ExplicitNulls = map[string]bool{"ToIds": true}
				}
				var results []Result
				switch route {
				case "insert":
					results = engine.Insert([]storage.Record{record})
				case "update":
					results = engine.Update([]storage.Record{record})
				default:
					results = engine.Upsert([]storage.Record{record})
				}
				wantSuccess := route == "insert" || route == "upsert-create"
				if len(results) != 1 || results[0].Success != wantSuccess {
					t.Fatalf("route success want %v: %+v", wantSuccess, results)
				}
				if !wantSuccess && !strings.Contains(results[0].Error, "unknown field EmailMessage.ToIds") {
					t.Fatalf("update boundary: %+v", results)
				}
			})
		}
	}
}
