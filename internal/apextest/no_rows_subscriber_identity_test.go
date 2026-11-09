package apextest

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestNoRowsSubscriberTriggerURLIdentity(t *testing.T) {
	const fixture = "testdata/conformance/no_rows_subscriber/project"
	root := t.TempDir()
	if err := filepath.WalkDir(fixture, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, name), string(body))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	org := orgFromIndex(loadTestIndex(t, root))
	var triggerID, subscriberIdentity string
	for _, row := range org.Objects["ApexTrigger"].Records {
		if row.Fields["Name"].String == "SyntheticEventTrigger" {
			triggerID = string(row.ID)
		}
	}
	for _, row := range org.Objects["EventBusSubscriber"].Records {
		if row.Fields["Name"].String == "SyntheticEventTrigger" {
			subscriberIdentity = row.MetadataSourceUnit
		}
	}
	if triggerID == "" || subscriberIdentity != "glade:event-subscriber:"+triggerID {
		t.Fatalf("subscriber URL identity did not match its project trigger")
	}
}
