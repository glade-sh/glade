package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSearchFindFixedSnippetNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/search_find_fixed_snippet"
	cases := readSearchFindSnippetCases(t, filepath.Join(fixture, "case-table.tsv"))
	if len(cases) != 4 {
		t.Fatalf("expected four API/route cases, got %d", len(cases))
	}

	for _, testCase := range cases {
		t.Run(testCase.api+"/"+testCase.route, func(t *testing.T) {
			version := "api" + strings.TrimSuffix(testCase.api, ".0")
			capture := filepath.Join(fixture, "native", version, testCase.route)
			rows := readSearchFindSnippetRows(t, filepath.Join(capture, "org.tsv"))
			if len(rows) != 1 {
				t.Fatalf("expected one native row, got %d", len(rows))
			}
			native := rows[0]
			if native.id != testCase.id || native.group != "SEARCH_FIND_FIXED_SNIPPET" || native.route != testCase.route || native.status != "runtime" {
				t.Fatalf("native row does not match case table: %+v", native)
			}
			validateSearchFindSnippetRun(t, filepath.Join(capture, "RUN-FACTS.txt"), testCase)

			sourcePath := "sources/anonymous.apex"
			if testCase.route == "istest" {
				sourcePath = "sources/istest.apex"
			}
			source, err := os.ReadFile(filepath.Join(fixture, sourcePath))
			if err != nil {
				t.Fatal(err)
			}
			assertion := searchFindSnippetNativeAssertion(t, testCase.route, native)
			program, err := CompileAnonymousWithOptions(string(source)+"\n"+assertion, CompileOptions{APIVersion: testCase.api})
			if err != nil {
				t.Fatal(err)
			}

			org := searchFindSnippetOrg(testCase.api)
			machine := New(nil)
			machine.SetOrg(&org)
			if testCase.route == "istest" {
				machine.EnableTestContext()
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatalf("Glade result diverged from native row %+v: %v", native, err)
			}
		})
	}
}

type searchFindSnippetCase struct {
	id, api, route string
}

type searchFindSnippetNativeRow struct {
	id, group, route, status string
	value                    struct {
		Outcome     string          `json:"outcome"`
		Type        string          `json:"type"`
		Message     string          `json:"message"`
		ResultCount int             `json:"result_count"`
		Snippet     json.RawMessage `json:"snippet"`
	}
}

func readSearchFindSnippetCases(t *testing.T, path string) []searchFindSnippetCase {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(rows) < 2 || rows[0] != "case_id\tapi\troute" {
		t.Fatal("invalid native case table")
	}
	cases := make([]searchFindSnippetCase, 0, len(rows)-1)
	for _, line := range rows[1:] {
		row := strings.Split(line, "\t")
		if len(row) != 3 {
			t.Fatalf("invalid native case row %q", line)
		}
		cases = append(cases, searchFindSnippetCase{id: row[0], api: row[1], route: row[2]})
	}
	return cases
}

func readSearchFindSnippetRows(t *testing.T, path string) []searchFindSnippetNativeRow {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(rows) < 2 || rows[0] != "id\tgroup\troute\tstatus\tvalue" {
		t.Fatal("invalid native org.tsv")
	}
	result := make([]searchFindSnippetNativeRow, 0, len(rows)-1)
	for _, line := range rows[1:] {
		row := strings.SplitN(line, "\t", 5)
		if len(row) != 5 {
			t.Fatalf("invalid native row %q", line)
		}
		var value struct {
			Outcome     string          `json:"outcome"`
			Type        string          `json:"type"`
			Message     string          `json:"message"`
			ResultCount int             `json:"result_count"`
			Snippet     json.RawMessage `json:"snippet"`
		}
		if err := json.Unmarshal([]byte(row[4]), &value); err != nil {
			t.Fatalf("invalid native observation %q: %v", row[4], err)
		}
		result = append(result, searchFindSnippetNativeRow{
			id: row[0], group: row[1], route: row[2], status: row[3], value: value,
		})
	}
	return result
}

func validateSearchFindSnippetRun(t *testing.T, path string, testCase searchFindSnippetCase) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	run := string(raw)
	for _, fact := range []string{
		"api=" + testCase.api,
		"context=" + testCase.route,
		"route=" + testCase.route,
		"rows=1",
		"native_complete=true",
		"deployment_success=true",
	} {
		if !strings.Contains("\n"+run, "\n"+fact+"\n") {
			t.Fatalf("%s missing native capture fact %q", path, fact)
		}
	}
}

func searchFindSnippetNativeAssertion(t *testing.T, route string, native searchFindSnippetNativeRow) string {
	t.Helper()
	switch route {
	case "anonymous":
		if native.value.Outcome != "exception" || native.value.Type == "" || native.value.Message == "" {
			t.Fatalf("anonymous route lacks native exception result: %+v", native)
		}
		want := native.value.Type + "|" + native.value.Message
		want = strings.ReplaceAll(want, "\\", "\\\\")
		want = strings.ReplaceAll(want, "'", "\\'")
		return fmt.Sprintf("System.assertEquals('%s', observation, 'anonymous fixed-search route');", want)
	case "istest":
		if native.value.Outcome != "value" || native.value.ResultCount != 1 || string(native.value.Snippet) != "null" {
			t.Fatalf("test route lacks native null snippet result: %+v", native)
		}
		return "System.assertEquals(true, snippet == null, 'fixed-search getSnippet() native result');"
	default:
		t.Fatalf("unknown route %q", route)
		return ""
	}
}

func searchFindSnippetOrg(api string) storage.OrgState {
	org := storage.NewOrgState()
	org.APIVersion = api
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields:    map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{},
	}
	return org
}
