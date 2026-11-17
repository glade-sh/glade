package soql

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// TestSortRecordsByOrderMatchesReference pins the sort keys, the stored-field
// value shortcut and the top-N selection to a plain stable sort over
// recordValue and orderCompare.
func TestSortRecordsByOrderMatchesReference(t *testing.T) {
	words := []string{"acme", "ACME", "Acme x", "acme  y", "b", "B", "bb", "", "Zed", "zed", "_a", "~t", "Ünï", "ünï", "a1", "A10", "a2"}
	for _, locale := range []string{"", "en_US", "de_DE"} {
		comparator := newTextComparator(locale)
		for _, desc := range []bool{false, true} {
			for seed := int64(1); seed <= 12; seed++ {
				for _, ascii := range []bool{true, false} {
					random := rand.New(rand.NewSource(seed))
					org := likeBenchOrg(0)
					state := org.Objects["Account"]
					n := 20 + random.Intn(200)
					nulls := seed%4 == 0
					for i := 0; i < n; i++ {
						id := storage.ID(fmt.Sprintf("001%012d", i+1))
						name := words[random.Intn(len(words))]
						if ascii && name != "" && name[0] >= 0x80 {
							name = "plain"
						}
						rec := storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{
							"Name":        storage.StringValue(name),
							"BillingCity": storage.StringValue(words[random.Intn(len(words))]),
						}}
						if nulls && i%7 == 0 {
							delete(rec.Fields, "Name")
							rec.ExplicitNulls = map[string]bool{"Name": true}
						}
						state.Records[id] = rec
					}
					definition := state.Definition
					var records []storage.Record
					for i := 0; i < n; i++ {
						records = append(records, state.Records[storage.ID(fmt.Sprintf("001%012d", i+1))])
					}
					random.Shuffle(len(records), func(i, j int) { records[i], records[j] = records[j], records[i] })
					order := collateOrder([]OrderSpec{{Field: "Name", Desc: desc}, {Field: "BillingCity"}}, comparator)
					want := append([]storage.Record(nil), records...)
					sort.SliceStable(want, func(i, j int) bool {
						for _, spec := range order {
							left, _ := recordValue(org, definition, want[i], spec.Field)
							right, _ := recordValue(org, definition, want[j], spec.Field)
							if cmp := orderCompare(left, right, spec); cmp != 0 {
								return cmp < 0
							}
						}
						return false
					})
					for _, top := range []int{0, 1, 5, 17, n / 3} {
						got := append([]storage.Record(nil), records...)
						sortRecordsByOrderTop(org, definition, got, order, false, false, top)
						keep := len(got)
						if top > 0 && top < len(got)/2 {
							keep = top
						}
						for i := 0; i < keep; i++ {
							if got[i].ID != want[i].ID {
								t.Fatalf("locale %q desc %v seed %d ascii %v top %d row %d: got %s want %s", locale, desc, seed, ascii, top, i, got[i].ID, want[i].ID)
							}
						}
						seen := map[storage.ID]bool{}
						for _, record := range got {
							seen[record.ID] = true
						}
						if len(seen) != len(records) {
							t.Fatalf("top %d lost or duplicated rows", top)
						}
					}
				}
			}
		}
	}
}
