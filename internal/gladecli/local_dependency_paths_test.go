package gladecli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

func TestLocalDependencyPathsCLIRoutesAPI62And67(t *testing.T) {
	runLocalDependencyPathsCLIRoutes(t, nil)
}

func TestLocalDependencyPathsInitializedCLIRoutesAPI62And67(t *testing.T) {
	for _, command := range [][]string{{"init"}, {"config", "init"}} {
		t.Run(strings.Join(command, "_"), func(t *testing.T) {
			runLocalDependencyPathsCLIRoutes(t, command)
		})
	}
}

func runLocalDependencyPathsCLIRoutes(t *testing.T, initializer []string) {
	t.Helper()
	const fixture = "../apextest/testdata/conformance/local_dependency_paths"
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			root := copyLocalDependencyFixture(t, filepath.Join(fixture, "fixture"))
			consumer := filepath.Join(root, "consumer")
			writeTestFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"LocalBridgeSupport"}]}],"sourceApiVersion":"`+api+`.0"}`)
			if len(initializer) > 0 {
				configPath := filepath.Join(consumer, "glade.yml")
				if err := os.Remove(configPath); err != nil {
					t.Fatal(err)
				}
				var stdout, stderr bytes.Buffer
				args := append(append([]string(nil), initializer...), "--project", consumer, "--yes")
				if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
					t.Fatalf("initialize exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
				}
				data, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(data), `packageDirs: [force-app]`) || !strings.Contains(string(data), "dependencyPaths: []") {
					t.Fatalf("unexpected initialized config: %s", data)
				}
				writeTestFile(t, configPath, strings.Replace(string(data), "dependencyPaths: []", `dependencyPaths: ["../dependencies"]`, 1))
			}
			for _, route := range []struct {
				name string
				args []string
			}{
				{"check_named_and_isTest", []string{"check", "--project", consumer, "--json"}},
				{"test_runner", []string{"test", "--project", consumer, "--no-serve", "--no-cache", "--no-progress", "--json", "--class", "LocalBridgeDependency" + api + "Test"}},
				{"lsp_project_load", []string{"lsp", "--project", consumer, "--diagnostics-once"}},
				{"config_show", []string{"config", "show", "--project", consumer, "--json"}},
			} {
				t.Run(route.name, func(t *testing.T) {
					var stdout, stderr bytes.Buffer
					code := Run(context.Background(), route.args, &stdout, &stderr)
					if code != 0 {
						t.Fatalf("exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
					}
					switch route.name {
					case "test_runner":
						var envelope struct{ Data testreport.Run }
						if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
							t.Fatal(err)
						}
						if s := envelope.Data.Summary(); !s.OK() || s.Total != 1 || s.Passed != 1 {
							t.Fatalf("test summary=%+v", s)
						}
					case "lsp_project_load":
						if strings.Contains(stdout.String(), `"severity":1`) {
							t.Fatalf("LSP errors: %s", &stdout)
						}
					case "config_show":
						var envelope struct {
							Data struct{ DependencyPaths []string }
						}
						if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
							t.Fatal(err)
						}
						if got := envelope.Data.DependencyPaths; len(got) != 1 || got[0] != filepath.Join(root, "dependencies") {
							t.Fatalf("configured dependencyPaths=%v", got)
						}
					}
				})
			}
			raw, err := os.ReadFile(filepath.Join(fixture, "api"+api, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			reader := csv.NewReader(bytes.NewReader(raw))
			reader.Comma = '\t'
			rows, err := reader.ReadAll()
			if err != nil || len(rows) != 4 {
				t.Fatalf("native rows=%v error=%v", rows, err)
			}
			for _, row := range rows[1:] {
				t.Run("exec_anonymous/"+row[1], func(t *testing.T) {
					if row[0] != api+".0" || (row[2] != "accepted" && row[2] != "rejected") {
						t.Fatalf("invalid native row=%v", row)
					}
					var stdout, stderr bytes.Buffer
					source, err := os.ReadFile(filepath.Join(fixture, "api"+api, row[1]+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					code := Run(context.Background(), []string{"exec", "--project", consumer, "--json", string(source)}, &stdout, &stderr)
					if (code == 0) != (row[2] == "accepted") {
						t.Fatalf("native=%s exit=%d stdout=%s stderr=%s", row[2], code, &stdout, &stderr)
					}
					if row[2] == "rejected" && !strings.Contains(stdout.String()+stderr.String(), "MissingLocalBridgeSupport") {
						t.Fatalf("wrong rejection: stdout=%s stderr=%s", &stdout, &stderr)
					}
				})
			}
		})
	}
}

func copyLocalDependencyFixture(t *testing.T, source string) string {
	t.Helper()
	root := t.TempDir()
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		writeTestFile(t, filepath.Join(root, relative), string(data))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return root
}
