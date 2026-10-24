package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

type capturedBodyCase struct {
	ID, Body          string
	Reject            bool
	AnonymousAccepted bool
	AnonymousRejected bool
	CarryAnonymous    string
}

func readCapturedBodyRows(t *testing.T, dir string) (map[string]capturedBodyCase, [][]string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []capturedBodyCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	byID := map[string]capturedBodyCase{}
	for _, c := range cases {
		if c.ID == "" || c.Body == "" || byID[c.ID].ID != "" {
			t.Fatal("invalid captured case")
		}
		byID[c.ID] = c
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1+6*len(cases) || strings.Join(rows[0], ",") != "api,route,case,outcome,diagnostic" {
		t.Fatal("incomplete native matrix")
	}
	run, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil || !strings.Contains(string(run), "native_complete=true\n") {
		t.Fatal("incomplete native run")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 5 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "named" && row[1] != "istest") {
			t.Fatalf("invalid native row: %v", row)
		}
		c, ok := byID[row[2]]
		key := strings.Join(row[:3], "/")
		if !ok || seen[key] || (row[3] != "accepted" && row[3] != "rejected") || (row[3] == "rejected") != ((c.Reject && !(row[1] == "anonymous" && c.AnonymousAccepted)) || (row[1] == "anonymous" && c.AnonymousRejected)) {
			t.Fatalf("invalid native outcome: %v", row)
		}
		seen[key] = true
	}
	return byID, rows[1:]
}

func capturedBodyIndex(t *testing.T, dir, api, route, body string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	write := func(path, source string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, path, source)
	}
	write(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
	files, err := filepath.Glob(filepath.Join(dir, "classes", "*.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(root, "force-app/main/default/classes", filepath.Base(file)), string(raw))
	}
	object, err := os.ReadFile(filepath.Join(dir, "metadata/TokenEntry__c.object-meta.xml"))
	if err == nil {
		write(filepath.Join(root, "force-app/main/default/objects/TokenEntry__c/TokenEntry__c.object-meta.xml"), string(object))
		field, err := os.ReadFile(filepath.Join(dir, "metadata/Value__c.field-meta.xml"))
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(root, "force-app/main/default/objects/TokenEntry__c/fields/Value__c.field-meta.xml"), string(field))
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if route != "anonymous" {
		source := "public class GardenBodyProbe { public static void observe() { " + body + " } }"
		if route == "istest" {
			source = "@IsTest private class GardenBodyProbe { @IsTest static void observe() { " + body + " } }"
		}
		write(filepath.Join(root, "force-app/main/default/classes/GardenBodyProbe.cls"), source)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return typesys.Build(p, s)
}

func TestQualifiedFieldTokensNativeRoutes(t *testing.T) {
	runCapturedSemanticBodyAnalysis(t, "testdata/qualified_field_tokens")
}

func runCapturedSemanticBodyAnalysis(t *testing.T, dir string) {
	t.Helper()
	cases, rows := readCapturedBodyRows(t, dir)
	for _, row := range rows {
		if row[1] == "anonymous" && cases[row[2]].CarryAnonymous != "" {
			t.Logf("carried native row %s: %s", strings.Join(row[:3], "/"), cases[row[2]].CarryAnonymous)
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			idx := capturedBodyIndex(t, dir, row[0], row[1], cases[row[2]].Body)
			result := Analyze(idx)
			if row[1] == "anonymous" {
				result = AnalyzeAnonymous(idx, cases[row[2]].Body, row[0])
			}
			if (idx.HasErrors() || result.HasErrors()) != (row[3] == "rejected") {
				t.Fatalf("native %s (%s); Glade index=%v diagnostics=%v", row[3], row[4], idx.Diagnostics, result.Diagnostics)
			}
		})
	}
}

func TestQualifiedFieldTokenRecognizerNativeRows(t *testing.T) {
	cases, rows := readCapturedBodyRows(t, "testdata/qualified_field_tokens")
	for _, row := range rows {
		if row[3] != "accepted" || !strings.Contains(row[2], "assignment") {
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			_, expr, ok := strings.Cut(cases[row[2]].Body, " = ")
			if !ok {
				t.Fatal("missing captured assignment")
			}
			expr, _, _ = strings.Cut(expr, ";")
			if !semaLooksLikeSObjectFieldToken(expr) {
				t.Fatalf("native field-token assignment not recognized: %s", expr)
			}
		})
	}
}
