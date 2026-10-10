package apextest

import (
	"path/filepath"
	"testing"
)

func TestRunConstructsCanvasTestWithUppercaseObjectKeys(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/CanvasTestConstructorTest.cls"), `
@IsTest
private class CanvasTestConstructorTest {
  @IsTest
  static void constructsCanvasTest() {
    Canvas.Test value = new Canvas.Test();
    System.assertNotEquals(null, value);
  }
}
`)

	run := Run(loadTestIndex(t, root), Options{})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		if len(run.Suites) > 0 && len(run.Suites[0].Cases) > 0 {
			t.Fatalf("summary = %#v problem = %#v", summary, run.Suites[0].Cases[0].Problem)
		}
		t.Fatalf("summary = %#v suites = %#v", summary, run.Suites)
	}
}
