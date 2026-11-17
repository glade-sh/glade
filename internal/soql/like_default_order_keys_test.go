package soql

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// asciiPairTexts holds every one-character ASCII string, the empty string and
// every two-character string over a mixed alphabet, so prefix and case ties are
// covered.
func asciiPairTexts() []string {
	texts := []string{""}
	for c := 0; c < 128; c++ {
		texts = append(texts, string(rune(c)))
	}
	alphabet := "aAbB zZ_~!09\x00\x1f\x7f"
	for _, a := range alphabet {
		for _, b := range alphabet {
			texts = append(texts, string(a)+string(b))
		}
	}
	return texts
}

func TestASCIITextSortKeysMatchComparators(t *testing.T) {
	texts := asciiPairTexts()
	for _, test := range []struct {
		name       string
		comparator *textComparator
		compare    func(a, b string) int
	}{
		{"folded", nil, compareSOQLTextFolded},
		{"english", &textComparator{locale: "en_US", english: true}, compareSOQLTextCollated},
	} {
		keys := asciiTextSortKeys(texts, test.comparator)
		if keys == nil {
			t.Fatalf("%s: no keys for ASCII text", test.name)
		}
		common, prefixes := keyPrefixes(keys)
		for i := range texts {
			for j := range texts {
				want := sign(test.compare(texts[i], texts[j]))
				if got := sign(bytes.Compare(keys[i], keys[j])); got != want {
					t.Fatalf("%s: %q vs %q: key order %d, comparator %d", test.name, texts[i], texts[j], got, want)
				}
				got := 0
				if prefixes[i] != prefixes[j] {
					got = 1
					if prefixes[i] < prefixes[j] {
						got = -1
					}
				} else {
					got = sign(bytes.Compare(keys[i][common:], keys[j][common:]))
				}
				if got != want {
					t.Fatalf("%s: %q vs %q: prefix order %d, comparator %d", test.name, texts[i], texts[j], got, want)
				}
			}
		}
	}
	// Non-ASCII text, a non-English locale, or one null disables the keys.
	if asciiTextSortKeys([]string{"a", "é"}, nil) != nil {
		t.Fatal("non-ASCII text produced keys")
	}
	if asciiTextSortKeys([]string{"a"}, &textComparator{locale: "de_DE"}) != nil {
		t.Fatal("non-English comparator produced keys")
	}
}

func TestPlainStoredStringOrderFieldFallbacks(t *testing.T) {
	org := storage.NewOrgState()
	str := func(name string) storage.Field { return storage.Field{APIName: name, Type: storage.FieldString} }
	account := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{
		"Id": {APIName: "Id", Type: storage.FieldID}, "Name": str("Name"), "Site": str("Site"),
		"NamespacePrefix": str("NamespacePrefix"), "OwnerId": str("OwnerId"),
		"Calc__c":  {APIName: "Calc__c", Type: storage.FieldString, Formula: "Name"},
		"Count__c": {APIName: "Count__c", Type: storage.FieldCalculated},
	}}
	contact := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
		"Id": {APIName: "Id", Type: storage.FieldID}, "LastName": str("LastName"),
	}}
	mdt := storage.ObjectDefinition{APIName: "Thing__mdt", Fields: map[string]storage.Field{"Name": str("Name")}}
	organization := storage.ObjectDefinition{APIName: "Organization", Fields: map[string]storage.Field{"Name": str("Name")}}
	for _, test := range []struct {
		name       string
		definition storage.ObjectDefinition
		field      string
		plain      bool
	}{
		{"stored text", account, "Name", true},
		{"stored text case", account, "site", true},
		{"formula", account, "Calc__c", false},
		{"non-text type", account, "Count__c", false},
		{"id", account, "Id", false},
		{"system field", account, "OwnerId", false},
		{"namespace prefix", account, "NamespacePrefix", false},
		{"relationship path", account, "Owner.Name", false},
		{"function expression", account, "toLabel(Name)", false},
		{"contact derived Name", contact, "Name", false},
		{"custom metadata", mdt, "Name", false},
		{"organization", organization, "Name", false},
	} {
		if _, got := plainStoredStringOrderField(org, test.definition, test.field); got != test.plain {
			t.Errorf("%s: plain = %v; want %v", test.name, got, test.plain)
		}
	}
}

