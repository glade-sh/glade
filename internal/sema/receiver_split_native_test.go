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

// The supplied anonymous native rows reject a class name followed by a
// newline and a static call at both API 62.0 and 67.0. This uses a synthetic
// equivalent because the original capture does not include its source file.
func TestReceiverSplitSuppliedShape(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "ReceiverProbe.cls")
			writeSemaFile(t, path, "public class ReceiverProbe { public static void value() {} }")
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
			result := AnalyzeAnonymous(index, "ReceiverProbe\n.value();", api)
			for _, d := range result.Diagnostics {
				if d.NativeMessage == "Variable does not exist: ReceiverProbe" {
					return
				}
			}
			t.Fatalf("expected receiver rejection, got %v", result.Diagnostics)
		})
	}
}
