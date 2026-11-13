package soql

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The rows in testdata/collation are native org captures (RUN-FACTS.txt).
// Every English row must match. The locale_collation fixture now supplies
// broad native rows for the T8 locales, so those rows also check native order
// instead of asserting the previous unmodeled fallback.

type collationRow struct {
	id, api, route, run, valueSet, observation string
}

func (r collationRow) field(name string) (string, bool) {
	for _, part := range strings.Split(r.observation, "|") {
		if strings.HasPrefix(part, name+":") {
			return strings.TrimPrefix(part, name+":"), true
		}
	}
	return "", false
}

func readCollationRows(t *testing.T) []collationRow {
	t.Helper()
	data, err := os.ReadFile("testdata/collation/observations.tsv")
	if err != nil {
		t.Fatal(err)
	}
	var rows []collationRow
	for i, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if i == 0 {
			continue
		}
		cells := strings.Split(line, "\t")
		if len(cells) != 6 {
			t.Fatalf("observations.tsv line %d has %d cells", i+1, len(cells))
		}
		rows = append(rows, collationRow{cells[0], cells[1], cells[2], cells[3], cells[4], cells[5]})
	}
	return rows
}

func readCollationValues(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile("testdata/collation/" + path)
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	for i, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if i == 0 {
			continue
		}
		cells := strings.Split(line, "\t")
		var text []rune
		for _, cp := range strings.Fields(cells[1]) {
			n, err := strconv.ParseInt(strings.TrimPrefix(cp, "U+"), 16, 32)
			if err != nil {
				t.Fatal(err)
			}
			text = append(text, rune(n))
		}
		index, err := strconv.ParseInt(cells[0], 36, 32)
		if err != nil || int(index) != len(values) {
			t.Fatalf("%s label %q out of sequence", path, cells[0])
		}
		values = append(values, string(text))
	}
	return values
}

func collationLabel(i int) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	return string([]byte{digits[i/36], digits[i%36]})
}

func collationOrg(values []string, objectName string, fields map[string]bool) storage.OrgState {
	org := storage.NewOrgState()
	definition := storage.ObjectDefinition{APIName: objectName, Fields: map[string]storage.Field{
		"Id":            {APIName: "Id", Type: storage.FieldID},
		"Name":          {APIName: "Name", Type: storage.FieldString},
		"BillingCity":   {APIName: "BillingCity", Type: storage.FieldString},
		"Probe_Text__c": {APIName: "Probe_Text__c", Type: storage.FieldString},
		"LastName":      {APIName: "LastName", Type: storage.FieldString},
		"MailingCity":   {APIName: "MailingCity", Type: storage.FieldString},
	}}
	state := storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
	for i, value := range values {
		id := storage.ID(fmt.Sprintf("%s%012d", map[string]string{"Account": "001", "Contact": "003"}[objectName], i+1))
		label := collationLabel(i)
		record := map[string]storage.Value{}
		switch {
		case objectName == "Contact":
			record["LastName"] = storage.StringValue(value)
			record["MailingCity"] = storage.StringValue(label)
		case fields["Probe_Text__c"] && !fields["Name"]:
			record["Name"] = storage.StringValue("pt" + label)
			record["Probe_Text__c"] = storage.StringValue(value)
			record["BillingCity"] = storage.StringValue(label)
		default:
			record["Name"] = storage.StringValue(value)
			record["BillingCity"] = storage.StringValue(label)
			if fields["Probe_Text__c"] {
				record["Probe_Text__c"] = storage.StringValue(value)
			}
		}
		state.Records[id] = storage.Record{ID: id, Object: objectName, Fields: record}
	}
	org.Objects[objectName] = state
	return org
}

func collationQuery(t *testing.T, org storage.OrgState, locale, query string) Result {
	t.Helper()
	parsed, err := Parse(query)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	result, err := ExecuteWithCacheAndContext(org, parsed, nil, ExecutionContext{Locale: locale})
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return result
}

