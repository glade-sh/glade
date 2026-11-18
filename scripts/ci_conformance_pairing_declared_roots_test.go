package scripts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConformancePairingDeclaredFixtureRoots(t *testing.T) {
	for _, tc := range []struct{ name, decl, symbol string }{
		{"typed", `var FixturePath string = "testdata/family/case.tsv"`, "FixturePath"},
		{"grouped_const", "const (\n FixturePath string = \"testdata/family/case.tsv\"\n)", "FixturePath"},
		{"grouped_var", "var (\n FixturePath = \"testdata/family/case.tsv\"\n)", "FixturePath"},
		{"multiple", `var Other, FixturePath string = "other", "testdata/family/case.tsv"`, "FixturePath"},
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
			write("support/helper.go", "package support\n"+tc.decl+"\n")
			write("support/direct_test.go", `package support
import "testing"
func TestDirect(t *testing.T) { _ = "testdata/family/case.tsv" }
`)
			write("client/client_test.go", "package client\nimport (\"testing\"; \"example/support\")\nfunc TestConsumer(t *testing.T) { _ = support."+tc.symbol+" }\n")
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

func TestConformancePairingApprovedSkipPolicy(t *testing.T) {
	const root = "TestSOQLSafeNavigationBindNativeConformance"
	const reason = "native boundary is captured but outside this bind replay"
	for _, tc := range []struct {
		name, leaf, reason, pkg string
		pass                    bool
	}{
		{"api62", root + "/62.0/left_member", reason, "internal/vm", true},
		{"api67", root + "/67.0/left_member", reason, "internal/vm", true},
		{"wrong reason", root + "/62.0/left_member", "different reason", "internal/vm", false},
		{"wrong leaf", root + "/62.0/other", reason, "internal/vm", false},
		{"wrong package", root + "/62.0/left_member", reason, "internal/other", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script, dir, fake, carries := pairingFixture(t, tc.pkg, "native_test.go", "package p\nimport \"testing\"\nfunc "+root+"(t *testing.T) {}\n", "", "[]")
			pkg := "github.com/glade-sh/glade/" + tc.pkg
			event := func(action, name, output string) string {
				m := map[string]string{"Action": action, "Package": pkg}
				if name != "" {
					m["Test"] = name
				}
				if output != "" {
					m["Output"] = output
				}
				b, _ := json.Marshal(m)
				return string(b)
			}
			data := strings.Join([]string{event("output", tc.leaf, "    native_test.go:57: "+tc.reason+"\n"), event("skip", tc.leaf, ""), event("pass", root, ""), event("pass", "", "")}, "\n") + "\n"
			if err := os.WriteFile(fake, []byte("#!/bin/sh\ncat <<'EVENTS'\n"+data+"EVENTS\n"), 0755); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%v: %v %s", tc.pass, err, out)
			}
		})
	}
}

func TestConformancePairingChangedPackageSetup(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"testmain", `func TestMain(m *testing.M) { m.Run() }`},
		{"init", `func init() { setup() }`},
		{"initializer", `var x = setup()`},
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
			if err := os.MkdirAll(filepath.Join(dir, "p"), 0755); err != nil {
				t.Fatal(err)
			}
			write := func(p, s string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, p), []byte(s), 0644); err != nil {
					t.Fatal(err)
				}
			}
			run("git", "init", "-q")
			run("git", "config", "user.name", "Fixture")
			run("git", "config", "user.email", "fixture@example.invalid")
			write("go.mod", "module example\n\ngo 1.26\n")
			write("p/other_test.go", "package p\nimport \"testing\"\nfunc TestOther(t *testing.T) {}\n")
			run("git", "add", ".")
			run("git", "commit", "-qm", "base")
			imports := ""
			if tc.name == "testmain" {
				imports = "import \"testing\"\n"
			}
			write("p/setup_test.go", "package p\n"+imports+tc.source+"\nfunc setup() int { return 1 }\n")
			run("git", "add", ".")
			run("git", "commit", "-qm", "change")
			script, _ := filepath.Abs("conformance-pairing.py")
			fake := filepath.Join(t.TempDir(), "go")
			carries := filepath.Join(t.TempDir(), "carries")
			if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
				t.Fatal(err)
			}
			events := `#!/bin/sh
printf '%s\n' '{"Action":"fail","Package":"example/p","Test":"TestOther"}' '{"Action":"fail","Package":"example/p"}'
exit 1
`
			if err := os.WriteFile(fake, []byte(events), 0755); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			if err == nil || !strings.Contains(string(out), "p/TestOther: red without matching") {
				t.Fatalf("setup consumer missed: %v %s", err, out)
			}
		})
	}
}
