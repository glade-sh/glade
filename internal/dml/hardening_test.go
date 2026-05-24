package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestNoPanicOnMalformedRecords(t *testing.T) {
	org := storage.NewOrgState()
	engine := NewEngine(&org)
	records := []storage.Record{
		{},
		{Object: "Missing__c"},
		{Object: "Account", ID: "bad-id"},
	}
	assertNoPanic(t, func() {
		_ = engine.Insert(records)
		_ = engine.Update(records)
		_ = engine.Delete(records)
		_ = engine.Upsert(records)
		_ = engine.Undelete(records)
	})
}

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}