func collationLabels(result Result, field string) string {
	var out strings.Builder
	for _, record := range result.Records {
		out.WriteString(record.Fields[field].String)
	}
	return out.String()
}

// foldedSequence maps a label sequence to case-folded sort keys. Ties inside a
// case-equal group follow Id order, which the capture does not fix.
func foldedSequence(values []string, labels string) []string {
	var keys []string
	for i := 0; i+2 <= len(labels); i += 2 {
		index, _ := strconv.ParseInt(labels[i:i+2], 36, 32)
		keys = append(keys, textGroupKey(values[index]))
	}
	return keys
}

func collatedSequence(values []string, labels, locale string) []string {
	comparator := newTextComparator(locale)
	if comparator == nil {
		return foldedSequence(values, labels)
	}
	var keys []string
	for i := 0; i+2 <= len(labels); i += 2 {
		index, _ := strconv.ParseInt(labels[i:i+2], 36, 32)
		keys = append(keys, comparator.groupKey(values[index]))
	}
	return keys
}

func sortedLabels(labels string) []string {
	var out []string
	for i := 0; i+2 <= len(labels); i += 2 {
		out = append(out, labels[i:i+2])
	}
	sort.Strings(out)
	return out
}

func TestTextCollationNativeRows(t *testing.T) {
	rows := readCollationRows(t)
	if len(rows) != 70 {
		t.Fatalf("observations rows=%d; want 70", len(rows))
	}
	sets := map[string][]string{}
	for _, name := range []string{"T1", "T2", "T3", "T4", "T5", "M"} {
		sets[name] = readCollationValues(t, "maps/"+name+"-values.tsv")
	}
	setFor := func(id string) string {
		switch {
		case id == "t1", id == "t2", id == "t3", id == "t4", id == "t5":
			return strings.ToUpper(id)
		}
		return "M"
	}
	matched, notEnglish, excluded := 0, 0, 0
	for _, row := range rows {
		row := row
		name := strings.Join([]string{row.id, row.api, row.route, row.run}, "/")
		if row.id == "facts" {
			excluded++
			continue
		}
		locale := "en_US"
		if user, ok := row.field("user"); ok && user != "default" {
			locale = strings.SplitN(user, "/", 2)[0]
		}
		values := sets[setFor(row.id)]
		t.Run(name, func(t *testing.T) {
			if strings.HasPrefix(row.id, "t7") {
				checkCollationOps(t, row, values)
				return
			}
			object, field, labelField := "Account", "Name", "BillingCity"
			fields := map[string]bool{"Name": true}
			switch row.id {
			case "t6ptext":
				field, fields = "Probe_Text__c", map[string]bool{"Probe_Text__c": true}
			case "t6contact":
				object, field, labelField = "Contact", "LastName", "MailingCity"
			}
			org := collationOrg(values, object, fields)
			for _, dir := range []string{"asc", "desc"} {
				want, _ := row.field(dir)
				query := fmt.Sprintf("SELECT %s FROM %s ORDER BY %s %s, Id ASC", labelField, object, field, strings.ToUpper(dir))
				got := collationLabels(collationQuery(t, org, locale, query), labelField)
				if newTextComparator(locale) == nil {
					// A locale without a captured fitting model retains the
					// previous comparator. The locale fixture now supplies
					// the full native rows for the other T8 locales.
					if legacy := collationLabels(collationQuery(t, org, "", query), labelField); got != legacy {
						t.Fatalf("%s %s: non-English locale %s changed from the folded comparator", row.id, dir, locale)
					}
					continue
				}
				if !reflect.DeepEqual(sortedLabels(got), sortedLabels(want)) {
					t.Fatalf("%s %s: row sets differ", row.id, dir)
				}
				if !reflect.DeepEqual(collatedSequence(values, got, locale), collatedSequence(values, want, locale)) {
					t.Fatalf("%s %s order\n got  %v\n want %v", row.id, dir, collatedSequence(values, got, locale), collatedSequence(values, want, locale))
				}
			}
		})
		if englishTextLocale(locale) {
			if !t.Failed() {
				matched++
			}
		} else {
			notEnglish++
		}
	}
	t.Logf("en_* rows matched %d; non-English rows checked against their native order %d; facts rows %d; total %d", matched, notEnglish, excluded, matched+notEnglish+excluded)
	if matched+notEnglish+excluded != 70 {
		t.Fatalf("rows accounted %d; want 70", matched+notEnglish+excluded)
	}
}

