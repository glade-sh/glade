package compile_test

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/gladehome"
	lwccompile "github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/project"
)

type nativeCompilePass struct {
	caseID   string
	api      string
	expected string
}

type localCompilePass struct {
	nativeCompilePass
	actual string
	err    error
}

func TestLWCParallelPassesMatchNative(t *testing.T) {
	caseRows, err := readCompilePassRows("testdata/lwc_parallel_passes/cases.tsv")
	if err != nil {
		t.Fatal(err)
	}
	orgRows, err := readCompilePassRows("testdata/lwc_parallel_passes/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	if len(caseRows) != 2 || len(orgRows) != len(caseRows) {
		t.Fatalf("case/native row count = %d/%d; want two matching API rows", len(caseRows), len(orgRows))
	}
	for index := range caseRows {
		if caseRows[index] != orgRows[index] {
			t.Fatalf("case/native row %d differs: %#v / %#v", index, caseRows[index], orgRows[index])
		}
	}

	sources := make(map[string][]byte, 3)
	for _, name := range []string{"simpleProbe.html", "simpleProbe.js", "simpleProbe.js-meta.xml"} {
		sources[name], err = os.ReadFile(filepath.Join("testdata/lwc_parallel_passes/minimal_component", name))
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("GLADE_LWC_BROWSER_API_PARALLEL", "")
	t.Setenv("GLADE_LWC_BROWSER_CONCURRENCY", "")
	parallel := browserAPIParallelism(len(caseRows))
	versions := make([]string, len(caseRows))
	for pass, row := range caseRows {
		versions[pass] = row.api
	}
	if _, err := gladehome.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	got := runBrowserPasses(t, versions, parallel, func(t *testing.T, pass int) localCompilePass {
		row := caseRows[pass]
		result := localCompilePass{nativeCompilePass: row}
		root, err := os.MkdirTemp("", "glade-lwc-native-pass-")
		if err != nil {
			result.err = err
			return result
		}
		defer os.RemoveAll(root)

		bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "simpleProbe")
		for name, content := range sources {
			if name == "simpleProbe.js-meta.xml" {
				content = []byte(strings.ReplaceAll(string(content), "<apiVersion>67.0</apiVersion>", "<apiVersion>"+row.api+"</apiVersion>"))
			}
			path := filepath.Join(bundle, name)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				result.err = err
				return result
			}
			if err := os.WriteFile(path, content, 0600); err != nil {
				result.err = err
				return result
			}
		}
		projectFile := filepath.Join(root, "sfdx-project.json")
		projectJSON := "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"sourceApiVersion\":\"" + row.api + "\"}"
		if err := os.WriteFile(projectFile, []byte(projectJSON), 0600); err != nil {
			result.err = err
			return result
		}
		p, err := project.Load(root)
		if err != nil {
			result.err = err
			return result
		}
		_, result.err = lwccompile.Compile(p, lwccompile.Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
		result.actual = "COMPILE_OK"
		if result.err != nil {
			result.actual = "COMPILE_ERROR"
		}
		return result
	})

	for index, result := range got {
		if result.api != caseRows[index].api {
			t.Fatalf("pass %d returned API %s, want %s", index, result.api, caseRows[index].api)
		}
		l07CheckCompilerEnvironment(t, result.err)
		if result.err != nil && result.actual == "" {
			t.Fatalf("API %s compile failed without a result: %v", result.api, result.err)
		}
		if result.actual != result.expected {
			t.Errorf("API %s local compile = %s, Salesforce = %s: %v", result.api, result.actual, result.expected, result.err)
		}
	}

	packageBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "lwcruntime", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var runtimePackage struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(packageBytes, &runtimePackage); err != nil {
		t.Fatal(err)
	}
	runtimeTest := runtimePackage.Scripts["test"]
	if !strings.Contains(runtimeTest, "--test-concurrency=${GLADE_LWCRUNTIME_TEST_CONCURRENCY:-2}") ||
		!strings.Contains(runtimeTest, "GLADE_LWCRUNTIME_TEST_BINARY=bin/glade") ||
		!strings.Contains(runtimeTest, "go build -o bin/glade ./cmd/glade") {
		t.Fatalf("LWC runtime test command must use bounded passes and one shared CLI build; got %q", runtimeTest)
	}
}

func readCompilePassRows(path string) ([]nativeCompilePass, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = 3
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 || strings.Join(rows[0], ",") != "case_id,api_version,observed" {
		return nil, os.ErrInvalid
	}
	parsed := make([]nativeCompilePass, 0, len(rows)-1)
	for _, row := range rows[1:] {
		parsed = append(parsed, nativeCompilePass{caseID: row[0], api: row[1], expected: row[2]})
	}
	return parsed, nil
}
