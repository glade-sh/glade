package sema

import (
	"os"
	"path/filepath"
	"testing"
)

func unlockedCapturedProvider(t *testing.T, root string) string {
	t.Helper()
	fixture := filepath.Join("..", "..", "testdata", "conformance", "namespace_unlocked_subscriber", "provider", "package", "main", "default", "classes", "V036PkgOuter.cls")
	source, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "V036PkgOuter.cls")
	writeSemaFile(t, path, string(source))
	return path
}
