package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConformancePairingFixtureCallersAndNestedPaths(t *testing.T) {
	cases := []struct {
		name, helper, extra string
		red                 bool
	}{
		{"helper caller", `filepath.Join("testdata", "family")`, "", true},
		{"helper without local test", `filepath.Join("testdata", "family")`, "", true},
		{"helper in go file", `filepath.Join("testdata", "family")`, "", true},
		{"nested variable root", `filepath.Join(root(), "testdata", "family")`, "", true},
		{"nested constant join", `filepath.Join(filepath.Join("testdata", "family"), "case.tsv")`, "", true},
		{"unrelated raw formatting", `"testdata/family/case.tsv"`, "var script = `printf '%s' arg\ncat testdata/other`\n", false},
		{"unrelated formatting call", `"testdata/family/case.tsv"`, "var unrelated = fmt.Sprintf(\"diagnostic %s testdata/other\", \"x\")\n", false},
		{"unrelated format-like literal", `"testdata/family/case.tsv"`, "var unrelated = \"testdata/%s\"\n", false},
		{"unrelated multiline format", `"testdata/family/case.tsv"`, "var script = `format %04d\ntext testdata/other`\n", false},
	}
	for _, tc := range cases {
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
			imports := "\"testing\""
			if strings.Contains(tc.extra, "fmt.") {
				imports += "; \"fmt\""
			}
			if strings.Contains(tc.helper, "filepath.") {
				imports += "; \"path/filepath\""
			}
			write("p/helper_test.go", fmt.Sprintf("package p\nimport (%s)\n%sfunc root() string { return \".\" }\nfunc helper() string { return %s }\nfunc TestHelperUnrelated(t *testing.T) {}\n", imports, tc.extra, tc.helper))
			if tc.name == "helper without local test" {
				data, err := os.ReadFile(filepath.Join(dir, "p/helper_test.go"))
				if err != nil {
					t.Fatal(err)
				}
				write("p/helper_test.go", strings.Replace(string(data), "func TestHelperUnrelated(t *testing.T) {}", "var _ = testing.Short", 1))
			}
			if tc.name == "helper in go file" {
				if err := os.Remove(filepath.Join(dir, "p/helper_test.go")); err != nil {
					t.Fatal(err)
				}
				write("p/helper.go", "package p\nimport \"path/filepath\"\nfunc helper() string { return filepath.Join(\"testdata\", \"family\") }\n")
			}
			write("p/caller_test.go", "package p\nimport \"testing\"\nfunc TestConsumer(t *testing.T) { _ = helper() }\n")
			write("p/testdata/family/case.tsv", "old")
			run("git", "add", ".")
			run("git", "commit", "-qm", "base")
			write("p/testdata/family/case.tsv", "new")
			run("git", "add", ".")
			run("git", "commit", "-qm", "fixture")
			carries := filepath.Join(t.TempDir(), "carries")
			if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(t.TempDir(), "go")
			events := "#!/bin/sh\ncase \"$*\" in *TestHelperUnrelated*) printf '%s\\n' '{\"Action\":\"pass\",\"Package\":\"example/p\",\"Test\":\"TestHelperUnrelated\"}';; esac\ncase \"$*\" in *TestConsumer*) printf '%s\\n' '{\"Action\":\""
			if tc.red {
				events += "fail"
			} else {
				events += "pass"
			}
			events += "\",\"Package\":\"example/p\",\"Test\":\"TestConsumer\"}';; esac\n"
			if tc.red {
				events += "case \"$*\" in *TestConsumer*) printf '%s\\n' '{\"Action\":\"fail\",\"Package\":\"example/p\"}'; exit 1;; esac\n"
			}
			events += "printf '%s\\n' '{\"Action\":\"pass\",\"Package\":\"example/p\"}'\n"
			if err := os.WriteFile(fake, []byte(events), 0755); err != nil {
				t.Fatal(err)
			}
			script, err := filepath.Abs("conformance-pairing.py")
			if err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			if tc.red {
				if err == nil || !strings.Contains(string(out), "p/TestConsumer: red without matching") {
					t.Fatalf("consumer missed: %v %s", err, out)
				}
			} else if err != nil || !strings.Contains(string(out), "PASS p/TestConsumer") {
				t.Fatalf("unrelated formatting rejected: %v %s", err, out)
			}
		})
	}
}
