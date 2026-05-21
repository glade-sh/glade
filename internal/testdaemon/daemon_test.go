package testdaemon

import (
	"os"
	"path/filepath"
	"testing"
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
