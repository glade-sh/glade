package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConformancePairingChangedTestEntryPoints(t *testing.T) {
	for _, tc := range []struct{ name, changed, other, target string }{
		{"fuzz", `package p
import "testing"
func FuzzInput(f *testing.F) { f.Add("seed"); f.Fuzz(func(t *testing.T, s string) {}) }
`, "", "FuzzInput"},
		{"example", `package p
func ExampleThing() { println("x")
// Output: x
}
`, "", "ExampleThing"},
		{"helper", `package p
func helper() string { return "changed" }
`, `package p
import "testing"
func TestCaller(t *testing.T) { _ = helper() }
`, "TestCaller"},
		{"helper_chain", `package p
func helper() string { return "changed" }
`, `package p
import "testing"
func middle() string { return helper() }
func TestCaller(t *testing.T) { _ = middle() }
`, "TestCaller"},
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
			write := func(name, value string) {
				t.Helper()
				p := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(value), 0644); err != nil {
					t.Fatal(err)
				}
			}
			run("git", "init", "-q")
			run("git", "config", "user.name", "Fixture")
			run("git", "config", "user.email", "fixture@example.invalid")
			write("go.mod", "module example\n\ngo 1.26\n")
			write("p/changed_test.go", tc.changed+"\n// before\n")
			if tc.other != "" {
				write("p/caller_test.go", tc.other)
			}
			run("git", "add", ".")
			run("git", "commit", "-qm", "base")
			write("p/changed_test.go", tc.changed+"\n// after\n")
			run("git", "add", ".")
			run("git", "commit", "-qm", "changed")
			carries := filepath.Join(t.TempDir(), "carries")
			if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(t.TempDir(), "go")
			events := "{\"Action\":\"fail\",\"Package\":\"example/p\",\"Test\":\"" + tc.target + "\"}\n{\"Action\":\"fail\",\"Package\":\"example/p\"}\n"
			if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+events+"'\nexit 1\n"), 0755); err != nil {
				t.Fatal(err)
			}
			script, _ := filepath.Abs("conformance-pairing.py")
			out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			if err == nil || !strings.Contains(string(out), "p/"+tc.target+": red without matching") {
				t.Fatalf("changed test consumer missed: %v %s", err, out)
			}
		})
	}
}
