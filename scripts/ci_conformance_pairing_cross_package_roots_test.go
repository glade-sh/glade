package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConformancePairingCrossPackageFixtureRoots(t *testing.T) {
	for _, tc := range []struct{ name, expr string }{
		{"variable root", `filepath.Join(root, "testdata", "shared", "family")`},
		{"variable middle", `filepath.Join(root, segment, "testdata", "shared", "family")`},
		{"embedded relative root", `filepath.Join(root, "../testdata", "shared", "family")`},
		{"constant shared root", `filepath.Join("..", "..", "testdata")`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			run := func(args ...string) {
				t.Helper()
				c := exec.Command(args[0], args[1:]...)
				c.Dir = dir
				if out, err := c.CombinedOutput(); err != nil {
					t.Fatalf("%v: %v %s", args, err, out)
				}
			}
			write := func(path, data string) {
				t.Helper()
				path = filepath.Join(dir, path)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(data), 0644); err != nil {
					t.Fatal(err)
				}
			}
			run("git", "init", "-q")
			run("git", "config", "user.name", "Fixture")
			run("git", "config", "user.email", "fixture@example.invalid")
			write("go.mod", "module example\n\ngo 1.26\n")
			write("direct/direct_test.go", "package direct\nimport \"testing\"\nfunc TestDirect(t *testing.T) { _ = \"../testdata/shared/family/case.tsv\" }\n")
			root := "../.."
			if tc.name == "embedded relative root" {
				root = "../../stage"
			}
			write("internal/consumer/consumer_test.go", fmt.Sprintf("package consumer\nimport (\"testing\"; \"path/filepath\")\nfunc TestConsumer(t *testing.T) { root := %q; segment := \".\"; _, _ = root, segment; _ = %s }\n", root, tc.expr))
			write("testdata/shared/family/case.tsv", "old")
			run("git", "add", ".")
			run("git", "commit", "-qm", "base")
			write("testdata/shared/family/case.tsv", "new")
			run("git", "add", ".")
			run("git", "commit", "-qm", "fixture")
			carries := filepath.Join(t.TempDir(), "carries")
			if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(t.TempDir(), "go")
			data := "#!/bin/sh\ncase \"$*\" in\n  *./direct) printf '%s\\n' '{\"Action\":\"pass\",\"Package\":\"example/direct\",\"Test\":\"TestDirect\"}' '{\"Action\":\"pass\",\"Package\":\"example/direct\"}'; exit 0;;\n  *./internal/consumer) printf '%s\\n' '{\"Action\":\"fail\",\"Package\":\"example/internal/consumer\",\"Test\":\"TestConsumer\"}' '{\"Action\":\"fail\",\"Package\":\"example/internal/consumer\"}'; exit 1;;\nesac\nexit 3\n"
			if err := os.WriteFile(fake, []byte(data), 0755); err != nil {
				t.Fatal(err)
			}
			script, err := filepath.Abs("conformance-pairing.py")
			if err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			if err == nil || !strings.Contains(string(out), "internal/consumer/TestConsumer: red without matching") {
				t.Fatalf("shared fixture consumer missed or rejected: %v %s", err, out)
			}
		})
	}
}
