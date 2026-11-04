package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConformancePairingIgnoresNonCodeDeclarations(t *testing.T) {
	for _, source := range []string{
		"package fixture\nimport \"testing\"\n// func TestGhost(t *testing.T) {}\nfunc TestReal(t *testing.T) {}\n",
		"package fixture\nimport \"testing\"\nvar s = `func TestGhost(t *testing.T) {}`\nfunc TestReal(t *testing.T) {}\n",
		"package fixture\nimport \"testing\"\nvar s = \"func TestGhost(t *testing.T) {}\"\n/* func TestPhantom(t *testing.T) {} */\nfunc TestReal(t *testing.T) {}\n",
	} {
		events := "{\"Action\":\"pass\",\"Package\":\"example/p\",\"Test\":\"TestReal\"}\n{\"Action\":\"pass\",\"Package\":\"example/p\"}\n"
		script, dir, fake, carries := pairingFixture(t, "p", "case_test.go", source, events, "[]")
		if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+events+"'\nexit 0\n"), 0755); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
		if err != nil || !strings.Contains(string(out), "checked 1 tests") {
			t.Fatalf("non-code declaration selected: %v %s", err, out)
		}
	}
}

func TestConformancePairingFixtureConsumers(t *testing.T) {
	for _, tc := range []struct {
		name, fixture, literal string
		deleted                bool
	}{
		{"deleted", "internal/p/testdata/cases.json", "testdata/cases.json", true},
		{"unnamed driver", "internal/p/testdata/cases.json", "testdata/cases.json", false},
		{"shared root", "testdata/shared/cases.json", "../../testdata/shared/cases.json", false},
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
			run("git", "init", "-q")
			run("git", "config", "user.email", "fixture@example.invalid")
			run("git", "config", "user.name", "Fixture")
			write := func(path, value string) {
				t.Helper()
				path = filepath.Join(dir, path)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(value), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("go.mod", "module example\n\ngo 1.26\n")
			write("internal/p/consumer_test.go", fmt.Sprintf("package p\nimport \"testing\"\nfunc TestActualConsumer(t *testing.T) { _ = %q }\n", tc.literal))
			write("internal/p/unrelated_conformance_test.go", "package p\nimport \"testing\"\nfunc TestUnrelatedConformance(t *testing.T) {}\n")
			write(tc.fixture, "before\n")
			run("git", "add", ".")
			run("git", "commit", "-qm", "base")
			if tc.deleted {
				if err := os.Remove(filepath.Join(dir, tc.fixture)); err != nil {
					t.Fatal(err)
				}
			} else {
				write(tc.fixture, "after\n")
			}
			run("git", "add", "-A")
			run("git", "commit", "-qm", "fixture")
			carries := filepath.Join(t.TempDir(), "carries.json")
			if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(t.TempDir(), "fake-go")
			events := "{\"Action\":\"fail\",\"Package\":\"example/internal/p\",\"Test\":\"TestActualConsumer\"}\n{\"Action\":\"fail\",\"Package\":\"example/internal/p\"}\n"
			if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+events+"'\nexit 1\n"), 0755); err != nil {
				t.Fatal(err)
			}
			script, err := filepath.Abs("conformance-pairing.py")
			if err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			if err == nil || !strings.Contains(string(out), "TestActualConsumer: red without matching") || strings.Contains(string(out), "TestUnrelatedConformance") {
				t.Fatalf("wrong fixture selection: %v %s", err, out)
			}
		})
	}
}

func TestConformancePairingRejectsUnmappedFixture(t *testing.T) {
	script, dir, fake, carries := pairingFixture(t, "internal/p", "consumer_test.go", "package p\nimport \"testing\"\nfunc TestConsumer(t *testing.T) {}\n", "", "[]")
	path := filepath.Join(dir, "internal/p/testdata/unknown.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("git", "add", ".")
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	c = exec.Command("git", "commit", "-qm", "fixture")
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v %s", err, out)
	}
	out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "no discoverable test consumer") {
		t.Fatalf("unmapped fixture accepted: %v %s", err, out)
	}
}
