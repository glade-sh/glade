package gladecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalDependencyPathsInitRoutesAPI62And67(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, command := range [][]string{{"init"}, {"config", "init"}} {
			t.Run(api+"/"+strings.Join(command, "_"), func(t *testing.T) {
				root := t.TempDir()
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				var stdout, stderr bytes.Buffer
				args := append(append([]string(nil), command...), "--project", root, "--yes")
				if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
					t.Fatalf("exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
				}
				data, err := os.ReadFile(filepath.Join(root, "glade.yml"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(data), "  dependencyPaths: []\n") {
					t.Fatalf("generated config lacks dependencyPaths: %s", data)
				}
			})
		}
	}
}
