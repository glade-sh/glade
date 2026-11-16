package apextest

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

const testRunnerCoverageDir = "testdata/native_conformance/test_runner_coverage"

// nativeTestRows returns the org.tsv "test" rows for one capture tag as
// method -> {outcome, message}, with the org.tsv line of the first row.
func nativeTestRows(t *testing.T, api, tag string) (map[string][2]string, int) {
	t.Helper()
	f, err := os.Open(filepath.Join(testRunnerCoverageDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := map[string][2]string{}
	first := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	n := 0
	for scanner.Scan() {
		n++
		cols := strings.Split(scanner.Text(), "\t")
		if len(cols) != 8 || cols[1] != api || cols[2] != "test" || cols[3] != tag {
			continue
		}
		parts := strings.SplitN(cols[7], "|", 4)
		if len(parts) < 3 {
			t.Fatalf("org.tsv:%d malformed test row", n)
		}
		if first == 0 {
			first = n
		}
		out[parts[0]] = [2]string{parts[1], parts[2]}
	}
	if len(out) == 0 {
		t.Fatalf("no native rows for %s %s", api, tag)
	}
	return out, first
}

func fixtureClass(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testRunnerCoverageDir, "fixtures", name+".cls"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func comparableMessage(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "System.", "")
}

// NATIVE a TestSetup exception fails the class: the setup row and every test
// method row are Fail with the setup exception and its stack
// test_runner_coverage/org.tsv sf-*-sync, ic04-sync, cr4-fail-sync, API 62.0 and 67.0.
// A test method's own exception fails only that method.
func TestSetupFailureFailsEveryMethodNative(t *testing.T) {
	cases := []struct {
		class, tag string
		strict     bool
		failing    []string
		passing    []string
	}{
		{"SetupFailureProbe", "ic04-sync", false, []string{"first", "second"}, nil},
		{"RunnerSetupFail", "cr4-fail-sync", false, []string{"one", "two"}, nil},
		{"SetupThrowCustom", "sf-SetupThrowCustom-sync", false, []string{"first", "second"}, nil},
		{"SetupDmlFail", "sf-SetupDmlFail-sync", false, []string{"first", "second"}, nil},
		{"SetupNullDeref", "sf-SetupNullDeref-sync", false, []string{"first", "second"}, nil},
		{"SetupLimitFail", "sf-SetupLimitFail-sync", true, []string{"first", "second"}, nil},
		{"SetupMethodFail", "sf-SetupMethodFail-sync", false, []string{"first"}, []string{"second"}},
		{"SetupPass", "sf-SetupPass-sync", false, nil, []string{"first", "second"}},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.class, func(t *testing.T) {
				native, line := nativeTestRows(t, api, tc.tag)
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				for _, name := range []string{tc.class, "CvException"} {
					path := filepath.Join(root, "force-app/classes", name+".cls")
					writeFile(t, path, fixtureClass(t, name))
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				}
				opts := Options{NoDiskCache: true, Parallelism: 1}
				if tc.strict {
					opts.LimitMode = vm.LimitModeStrict
				}
				run := Run(loadTestIndex(t, root), opts)
				got := map[string]testreport.Case{}
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						if c.ClassName == tc.class {
							got[c.MethodName] = c
						}
					}
				}
				if len(got) != len(tc.failing)+len(tc.passing) {
					t.Fatalf("org.tsv:%d cases=%d want %d: %+v", line, len(got), len(tc.failing)+len(tc.passing), run.Summary())
				}
				for _, method := range tc.failing {
					c := got[method]
					if native[method][0] != "Fail" {
						t.Fatalf("org.tsv:%d native %s outcome=%s", line, method, native[method][0])
					}
					if c.Status != testreport.StatusFail || c.Problem == nil {
						t.Fatalf("org.tsv:%d %s status=%s want fail: %+v", line, method, c.Status, c.Problem)
					}
					have := comparableMessage(c.Problem.Type + ": " + c.Problem.Message)
					want := comparableMessage(native[method][1])
					if len(want) > 60 {
						want = want[:60]
					}
					if !strings.HasPrefix(have, want) {
						t.Fatalf("org.tsv:%d %s message %q does not start with native %q", line, method, have, want)
					}
				}
				for _, method := range tc.passing {
					if native[method][0] != "Pass" || got[method].Status != testreport.StatusPass {
						t.Fatalf("org.tsv:%d %s native=%s glade=%s", line, method, native[method][0], got[method].Status)
					}
				}
			})
		}
	}
}

// NATIVE SeeAllData precedence, test_runner_coverage/org.tsv run rows sad-*:
// class true with method false still sees org data (both methods saw the
// fixture), class false with method true opens it for that method only, and
// class false with method false or no annotation sees none.
func TestSeeAllDataPrecedenceNative(t *testing.T) {
	visible := map[string]map[string]bool{
		"SadClassTrueMethodFalse":  {"methodFalse": true, "inherits": true},
		"SadClassFalseMethodTrue":  {"methodTrue": true, "plain": false},
		"SadClassFalseMethodFalse": {"t": false},
		"SadDefault":               {"t": false},
	}
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
		for name := range visible {
			path := filepath.Join(root, "force-app/classes", name+".cls")
			writeFile(t, path, fixtureClass(t, name))
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
		}
		cases := Discover(loadTestIndex(t, root), Options{})
		seen := 0
		for _, c := range cases {
			want, ok := visible[c.ClassName][c.MethodName]
			if !ok {
				t.Fatalf("unexpected case %s.%s", c.ClassName, c.MethodName)
			}
			seen++
			if c.SeeAllData != want {
				t.Errorf("%s %s.%s SeeAllData=%v, native visible=%v", api, c.ClassName, c.MethodName, c.SeeAllData, want)
			}
		}
		if seen != 6 {
			t.Fatalf("%s: discovered %d methods", api, seen)
		}
	}
	// The native rows must say what the table above claims.
	for _, tag := range []string{"sad-SadClassTrueMethodFalse", "sad-SadClassFalseMethodTrue", "sad-SadClassFalseMethodFalse", "sad-SadDefault"} {
		for _, api := range []string{"62.0", "67.0"} {
			f, err := os.ReadFile(filepath.Join(testRunnerCoverageDir, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, line := range strings.Split(string(f), "\n") {
				cols := strings.Split(line, "\t")
				if len(cols) == 8 && cols[1] == api && cols[2] == "run" && cols[3] == tag {
					found = strings.Contains(cols[7], "outcome=Passed")
				}
			}
			if !found {
				t.Errorf("native %s %s did not pass; the visible-count assertions in the fixture encode the expectation", api, tag)
			}
		}
	}
}
