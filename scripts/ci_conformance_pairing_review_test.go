package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func pairingFixture(t *testing.T, pkg, file, source, output, carries string) (string, string, string, string) {
	t.Helper()
	script, err := filepath.Abs("conformance-pairing.py")
	if err != nil {
		t.Fatal(err)
	}
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
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "base")
	path := filepath.Join(dir, pkg)
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, file), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "test")
	carryPath := filepath.Join(t.TempDir(), "carries.json")
	if err := os.WriteFile(carryPath, []byte(carries), 0644); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(t.TempDir(), "fake-go")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+output+"'\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	return script, dir, fake, carryPath
}

func TestConformancePairingRejectsUnpairedOutsideInternal(t *testing.T) {
	for _, pkg := range []string{"cmd/glade", "scripts", "."} {
		t.Run(pkg, func(t *testing.T) {
			pkgID := "example"
			if pkg != "." {
				pkgID += "/" + pkg
			}
			events := fmt.Sprintf("{\"Action\":\"output\",\"Package\":%q,\"Test\":\"TestNativeConformance\",\"Output\":\"assertion failed\\n\"}\n{\"Action\":\"fail\",\"Package\":%q,\"Test\":\"TestNativeConformance\"}\n{\"Action\":\"fail\",\"Package\":%q}\n", pkgID, pkgID, pkgID)
			script, dir, fake, carries := pairingFixture(t, pkg, "native_conformance_test.go", "package fixture\nfunc TestNativeConformance(t *testing.T) {}\n", events, "[]")
			c := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries)
			out, err := c.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "red without matching counted CARRIED record") {
				t.Fatalf("unpaired red conformance test accepted: %v %s", err, out)
			}
		})
	}
}

func TestConformancePairingRejectsChangedFailureSignatures(t *testing.T) {
	for _, tc := range []struct{ name, testOutput, packageOutput string }{
		{"changed assertion", "new assertion\n", "package cleanup\n"},
		{"changed package failure", "old assertion\n", "new panic\n"},
		{"changed runtime diagnostic", "new runtime error\n", "package cleanup\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := fmt.Sprintf("{\"Action\":\"output\",\"Package\":\"example/internal/apextest\",\"Test\":\"TestNativeConformance\",\"Output\":%q}\n{\"Action\":\"fail\",\"Package\":\"example/internal/apextest\",\"Test\":\"TestNativeConformance\"}\n{\"Action\":\"output\",\"Package\":\"example/internal/apextest\",\"Output\":%q}\n{\"Action\":\"fail\",\"Package\":\"example/internal/apextest\"}\n", tc.testOutput, tc.packageOutput)
			oldTest := pairingBoundSignature([2]string{"TestNativeConformance", "old assertion\n"})
			oldPackage := pairingBoundSignature([2]string{"TestNativeConformance", "old assertion\n"}, [2]string{"", "package cleanup\n"})
			legacy := `[{"package":"internal/apextest","test":"TestNativeConformance","count":1,"failures":["TestNativeConformance"],"owner":"owner","reason":"old failure"},{"package":"internal/apextest","test":"@package","count":1,"owner":"owner","reason":"old package failure"}]`
			script, dir, fake, carries := pairingFixture(t, "internal/apextest", "native_conformance_test.go", "package fixture\nfunc TestNativeConformance(t *testing.T) {}\n", events, legacy)
			invoke := func() ([]byte, error) {
				t.Helper()
				return exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			}
			out, err := invoke()
			if err == nil || !strings.Contains(string(out), "carry requires failure signature") {
				t.Fatalf("unsigned stale carry accepted: %v %s", err, out)
			}
			record := func(testSig, pkgSig string) string {
				return fmt.Sprintf(`[{"package":"internal/apextest","test":"TestNativeConformance","count":1,"failures":["TestNativeConformance"],"signature":%q,"owner":"owner","reason":"old failure"},{"package":"internal/apextest","test":"@package","count":1,"signature":%q,"owner":"owner","reason":"old package failure"}]`, testSig, pkgSig)
			}
			if err := os.WriteFile(carries, []byte(record(oldTest, oldPackage)), 0644); err != nil {
				t.Fatal(err)
			}
			out, err = invoke()
			if err == nil || !strings.Contains(string(out), "signature mismatch") {
				t.Fatalf("changed diagnostic accepted: %v %s", err, out)
			}
			newTest := pairingBoundSignature([2]string{"TestNativeConformance", tc.testOutput})
			newPackage := pairingBoundSignature([2]string{"TestNativeConformance", tc.testOutput}, [2]string{"", tc.packageOutput})
			if err := os.WriteFile(carries, []byte(record(newTest, newPackage)), 0644); err != nil {
				t.Fatal(err)
			}
			out, err = invoke()
			if err != nil {
				t.Fatalf("matching signatures rejected: %v %s", err, out)
			}
		})
	}
}

