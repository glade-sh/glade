package automation

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowMergeFormatNativeConstants(t *testing.T) {
	const dir = "testdata/flow_merge_format"
	b, err := os.ReadFile(filepath.Join(dir, "Format_constant.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var raw flowXML
	if err := xml.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	rows, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range strings.Split(string(rows), "\n") {
		cells := strings.Split(row, "\t")
		if len(cells) != 4 || cells[2] != "constant" {
			continue
		}
		count++
		t.Run(cells[0]+"/"+cells[1], func(t *testing.T) {
			got := modeledFlowCustomError(raw.CustomErrors[0], raw.Constants).Messages[0].Message
			if got != "constant={!large}|negative={!negative}|boundary={!boundary}|below=999.5" {
				t.Fatalf("metadata resolved numeric text early: %q (native runtime row %q)", got, cells[3])
			}
		})
	}
	if count != 4 {
		t.Fatalf("got %d native rows", count)
	}
}
