package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The branch checker receives synthetic Go test events; no Salesforce or runtime is involved.
func TestConformancePairingGuard(t *testing.T) {
	script, err := filepath.Abs("conformance-pairing.py")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, events, carries string
		pass                  bool
	}{
		{"passing fix", `{"Action":"pass","Package":"example/internal/apextest","Test":"TestSampleConformance"}` + "\n" + `{"Action":"pass","Package":"example/internal/apextest"}`, `[]`, true},
		{"red unpaired", `{"Action":"output","Package":"example/internal/apextest","Test":"TestSampleConformance","Output":"assertion failed\n"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest","Test":"TestSampleConformance"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest"}`, `[]`, false},
		{"counted carry", `{"Action":"output","Package":"example/internal/apextest","Test":"TestSampleConformance","Output":"assertion failed\n"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest","Test":"TestSampleConformance"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest"}`, `[{"package":"internal/apextest","test":"TestSampleConformance","count":1,"failures":["TestSampleConformance"],"signature":"9597a00ccb6aa9c8e4002e6c7fea4bcb44dd76903133fd5366e71f56ccb6f710","owner":"owner","reason":"tracked failure"},{"package":"internal/apextest","test":"@package","count":1,"signature":"9597a00ccb6aa9c8e4002e6c7fea4bcb44dd76903133fd5366e71f56ccb6f710","owner":"owner","reason":"tracked package failure"}]`, true},
		{"wrong count", `{"Action":"output","Package":"example/internal/apextest","Test":"TestSampleConformance","Output":"assertion failed\n"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest","Test":"TestSampleConformance"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest"}`, `[{"package":"internal/apextest","test":"TestSampleConformance","count":2,"failures":["TestSampleConformance","TestSampleConformance/other"],"signature":"9597a00ccb6aa9c8e4002e6c7fea4bcb44dd76903133fd5366e71f56ccb6f710","owner":"owner","reason":"tracked failure"}]`, false},
		{"inconsistent package", `{"Action":"fail","Package":"example/internal/apextest","Test":"TestSampleConformance"}` + "\n" + `{"Action":"pass","Package":"example/internal/apextest"}`, `[{"package":"internal/apextest","test":"TestSampleConformance","count":1,"failures":["TestSampleConformance"],"signature":"9597a00ccb6aa9c8e4002e6c7fea4bcb44dd76903133fd5366e71f56ccb6f710","owner":"owner","reason":"tracked failure"},{"package":"internal/apextest","test":"@package","count":1,"signature":"9597a00ccb6aa9c8e4002e6c7fea4bcb44dd76903133fd5366e71f56ccb6f710","owner":"owner","reason":"tracked package failure"}]`, false},
		{"missing package result", `{"Action":"pass","Package":"example/internal/apextest","Test":"TestSampleConformance"}`, `[]`, false},
		{"missing result", `{"Action":"pass","Package":"example/internal/apextest"}`, `[]`, false},
		{"skip", `{"Action":"skip","Package":"example/internal/apextest","Test":"TestSampleConformance"}` + "\n" + `{"Action":"pass","Package":"example/internal/apextest"}`, `[]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			run("git", "commit", "-q", "--allow-empty", "-m", "base")
			os.MkdirAll(filepath.Join(dir, "internal/apextest"), 0755)
			os.WriteFile(filepath.Join(dir, "internal/apextest/sample_conformance_test.go"), []byte("package apextest\nfunc TestSampleConformance(t *testing.T) {}\n"), 0644)
			run("git", "add", ".")
			run("git", "commit", "-q", "-m", "test")
			events := filepath.Join(t.TempDir(), "events.jsonl")
			carries := filepath.Join(t.TempDir(), "carries.json")
			os.WriteFile(events, []byte(tc.events+"\n"), 0644)
			os.WriteFile(carries, []byte(tc.carries), 0644)
			fake := filepath.Join(t.TempDir(), "fake-go")
			status := "0"
			if strings.Contains(tc.events, `"Action":"fail"`) {
				status = "1"
			}
			if err := os.WriteFile(fake, []byte("#!/bin/sh\ncat '"+events+"'\nexit "+status+"\n"), 0755); err != nil {
				t.Fatal(err)
			}
			c := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries)
			out, err := c.CombinedOutput()
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%v, error=%v output=%s", tc.pass, err, strings.TrimSpace(string(out)))
			}
			if tc.name == "red unpaired" && !strings.Contains(string(out), "red without matching counted CARRIED record") {
				t.Fatalf("unpaired test was not rejected for the expected reason: %s", out)
			}
		})
	}
}

func TestConformancePairingGuardModified(t *testing.T) {
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
	if err := os.MkdirAll(filepath.Join(dir, "internal/apextest"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "internal/apextest/example_conformance_test.go")
	if err := os.WriteFile(path, []byte("package apextest\nfunc TestExistingConformance(t *testing.T) {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "base")
	if err := os.WriteFile(path, []byte("package apextest\nfunc TestExistingConformance(t *testing.T) { t.Fatal(\"red\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "modified")
	events := filepath.Join(t.TempDir(), "events.jsonl")
	carries := filepath.Join(t.TempDir(), "carries.json")
	if err := os.WriteFile(events, []byte(`{"Action":"fail","Package":"example/internal/apextest","Test":"TestExistingConformance"}`+"\n"+`{"Action":"fail","Package":"example/internal/apextest"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(t.TempDir(), "fake-go")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\ncat '"+events+"'\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries)
	if out, err := c.CombinedOutput(); err == nil {
		t.Fatalf("modified red conformance test accepted: %s", out)
	}
}

// A fabricated event file must not be accepted as proof of a green branch.
func TestConformancePairingRejectsSuppliedEvents(t *testing.T) {
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
	run("git", "commit", "-q", "--allow-empty", "-m", "base")
	if err := os.MkdirAll(filepath.Join(dir, "internal/apextest"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/apextest/case_test.go"), []byte("package apextest\nfunc TestUnnamed(t *testing.T) { t.Fatal(\"red\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "test")
	events := filepath.Join(t.TempDir(), "events.jsonl")
	carries := filepath.Join(t.TempDir(), "carries.json")
	if err := os.WriteFile(events, []byte(`{"Action":"pass","Package":"example/internal/apextest","Test":"TestUnnamed"}`+"\n"+`{"Action":"pass","Package":"example/internal/apextest"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--events", events, "--carries", carries)
	if out, err := c.CombinedOutput(); err == nil {
		t.Fatalf("accepted fabricated passing event file: %s", out)
	}
}

func TestConformancePairingRejectsRunnerFailure(t *testing.T) {
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
	run("git", "commit", "-q", "--allow-empty", "-m", "base")
	if err := os.MkdirAll(filepath.Join(dir, "internal/apextest"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/apextest/case_test.go"), []byte("package apextest\nfunc TestUnnamed(t *testing.T) {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "test")
	carries := filepath.Join(t.TempDir(), "carries.json")
	if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(t.TempDir(), "fake-go")
	output := `{"Action":"pass","Package":"example/internal/apextest","Test":"TestUnnamed"}` + "\n" + `{"Action":"pass","Package":"example/internal/apextest"}` + "\n"
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+output+"'\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries)
	if out, err := c.CombinedOutput(); err == nil {
		t.Fatalf("accepted failing runner with green events: %s", out)
	}
}

func TestConformancePairingCarryMustNamePackage(t *testing.T) {
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
	run("git", "commit", "-q", "--allow-empty", "-m", "base")
	if err := os.MkdirAll(filepath.Join(dir, "internal/apextest"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/apextest/case_test.go"), []byte("package apextest\nfunc TestShared(t *testing.T) {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "test")
	carries := filepath.Join(t.TempDir(), "carries.json")
	if err := os.WriteFile(carries, []byte(`[{"test":"TestShared","count":1,"owner":"owner","reason":"tracked"}]`), 0644); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(t.TempDir(), "fake-go")
	output := `{"Action":"fail","Package":"example/internal/apextest","Test":"TestShared"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest"}` + "\n"
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+output+"'\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries)
	if out, err := c.CombinedOutput(); err == nil {
		t.Fatalf("accepted carry without package identity: %s", out)
	}
}

func TestConformancePairingDiscoverySpelling(t *testing.T) {
	script, err := filepath.Abs("conformance-pairing.py")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, decl, test string }{
		{"comment between tokens", "func /* comment */ TestOdd(t *testing.T) {}", "TestOdd"},
		{"unicode identifier", "func TestÉclair(t *testing.T) {}", "TestÉclair"},
		{"bare Test identifier", "func Test(t *testing.T) {}", "Test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			run("git", "commit", "-q", "--allow-empty", "-m", "base")
			if err := os.MkdirAll(filepath.Join(dir, "internal/apextest"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "internal/apextest/case_test.go"), []byte("package apextest\n"+tc.decl+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			run("git", "add", ".")
			run("git", "commit", "-q", "-m", "test")
			carries := filepath.Join(t.TempDir(), "carries.json")
			if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(t.TempDir(), "fake-go")
			output := `{"Action":"fail","Package":"example/internal/apextest","Test":"` + tc.test + `"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest"}` + "\n"
			if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+output+"'\nexit 1\n"), 0755); err != nil {
				t.Fatal(err)
			}
			c := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries)
			if out, err := c.CombinedOutput(); err == nil {
				t.Fatalf("red declaration not selected: %s", out)
			}
		})
	}
}

// A red package without a failed selected test is an unpaired failure, even
// when all test events are green. Check the reason, not just the exit status.
func TestConformancePairingRejectsUnpairedPackageFailure(t *testing.T) {
	script, err := filepath.Abs("conformance-pairing.py")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, detail string }{
		{"testmain exit", "TestMain exited after tests"},
		{"package teardown", "package cleanup failed"},
		{"package panic", "panic during package shutdown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			run("git", "commit", "-q", "--allow-empty", "-m", "base")
			if err := os.MkdirAll(filepath.Join(dir, "internal/apextest"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "internal/apextest/case_test.go"), []byte("package apextest\nfunc TestObserved(t *testing.T) {}\n"), 0644); err != nil {
				t.Fatal(err)
			}
			run("git", "add", ".")
			run("git", "commit", "-q", "-m", "test")
			carries := filepath.Join(t.TempDir(), "carries.json")
			if err := os.WriteFile(carries, []byte("[]"), 0644); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(t.TempDir(), "fake-go")
			events := `{"Action":"pass","Package":"example/internal/apextest","Test":"TestObserved"}` + "\n" + `{"Action":"output","Package":"example/internal/apextest","Output":"` + tc.detail + `"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest"}` + "\n"
			if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+events+"'\nexit 1\n"), 0755); err != nil {
				t.Fatal(err)
			}
			c := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries)
			out, err := c.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "package failed without a failed selected test") {
				t.Fatalf("unpaired red package was not correctly rejected: error=%v output=%s", err, out)
			}
		})
	}
}

