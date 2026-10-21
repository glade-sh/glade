package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestSOQLDateTimeLiteralNativeConformance(t *testing.T) {
	type caseRow struct {
		id       string
		apexFile string
	}
	const fixture = "testdata/conformance/soql_datetime_literals"
	casesFile, err := os.Open(filepath.Join(fixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	casesReader := csv.NewReader(casesFile)
	casesReader.Comma = '\t'
	caseRecords, err := casesReader.ReadAll()
	closeErr := casesFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if len(caseRecords) != 5 || strings.Join(caseRecords[0], "|") != "case_id|apex_file" {
		t.Fatalf("case table header/rows = %v", caseRecords)
	}
	var cases []caseRow
	seen := make(map[string]bool)
	for _, record := range caseRecords[1:] {
		if len(record) != 2 || record[0] == "" || record[1] == "" || seen[record[0]] {
			t.Fatalf("invalid or duplicate case row: %v", record)
		}
		seen[record[0]] = true
		cases = append(cases, caseRow{id: record[0], apexFile: record[1]})
	}

	oracleFile, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	oracleReader := csv.NewReader(oracleFile)
	oracleReader.Comma = '\t'
	oracleRecords, err := oracleReader.ReadAll()
	closeErr = oracleFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if len(oracleRecords) != 9 || strings.Join(oracleRecords[0], "|") != "source_api|case_id|route|observation" {
		t.Fatalf("oracle header/rows = %v", oracleRecords)
	}
	oracle := make(map[string]map[string]string)
	for _, record := range oracleRecords[1:] {
		if len(record) != 4 || record[2] != "anonymous" || !seen[record[1]] || record[3] == "" {
			t.Fatalf("invalid oracle row: %v", record)
		}
		if oracle[record[0]] == nil {
			oracle[record[0]] = make(map[string]string)
		}
		if _, duplicate := oracle[record[0]][record[1]]; duplicate {
			t.Fatalf("duplicate oracle row: %v", record)
		}
		oracle[record[0]][record[1]] = record[3]
	}

	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			metadata := gladeschema.Schema{Objects: []gladeschema.Object{
				{Name: "Event", Fields: []gladeschema.Field{{Name: "Id", Type: "Id"}}},
			}}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api}, metadata)
			if index.HasErrors() {
				t.Fatalf("anonymous index: %v", index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("anonymous index semantics: %v", analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range cases {
				row := row
				t.Run(row.id, func(t *testing.T) {
					byCase := oracle[api]
					expected, ok := byCase[row.id]
					if !ok {
						t.Fatalf("missing native row for %s/%s", api, row.id)
					}
					code, err := os.ReadFile(filepath.Join(fixture, row.apexFile))
					if err != nil {
						t.Fatal(err)
					}
					source := string(code) + "\nSystem.assertEquals(" + conformanceApexString(expected) + ", String.valueOf(r), 'native SOQL DateTime row mismatch');\n"
					if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
						t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}
