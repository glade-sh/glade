package lwcbrowser

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

// A named import must load even when the native Aura host exposes undefined.
func TestB11NavigationAuraBindingsNative(t *testing.T) {
	f, err := os.Open("testdata/b11_small_exports/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[0] != "nav" {
			continue
		}
		t.Run(row[1], func(t *testing.T) {
			input, err := json.Marshal(map[string]string{"source": NavigationModuleJS(), "expected": row[2]})
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("node", "testdata/b11_small_exports/navigation.mjs")
			cmd.Stdin = bytes.NewReader(input)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("navigation binding: %v\n%s", err, out)
			}
		})
	}
}