// A test carry does not establish that a package-level failure has been counted.
func TestConformancePairingRequiresPackageCarry(t *testing.T) {
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
	run("git", "commit", "-q", "--allow-empty", "-m", "base")
	if err := os.MkdirAll(filepath.Join(dir, "internal/apextest"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/apextest/case_test.go"), []byte("package apextest\nfunc TestCarried(t *testing.T) {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "test")
	carries := filepath.Join(t.TempDir(), "carries.json")
	if err := os.WriteFile(carries, []byte(`[{"package":"internal/apextest","test":"TestCarried","count":1,"failures":["TestCarried"],"signature":"c106378342cc1ba92aae53e481812cc091d8e2781eeb186173bee5acfaf86fd5","owner":"owner","reason":"known test"}]`), 0644); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(t.TempDir(), "fake-go")
	events := `{"Action":"output","Package":"example/internal/apextest","Test":"TestCarried","Output":"old assertion\n"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest","Test":"TestCarried"}` + "\n" + `{"Action":"output","Package":"example/internal/apextest","Output":"TestMain failed too"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest"}` + "\n"
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+events+"'\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries)
	out, err := c.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "package failure requires counted CARRIED record") {
		t.Fatalf("package failure was not correctly rejected: error=%v output=%s", err, out)
	}
}

// A carry for one native case must not authorize another failure with the same count.
func TestConformancePairingRejectsUnpairedChangedCases(t *testing.T) {
	script, err := filepath.Abs("conformance-pairing.py")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, carried, actual string }{
		{"different API route", "62/anonymous", "67/anonymous"},
		{"different assertion", "67/accepted", "67/rejected"},
		{"different execution route", "62/anonymous", "62/project"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			run("git", "commit", "-q", "--allow-empty", "-m", "base")
			if err := os.MkdirAll(filepath.Join(dir, "internal/apextest"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "internal/apextest/native_conformance_test.go"), []byte("package apextest\nfunc TestNativeConformance(t *testing.T) {}\n"), 0644); err != nil {
				t.Fatal(err)
			}
			run("git", "add", ".")
			run("git", "commit", "-q", "-m", "test")
			carries := filepath.Join(t.TempDir(), "carries.json")
			records := `[{"package":"internal/apextest","test":"TestNativeConformance","count":1,"owner":"owner","reason":"previously carried ` + tc.carried + `"},{"package":"internal/apextest","test":"@package","count":1,"signature":"9597a00ccb6aa9c8e4002e6c7fea4bcb44dd76903133fd5366e71f56ccb6f710","owner":"owner","reason":"known package"}]`
			if err := os.WriteFile(carries, []byte(records), 0644); err != nil {
				t.Fatal(err)
			}
			fake := filepath.Join(t.TempDir(), "fake-go")
			events := `{"Action":"output","Package":"example/internal/apextest","Test":"TestNativeConformance/` + tc.actual + `","Output":"assertion failed\n"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest","Test":"TestNativeConformance/` + tc.actual + `"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest","Test":"TestNativeConformance"}` + "\n" + `{"Action":"fail","Package":"example/internal/apextest"}` + "\n"
			if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' '"+events+"'\nexit 1\n"), 0755); err != nil {
				t.Fatal(err)
			}
			c := exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries)
			out, err := c.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "carry requires exact failed case identities") {
				t.Fatalf("unpaired changed failure was not correctly rejected: error=%v output=%s", err, out)
			}
			for _, check := range []struct {
				caseID string
				pass   bool
			}{{tc.carried, false}, {tc.actual, true}} {
				records = `[{"package":"internal/apextest","test":"TestNativeConformance","count":1,"failures":["TestNativeConformance/` + check.caseID + `"],"signature":"` + pairingBoundSignature([2]string{"TestNativeConformance/" + tc.actual, "assertion failed\n"}) + `","owner":"owner","reason":"counted case"},{"package":"internal/apextest","test":"@package","count":1,"signature":"` + pairingBoundSignature([2]string{"TestNativeConformance/" + tc.actual, "assertion failed\n"}) + `","owner":"owner","reason":"known package"}]`
				if err := os.WriteFile(carries, []byte(records), 0644); err != nil {
					t.Fatal(err)
				}
				c = exec.Command("python3", script, "--repo", dir, "--base", "HEAD^", "--go", fake, "--carries", carries)
				out, err = c.CombinedOutput()
				if check.pass && err != nil {
					t.Fatalf("matching carry rejected: %v %s", err, out)
				}
				if !check.pass && (err == nil || !strings.Contains(string(out), "red without matching counted CARRIED record")) {
					t.Fatalf("different case was not rejected: %v %s", err, out)
				}
			}
		})
	}
}
