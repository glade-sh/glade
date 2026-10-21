package apextest

import (
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceTriggerCatalog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "OriginWitness.trigger")
	source := "trigger OriginWitness on Contact (before insert) {}"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	index := typesys.Index{Triggers: []typesys.TriggerSymbol{{Name: "OriginWitness", File: path, ObjectName: "Contact", EffectiveAPIVersion: "62.0"}}}
	org := storage.NewOrgState()
	applyApexTriggerRecords(&org, index)
	applyApexTriggerRecords(&org, index)
	rows := org.Objects["ApexTrigger"].Records
	if len(rows) != 1 {
		t.Fatalf("catalog rows=%d", len(rows))
	}
	for _, row := range rows {
		if row.Fields["Name"].String != "OriginWitness" || row.Fields["Body"].String != source {
			t.Fatalf("catalog row=%v", row)
		}
	}
}
