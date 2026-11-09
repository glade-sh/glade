package lwcbrowser

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// These rows record the desktop Aura route, not mobile hardware or LWR.
func TestB11SmallExportsNative(t *testing.T) {
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
		if row[0] != "graph" && row[0] != "refresh" && row[0] != "mobile" {
			continue
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			source := ""
			switch row[0] {
			case "refresh":
				source = RefreshModuleJS()
			case "mobile":
				source, _ = SimulatedNativeAPIModuleJS("mobileCapabilities")
			}
			input, err := json.Marshal(map[string]string{"case": row[0], "source": source, "expected": row[2]})
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("node", "testdata/b11_small_exports/module.mjs")
			cmd.Stdin = bytes.NewReader(input)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("native comparison: %v: %s", err, strings.TrimSpace(string(out)))
			}
		})
	}
}
