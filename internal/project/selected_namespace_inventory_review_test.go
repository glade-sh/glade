package project

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestNamespaceInventoryUsesSelectedApexReview(t *testing.T) {
	for _, tc := range []struct {
		name, duplicate string
		trigger         bool
		want            []string
	}{
		{"duplicate", "Shared.cls", false, []string{"Second"}},
		{"case folded duplicate", "shared.cls", false, []string{"Second"}},
		{"selected trigger", "Shared.cls", true, []string{"First", "Second"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"a/src","default":true},{"path":"z/src"}]}`)
			writeFile(t, filepath.Join(root, "a", "sfdx-project.json"), `{"namespace":"First"}`)
			writeFile(t, filepath.Join(root, "z", "sfdx-project.json"), `{"namespace":"Second"}`)
			writeFile(t, filepath.Join(root, "a", "src", tc.duplicate), `public class Shared {}`)
			writeFile(t, filepath.Join(root, "z", "src", "Shared.cls"), `public class Shared {}`)
			if tc.trigger {
				writeFile(t, filepath.Join(root, "a", "src", "Other.trigger"), `trigger Other on Account (before insert) {}`)
			}
			p, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if got := p.SourceNamespaces; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("namespaces=%q want %q; selected=%q", got, tc.want, p.SelectedApexFiles())
			}
		})
	}
}
