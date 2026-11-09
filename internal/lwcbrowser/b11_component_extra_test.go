package lwcbrowser

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

func TestB11ComponentInteractionNative(t *testing.T) {
	f, err := os.Open("testdata/b11_component_extra/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			name := map[string]string{"empty": "emptyState", "button": "richTextToolbarButton"}[row[0]]
			data, err := json.Marshal(map[string]string{"case": row[0], "expected": row[2], "source": LightningBaseComponentModuleJS(name)})
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("node", "testdata/b11_components/extra.mjs")
			cmd.Stdin = bytes.NewReader(data)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("native interaction: %v\n%s", err, out)
			}
		})
	}
}
