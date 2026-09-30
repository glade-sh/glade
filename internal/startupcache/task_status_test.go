package startupcache

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestTaskStatusMetadataSurvivesStartupCache(t *testing.T) {
	for _, tc := range []struct {
		subdir            string
		version, previous int
	}{{SubdirTest, Version, 7}} {
		t.Run(tc.subdir, func(t *testing.T) {
			root := t.TempDir()
			writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[],"sourceApiVersion":"53.0"}`)
			input, err := ValidateInputWithSourceDigests(root, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			entry, err := NewEntryWithValidatedInput(input, org, CompiledRuntime{})
			if err != nil {
				t.Fatal(err)
			}
			entry.Version = tc.version
			entry.RuntimeABI, entry.RuntimeKey = "task-status-test", "task-status"
			if err := Write(&entry, tc.subdir); err != nil {
				t.Fatal(err)
			}
			restored, err := ReadFreshRuntimeWithValidatedInput(root, tc.subdir, tc.version, entry.RuntimeABI, entry.RuntimeKey, input)
			if err != nil || restored == nil {
				t.Fatalf("restore=%#v err=%v", restored, err)
			}
			rows := restored.Org.Objects["TaskStatus"].Records
			if len(rows) != 5 {
				t.Fatalf("TaskStatus rows=%d, want 5", len(rows))
			}
			closed := 0
			for _, row := range rows {
				if row.Fields["IsClosed"].Boolean {
					closed++
				}
			}
			if closed != 1 {
				t.Fatalf("closed statuses=%d, want 1", closed)
			}
			delete(entry.Org.Objects, "TaskStatus")
			entry.Version = tc.previous
			if err := Write(&entry, tc.subdir); err != nil {
				t.Fatal(err)
			}
			restored, err = ReadFreshRuntimeWithValidatedInput(root, tc.subdir, tc.version, entry.RuntimeABI, entry.RuntimeKey, input)
			if err != nil || restored != nil {
				t.Fatalf("old TaskStatus-free snapshot accepted=%#v err=%v", restored, err)
			}
		})
	}
}
