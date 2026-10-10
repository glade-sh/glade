package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSOSLUnimplementedMatcherStillRejectsWithoutTestResults(t *testing.T) {
	machine := New(nil)
	// R004/R007: a valid FIND does not require an index matcher without hits.
	_, query, err := machine.parseSOSLQuery("FIND 'Alpha?' RETURNING Contact(Id)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.soslRecordMatchesExpression("Contact", storage.Record{}, query.Terms, query.Expression, query.Scope, Null)
	if err == nil || !strings.Contains(err.Error(), "SOSL wildcard index matching") {
		t.Fatalf("actual unsupported index matching accepted: %v", err)
	}
}
