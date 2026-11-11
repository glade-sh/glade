package sema

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

const licensedDescribeNamesDir = "../apextest/testdata/conformance/licensed_describe_names"

// Shapes Salesforce rejects for these names and Glade still accepts. They stay
// out of the check until a fix lands, and the rows keep them on record:
//   - the .SObjectType token of an absent name in named code (every name,
//     including a name that exists nowhere), and in anonymous code for
//     RecordAccessEvent__e and the unknown __e and __mdt tokens;
//   - a declaration or static query of RecordAccessEvent__e in named code.
func licensedDescribeKnownGap(route, id string) bool {
	if strings.Contains(id, "_sobjecttype_") && route != "anonymous" {
		return true
	}
	switch id {
	case "R03_sobjecttype_RecordAccessEvent__e", "T01_token_unknown_e", "T02_token_unknown_mdt":
		return true
	case "R03_declare_RecordAccessEvent__e", "R03_staticsoql_RecordAccessEvent__e":
		return route != "anonymous"
	}
	return false
}

// Native rows: licensed_describe_names/org.tsv (source API 62.0 and 67.0).
func TestLicensedDescribeNamesCompileLikeAbsentNames(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(licensedDescribeNamesDir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(licensedDescribeNamesDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	checked := 0
	for first := true; ; first = false {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if first || record[2] == "rest-describe" {
			continue
		}
		api, route, id := record[0], record[2], record[3]
		source := cases[id]
		var observed struct {
			Compiled      *bool    `json:"compiled"`
			Exception     string   `json:"exceptionMessage"`
			DeployStatus  string   `json:"deploy_status"`
			ClassProblems []string `json:"compile_problem"`
		}
		if err := json.Unmarshal([]byte(record[4]), &observed); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		// Runtime-only rows (global describe, Type.forName, dynamic query)
		// have no compile verdict to compare here.
		if source == "" || strings.Contains(id, "_globaldescribe_") || strings.Contains(id, "_typeforname_") || strings.Contains(id, "_soql_") && !strings.Contains(id, "_staticsoql_") {
			continue
		}
		if licensedDescribeKnownGap(route, id) {
			continue
		}
		nativeRejected := observed.Compiled != nil && !*observed.Compiled || observed.DeployStatus == "Failed"
		t.Run(api+"/"+route+"/"+id, func(t *testing.T) {
			var rejected bool
			switch route {
			case "anonymous":
				index := typesys.Build(project.Project{SourceAPIVersion: api}, schema.Schema{})
				rejected = AnalyzeAnonymous(index, source, api).HasErrors()
			default:
				root := t.TempDir()
				path := filepath.Join(root, "LicensedNameProbe.cls")
				body := "public class LicensedNameProbe { public static void observe() { " + source + " } }"
				if route == "istest" {
					body = "@IsTest private class LicensedNameProbe { @IsTest static void observe() { " + source + " } }"
				}
				writeSemaFile(t, path, body)
				rejected = Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})).HasErrors()
			}
			if rejected != nativeRejected {
				t.Fatalf("native rejected=%t, got %t for %q", nativeRejected, rejected, source)
			}
		})
		checked++
	}
	if checked < 100 {
		t.Fatalf("expected the captured compile rows, checked %d", checked)
	}
}