func TestConformancePairingPreservesFrameLikeDiagnostics(t *testing.T) {
	for _, message := range []string{"--- FAIL: permission denied\n", "=== RUN database unavailable\n"} {
		t.Run(strings.TrimSpace(message), func(t *testing.T) {
			output := "assertion prefix\n" + message
			events := fmt.Sprintf("{\"Action\":\"output\",\"Package\":\"example/internal/apextest\",\"Test\":\"TestNativeConformance\",\"Output\":%q}\n{\"Action\":\"fail\",\"Package\":\"example/internal/apextest\",\"Test\":\"TestNativeConformance\"}\n{\"Action\":\"fail\",\"Package\":\"example/internal/apextest\"}\n", output)
			oldSig := pairingBoundSignature([2]string{"TestNativeConformance", "assertion prefix\n"})
			record := func(sig string) string {
				return fmt.Sprintf(`[{"package":"internal/apextest","test":"TestNativeConformance","count":1,"failures":["TestNativeConformance"],"signature":%q,"owner":"owner","reason":"old failure"},{"package":"internal/apextest","test":"@package","count":1,"signature":%q,"owner":"owner","reason":"old package"}]`, sig, sig)
			}
			script, dir, fake, carries := pairingFixture(t, "internal/apextest", "native_conformance_test.go", "package fixture\nfunc TestNativeConformance(t *testing.T) {}\n", events, record(oldSig))
			invoke := func() ([]byte, error) {
				return exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			}
			out, err := invoke()
			if err == nil || !strings.Contains(string(out), "signature mismatch") {
				t.Fatalf("changed diagnostic was ignored: %v %s", err, out)
			}
			newSig := pairingBoundSignature([2]string{"TestNativeConformance", output})
			if err := os.WriteFile(carries, []byte(record(newSig)), 0644); err != nil {
				t.Fatal(err)
			}
			out, err = invoke()
			if err != nil {
				t.Fatalf("matching full diagnostic rejected: %v %s", err, out)
			}
		})
	}
}

// Output may be produced by the test even when it looks exactly like Go framing.
func TestConformancePairingPreservesExactHarnessLikeOutput(t *testing.T) {
	for _, tc := range []struct{ name, message string }{
		{"result line", "--- FAIL: database (0.00s)\n"},
		{"run line", "=== RUN   TestDatabase\n"},
		{"package timing line", "FAIL\texample/internal/apextest\t0.10s\n"},
		{"pause line", "=== PAUSE TestDatabase\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// PAUSE has two spaces so it is exactly a framing match, too.
			message := tc.message
			if tc.name == "pause line" {
				message = "=== PAUSE  TestDatabase\n"
			}
			output := "assertion prefix\n" + message
			events := fmt.Sprintf("{\"Action\":\"output\",\"Package\":\"example/internal/apextest\",\"Test\":\"TestNativeConformance\",\"Output\":%q}\n{\"Action\":\"fail\",\"Package\":\"example/internal/apextest\",\"Test\":\"TestNativeConformance\"}\n{\"Action\":\"fail\",\"Package\":\"example/internal/apextest\"}\n", output)
			oldSig := pairingBoundSignature([2]string{"TestNativeConformance", "assertion prefix\n"})
			record := func(sig string) string {
				return fmt.Sprintf(`[{"package":"internal/apextest","test":"TestNativeConformance","count":1,"failures":["TestNativeConformance"],"signature":%q,"owner":"owner","reason":"old failure"},{"package":"internal/apextest","test":"@package","count":1,"signature":%q,"owner":"owner","reason":"old package"}]`, sig, sig)
			}
			script, dir, fake, carries := pairingFixture(t, "internal/apextest", "native_conformance_test.go", "package fixture\nfunc TestNativeConformance(t *testing.T) {}\n", events, record(oldSig))
			invoke := func() ([]byte, error) {
				return exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries).CombinedOutput()
			}
			out, err := invoke()
			if err == nil || !strings.Contains(string(out), "signature mismatch") {
				t.Fatalf("changed harness-like output was discarded: %v %s", err, out)
			}
			newSig := pairingBoundSignature([2]string{"TestNativeConformance", output})
			if err := os.WriteFile(carries, []byte(record(newSig)), 0644); err != nil {
				t.Fatal(err)
			}
			out, err = invoke()
			if err != nil {
				t.Fatalf("matching full output rejected: %v %s", err, out)
			}
		})
	}
}
