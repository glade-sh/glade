package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestEventSubscriberReloadTransitions(t *testing.T) {
	root := t.TempDir()
	trigger := filepath.Join(root, "SyntheticEventTrigger.trigger")
	org := storage.NewOrgState()
	org.Objects["SyntheticEvent__e"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "SyntheticEvent__e"}}
	s := New(&org)
	index := typesys.Index{Triggers: []typesys.TriggerSymbol{{Name: "SyntheticEventTrigger", ObjectName: "SyntheticEvent__e", File: trigger}}}
	for _, step := range []struct {
		status string
		count  int
	}{{"Active", 1}, {"Inactive", 0}, {"Active", 1}} {
		meta := `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>` + step.status + `</status></ApexTrigger>`
		if err := os.WriteFile(trigger+"-meta.xml", []byte(meta), 0600); err != nil {
			t.Fatal(err)
		}
		s.ReloadProjectState(SourceMetadata{}, index, nil, nil)
		if s.runtimeErr != nil {
			t.Fatal(s.runtimeErr)
		}
		if got := len(org.Objects["EventBusSubscriber"].Records); got != step.count {
			t.Fatalf("status %s: got %d rows, want %d", step.status, got, step.count)
		}
		for _, row := range org.Objects["EventBusSubscriber"].Records {
			if !strings.HasPrefix(row.MetadataSourceUnit, "glade:event-subscriber:01q") {
				t.Fatal("subscriber did not retain the trigger URL identity")
			}
		}
	}
}
