package soql

import (
	"testing"

	"github.com/open-aer/oaer/internal/storage"
)

func TestNoPanicOnMalformedQueries(t *testing.T) {
	queries := []string{
		"",
		"SELECT",
		"SELECT Name FROM",
		"SELECT Name FROM Account WHERE Name LIKE 'A%'",
		"SELECT Name FROM Account ORDER BY",
	}
	org := storage.NewOrgState()
	for _, query := range queries {
		assertNoPanic(t, func() {
			_, _ = Parse(query)
			_, _ = ParseAndExecute(org, query)
		})
	}
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
