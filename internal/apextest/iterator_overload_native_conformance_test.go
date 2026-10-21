package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestIteratorOverloadNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "iterator_overload", "CursorSinkTest", []string{"62.0", "67.0"}, 4)
}

func TestIteratorOverloadInlineNativeRejection(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "iterator_overload")
	raw, err := os.Open(filepath.Join(dir, "rejections.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || strings.Join(rows[0], ",") != "source_api,status,diagnostic" {
		t.Fatal("incomplete native rejection rows")
	}
	for _, row := range rows[1:] {
		t.Run(row[0], func(t *testing.T) {
			if row[1] != "COMPILE_ERROR" || !strings.Contains(row[2], "System.ListIterator<Account>") {
				t.Fatal("unexpected native rejection")
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+row[0]+`"}`)
			for _, name := range []string{"InlineCursorReject.cls", "InlineCursorReject.cls-meta.xml"} {
				body, err := os.ReadFile(filepath.Join(dir, "rejected", name))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app", "main", "default", "classes", name), strings.ReplaceAll(string(body), "62.0", row[0]))
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("native source parsed; Glade index diagnostics: %v", index.Diagnostics)
			}
			analysis := sema.Analyze(index)
			rejected := false
			for _, d := range analysis.Diagnostics {
				if d.Code == "GLADESEMA008" && strings.Contains(d.NativeMessage, "System.ListIterator<Account>") {
					rejected = true
				}
			}
			if !rejected {
				t.Fatalf("native rejected the direct ListIterator argument; Glade diagnostics: %v", analysis.Diagnostics)
			}
		})
	}
}
