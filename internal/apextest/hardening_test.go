package apextest

import (
	"path/filepath"
	"testing"
)

func TestNoPanicOnUnsupportedTestSource(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/classes/BrokenTest.cls"), `
@isTest
private class BrokenTest {
  @isTest static void unsupported() {
    System.assertEquals(1, MissingType.call());
  }
}
`)
	index := loadTestIndex(t, root)
	assertNoPanic(t, func() {
		_ = Run(index, Options{})
	})
}

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}
