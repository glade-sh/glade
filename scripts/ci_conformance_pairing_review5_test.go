package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A fixture-only branch can make an existing conformance driver red.
func TestConformancePairingRejectsUnpairedFixtureChanges(t *testing.T) {
	script, err := filepath.Abs("conformance-pairing.py")
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []string{"api62.tsv", "api67.tsv", "rejection.tsv"} {
		t.Run(fixture, func(t *testing.T) {
			dir := t.TempDir()
			run := func(args ...string) {
				t.Helper()
				c := exec.Command(args[0], args[1:]...)
				c.Dir = dir
				if out, err := c.CombinedOutput(); err != nil {
					t.Fatalf("%v: %v: %s", args, err, out)
				}
			}
			run("git", "init", "-q")
			run("git", "config", "user.email", "fixture@example.invalid")
			run("git", "config", "user.name", "Fixture")
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.26\n"), 0644); err != nil {
				t.Fatal(err)
			}
			pkg := filepath.Join(dir, "internal/apextest")
			data := filepath.Join(pkg, "testdata/conformance/family")
			if err := os.MkdirAll(data, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(pkg, "family_conformance_test.go"), []byte(fmt.Sprintf("package apextest\nimport \"testing\"\nfunc TestFamilyNativeConformance(t *testing.T) { _ = %q }\n", "testdata/conformance/family/"+fixture)), 0644); err != nil {
				t.Fatal(err)
			}
			f := filepath.Join(data, fixture)
			if err := os.WriteFile(f, []byte("old\n"), 0644); err != nil {
				t.Fatal(err)
			}
			run("git", "add", ".")
			run("git", "commit", "-q", "-m", "base")
			if err := os.WriteFile(f, []byte("new\n"), 0644); err != nil {
				t.Fatal(err)
			}
			run("git", "add", ".")
			run("git", "commit", "-q", "-m", "fixture")
			carries := filepath.Join(t.TempDir(), "carries.json")
			if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(t.TempDir(), "fake-go")
			events := `{"Action":"fail","Package":"example/internal/apextest","Test":"TestFamilyNativeConformance"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest"}` + "\n"
			if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+events+"'\nexit 1\n"), 0755); err != nil {
				t.Fatal(err)
			}
			c := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries)
			out, err := c.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "red without matching counted CARRIED record") {
				t.Fatalf("fixture-only unpaired failure accepted: %v %s", err, out)
			}
		})
	}
}

func TestConformancePairingStableCarriedFailures(t *testing.T) {
	testName := "TestNativeConformance"
	pkg := "example/internal/apextest"
	diag := "assertion changed\n--- FAIL: database (0.00s)\n"
	packageDiag := "package diagnostic\n"
	sig := func(s string) string { return pairingBoundSignature([2]string{"TestNativeConformance", s}) }
	records := fmt.Sprintf(`[{"package":"internal/apextest","test":%q,"count":1,"failures":[%q],"signature":%q,"owner":"owner","reason":"known failure"},{"package":"internal/apextest","test":"@package","count":1,"signature":%q,"owner":"owner","reason":"known package"}]`, testName, testName, sig(diag), pairingBoundSignature([2]string{testName, diag}, [2]string{"", packageDiag}))
	events := func(elapsed, passed, diagnostic string) string {
		return fmt.Sprintf("{\"Action\":\"output\",\"Package\":%q,\"Test\":\"TestOther\",\"Output\":%q}\n{\"Action\":\"pass\",\"Package\":%q,\"Test\":\"TestOther\"}\n{\"Action\":\"output\",\"Package\":%q,\"Test\":%q,\"Output\":%q}\n{\"Action\":\"output\",\"Package\":%q,\"Test\":%q,\"Output\":%q}\n{\"Action\":\"fail\",\"Package\":%q,\"Test\":%q,\"Elapsed\":%s}\n{\"Action\":\"output\",\"Package\":%q,\"Output\":%q}\n{\"Action\":\"output\",\"Package\":%q,\"Output\":%q}\n{\"Action\":\"fail\",\"Package\":%q}\n", pkg, passed, pkg, pkg, testName, diagnostic, pkg, testName, "--- FAIL: "+testName+" ("+elapsed+"s)\n", pkg, testName, elapsed, pkg, packageDiag, pkg, "FAIL\t"+pkg+"\t"+elapsed+"s\n", pkg)
	}
	script, dir, fake, carries := pairingFixture(t, "internal/apextest", "native_conformance_test.go", "package fixture\nfunc TestNativeConformance(t *testing.T) {}\nfunc TestOther(t *testing.T) {}\n", events("0.10", "old passing output\n", diag), records)
	invoke := func() ([]byte, error) {
		return exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
	}
	for _, tc := range []struct {
		elapsed, passed, diagnostic string
		pass                        bool
	}{{"0.10", "old passing output\n", diag, true}, {"4.28", "new passing output\n", diag, true}, {"4.28", "new passing output\n", "assertion changed\n--- FAIL: runtime (0.00s)\n", false}} {
		body := events(tc.elapsed, tc.passed, tc.diagnostic)
		if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+body+"'\nexit 1\n"), 0755); err != nil {
			t.Fatal(err)
		}
		out, err := invoke()
		if (err == nil) != tc.pass {
			t.Fatalf("pass=%v: %v %s", tc.pass, err, out)
		}
		if !tc.pass && !strings.Contains(string(out), "signature mismatch") {
			t.Fatalf("wrong rejection: %s", out)
		}
	}
}

