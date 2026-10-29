package dml

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestEmailRecipientIDsWithoutAddressEngineNativeConformance(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/email_relation_runtime_boundaries/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		// This is a direct engine test of the accepted API 62 relation rule.
		// Real anonymous and named API 62/67 routes, including API 67 rejection,
		// are covered by apextest.TestEmailRecipientInsertNativeConformance.
		if row[0] != "62.0" || row[1] != "isTest" {
			continue
		}
		var native map[string]struct {
			Automatic, AfterExplicit int
			ExplicitSuccess          bool
		}
		if err := json.Unmarshal([]byte(row[2]), &native); err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"outgoing", "incoming", "clientManaged"} {
			t.Run(row[0]+"/engine/"+kind, func(t *testing.T) {
				want, ok := native[kind]
				if !ok || want.Automatic != 0 || want.AfterExplicit != 1 || !want.ExplicitSuccess {
					t.Fatal("missing accepted native recipient boundary")
				}
				org := storage.NewOrgState()
				org.APIVersion = row[0]
				storage.EnsureDeterministicPlatformData(&org)
				storage.EnsureStandardObject(&org, "EmailMessage")
				storage.EnsureStandardObject(&org, "EmailMessageRelation")
				engine := NewEngine(&org)
				contacts := engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{
					"LastName": storage.StringValue("Boundary " + kind),
					"Email":    storage.StringValue(kind + "@example.invalid"),
				}}})
				if len(contacts) != 1 || !contacts[0].Success {
					t.Fatalf("recipient insert: %+v", contacts)
				}
				recipientID := contacts[0].ID
				fields := map[string]storage.Value{
					"Subject":     storage.StringValue("Recipient boundary"),
					"FromAddress": storage.StringValue("author@example.invalid"),
					"Status":      storage.StringValue("3"),
					"ToIds":       storage.ListValue(storage.IDValue(recipientID)),
				}
				if kind != "outgoing" {
					fields["Incoming"] = storage.BooleanValue(true)
				}
				if kind == "clientManaged" {
					fields["IsClientManaged"] = storage.BooleanValue(true)
				}
				inserted := engine.Insert([]storage.Record{{Object: "EmailMessage", Fields: fields}})
				if len(inserted) != 1 || !inserted[0].Success {
					t.Fatalf("message insert: %+v", inserted)
				}
				if got := len(org.Objects["EmailMessageRelation"].Records); got != want.Automatic {
					t.Errorf("automatic relations=%d; native=%d", got, want.Automatic)
				}
				explicit := engine.Insert([]storage.Record{{Object: "EmailMessageRelation", Fields: map[string]storage.Value{
					"EmailMessageId": storage.IDValue(inserted[0].ID),
					"RelationId":     storage.IDValue(recipientID),
					"RelationType":   storage.StringValue("ToAddress"),
				}}})
				if len(explicit) != 1 || explicit[0].Success != want.ExplicitSuccess {
					t.Fatalf("explicit insert: %+v", explicit)
				}
				if got := len(org.Objects["EmailMessageRelation"].Records); got != want.AfterExplicit {
					t.Errorf("relations after explicit insert=%d; native=%d", got, want.AfterExplicit)
				}
			})
		}
	}
}
