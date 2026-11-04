package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCase324NamespaceTokenScan(t *testing.T) {
	root := t.TempDir()
	for path, data := range map[string]string{"a.cls": "Alpha__Thing; PrefixLong__Thing", "nested/b.trigger": "Beta__Thing", ".hidden/c.cls": "Hidden__Thing", "ignored.txt": "Text__Thing"} {
		file := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for token, want := range map[string]bool{"Alpha__": true, "Beta__": true, "Prefix__": false, "PrefixLong__": true, "alpha__": false, "Hidden__": false, "Text__": false} {
		if got := referencesNamespaceTokenInRoot(root, token); got != want {
			t.Errorf("%s: got %t want %t", token, got, want)
		}
	}
}

func TestCase324SharedNamespaceScanMatchesSingleScans(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sample.cls"), []byte("Alpha__X AlphaLong__Y BETA__Z"), 0644); err != nil {
		t.Fatal(err)
	}
	tokens := map[string]bool{"Alpha__": false, "AlphaLong__": false, "alpha__": false, "BETA__": false, "Absent__": false}
	referencesNamespaceTokensInRoot(root, tokens)
	for token, got := range tokens {
		if want := referencesNamespaceTokenInRoot(root, token); got != want {
			t.Errorf("%q got %v want %v", token, got, want)
		}
	}
}
