package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestReceiverSplitNative(t *testing.T) {
	const fixture = "testdata/receiver_split"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join(fixture, "apex", row[0]+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			data, err := os.ReadFile(filepath.Join(fixture, "ReceiverProbe.cls"))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "ReceiverProbe.cls")
			writeSemaFile(t, path, string(data))
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[1], ApexFiles: []string{path}}, schema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			result := AnalyzeAnonymous(index, string(body), row[1])
			if row[3] == "true" {
				if result.HasErrors() {
					t.Fatal(result.Diagnostics)
				}
				return
			}
			for _, d := range result.Diagnostics {
				if strings.Contains(d.NativeMessage, row[4]) || strings.Contains(d.Message, row[4]) {
					return
				}
			}
			t.Fatalf("want %s; got %v", row[4], result.Diagnostics)
		})
	}
}
