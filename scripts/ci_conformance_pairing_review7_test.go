package scripts

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func pairingBoundSignature(parts ...[2]string) string {
	data, err := json.Marshal(parts)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func TestConformancePairingSkipsTestMain(t *testing.T) {
	for _, special := range []string{"TestMain", "Testlower", "Testéclair"} {
		source := fmt.Sprintf("package p\nimport \"testing\"\nfunc %s(m *testing.M) {}\nfunc TestReal(t *testing.T) {}\n", special)
		events := "{\"Action\":\"pass\",\"Package\":\"example/p\",\"Test\":\"TestReal\"}\n{\"Action\":\"pass\",\"Package\":\"example/p\"}\n"
		script, dir, fake, carries := pairingFixture(t, "p", "case_test.go", source, events, "[]")
		if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+events+"'\nexit 0\n"), 0755); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
		if err != nil || !strings.Contains(string(out), "checked 1 tests") {
			t.Fatalf("special name %s selected: %v %s", special, err, out)
		}
	}
}

func TestConformancePairingBindsDiagnosticIdentity(t *testing.T) {
	pkg, name := "example/internal/apextest", "TestNativeConformance"
	for _, tc := range []struct{ from, to string }{{"a", "b"}, {"b", "c"}, {"parent", "a"}} {
		t.Run(tc.from+"-"+tc.to, func(t *testing.T) {
			testName := func(n string) string {
				if n == "parent" {
					return name
				}
				return name + "/" + n
			}
			events := func(which string) string {
				return fmt.Sprintf("{\"Action\":\"output\",\"Package\":%q,\"Test\":%q,\"Output\":\"diagnostic\\n\"}\n{\"Action\":\"fail\",\"Package\":%q,\"Test\":%q}\n{\"Action\":\"fail\",\"Package\":%q,\"Test\":%q}\n{\"Action\":\"fail\",\"Package\":%q,\"Test\":%q}\n{\"Action\":\"fail\",\"Package\":%q}\n", pkg, testName(which), pkg, name+"/a", pkg, name+"/b", pkg, name, pkg)
			}
			// The old signature concatenated output and cannot tell which test emitted it.
			legacy := fmt.Sprintf("%x", sha256.Sum256([]byte("diagnostic\n")))
			record := func(sig string) string {
				return fmt.Sprintf(`[{"package":"internal/apextest","test":%q,"count":2,"failures":[%q,%q],"signature":%q,"owner":"owner","reason":"known"},{"package":"internal/apextest","test":"@package","count":1,"signature":%q,"owner":"owner","reason":"known"}]`, name, name+"/a", name+"/b", sig, sig)
			}
			script, dir, fake, carries := pairingFixture(t, "internal/apextest", "native_test.go", "package fixture\nimport \"testing\"\nfunc TestNativeConformance(t *testing.T) {}\n", events(tc.from), record(legacy))
			invoke := func() ([]byte, error) {
				return exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			}
			if out, err := invoke(); err == nil || !strings.Contains(string(out), "signature mismatch") {
				t.Fatalf("unbound legacy signature accepted: %v %s", err, out)
			}
			encoded, err := json.Marshal([][]string{{testName(tc.from), "diagnostic\n"}})
			if err != nil {
				t.Fatal(err)
			}
			bound := fmt.Sprintf("%x", sha256.Sum256(encoded))
			if err := os.WriteFile(carries, []byte(record(bound)), 0644); err != nil {
				t.Fatal(err)
			}
			if out, err := invoke(); err != nil {
				t.Fatalf("bound signature rejected: %v %s", err, out)
			}
			if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+events(tc.to)+"'\nexit 1\n"), 0755); err != nil {
				t.Fatal(err)
			}
			if out, err := invoke(); err == nil || !strings.Contains(string(out), "signature mismatch") {
				t.Fatalf("moved diagnostic accepted: %v %s", err, out)
			}
		})
	}
}

func TestConformancePairingFindsConstructedFixtureConsumers(t *testing.T) {
	for _, tc := range []struct{ name, expr string }{{"join", `filepath.Join("testdata", "conformance", "family")`}, {"nested", `path.Join("testdata", "conformance", "family", "api62")`}, {"concat", `"testdata/" + "conformance/" + "family"`}, {"dynamic root", `filepath.Join("testdata", segment)`}, {"formatted root", `fmt.Sprintf("testdata/%s", segment)`}} {
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
			run("git", "config", "user.name", "Fixture")
			run("git", "config", "user.email", "fixture@example.invalid")
			write := func(path, s string) {
				t.Helper()
				p := filepath.Join(dir, path)
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(s), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("go.mod", "module example\n\ngo 1.26\n")
			write("p/direct_test.go", "package p\nimport \"testing\"\nfunc TestDirect(t *testing.T) { _ = \"testdata/conformance/family/api62/case.tsv\" }\n")
			imports := "\"testing\""
			if strings.HasPrefix(tc.expr, "filepath.") {
				imports += "; \"path/filepath\""
			}
			if strings.HasPrefix(tc.expr, "path.") {
				imports += "; \"path\""
			}
			if strings.HasPrefix(tc.expr, "fmt.") {
				imports += "; \"fmt\""
			}
			write("p/constructed_test.go", fmt.Sprintf("package p\nimport (%s)\nvar segment = \"conformance\"\nfunc TestConstructed(t *testing.T) { _ = %s }\n", imports, tc.expr))
			write("p/testdata/conformance/family/api62/case.tsv", "before")
			run("git", "add", ".")
			run("git", "commit", "-qm", "base")
			write("p/testdata/conformance/family/api62/case.tsv", "after")
			run("git", "add", ".")
			run("git", "commit", "-qm", "fixture")
			carries := filepath.Join(t.TempDir(), "carries")
			if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(t.TempDir(), "go")
			events := "{\"Action\":\"pass\",\"Package\":\"example/p\",\"Test\":\"TestDirect\"}\n{\"Action\":\"fail\",\"Package\":\"example/p\",\"Test\":\"TestConstructed\"}\n{\"Action\":\"fail\",\"Package\":\"example/p\"}\n"
			if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+events+"'\nexit 1\n"), 0755); err != nil {
				t.Fatal(err)
			}
			script, err := filepath.Abs("conformance-pairing.py")
			if err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			want := "TestConstructed: red without matching"
			if tc.name == "dynamic root" || tc.name == "formatted root" {
				want = "dynamic fixture root cannot be mapped safely"
			}
			if err == nil || !strings.Contains(string(out), want) {
				t.Fatalf("constructed consumer missed: %v %s", err, out)
			}
		})
	}
}
