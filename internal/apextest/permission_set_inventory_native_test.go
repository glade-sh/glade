package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The query headers are native unsaturated counts, captured independently for
// both API versions and routes in permission_set_inventory/org.tsv.
func TestPermissionSetInventoryNativeCounts(t *testing.T) {
	f, err := os.Open("testdata/conformance/permission_set_inventory/org.tsv")
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
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	observations := 0
	for _, row := range rows[1:] {
		if row[5] != "queryHeader" {
			continue
		}
		var header struct {
			QueryID string `json:"queryId"`
			Count   int    `json:"reportedCount"`
		}
		if err := json.Unmarshal([]byte(row[8]), &header); err != nil {
			t.Fatal(err)
		}
		if header.QueryID != "all" {
			continue
		}
		observations++
		if got := len(org.Objects["PermissionSet"].Records); got != header.Count {
			t.Errorf("API %s %s: permission sets = %d, native = %d", row[2], row[3], got, header.Count)
		}
	}
	if observations != 4 {
		t.Fatalf("expected four native observations, got %d", observations)
	}
}
