package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

func TestSObjectTokenDiagnosticSpanNativeConformance(t *testing.T) {
	const dataDir = "testdata/sobject_token_span"
	var cases []struct{ ID, Body, Expression string }
	raw, err := os.ReadFile(filepath.Join(dataDir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(dataDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || strings.Join(rows[0], ",") != "id,api,route,compiled,compileProblem,line,column" {
		t.Fatal("missing native diagnostic positions")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 7 || row[3] != "false" || row[4] != "Variable does not exist: SObjectType" ||
			(row[1] != "62.0" && row[1] != "67.0") || row[2] != "isTest" {
			t.Fatalf("invalid native row: %v", row)
		}
		key := row[1] + "/" + row[2] + "/" + row[0]
		if seen[key] {
			t.Fatalf("duplicate native row: %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			var body, expression string
			for _, probe := range cases {
				if probe.ID == row[0] {
					body, expression = probe.Body, probe.Expression
					break
				}
			}
			if body == "" || expression == "" {
				t.Fatal("missing captured source")
			}
			source := "@IsTest private class TokenSpanObservation {\n @IsTest static void observe() {\n" + body + " }\n}\n"
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"TokenSpanObservation.cls": source}, row[1])
			nativeLine, err := strconv.Atoi(row[5])
			if err != nil || nativeLine < 1 {
				t.Fatalf("invalid native line: %q", row[5])
			}
			nativeColumn, err := strconv.Atoi(row[6])
			if err != nil || nativeColumn < 1 {
				t.Fatalf("invalid native column: %q", row[6])
			}
			start := strings.LastIndex(source, expression)
			if start < 0 {
				t.Fatal("rejected expression absent from captured source")
			}
			want := semaRange(source, start, start+len(expression))
			if want.Start.Line != nativeLine || nativeColumn < want.Start.Column || nativeColumn >= want.End.Column {
				t.Fatalf("native position %d:%d is outside rejected expression %v", nativeLine, nativeColumn, want)
			}
			matched := false
			for _, item := range result.Diagnostics {
				if item.Severity != diagnostic.Error {
					continue
				}
				if item.NativeMessage != row[4] && item.Message != row[4] {
					t.Fatalf("unexpected diagnostic: %#v", item)
				}
				matched = matched || item.Range != nil && *item.Range == *want
			}
			if !matched {
				t.Fatalf("native rejection at %d:%d belongs to %q at %v; Glade diagnostics: %#v", nativeLine, nativeColumn, expression, want, result.Diagnostics)
			}
		})
	}
	for _, probe := range cases {
		for _, api := range []string{"62.0", "67.0"} {
			if !seen[api+"/isTest/"+probe.ID] {
				t.Errorf("missing native row: %s/isTest/%s", api, probe.ID)
			}
		}
	}
}