func checkCollationOps(t *testing.T, row collationRow, values []string) {
	t.Helper()
	field := "Name"
	if row.id == "t7ptext" {
		field = "Probe_Text__c"
	}
	org := collationOrg(values, "Account", map[string]bool{"Name": true, "Probe_Text__c": true})
	label := map[string]string{}
	for i, value := range values {
		label[value] = collationLabel(i)
	}
	queries := map[string]string{
		"gt_":    "WHERE " + field + " > '_'",
		"lta":    "WHERE " + field + " < 'a'",
		"gtz":    "WHERE " + field + " > 'z'",
		"geA1":   "WHERE " + field + " >= 'Á' AND " + field + " < 'à'",
		"gtAltb": "WHERE " + field + " > 'A' AND " + field + " < 'b'",
	}
	for key, where := range queries {
		want, _ := row.field(key)
		got := collationLabels(collationQuery(t, org, "en_US", "SELECT BillingCity FROM Account "+where+" ORDER BY Id"), "BillingCity")
		if got != want {
			t.Errorf("%s %s\n got  %s\n want %s", key, where, got, want)
		}
	}
	minmax := collationQuery(t, org, "en_US", fmt.Sprintf("SELECT MIN(%s) mn, MAX(%s) mx FROM Account", field, field))
	wantMin, _ := row.field("min")
	wantMax, _ := row.field("max")
	if len(minmax.Records) != 1 || label[minmax.Records[0].Fields["mn"].String] != wantMin || label[minmax.Records[0].Fields["mx"].String] != wantMax {
		t.Errorf("min/max = %v; want %s/%s", minmax.Records, wantMin, wantMax)
	}
	groups := collationQuery(t, org, "en_US", fmt.Sprintf("SELECT %s k, COUNT(Id) c FROM Account GROUP BY %s ORDER BY %s", field, field, field))
	var gotGroups strings.Builder
	for _, record := range groups.Records {
		gotGroups.WriteString(label[record.Fields["k"].String])
		if n := record.Fields["c"].Integer; n > 1 {
			fmt.Fprintf(&gotGroups, "x%d", n)
		}
		gotGroups.WriteString(",")
	}
	if want, _ := row.field("group"); gotGroups.String() != want {
		t.Errorf("group by\n got  %s\n want %s", gotGroups.String(), want)
	}
}

func TestTextCollationEnglishLocaleGate(t *testing.T) {
	for locale, want := range map[string]bool{"en_US": true, "en_GB": true, "en_AU": true, "de_DE": false, "ja_JP": false, "sv_SE": false, "": false, "en": false} {
		if got := englishTextLocale(locale); got != want {
			t.Errorf("englishTextLocale(%q)=%v; want %v", locale, got, want)
		}
	}
}

func TestTextCollationAdjacentRules(t *testing.T) {
	for _, tc := range []struct {
		left, right string
		want        int
	}{
		{"_", "-", -1}, {"-", "!", -1}, {"a", "A", 0}, {"Z", "a", 1}, {"9", "a", -1}, {" ", "_", -1},
		{"z", "Á", -1}, {"Á", "á", -1}, {"é", "á", 1},
		// Code point order, not UTF-16 order: U+1F600 follows U+FF5E.
		{"\U0001F600", "～", 1}, {"a", "aa", -1}, {"", "a", -1},
	} {
		if got := compareSOQLTextCollated(tc.left, tc.right); got != tc.want {
			t.Errorf("compare(%q,%q)=%d; want %d", tc.left, tc.right, got, tc.want)
		}
	}
}
