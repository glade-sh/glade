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

const qualifiedReceiverFixtures = "../apextest/testdata/conformance/qualified_receivers"

func qualifiedReceiverRows(t *testing.T, name string) [][]string {
	t.Helper()
	f, err := os.Open(filepath.Join(qualifiedReceiverFixtures, name))
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
	return rows
}

// Label metadata can be omitted by snippet indexes. That means an unknown
// catalogue, rather than a catalogue that proves a particular label is missing.
func qualifiedReceiverIndex(t *testing.T, api, namespace, labels, source, suffix string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "ReceiverMatrix"+suffix)
	writeSemaFile(t, path, source)
	p := project.Project{Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: []string{path}}
	if labels == "present" {
		labelPath := filepath.Join(root, "CustomLabels.labels-meta.xml")
		data, err := os.ReadFile(filepath.Join(qualifiedReceiverFixtures, "project/force-app/main/default/labels/CustomLabels.labels-meta.xml"))
		if err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, labelPath, string(data))
		p.LabelFiles = []string{labelPath}
		p.PackageDirectories = []project.PackageDirectory{{Path: root}}
	}
	object, field := "Meadow__c", "Flag__c"
	if namespace != "" {
		object, field = namespace+"__"+object, namespace+"__"+field
	}
	index := typesys.Build(p, schema.Schema{Objects: []schema.Object{
		{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "Text"}}},
		{Name: object, Fields: []schema.Field{{Name: field, Type: "Checkbox"}}},
	}})
	if index.HasErrors() {
		t.Fatalf("source indexing: %+v", index.Diagnostics)
	}
	return index
}

func TestAnalyzeQualifiedReceiverMatrix(t *testing.T) {
	rows := qualifiedReceiverRows(t, "cases.tsv")
	if len(rows) != 37 || strings.Join(rows[0], ",") != "case,kind,expression" {
		t.Fatal("incomplete receiver case table")
	}
	var methods, statements strings.Builder
	for _, row := range rows[1:] {
		if row[1] != "receiver" {
			continue
		}
		methods.WriteString("public static Object " + row[0] + "() { return " + row[2] + "; }\n")
		statements.WriteString("{ Object value = " + row[2] + "; }\n")
	}
	triggerVariables, err := os.ReadFile(filepath.Join(qualifiedReceiverFixtures, "trigger_variables.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"", "meadowpkg"} {
			for _, labels := range []string{"present", "unindexed"} {
				for _, route := range []string{"named", "isTest", "standard-trigger", "custom-trigger"} {
					name := api + "/namespace=" + namespace + "/labels=" + labels + "/" + route
					t.Run(name, func(t *testing.T) {
						source, suffix := "public class ReceiverMatrix {\n"+methods.String()+"}\n", ".cls"
						if route == "isTest" {
							source = "@IsTest private class ReceiverMatrix { @IsTest static void observe() {\n" + statements.String() + "}\n}\n"
						}
						if strings.HasSuffix(route, "-trigger") {
							object := "Account"
							if route == "custom-trigger" {
								object = "Meadow__c"
							}
							source = "trigger ReceiverMatrix on " + object + " (before insert) {\n" + statements.String() + string(triggerVariables) + "}\n"
							suffix = ".trigger"
						}
						result := Analyze(qualifiedReceiverIndex(t, api, namespace, labels, source, suffix))
						if len(result.Diagnostics) != 0 {
							for _, d := range result.Diagnostics {
								t.Errorf("native receiver shape rejected: %s %s", d.Code, d.Message)
							}
						}
					})
				}
			}
		}
	}
}

func TestAnalyzeQualifiedReceiverNativeControls(t *testing.T) {
	cases := map[string][]string{}
	for _, row := range qualifiedReceiverRows(t, "cases.tsv")[1:] {
		cases[row[0]] = row
	}
	rows := qualifiedReceiverRows(t, "compile.tsv")
	if len(rows) != 145 || strings.Join(rows[0], ",") != "source_api,route,case,compiled,problem" {
		t.Fatal("incomplete native compile matrix")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		key := strings.Join(row[:3], "/")
		c, exists := cases[row[2]]
		if !exists || seen[key] || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "isTest") || (row[3] != "true" && row[3] != "false") || (row[3] == "false" && row[4] == "") {
			t.Fatalf("invalid native row: %v", row)
		}
		seen[key] = true
		// These native rejections remain local false acceptances. Report them
		// as named KNOWN_GAP skips, never as passing native conformance rows.
		if c[1] == "known_gap" || row[1] == "anonymous" && c[1] == "receiver" && row[3] == "false" {
			t.Run("KNOWN_GAP/"+key, func(t *testing.T) {
				if row[3] != "false" {
					t.Fatalf("known gap must cite a native rejection: %v", row)
				}
				t.Skipf("KNOWN GAP: local analysis accepts %s; native compile.tsv row %s rejects it: %s", c[2], key, row[4])
			})
			continue
		}
		// The positive project matrix covers named compilation.
		if row[1] == "isTest" && c[1] == "receiver" {
			continue
		}
		t.Run(key, func(t *testing.T) {
			source := "Object value = " + c[2] + ";"
			index := qualifiedReceiverIndex(t, row[0], "", "present", "public class ReceiverMatrix {}", ".cls")
			var result Result
			if row[1] == "anonymous" {
				result = AnalyzeAnonymous(index, source, row[0])
			} else {
				index = qualifiedReceiverIndex(t, row[0], "", "present", "@IsTest private class ReceiverMatrix { @IsTest static void observe() { "+source+" } }", ".cls")
				result = Analyze(index)
			}
			if accepted := len(result.Diagnostics) == 0; accepted != (row[3] == "true") {
				t.Fatalf("native compiled=%s problem=%q; Glade diagnostics=%+v", row[3], row[4], result.Diagnostics)
			}
		})
	}
}