// A user diagnostic identical to a later real frame must remain signed.
func TestConformancePairingKeepsDuplicateFrameDiagnostics(t *testing.T) {
	pkg := "example/internal/apextest"
	name := "TestNativeConformance"
	for _, tc := range []struct{ name, diagnostic, packageDiagnostic string }{
		{"test frame", "prefix\n--- FAIL: TestNativeConformance (0.10s)\n", "package detail\n"},
		{"package frame", "prefix\n", "package detail\nFAIL\texample/internal/apextest\t0.10s\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := fmt.Sprintf("{\"Action\":\"output\",\"Package\":%q,\"Test\":%q,\"Output\":%q}\n{\"Action\":\"output\",\"Package\":%q,\"Test\":%q,\"Output\":%q}\n{\"Action\":\"fail\",\"Package\":%q,\"Test\":%q,\"Elapsed\":0.1}\n{\"Action\":\"output\",\"Package\":%q,\"Output\":%q}\n{\"Action\":\"output\",\"Package\":%q,\"Output\":%q}\n{\"Action\":\"fail\",\"Package\":%q}\n", pkg, name, tc.diagnostic, pkg, name, "--- FAIL: "+name+" (0.10s)\n", pkg, name, pkg, tc.packageDiagnostic, pkg, "FAIL\t"+pkg+"\t0.10s\n", pkg)
			sig := func(s string) string { return pairingBoundSignature([2]string{"TestNativeConformance", s}) }
			record := func(testSig, pkgSig string) string {
				return fmt.Sprintf(`[{"package":"internal/apextest","test":%q,"count":1,"failures":[%q],"signature":%q,"owner":"owner","reason":"known"},{"package":"internal/apextest","test":"@package","count":1,"signature":%q,"owner":"owner","reason":"known"}]`, name, name, testSig, pkgSig)
			}
			script, dir, fake, carries := pairingFixture(t, "internal/apextest", "native_conformance_test.go", "package fixture\nfunc TestNativeConformance(t *testing.T) {}\n", events, record(sig("prefix\n"), pairingBoundSignature([2]string{name, "prefix\n"}, [2]string{"", "package detail\n"})))
			invoke := func() ([]byte, error) {
				return exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			}
			out, err := invoke()
			if err == nil || !strings.Contains(string(out), "signature mismatch") {
				t.Fatalf("duplicate frame diagnostic disappeared: %v %s", err, out)
			}
			if err := os.WriteFile(carries, []byte(record(sig(tc.diagnostic), pairingBoundSignature([2]string{name, tc.diagnostic}, [2]string{"", tc.packageDiagnostic}))), 0644); err != nil {
				t.Fatal(err)
			}
			out, err = invoke()
			if err != nil {
				t.Fatalf("matching signature rejected: %v %s", err, out)
			}
		})
	}
}