// TestSortShortcutFallbackValues checks that rows the shortcut cannot read
// (derived Contact Name, explicit nulls, missing values) sort as the generic
// recordValue path orders them.
func TestSortShortcutFallbackValues(t *testing.T) {
	org := storage.NewOrgState()
	definition := storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
		"Id":        {APIName: "Id", Type: storage.FieldID},
		"FirstName": {APIName: "FirstName", Type: storage.FieldString},
		"LastName":  {APIName: "LastName", Type: storage.FieldString},
	}}
	var records []storage.Record
	for i, name := range []string{"Zed", "adams", "Brown", "adams", "", "Cole", "brown"} {
		fields := map[string]storage.Value{"FirstName": storage.StringValue(fmt.Sprintf("F%d", 9-i))}
		rec := storage.Record{ID: storage.ID(fmt.Sprintf("003%012d", i+1)), Object: "Contact", Fields: fields}
		if name == "" {
			rec.ExplicitNulls = map[string]bool{"LastName": true}
		} else {
			fields["LastName"] = storage.StringValue(name)
		}
		records = append(records, rec)
	}
	for _, field := range []string{"LastName", "Name"} {
		for _, desc := range []bool{false, true} {
			order := []OrderSpec{{Field: field, Desc: desc}}
			want := append([]storage.Record(nil), records...)
			sort.SliceStable(want, func(i, j int) bool {
				left, ok := recordValue(org, definition, want[i], field)
				if !ok {
					left = storage.NullValue()
				}
				right, ok := recordValue(org, definition, want[j], field)
				if !ok {
					right = storage.NullValue()
				}
				return orderCompare(left, right, order[0]) < 0
			})
			got := append([]storage.Record(nil), records...)
			sortRecordsByOrderTop(org, definition, got, order, false, false, 0, nil)
			for i := range got {
				if got[i].ID != want[i].ID {
					t.Fatalf("%s desc=%v row %d: got %s want %s", field, desc, i, got[i].ID, want[i].ID)
				}
			}
		}
	}
}

// TestDefaultOrderLimitOffsetWindows runs the full Execute path with ties
// (case variants of one name) crossing every LIMIT/OFFSET boundary.
func TestDefaultOrderLimitOffsetWindows(t *testing.T) {
	names := []string{"nacme", "nACME", "nAcme", "nbolt", "nBolt", "nBOLT", "ncog", "nCog", "ndyn", "nDYN", "nea", "nEA", "nfz"}
	org := likeBenchOrg(0)
	state := org.Objects["Account"]
	type row struct {
		id   storage.ID
		name string
	}
	var rows []row
	for i := 0; i < 90; i++ {
		id := storage.ID(fmt.Sprintf("001%012d", i+1))
		name := names[(i*5)%len(names)]
		state.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(name), "BillingCity": storage.StringValue("x")}}
		rows = append(rows, row{id, name})
	}
	// Stable sort of the Id-ordered rows by folded name.
	sort.SliceStable(rows, func(i, j int) bool { return strings.ToLower(rows[i].name) < strings.ToLower(rows[j].name) })
	for _, locale := range []string{"", "en_US"} {
		for _, window := range [][2]int{{1, 0}, {2, 0}, {3, 0}, {7, 0}, {7, 2}, {5, 11}, {13, 4}, {20, 9}, {44, 0}, {44, 3}, {45, 0}, {89, 0}, {100, 0}, {10, 85}, {10, 95}, {30, 40}} {
			limit, offset := window[0], window[1]
			parsed, err := Parse(fmt.Sprintf("SELECT Id, Name FROM Account WHERE Name LIKE 'n%%' LIMIT %d OFFSET %d", limit, offset))
			if err != nil {
				t.Fatal(err)
			}
			result, err := ExecuteWithCacheAndContext(org, parsed, nil, ExecutionContext{Locale: locale})
			if err != nil {
				t.Fatal(err)
			}
			want := rows
			if offset >= len(want) {
				want = nil
			} else {
				want = want[offset:]
			}
			if limit < len(want) {
				want = want[:limit]
			}
			if len(result.Records) != len(want) {
				t.Fatalf("locale %q limit %d offset %d: %d rows, want %d", locale, limit, offset, len(result.Records), len(want))
			}
			for i, record := range result.Records {
				if record.ID != want[i].id {
					t.Fatalf("locale %q limit %d offset %d row %d: got %s want %s", locale, limit, offset, i, record.ID, want[i].id)
				}
			}
		}
	}
}

