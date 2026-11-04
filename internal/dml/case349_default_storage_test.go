package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Reviewed business controls store both phone fields as null for tab/space input.
func TestCase349BusinessPhoneWhitespaceStored(t *testing.T) {
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone"} {
		for _, input := range []string{"   ", "\t  "} {
			t.Run(field+"/"+input, func(t *testing.T) {
				org := testOrg()
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				engine := NewEngine(&org)
				result := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Business"), field: storage.StringValue(input)}}})
				if len(result) != 1 || !result[0].Success {
					t.Fatalf("insert = %#v", result)
				}
				row := org.Objects["Account"].Records[result[0].ID]
				got, exists := row.GetField(field)
				if exists && got.Kind != storage.ValueNull {
					t.Errorf("stored %s = %#v; want null", field, got)
				}
			})
		}
	}
}

func TestCase349BusinessEmailValidation(t *testing.T) {
	for _, tc := range []struct{ value, status string }{
		{"x", "INVALID_EMAIL_ADDRESS"}, {"x@y", "INVALID_EMAIL_ADDRESS"}, {"synthetic@example.invalid", "INVALID_FIELD_FOR_INSERT_UPDATE"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			engine := NewEngine(&org)
			result := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Business"), "PersonEmail": storage.StringValue(tc.value)}}})
			if len(result) != 1 || result[0].Success || result[0].StatusCode != tc.status {
				t.Fatalf("insert = %#v; want %s", result, tc.status)
			}
		})
	}
}
