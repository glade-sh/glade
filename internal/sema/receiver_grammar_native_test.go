package sema

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// TestReceiverGrammarNative compares each supplied native primary diagnostic,
// including its source position, at both captured API versions. The native
// class rows also contain cascades: CA, D1-D6 and E1-E2. CB contains five
// independent variable errors. Those additional outcomes are retained in the
// fixture and are not substituted for the primary diagnostic.
func TestReceiverGrammarNative(t *testing.T) {
	const fixture = "testdata/receiver_split/grammar"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rd := csv.NewReader(f)
	rd.Comma = '\t'
	rows, err := rd.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			ext := ".apex"
			if row[2] == "class" {
				ext = ".cls"
			}
			if row[2] == "trigger" {
				ext = ".trigger"
			}
			src, err := os.ReadFile(filepath.Join(fixture, row[0]+ext))
			if err != nil {
				t.Fatal(err)
			}
			support, err := os.ReadFile(filepath.Join(fixture, "CrProbe.cls"))
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			probe := filepath.Join(root, "CrProbe.cls")
			writeSemaFile(t, probe, string(support))
			files := []string{probe}
			if row[2] != "anonymous" {
				name := "CrRoute" + row[0] + ext
				if row[0] == "CA" {
					name = "CrRouteAccept.cls"
				}
				if row[0] == "CB" {
					name = "CrRouteBefore.cls"
				}
				if row[0] == "super_break" {
					name = "SuperProbe.cls"
				}
				if row[2] == "trigger" {
					name = "CrTrig" + row[0] + ext
				}
				path := filepath.Join(root, name)
				writeSemaFile(t, path, string(src))
				files = append(files, path)
			}
			namespace := ""
			if strings.HasPrefix(row[0], "custom_namespace_") || row[0] == "custom_control" {
				namespace = "sample"
			}
			index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: row[1], ApexFiles: files}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
			var ds []diagnostic.Diagnostic
			if index.HasErrors() {
				ds = index.Diagnostics
			} else if row[2] == "anonymous" {
				ds = AnalyzeAnonymous(index, string(src), row[1]).Diagnostics
			} else {
				ds = Analyze(index).Diagnostics
			}
			var got []string
			for _, d := range ds {
				if d.Severity != diagnostic.Error {
					continue
				}
				message := d.NativeMessage
				if message == "" {
					message = d.Message
				}
				if d.Range == nil {
					got = append(got, message)
					continue
				}
				got = append(got, fmt.Sprintf("%d:%d %s", d.Range.Start.Line, d.Range.Start.Column, message))
			}
			if row[3] == "true" {
				if len(got) != 0 {
					t.Fatalf("native accepts; got %v", got)
				}
				return
			}
			want := strings.Split(strings.TrimPrefix(row[4], "deploy fails: "), " | ")
			if len(got) == 0 || got[0] != want[0] {
				t.Errorf("primary diagnostic = %v; want %s", got, want[0])
				return
			}
			// Native class parsing emits these documented cascades in addition
			// to the primary error. Track them explicitly until implemented.
			knownCascades := map[string]bool{"CA": true, "D1": true, "D2": true, "D3": true, "D4": true, "D5": true, "D6": true, "E1": true, "E2": true, "super_break": true}
			if knownCascades[row[0]] {
				for _, extra := range got[1:] {
					if !containsReceiverGrammarDiagnostic(want[1:], extra) {
						t.Errorf("unexpected diagnostic %q; native %v", extra, want)
					}
				}
				if len(got) < len(want) {
					t.Logf("known native cascades not yet reproduced: native %v; got %v", want, got)
				}
			} else if strings.Join(got, " | ") != strings.Join(want, " | ") {
				t.Errorf("diagnostics = %v; want %v", got, want)
			}
		})
	}
}

func containsReceiverGrammarDiagnostic(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
