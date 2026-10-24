package gladecli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNamedDependencyMappingCommandRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"check", "exec_anonymous", "test_named"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				workspace := t.TempDir()
				root := filepath.Join(workspace, "consumer")
				fixture := filepath.Join("..", "apextest", "testdata", "conformance", "local_dependency_mapping", "native", "api"+strings.TrimSuffix(api, ".0"))
				if err := os.CopyFS(root, os.DirFS(fixture)); err != nil {
					t.Fatal(err)
				}
				library := filepath.Join(workspace, "vendor", "classes")
				if err := os.MkdirAll(library, 0755); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"MappedLibrary.cls", "MappedLibrary.cls-meta.xml"} {
					if err := os.Rename(filepath.Join(root, "force-app", "main", "default", "classes", name), filepath.Join(library, name)); err != nil {
						t.Fatal(err)
					}
				}
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"LibraryAlias"}]}],"packageAliases":{"LibraryAlias":"0Ho000000000000AAA"}}`, api))
				writeTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  dependencies:\n    - name: LibraryAlias\n      path: ../vendor\n")
				args := []string{"check", "--project", root, "--json", "--no-cache", "--no-progress"}
				switch route {
				case "exec_anonymous":
					args = []string{"exec", "--project", root, "--json", "System.assertEquals('local-dependency',MappedConsumer.value());"}
				case "test_named":
					args = []string{"test", "--project", root, "--class", "MappedConsumerTest", "--json", "--no-cache", "--no-serve", "--no-progress", "--parallelism", "1"}
				}
				var stdout, stderr bytes.Buffer
				if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
					t.Fatalf("native accepted mapped source; exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
				}
				if route == "test_named" {
					run, err := decodeTestRunJSON(stdout.Bytes())
					if err != nil {
						t.Fatal(err)
					}
					if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
						t.Fatalf("named route=%+v", s)
					}
				}
			})
		}
	}
}
