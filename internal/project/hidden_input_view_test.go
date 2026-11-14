package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/inputview"
)

func TestHiddenMetadataMembershipIsObserved(t *testing.T) {
	cases := []struct {
		name, relative, body string
		scan                 func(*inputview.View, string) []string
	}{
		{"record type", "objects/Account/recordTypes/.Person.recordType-meta.xml", `<RecordType><fullName>PersonAccount</fullName></RecordType>`, func(v *inputview.View, root string) []string {
			return personAccountRecordTypesIn(v, filepath.Join(root, "objects"))
		}},
		{"settings", "settings/.Feature.settings-meta.xml", `<Settings/>`, func(v *inputview.View, root string) []string {
			return sourceSettingsFilesIn(v, filepath.Join(root, "settings"))
		}},
		{"scratch definition", "config/.extra-scratch-def.json", `{}`, func(v *inputview.View, root string) []string { return extraScratchDefinitionFiles(v, root) }},
		{"hidden folder", ".package/settings/.Feature.settings-meta.xml", `<Settings/>`, func(v *inputview.View, root string) []string { return sourceSettingsFiles(v, root) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, tc.relative)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tc.body), 0644); err != nil {
				t.Fatal(err)
			}
			v := inputview.New()
			if files := tc.scan(v, root); len(files) != 1 || files[0] != path {
				t.Fatalf("hidden file not consumed: %v", files)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := v.Recheck(); err == nil {
				t.Fatal("hidden file removal was not observed")
			}
			v = inputview.New()
			if files := tc.scan(v, root); len(files) != 0 {
				t.Fatalf("unexpected files: %v", files)
			}
			if err := os.WriteFile(path, []byte(tc.body), 0644); err != nil {
				t.Fatal(err)
			}
			if err := v.Recheck(); err == nil {
				t.Fatal("hidden file addition was not observed")
			}
		})
	}
}
