package soql

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// referenceSortRecords orders records the way sortRecordsByOrder did before it
// resolved sort values once per record: a stable sort that reads both values
// through recordValue on every comparison. It covers plain fields only.
func referenceSortRecords(org storage.OrgState, definition storage.ObjectDefinition, records []storage.Record, order []OrderSpec) {
	sort.SliceStable(records, func(i, j int) bool {
		for _, spec := range order {
			left, ok := recordValue(org, definition, records[i], spec.Field)
			if !ok {
				left = storage.NullValue()
			}
			right, ok := recordValue(org, definition, records[j], spec.Field)
			if !ok {
				right = storage.NullValue()
			}
			if cmp := orderCompare(left, right, spec); cmp != 0 {
				return cmp < 0
			}
		}
		return false
	})
}

func TestSortRecordsByOrderMatchesPerComparisonResolution(t *testing.T) {
	org := benchmarkSOQLOrg(1)
	definition := org.Objects["Account"].Definition
	rng := rand.New(rand.NewSource(20261116))
	alphabet := []string{"a", "A", "b", "B", "-", " ", "_", "1", "é", "Z"}
	build := func(n int) []storage.Record {
		records := make([]storage.Record, n)
		for i := range records {
			fields := map[string]storage.Value{}
			if rng.Intn(10) != 0 {
				length := 1 + rng.Intn(5)
				text := ""
				for k := 0; k < length; k++ {
					text += alphabet[rng.Intn(len(alphabet))]
				}
				fields["Name"] = storage.StringValue(text)
			}
			if rng.Intn(4) != 0 {
				fields["Site"] = storage.StringValue(fmt.Sprintf("s%d", rng.Intn(5)))
			}
			records[i] = storage.Record{ID: storage.ID(fmt.Sprintf("001%012d", i+1)), Object: "Account", Fields: fields}
		}
		return records
	}
	orders := [][]OrderSpec{
		{{Field: "Name"}},
		{{Field: "Name", Desc: true}},
		{{Field: "Site", Nulls: "LAST"}, {Field: "Name", Desc: true}},
		{{Field: "Site", Desc: true}, {Field: "Name", Nulls: "LAST"}},
	}
	for _, n := range []int{0, 1, 2, 50, 10000} {
		for _, order := range orders {
			got := build(n)
			want := append([]storage.Record(nil), got...)
			sortRecordsByOrder(org, definition, got, collateOrder(order, newTextComparator("en_US")), false, false)
			referenceSortRecords(org, definition, want, collateOrder(order, newTextComparator("en_US")))
			for i := range got {
				if got[i].ID != want[i].ID {
					t.Fatalf("n=%d order=%v: position %d = %s; want %s", n, order, i, got[i].ID, want[i].ID)
				}
			}
		}
	}
}

// A column whose every value is text sorts by its texts alone, without
// reading value kinds. The order must stay the one orderCompare gives, for
// every comparator kind (folded, en_*, root and tailored locales).
func TestSortRecordsByOrderTextColumnsMatchOrderCompare(t *testing.T) {
	org := benchmarkSOQLOrg(1)
	definition := org.Objects["Account"].Definition
	rng := rand.New(rand.NewSource(20261122))
	alphabet := []string{"a", "A", "b", "B", "-", " ", "1", "é", "Z", "ä", "ö", "å", "ß", "ss", "中", "あ", "z"}
	build := func(n int) []storage.Record {
		records := make([]storage.Record, n)
		for i := range records {
			text := ""
			for k, length := 0, 1+rng.Intn(4); k < length; k++ {
				text += alphabet[rng.Intn(len(alphabet))]
			}
			site := alphabet[rng.Intn(len(alphabet))] + alphabet[rng.Intn(len(alphabet))]
			records[i] = storage.Record{ID: storage.ID(fmt.Sprintf("001%012d", i+1)), Object: "Account", Fields: map[string]storage.Value{
				"Name": storage.StringValue(text),
				"Site": storage.StringValue(site),
			}}
		}
		return records
	}
	orders := [][]OrderSpec{
		{{Field: "Name"}},
		{{Field: "Name", Desc: true}},
		{{Field: "Site", Nulls: "LAST"}, {Field: "Name", Desc: true}},
		{{Field: "Site", Desc: true}, {Field: "Name"}},
	}
	for _, locale := range []string{"", "en_US", "de_DE", "sv_SE", "ja_JP", "zh_CN"} {
		for _, n := range []int{2, 50, 1500} {
			for _, order := range orders {
				got := build(n)
				want := append([]storage.Record(nil), got...)
				sortRecordsByOrder(org, definition, got, collateOrder(order, newTextComparator(locale)), false, false)
				referenceSortRecords(org, definition, want, collateOrder(order, newTextComparator(locale)))
				for i := range got {
					if got[i].ID != want[i].ID {
						t.Fatalf("locale=%q n=%d order=%v: position %d = %s; want %s", locale, n, order, i, got[i].ID, want[i].ID)
					}
				}
			}
		}
	}
}
