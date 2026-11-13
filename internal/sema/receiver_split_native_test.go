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
// NATIVE argument_newline testdata/receiver_split/org.tsv:62 anonymous 62.0
// NATIVE argument_newline testdata/receiver_split/org.tsv:63 anonymous 67.0
// NATIVE argument_message testdata/receiver_split/org.tsv:64 anonymous 62.0
// NATIVE argument_message testdata/receiver_split/org.tsv:65 anonymous 67.0
// NATIVE declaration_before_dot testdata/receiver_split/org.tsv:66 anonymous 62.0
// NATIVE declaration_before_dot testdata/receiver_split/org.tsv:67 anonymous 67.0
// NATIVE debug_before_dot testdata/receiver_split/org.tsv:68 anonymous 62.0
// NATIVE debug_before_dot testdata/receiver_split/org.tsv:69 anonymous 67.0
// NATIVE builtin_before_dot testdata/receiver_split/org.tsv:70 anonymous 62.0
// NATIVE builtin_before_dot testdata/receiver_split/org.tsv:71 anonymous 67.0
// NATIVE statement_argument_before_dot testdata/receiver_split/org.tsv:72 anonymous 62.0
// NATIVE statement_argument_before_dot testdata/receiver_split/org.tsv:73 anonymous 67.0
// NATIVE fluent_before_dot testdata/receiver_split/org.tsv:74 anonymous 62.0
// NATIVE fluent_before_dot testdata/receiver_split/org.tsv:75 anonymous 67.0
// NATIVE line_comment_before_dot testdata/receiver_split/org.tsv:76 anonymous 62.0
// NATIVE line_comment_before_dot testdata/receiver_split/org.tsv:77 anonymous 67.0
// NATIVE qualified_inner_before_dot testdata/receiver_split/org.tsv:78 anonymous 62.0
// NATIVE qualified_inner_before_dot testdata/receiver_split/org.tsv:79 anonymous 67.0
// NATIVE class_literal_before_dot testdata/receiver_split/org.tsv:80 anonymous 62.0
// NATIVE class_literal_before_dot testdata/receiver_split/org.tsv:81 anonymous 67.0
// NATIVE sobject_member_before_dot testdata/receiver_split/org.tsv:82 anonymous 62.0
// NATIVE sobject_member_before_dot testdata/receiver_split/org.tsv:83 anonymous 67.0
// NATIVE block_before_dot testdata/receiver_split/org.tsv:84 anonymous 62.0
// NATIVE block_before_dot testdata/receiver_split/org.tsv:85 anonymous 67.0
// NATIVE fluent_space_control testdata/receiver_split/org.tsv:86 anonymous 62.0
// NATIVE fluent_space_control testdata/receiver_split/org.tsv:87 anonymous 67.0
// NATIVE statement_space_after_dot testdata/receiver_split/org.tsv:88 anonymous 62.0
// NATIVE statement_space_after_dot testdata/receiver_split/org.tsv:89 anonymous 67.0
// NATIVE statement_comment_after_dot testdata/receiver_split/org.tsv:90 anonymous 62.0
// NATIVE statement_comment_after_dot testdata/receiver_split/org.tsv:91 anonymous 67.0
// NATIVE argument_space_after_dot testdata/receiver_split/org.tsv:92 anonymous 62.0
// NATIVE argument_space_after_dot testdata/receiver_split/org.tsv:93 anonymous 67.0
// NATIVE statement_line_comment_after_dot testdata/receiver_split/org.tsv:94 anonymous 62.0
// NATIVE statement_line_comment_after_dot testdata/receiver_split/org.tsv:95 anonymous 67.0
// NATIVE statement_blank_after_dot testdata/receiver_split/org.tsv:96 anonymous 62.0
// NATIVE statement_blank_after_dot testdata/receiver_split/org.tsv:97 anonymous 67.0
// NATIVE fluent_after_dot testdata/receiver_split/org.tsv:98 anonymous 62.0
// NATIVE fluent_after_dot testdata/receiver_split/org.tsv:99 anonymous 67.0
// NATIVE assignment_after_dot testdata/receiver_split/org.tsv:100 anonymous 62.0
// NATIVE assignment_after_dot testdata/receiver_split/org.tsv:101 anonymous 67.0
// NATIVE binary_argument_after_dot testdata/receiver_split/org.tsv:102 anonymous 62.0
// NATIVE binary_argument_after_dot testdata/receiver_split/org.tsv:103 anonymous 67.0
// NATIVE block_after_dot testdata/receiver_split/org.tsv:104 anonymous 62.0
// NATIVE block_after_dot testdata/receiver_split/org.tsv:105 anonymous 67.0
// NATIVE mixed_before_after_dot testdata/receiver_split/org.tsv:106 anonymous 62.0
// NATIVE mixed_before_after_dot testdata/receiver_split/org.tsv:107 anonymous 67.0
// NATIVE builtin_after_dot testdata/receiver_split/org.tsv:108 anonymous 62.0
// NATIVE builtin_after_dot testdata/receiver_split/org.tsv:109 anonymous 67.0
// NATIVE class_argument_before_dot testdata/receiver_split/org.tsv:110 class 62.0
// NATIVE class_argument_before_dot testdata/receiver_split/org.tsv:111 class 67.0
// NATIVE class_statement_before_dot testdata/receiver_split/org.tsv:112 class 62.0
// NATIVE class_statement_before_dot testdata/receiver_split/org.tsv:113 class 67.0
// NATIVE class_statement_after_dot testdata/receiver_split/org.tsv:114 class 62.0
// NATIVE class_statement_after_dot testdata/receiver_split/org.tsv:115 class 67.0
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
					if row[2] == "class" && d.Code != "GLADESEMA013" {
						t.Errorf("named diagnostic code = %s, want GLADESEMA013", d.Code)
					}
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

// Before-dot name resolution is independent of the enclosing expression;
// the class-route and anonymous initializer rows establish the predicate.
// NATIVE class_statement_before_dot testdata/receiver_split/org.tsv:112 class 62.0
// NATIVE class_statement_before_dot testdata/receiver_split/org.tsv:113 class 67.0
// NATIVE declaration_before_dot testdata/receiver_split/org.tsv:66 anonymous 62.0
// NATIVE declaration_before_dot testdata/receiver_split/org.tsv:67 anonymous 67.0
func TestReceiverSplitBeforeDotInReturn(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		probe := filepath.Join(root, "ReceiverProbe.cls")
		caller := filepath.Join(root, "Caller.cls")
		writeSemaFile(t, probe, "public class ReceiverProbe { public static String value() { return 'ok'; } }")
		writeSemaFile(t, caller, "public class Caller { public static String run() { return ReceiverProbe\n.value(); } }")
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{probe, caller}}, schema.Schema{})
		result := Analyze(index)
		found := false
		for _, d := range result.Diagnostics {
			if d.NativeMessage == "Variable does not exist: ReceiverProbe" && d.Code == "GLADESEMA013" {
				found = true
			}
		}
		if !found {
			t.Errorf("api %s: got %v", api, result.Diagnostics)
		}
	}
}
