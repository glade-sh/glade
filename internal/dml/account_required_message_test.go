package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestInsertAccountMissingNameUsesApexRequiredMessage(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	engine := NewEngine(&org)

	result := engine.Insert([]storage.Record{{Object: "Account"}})
	if len(result) != 1 || result[0].Success {
		t.Fatalf("insert result = %#v", result)
	}
	if result[0].Error != "Required fields are missing: [Name]" {
		t.Fatalf("insert error = %q", result[0].Error)
	}
}
