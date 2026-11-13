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

// NATIVE static_same testdata/receiver_split/org.tsv:2 anonymous 62.0
// NATIVE static_same testdata/receiver_split/org.tsv:3 anonymous 67.0
// NATIVE static_comment testdata/receiver_split/org.tsv:4 anonymous 62.0
// NATIVE static_comment testdata/receiver_split/org.tsv:5 anonymous 67.0
// NATIVE static_inline_comment testdata/receiver_split/org.tsv:6 anonymous 62.0
// NATIVE static_inline_comment testdata/receiver_split/org.tsv:7 anonymous 67.0
// NATIVE static_blank testdata/receiver_split/org.tsv:8 anonymous 62.0
// NATIVE static_blank testdata/receiver_split/org.tsv:9 anonymous 67.0
// NATIVE instance_split testdata/receiver_split/org.tsv:10 anonymous 62.0
// NATIVE instance_split testdata/receiver_split/org.tsv:11 anonymous 67.0
// NATIVE chain_split testdata/receiver_split/org.tsv:12 anonymous 62.0
// NATIVE chain_split testdata/receiver_split/org.tsv:13 anonymous 67.0
// NATIVE qualified_split testdata/receiver_split/org.tsv:14 anonymous 62.0
// NATIVE qualified_split testdata/receiver_split/org.tsv:15 anonymous 67.0
// NATIVE qualified_same testdata/receiver_split/org.tsv:16 anonymous 62.0
// NATIVE qualified_same testdata/receiver_split/org.tsv:17 anonymous 67.0
// NATIVE static_dot_before testdata/receiver_split/org.tsv:18 anonymous 62.0
// NATIVE static_dot_before testdata/receiver_split/org.tsv:19 anonymous 67.0
// NATIVE dot_statement testdata/receiver_split/org.tsv:20 anonymous 62.0
// NATIVE dot_statement testdata/receiver_split/org.tsv:21 anonymous 67.0
// NATIVE dot_assign testdata/receiver_split/org.tsv:22 anonymous 62.0
// NATIVE dot_assign testdata/receiver_split/org.tsv:23 anonymous 67.0
// NATIVE dot_instance testdata/receiver_split/org.tsv:24 anonymous 62.0
// NATIVE dot_instance testdata/receiver_split/org.tsv:25 anonymous 67.0
// NATIVE static_space testdata/receiver_split/org.tsv:26 anonymous 62.0
// NATIVE static_space testdata/receiver_split/org.tsv:27 anonymous 67.0
// NATIVE split_statement testdata/receiver_split/org.tsv:28 anonymous 62.0
// NATIVE split_statement testdata/receiver_split/org.tsv:29 anonymous 67.0
// NATIVE repeat_first testdata/receiver_split/org.tsv:30 anonymous 62.0
// NATIVE repeat_first testdata/receiver_split/org.tsv:31 anonymous 67.0
// NATIVE repeat_second testdata/receiver_split/org.tsv:32 anonymous 62.0
// NATIVE repeat_second testdata/receiver_split/org.tsv:33 anonymous 67.0
// NATIVE repeat_both testdata/receiver_split/org.tsv:34 anonymous 62.0
// NATIVE repeat_both testdata/receiver_split/org.tsv:35 anonymous 67.0
// NATIVE repeat_same testdata/receiver_split/org.tsv:36 anonymous 62.0
// NATIVE repeat_same testdata/receiver_split/org.tsv:37 anonymous 67.0
// NATIVE comment_after testdata/receiver_split/org.tsv:38 anonymous 62.0
// NATIVE comment_after testdata/receiver_split/org.tsv:39 anonymous 67.0
// NATIVE comment_before_newline testdata/receiver_split/org.tsv:40 anonymous 62.0
// NATIVE comment_before_newline testdata/receiver_split/org.tsv:41 anonymous 67.0
// NATIVE chain_after testdata/receiver_split/org.tsv:42 anonymous 62.0
// NATIVE chain_after testdata/receiver_split/org.tsv:43 anonymous 67.0
// NATIVE chain_comment testdata/receiver_split/org.tsv:44 anonymous 62.0
// NATIVE chain_comment testdata/receiver_split/org.tsv:45 anonymous 67.0
// NATIVE after_first testdata/receiver_split/org.tsv:46 anonymous 62.0
// NATIVE after_first testdata/receiver_split/org.tsv:47 anonymous 67.0
// NATIVE after_second testdata/receiver_split/org.tsv:48 anonymous 62.0
// NATIVE after_second testdata/receiver_split/org.tsv:49 anonymous 67.0
// NATIVE after_both testdata/receiver_split/org.tsv:50 anonymous 62.0
// NATIVE after_both testdata/receiver_split/org.tsv:51 anonymous 67.0
// NATIVE nested testdata/receiver_split/org.tsv:52 anonymous 62.0
// NATIVE nested testdata/receiver_split/org.tsv:53 anonymous 67.0
// NATIVE instance_arg testdata/receiver_split/org.tsv:54 anonymous 62.0
// NATIVE instance_arg testdata/receiver_split/org.tsv:55 anonymous 67.0
// NATIVE instance_assign testdata/receiver_split/org.tsv:56 anonymous 62.0
// NATIVE instance_assign testdata/receiver_split/org.tsv:57 anonymous 67.0
// NATIVE comment_statement testdata/receiver_split/org.tsv:58 anonymous 62.0
// NATIVE comment_statement testdata/receiver_split/org.tsv:59 anonymous 67.0
// NATIVE comment_assignment testdata/receiver_split/org.tsv:60 anonymous 62.0
// NATIVE comment_assignment testdata/receiver_split/org.tsv:61 anonymous 67.0
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
			ext := ".apex"
			if row[2] == "class" {
				ext = ".cls"
			}
			body, err := os.ReadFile(filepath.Join(fixture, "apex", row[0]+ext))
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
			paths := []string{path}
			if row[2] == "class" {
				classPath := filepath.Join(root, "ReceiverRouteProbe.cls")
				writeSemaFile(t, classPath, string(body))
				paths = append(paths, classPath)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[1], ApexFiles: paths}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			result := Analyze(index)
			if row[2] == "anonymous" {
				result = AnalyzeAnonymous(index, string(body), row[1])
			}
			// Non-call member access is a captured boundary, outside this call-only change.
			if row[0] == "sobject_member_before_dot" {
				if result.HasErrors() {
					t.Fatal(result.Diagnostics)
				}
				return
			}
			if row[3] == "true" {
				if result.HasErrors() {
					t.Fatal(result.Diagnostics)
				}
				return
			}
			matches := 0
			want := strings.Split(row[4], " | ")[0]
			for _, d := range result.Diagnostics {
				if strings.Contains(d.NativeMessage, want) || strings.Contains(d.Message, want) {
					matches++
				}
			}
			if matches != 1 {
				t.Fatalf("want one %s diagnostic; got %v", row[4], result.Diagnostics)
			}
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

// API 64 is within the captured 62 through 67 range. A completed block must
// not hide a following split call.
func TestReceiverSplitIntermediateAPIAndAfterBlock(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ReceiverProbe.cls")
	writeSemaFile(t, path, "public class ReceiverProbe { public static String value() { return 'ok'; } }")
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "64.0", ApexFiles: []string{path}}, schema.Schema{})
	for _, source := range []string{"ReceiverProbe\n.value();", "if (true) { }\nReceiverProbe\n.value();"} {
		result := AnalyzeAnonymous(index, source, "64.0")
		found := false
		for _, d := range result.Diagnostics {
			if d.NativeMessage == "Variable does not exist: ReceiverProbe" {
				found = true
			}
		}
		if !found {
			t.Errorf("source %q: got %v", source, result.Diagnostics)
		}
	}
}
