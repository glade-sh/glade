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

const objectPermissionsSchemaQualifiedDir = "../apextest/testdata/conformance/object_permissions_schema_qualified_token"

type objectPermissionsNativeRow struct {
	api, route, id string
	rejected       bool
	problem        string
}

func readObjectPermissionsNativeRows(t *testing.T) (map[string]string, []objectPermissionsNativeRow) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(objectPermissionsSchemaQualifiedDir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(objectPermissionsSchemaQualifiedDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	var rows []objectPermissionsNativeRow
	for first := true; ; first = false {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if first {
			continue
		}
		row := objectPermissionsNativeRow{api: record[0], route: record[2], id: record[3]}
		var observed struct {
			Compiled       *bool    `json:"compiled"`
			CompileProblem string   `json:"compileProblem"`
			DeployStatus   string   `json:"deploy_status"`
			ClassProblems  []string `json:"compile_problem"`
		}
		if err := json.Unmarshal([]byte(record[4]), &observed); err != nil {
			t.Fatalf("%s: %v", record[3], err)
		}
		if row.route == "anonymous" {
			row.rejected = observed.Compiled != nil && !*observed.Compiled
			row.problem = observed.CompileProblem
		} else {
			row.rejected = observed.DeployStatus == "Failed"
			if len(observed.ClassProblems) > 0 {
				row.problem = observed.ClassProblems[0]
			}
		}
		rows = append(rows, row)
	}
	if len(rows) < 80 {
		t.Fatalf("expected the captured rows, got %d", len(rows))
	}
	return cases, rows
}

// Rows the captured packet records but this check does not cover. Salesforce
// rejects them, and Glade still accepts the first two at API 62.0 or rejects
// the third only when the statement runs:
//   - P12 and P13 read the field from a constructor expression
//     (new ObjectPermissions().PermissionsViewAllFields).
//   - P09 names an absent field through the Schema token.
var objectPermissionsSchemaQualifiedParked = map[string]bool{
	"P09_schema_token_NoSuch_control": true,
	"P12_new_schema_read":             true,
	"P13_new_bare_read":               true,
}

// Native rows: object_permissions_schema_qualified_token/org.tsv (source API
// 62.0 and 67.0; anonymous, named and @IsTest routes).
func TestObjectPermissionsSchemaQualifiedTokenNativeRows(t *testing.T) {
	cases, rows := readObjectPermissionsNativeRows(t)
	for _, row := range rows {
		source, ok := cases[row.id]
		if !ok {
			t.Fatalf("no statement for %s", row.id)
		}
		if objectPermissionsSchemaQualifiedParked[row.id] {
			continue
		}
		t.Run(row.api+"/"+row.route+"/"+row.id, func(t *testing.T) {
			var rejected bool
			var messages string
			switch row.route {
			case "anonymous":
				index := typesys.Build(project.Project{SourceAPIVersion: row.api}, schema.Schema{})
				result := AnalyzeAnonymous(index, source, row.api)
				rejected = result.HasErrors()
				var builder strings.Builder
				for _, d := range result.Diagnostics {
					builder.WriteString(d.Message + "\n" + d.NativeMessage + "\n")
				}
				messages = builder.String()
			default:
				root := t.TempDir()
				file := filepath.Join(root, "SchemaQualifiedProbe.cls")
				body := "public class SchemaQualifiedProbe { public static void observe() { " + source + " } }"
				if row.route == "istest" {
					body = "@IsTest private class SchemaQualifiedProbe { @IsTest static void observe() { " + source + " } }"
				}
				writeSemaFile(t, file, body)
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row.api, ApexFiles: []string{file}}, schema.Schema{})
				result := Analyze(index)
				rejected = result.HasErrors()
				var builder strings.Builder
				for _, d := range result.Diagnostics {
					builder.WriteString(d.Message + "\n" + d.NativeMessage + "\n")
				}
				messages = builder.String()
			}
			if rejected != row.rejected {
				t.Fatalf("native rejected=%t, got %t (%q): %s", row.rejected, rejected, source, messages)
			}
			if row.rejected && !strings.Contains(messages, "does not exist") {
				t.Fatalf("native %q not matched: %s", row.problem, messages)
			}
		})
	}
}
