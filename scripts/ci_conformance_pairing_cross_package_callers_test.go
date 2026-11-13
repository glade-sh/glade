package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConformancePairingCrossPackageCallers(t *testing.T) {
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
	write("support/helper.go", `package support
func Fixture() string { return "testdata/family/case.tsv" }
`)
	write("support/helper_test.go", `package support
import "testing"
func TestDirect(t *testing.T) { _ = Fixture() }
`)
	write("bridge/helper.go", `package fixturebridge
import fixture "example/support"
func Relay() string { return fixture.Fixture() }
`)
	write("client/helper_test.go", `package client
import ("example/bridge"; fixture "example/support")
func readData() string { return fixturebridge.Relay() }
func loadData() string { return fixture.Fixture() }
`)
	write("client/consumer_test.go", `package client
import "testing"
func TestViaBridge(t *testing.T) { _ = readData() }
func TestViaAlias(t *testing.T) { _ = loadData() }
`)
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
 *./client*) case "$*" in *TestViaAlias*) printf '%s\n' '{"Action":"fail","Package":"example/client","Test":"TestViaAlias"}';; esac; case "$*" in *TestViaBridge*) printf '%s\n' '{"Action":"fail","Package":"example/client","Test":"TestViaBridge"}';; esac; printf '%s\n' '{"Action":"fail","Package":"example/client"}'; exit 1;;
esac
`
	if err := os.WriteFile(fake, []byte(runner), 0755); err != nil {
		t.Fatal(err)
	}
	script, _ := filepath.Abs("conformance-pairing.py")
	out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "client/TestViaAlias: red without matching") || !strings.Contains(string(out), "client/TestViaBridge: red without matching") {
		t.Fatalf("cross-package consumers missed: %v %s", err, out)
	}
}
