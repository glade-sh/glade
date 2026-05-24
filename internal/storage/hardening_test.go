package storage

import (
	"strings"
	"testing"
)

func TestNoPanicOnMalformedFixtures(t *testing.T) {
	inputs := []string{
		"",
		"{",
		`{"version":"wrong"}`,
		`{"version":"glade.storage.v1","objects":[{"name":"Account","records":[{"id":"bad-id"}]}]}`,
	}
	for _, input := range inputs {
		assertNoPanic(t, func() {
			fixture, _ := ReadFixture(strings.NewReader(input))
			org := NewOrgState()
			_ = ApplyFixture(&org, fixture)
			_ = FixtureFromOrg(org)
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
