package apextest

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The native answers in testdata/conformance/c14_namespaced_cmdt_entity/org.tsv
// were captured on a namespaced org. An unprefixed and a prefixed custom
// metadata record both read back with prefixed QualifiedApiName values. The
// test class that ran in the org runs here unchanged and must pass with the
// same values.
func TestC14NamespacedCustomMetadataEntityConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "c14_namespaced_cmdt_entity")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var expected []string
	scan := bufio.NewScanner(file)
	scan.Scan()
	for scan.Scan() {
		cols := strings.Split(scan.Text(), "\t")
		if len(cols) != 5 {
			t.Fatalf("bad org.tsv row %q", scan.Text())
		}
		if cols[0] == "62.0" && cols[4] == "pass" && strings.Contains(cols[3], "__") {
			expected = append(expected, cols[3])
		}
	}
	if len(expected) != 3 {
		t.Fatalf("62.0 value rows = %v", expected)
	}
	source, err := os.ReadFile(filepath.Join(dir, "src", "classes", "C14Probe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range expected {
		if !strings.Contains(string(source), "'"+want+"'") {
			t.Fatalf("test class does not assert native value %q", want)
		}
	}

	root := t.TempDir()
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if rel == "org.tsv" || rel == "ORG-RUN.txt" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, rel), string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	data, _ := json.Marshal(run)
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Errors != 0 {
		t.Fatalf("namespaced custom metadata entity conformance: %s", data)
	}
}
