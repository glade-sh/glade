package testdaemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

func TestDaemonRunsFilterAgainstWarmProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmOneTest.cls"), `
@isTest
private class WarmOneTest {
  @isTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmTwoTest.cls"), `
@isTest
private class WarmTwoTest {
  @isTest static void passes() { System.assertEquals(3, 1 + 2); }
}
`)

	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	first := daemon.RunFilter("WarmOneTest")
	if got := first.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("first summary = %#v", got)
	}
	second := daemon.RunFilter("WarmTwoTest")
	if got := second.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("second summary = %#v", got)
	}
}

func TestDaemonRunSelectionNarrowsMultipleDirectClasses(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmOneTest.cls"), `
@isTest
private class WarmOneTest {
  @isTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmTwoTest.cls"), `
@isTest
private class WarmTwoTest {
  @isTest static void passes() { System.assertEquals(3, 1 + 2); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmThreeTest.cls"), `
@isTest
private class WarmThreeTest {
  @isTest static void passes() { System.assertEquals(4, 2 + 2); }
}
`)

	daemon, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	run := daemon.RunSelectionContext(context.Background(), apextest.Options{}, watch.TestSelection{
		Mode:        watch.SelectionDirect,
		TestClasses: []string{"WarmOneTest", "WarmTwoTest"},
	})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary = %#v run=%#v", got, run)
	}
	if runHasClass(run, "WarmThreeTest") {
		t.Fatalf("run included unselected class WarmThreeTest: %#v", run)
	}
}

func runHasClass(run testreport.Run, className string) bool {
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.ClassName == className {
				return true
			}
		}
	}
	return false
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
