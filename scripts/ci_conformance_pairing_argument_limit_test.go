package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConformancePairingReview10(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		dynamic      bool
	}{
		{"indexed", `import "fmt"; func path() string { return fmt.Sprintf("testdata/family/%[1]s", "case.tsv") }`, true},
		{"indexed width", `import "fmt"; func path() string { return fmt.Sprintf("testdata/family/%[1]*[2]s", 3, "case.tsv") }`, true},
		{"alias", `import f "fmt"; func path() string { return f.Sprintf("testdata/family/%s", "case.tsv") }`, true},
		{"alias error", `import f "fmt"; func path() error { return f.Errorf("testdata/family/%s", "case.tsv") }`, true},
		{"const format", `import "fmt"; const format = "testdata/family/%s"; func path() string { return fmt.Sprintf(format, "case.tsv") }`, true},
		{"typed format", `import "fmt"; const format string = "testdata/family/%s"; func path() string { return fmt.Sprintf(format, "case.tsv") }`, true},
		{"variable format", `import f "fmt"; var format = "testdata/family/%[1]s"; func path() string { return f.Sprintf(format, "case.tsv") }`, true},
		{"local format", `import "fmt"; func path() string { format := "testdata/family/%s"; return fmt.Sprintf(format, "case.tsv") }`, true},
		{"shadowed format", `import "fmt"; const format = "testdata/family/%s"; func other() { format := "unrelated"; _ = format }; func path() string { return fmt.Sprintf(format, "case.tsv") }`, true},
		{"forwarded format", `import "fmt"; const original = "testdata/family/%s"; func path() string { format := original; return fmt.Sprintf(format, "case.tsv") }`, true},
		{"format function value", `import "fmt"; func path() string { build := fmt.Sprintf; return build("testdata/family/%s", "case.tsv") }`, true},
		{"format function value alias", `import f "fmt"; func path() string { build := f.Sprintf; makePath := build; return makePath("testdata/family/%[1]s", "case.tsv") }`, true},
		{"concatenated format", `import "fmt"; const format = "testdata/family/" + "%s"; func path() string { return fmt.Sprintf(format, "case.tsv") }`, true},
		{"concatenated format prefix", `import "fmt"; func path() string { format := "testdata/" + "family/%s"; return fmt.Sprintf(format, "case.tsv") }`, true},
		{"helper selection", `func path() string { return "testdata/family/case.tsv" }`, false},
		{"global helper", `var fixture = "testdata/family/case.tsv"; func path() string { return fixture }`, false},
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
			write("p/helper.go", "package p\n"+tc.source+"\n")
			write("p/test_test.go", `package p
import "testing"
func TestConsumer(t *testing.T) { _ = path() }
func TestUnrelated(t *testing.T) { t.Skip("unrelated") }
`)
			write("p/other_test.go", `package p
import "testing"
func TestDirect(t *testing.T) { _ = "testdata/family/case.tsv" }
`)
			write("p/testdata/family/case.tsv", "old")
			run("git", "add", ".")
			run("git", "commit", "-qm", "base")
			write("p/testdata/family/case.tsv", "new")
			run("git", "add", ".")
			run("git", "commit", "-qm", "change")
			carries := filepath.Join(t.TempDir(), "carries")
			if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(t.TempDir(), "go")
			runner := `#!/bin/sh
case "$*" in *TestUnrelated*) printf '%s\n' '{"Action":"skip","Package":"example/p","Test":"TestUnrelated"}';; esac
case "$*" in *TestConsumer*) printf '%s\n' '{"Action":"pass","Package":"example/p","Test":"TestConsumer"}';; esac
case "$*" in *TestDirect*) printf '%s\n' '{"Action":"pass","Package":"example/p","Test":"TestDirect"}';; esac
printf '%s\n' '{"Action":"pass","Package":"example/p"}'
`
			if err := os.WriteFile(fake, []byte(runner), 0755); err != nil {
				t.Fatal(err)
			}
			script, _ := filepath.Abs("conformance-pairing.py")
			out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			if tc.dynamic {
				if err == nil || !strings.Contains(string(out), "dynamic fixture root") {
					t.Fatalf("dynamic consumer accepted: %v %s", err, out)
				}
			} else if err != nil || strings.Contains(string(out), "TestUnrelated") || !strings.Contains(string(out), "PASS p/TestConsumer") {
				t.Fatalf("wrong selection: %v %s", err, out)
			}
		})
	}
}

func TestConformancePairingReview10ArgumentLimit(t *testing.T) {
	// A generated changed test file can contain enough names to exceed Linux's
	// single-argument limit; the runner enforces a much smaller portable bound.
	var source strings.Builder
	source.WriteString("package p\nimport \"testing\"\n")
	for i := 0; i < 600; i++ {
		fmt.Fprintf(&source, "func TestCase%04dLongNameForSelection(t *testing.T) {}\n", i)
	}
	script, dir, fake, carries := pairingFixture(t, "p", "many_test.go", source.String(), "", "[]")
	runner := `#!/usr/bin/env python3
import json, re, sys
p=sys.argv[sys.argv.index('-run')+1]
if len(p)>4096: sys.exit(2)
for n in re.findall(r'TestCase[0-9]+LongNameForSelection',p): print(json.dumps({'Action':'pass','Package':'example/p','Test':n}))
print(json.dumps({'Action':'pass','Package':'example/p'}))
`
	if err := os.WriteFile(fake, []byte(runner), 0755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "checked 600 tests") {
		t.Fatalf("large selection failed: %v %s", err, out)
	}
}