// TestDefaultOrderCapturedTextsMatchStoredTexts pins the text the LIKE case
// hands back to the text the sort would read itself, and the Execute output
// to a reference stable sort. Names start with upper-case letters, and one
// set has non-ASCII names, which must fall back to the comparator.
func TestDefaultOrderCapturedTextsMatchStoredTexts(t *testing.T) {
	sets := map[string][]string{
		"ascii":     {"Nacme", "NACME", "Nzed", "nzed", "Nbolt", "N", "Nacme x", "NB"},
		"non-ascii": {"Nacme", "Née", "NACME", "Nzed", "Nézé", "Nbolt", "Nñu", "nzed"},
	}
	for setName, names := range sets {
		org := likeBenchOrg(0)
		state := org.Objects["Account"]
		var ids []storage.ID
		for i := 0; i < 60; i++ {
			id := storage.ID(fmt.Sprintf("001%012d", i+1))
			state.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{
				"Name": storage.StringValue(names[(i*3)%len(names)]), "BillingCity": storage.StringValue("x"),
			}}
			ids = append(ids, id)
		}
		query, err := Parse("SELECT Id, Name FROM Account WHERE Name LIKE 'n%'")
		if err != nil {
			t.Fatal(err)
		}
		scratch := &predicateScratch{captureLike: true}
		var matched []storage.Record
		var captured []string
		for _, id := range ids {
			scratch.like = storage.Value{}
			record := state.Records[id]
			if matchesWithScratch(org, state.Definition, record, query.Where, ExecutionContext{}, scratch) {
				matched = append(matched, record)
				captured = append(captured, scratch.like.String)
			}
		}
		stored, ok := storedTexts(org, matched, "Name")
		if !ok || len(stored) != len(captured) {
			t.Fatalf("%s: storedTexts ok=%v len %d, captured %d", setName, ok, len(stored), len(captured))
		}
		for i := range stored {
			if stored[i] != captured[i] {
				t.Fatalf("%s row %d: captured %q, stored %q", setName, i, captured[i], stored[i])
			}
		}
		keys := asciiTextSortKeys(captured, nil)
		if (keys != nil) != (setName == "ascii") {
			t.Fatalf("%s: keys built = %v", setName, keys != nil)
		}
		for _, locale := range []string{"", "en_US"} {
			for _, limit := range []string{"", " LIMIT 5", " LIMIT 9 OFFSET 4"} {
				parsed, err := Parse("SELECT Id, Name FROM Account WHERE Name LIKE 'n%'" + limit)
				if err != nil {
					t.Fatal(err)
				}
				result, err := ExecuteWithCacheAndContext(org, parsed, nil, ExecutionContext{Locale: locale})
				if err != nil {
					t.Fatal(err)
				}
				order := collateOrder([]OrderSpec{{Field: "Name"}}, newTextComparator(locale))
				want := append([]storage.Record(nil), matched...)
				sort.SliceStable(want, func(i, j int) bool {
					return orderCompare(want[i].Fields["Name"], want[j].Fields["Name"], order[0]) < 0
				})
				want = applyWindow(want, parsed.Offset, parsed.Limit, parsed.HasLimit)
				if len(result.Records) != len(want) {
					t.Fatalf("%s %q%s: %d rows, want %d", setName, locale, limit, len(result.Records), len(want))
				}
				for i := range want {
					if result.Records[i].ID != want[i].ID {
						t.Fatalf("%s %q%s row %d: got %s want %s", setName, locale, limit, i, result.Records[i].ID, want[i].ID)
					}
				}
			}
		}
	}
}
