package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

// Exercise the captured French translation values and existence observations
// using both freshly loaded metadata and its serialized form.
func TestLabelTranslationWhitespaceNative(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/label_translation_space"
	data, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	rows := make([][]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, strings.Split(line, "\t"))
	}
	if len(rows) != 63 {
		t.Fatalf("got %d native observations, want 62", len(rows)-1)
	}
	p, err := project.Load(filepath.Join(fixture, "project"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := resource.LoadProjectWithDependencies(p)
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	var restored storage.MetadataRegistry
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		registry storage.MetadataRegistry
	}{{"source", loaded}, {"restored", restored}} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Metadata = tc.registry
			machine := New(nil)
			machine.SetOrg(&org)
			for _, row := range rows[1:] {
				if len(row) != 5 {
					t.Fatalf("invalid native row: %v", row)
				}
				label := "Probe" + row[1]
				if strings.HasPrefix(row[1], "UnicodeU+") {
					code, err := strconv.ParseUint(strings.TrimPrefix(row[1], "UnicodeU+"), 16, 32)
					if err != nil {
						t.Fatal(err)
					}
					label = fmt.Sprintf("ProbeW%05d", code)
				}
				wantExists, err := strconv.ParseBool(row[3])
				if err != nil {
					t.Fatal(err)
				}
				if got := machine.labelTranslationExists("", label, "fr"); got != wantExists {
					t.Errorf("%s/%s existence=%v, native=%v", row[0], row[1], got, wantExists)
				}
				if row[4] != "-" {
					var want string
					if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
						t.Fatal(err)
					}
					if got, ok := machine.resolveLabelMethodValue("", label, "fr"); !ok || got != want {
						t.Errorf("%s/%s value=%q found=%v, native=%q", row[0], row[1], got, ok, want)
					}
				}
			}
		})
	}
}
