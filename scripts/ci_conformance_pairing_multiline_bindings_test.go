package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConformancePairingNestedAndMultilineBindings(t *testing.T) {
	for _, tc := range []struct{ name, support, client string }{
		{"group_after_call", `import "strings"
var (
 Other = strings.TrimSpace("other")
 FixturePath = "testdata/family/case.tsv"
)`, `import "example/support"
func consume() string { return support.FixturePath }`},
		{"group_after_nested", `import "strings"
var (
 Other = strings.TrimSpace(strings.TrimSpace("other"))
 FixturePath string = "testdata/family/case.tsv"
)`, `import "example/support"
func consume() string { return support.FixturePath }`},
		{"multiline_initializer", `func Fixture() string { return "testdata/family/case.tsv" }`, `import "example/support"
var captured =
 support.Fixture()
func consume() string { return captured }`},
		{"multiline_group", `func Fixture() string { return "testdata/family/case.tsv" }`, `import "example/support"
var (
 captured string =
  support.Fixture()
)
func consume() string { return captured }`},
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
			write := func(p, s string) {
				t.Helper()
				p = filepath.Join(dir, p)
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(s), 0644); err != nil {
					t.Fatal(err)
				}
			}
			run("git", "init", "-q")
			run("git", "config", "user.name", "Fixture")
			run("git", "config", "user.email", "fixture@example.invalid")
			write("go.mod", "module example\n\ngo 1.26\n")
			write("support/helper.go", "package support\n"+tc.support+"\n")
			write("support/direct_test.go", "package support\nimport \"testing\"\nfunc TestDirect(t *testing.T) { _ = \"testdata/family/case.tsv\" }\n")
			write("client/helper.go", "package client\n"+tc.client+"\n")
			write("client/consumer_test.go", "package client\nimport \"testing\"\nfunc TestConsumer(t *testing.T) { _ = consume() }\n")
			write("support/testdata/family/case.tsv", "old")
			run("git", "add", ".")
			run("git", "commit", "-qm", "base")
			write("support/testdata/family/case.tsv", "new")
			run("git", "add", ".")
			run("git", "commit", "-qm", "change")
			carries := filepath.Join(t.TempDir(), "carries")
			if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(t.TempDir(), "go")
			runner := `#!/bin/sh
case "$*" in
 *./support*) printf '%s\n' '{"Action":"pass","Package":"example/support","Test":"TestDirect"}' '{"Action":"pass","Package":"example/support"}';;
 *./client*) printf '%s\n' '{"Action":"fail","Package":"example/client","Test":"TestConsumer"}' '{"Action":"fail","Package":"example/client"}'; exit 1;;
esac
`
			if err := os.WriteFile(fake, []byte(runner), 0755); err != nil {
				t.Fatal(err)
			}
			script, _ := filepath.Abs("conformance-pairing.py")
			out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			if err == nil || !strings.Contains(string(out), "client/TestConsumer: red without matching") {
				t.Fatalf("fixture consumer missed: %v %s", err, out)
			}
		})
	}
}
