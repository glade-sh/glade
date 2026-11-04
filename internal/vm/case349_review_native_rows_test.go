package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase349DecimalScaleNativeRows(t *testing.T) {
	f, err := os.Open("../../testdata/conformance/case349_review/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range rows[1:] {
		if row[1] != "anonymous" || row[3] != "decimal" {
			continue
		}
		var native struct {
			Left  string `json:"left"`
			Right string `json:"right"`
			Same  bool   `json:"same"`
		}
		if err := json.Unmarshal([]byte(row[4]), &native); err != nil {
			t.Fatal(err)
		}
		if got := storageValuesEqualForVM(storage.Field{}, storage.DecimalValue(native.Left), storage.DecimalValue(native.Right)); got != native.Same {
			t.Errorf("API %s %s vs %s: got %v, native %v", row[0], native.Left, native.Right, got, native.Same)
		}
		count++
	}
	if count != 6 {
		t.Fatalf("decimal native rows: got %d, want 6", count)
	}
}
