package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/project"
	oaerschema "github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/testreport"
	"github.com/open-aer/oaer/internal/typesys"
)

func TestRunExecutesAnonymousSubsetTestMethods(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/MathTest.cls"), `
@isTest
private class MathTest {
  @isTest static void adds() {
    Integer x = 1 + 1;
    System.assertEquals(2, x);
  }
  @TestSetup static void setup() {
    System.debug('setup');
  }
}
`)

	index := loadTestIndex(t, root)
	run := Run(index, Options{})
	summary := run.Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestRunReportsAssertionFailures(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/FailingTest.cls"), `
@isTest
private class FailingTest {
  @isTest static void fails() {
    System.assertEquals(3, 1 + 1);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Failed != 1 {
		t.Fatalf("summary = %#v", got)
	}
	if run.Suites[0].Cases[0].Status != testreport.StatusFail {
		t.Fatalf("case = %#v", run.Suites[0].Cases[0])
	}
}

func TestDiscoverFilter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/ManyTest.cls"), `
@isTest
private class ManyTest {
  @isTest static void first() { System.assert(true); }
  @isTest static void second() { System.assert(true); }
}
`)

	cases := Discover(loadTestIndex(t, root), Options{Filter: "second"})
	if len(cases) != 1 || cases[0].MethodName != "second" {
		t.Fatalf("cases = %#v", cases)
	}
}

func loadTestIndex(t *testing.T, root string) typesys.Index {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := oaerschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return typesys.Build(p, s)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
