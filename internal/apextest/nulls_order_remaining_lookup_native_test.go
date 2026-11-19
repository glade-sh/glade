package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
)

func TestNullsOrderRemainingLookupClassification(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata/conformance/nulls_order_remaining_lookup", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 377 {
		t.Fatalf("native observations=%d, want 376", len(rows)-1)
	}
	checked := 0
	for _, row := range rows[1:] {
		var obs map[string]any
		if err := json.Unmarshal([]byte(row[3]), &obs); err != nil {
			t.Fatal(err)
		}
		if _, skip := obs["skip"]; skip {
			continue
		}
		checked++
		order := nullsOrderOf(obs)
		ignored := nullsOrderEqual(order, nullsOrderIgnored)
		if !ignored && !nullsOrderEqual(order, nullsOrderHonoured) {
			t.Errorf("%s %s.%s unclassified order %v", row[0], row[1], row[2], order)
			continue
		}
		if got := soql.StandardLookupIgnoresNullsClause(row[1], row[2]); got != ignored {
			t.Errorf("%s %s.%s native ignores NULLS=%v; Glade=%v", row[0], row[1], row[2], ignored, got)
		}
	}
	if checked != 22 {
		t.Fatalf("classified observations=%d, want 22", checked)
	}
}
