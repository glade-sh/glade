package soql

import (
	"encoding/csv"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func localeCollationRows(t *testing.T) []collationRow {
	t.Helper()
	raw, err := os.ReadFile("testdata/locale_collation/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	var rows []collationRow
	for i, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if i == 0 {
			continue
		}
		c := strings.Split(line, "\t")
		if len(c) != 6 {
			t.Fatalf("org.tsv:%d: %d cells", i+1, len(c))
		}
		rows = append(rows, collationRow{c[0], c[1], c[2], c[3], c[4], c[5]})
	}
	if len(rows) != 224 {
		t.Fatalf("native rows %d, want 224", len(rows))
	}
	return rows
}

func localeCollationValues(t *testing.T) []string {
	t.Helper()
	return readCollationValues(t, "../locale_collation/maps/V-values.tsv")
}

func nativeRowLocale(row collationRow) string {
	loc, ok := row.field("loc")
	if !ok {
		loc, _ = row.field("user")
	}
	return strings.SplitN(loc, "/", 2)[0]
}

func localeQuery(t *testing.T, org storage.OrgState, row collationRow, text string) Result {
	t.Helper()
	q, err := Parse(text)
	if err != nil {
		t.Fatalf("%s: %v", text, err)
	}
	result, err := ExecuteWithCacheAndContext(org, q, nil, ExecutionContext{Locale: nativeRowLocale(row), APIVersion: row.api})
	if err != nil {
		t.Fatalf("%s: %v", text, err)
	}
	return result
}

func localeLiteral(text string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(text, "\\", "\\\\"), "'", "\\'") + "'"
}

// secondaryEqualForNative bounds a native MIN/MAX tie by its range class:
// neither value is greater than the other under the same locale. The API
// does not fix the spelling/record chosen inside that class.
func secondaryEqualForNative(t *testing.T, org storage.OrgState, row collationRow, a, b string) bool {
	t.Helper()
	if a == b {
		return true
	}
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		q := "SELECT BillingCity FROM Account WHERE Name = " + localeLiteral(pair[0]) + " AND Name > " + localeLiteral(pair[1])
		if len(localeQuery(t, org, row, q).Records) != 0 {
			return false
		}
	}
	return true
}

type localeRowResult struct {
	checks, matched, adjacent, ordered int
	equalityChecks, equalityMatched    int
	equality                           string
	differences                        []string
}

func checkLocaleNativeRow(t *testing.T, org storage.OrgState, values []string, row collationRow) localeRowResult {
	t.Helper()
	var out localeRowResult
	check := func(name string, ok bool) {
		out.checks++
		if ok {
			out.matched++
		} else {
			out.differences = append(out.differences, name)
		}
	}
	ordered := localeQuery(t, org, row, "SELECT BillingCity, Name FROM Account ORDER BY Name, BillingCity")
	got := collationLabels(ordered, "BillingCity")
	want, _ := row.field("asc")
	want = strings.ReplaceAll(want, ",", "")
	check("ORDER BY", got == want)
	// Count native adjacent pairs in the produced order for a useful known-gap
	// receipt even when an unsupported locale remains on the folded comparator.
	positions := map[string]int{}
	for i := 0; i+2 <= len(got); i += 2 {
		positions[got[i:i+2]] = i / 2
	}
	for i := 0; i+4 <= len(want); i += 2 {
		out.adjacent++
		if positions[want[i:i+2]] < positions[want[i+2:i+4]] {
			out.ordered++
		}
	}
	ranges := []struct{ name, op, value string }{{"gtz", ">", "z"}, {"lta", "<", "a"}, {"gea", ">=", "ä"}, {"ltb", "<", "b"}, {"gtZ", ">", "Z"}, {"ltA", "<", "A"}, {"gtKa", ">", "ア"}, {"ltn", "<", "ñ"}}
	for _, r := range ranges {
		result := localeQuery(t, org, row, "SELECT BillingCity FROM Account WHERE Name "+r.op+" "+localeLiteral(r.value)+" ORDER BY BillingCity")
		expected, ok := row.field(r.name)
		if !ok {
			t.Fatal("missing native range", r.name)
		}
		check(r.name, collationLabels(result, "BillingCity") == strings.ReplaceAll(expected, ",", ""))
	}
	eq, _ := row.field("eq")
	var equality []string
	for _, part := range strings.Split(strings.TrimSuffix(eq, ";"), ";") {
		label, expected, ok := strings.Cut(part, "=")
		if !ok {
			t.Fatal("invalid native equality", part)
		}
		i, err := strconv.ParseInt(label, 36, 32)
		if err != nil || int(i) >= len(values) {
			t.Fatal("bad probe label", label)
		}
		result := localeQuery(t, org, row, "SELECT BillingCity FROM Account WHERE Name = "+localeLiteral(values[i])+" ORDER BY BillingCity")
		actual := collationLabels(result, "BillingCity")
		equality = append(equality, label+"="+actual)
		out.equalityChecks++
		if actual == strings.ReplaceAll(expected, ",", "") {
			out.equalityMatched++
		}
	}
	out.equality = strings.Join(equality, ";")
	// Group representatives may vary between capture chunks. Match the exact
	// group sizes and membership classes by comparing every selected spelling
	// against its captured class through GROUP BY itself.
	groups := localeQuery(t, org, row, "SELECT Name k, COUNT(Id) c FROM Account GROUP BY Name ORDER BY Name")
	var sizes []int
	for _, g := range groups.Records {
		sizes = append(sizes, int(g.Fields["c"].Integer))
	}
	sort.Ints(sizes)
	group, _ := row.field("group")
	nativeOrder, _ := row.field("asc")
	nativeLabels := strings.Split(nativeOrder, ",")
	cursor := 0
	var nativeSizes []int
	for _, part := range strings.Split(strings.TrimSuffix(group, ","), ",") {
		n := 1
		if len(part) > 2 && part[2] == 'x' {
			s := part[3:]
			var err error
			n, err = strconv.Atoi(s)
			if err != nil {
				t.Fatal(err)
			}
		}
		nativeSizes = append(nativeSizes, n)
		if cursor+n > len(nativeLabels) {
			t.Fatal("native group counts exceed native order")
		}
		if n > 1 {
			var labels []string
			for _, label := range nativeLabels[cursor : cursor+n] {
				labels = append(labels, localeLiteral(label))
			}
			probe := localeQuery(t, org, row, "SELECT Name k, COUNT(Id) c FROM Account WHERE BillingCity IN ("+strings.Join(labels, ",")+") GROUP BY Name")
			check("GROUP BY class:"+nativeLabels[cursor], len(probe.Records) == 1 && probe.Records[0].Fields["c"].Integer == int64(n))
		}
		cursor += n
	}
	if cursor != len(values) {
		t.Fatal("native group counts do not cover all values")
	}
	sort.Ints(nativeSizes)
	check("GROUP BY sizes", reflect.DeepEqual(sizes, nativeSizes))
	extremes := localeQuery(t, org, row, "SELECT MIN(Name) mn, MAX(Name) mx FROM Account")
	if len(extremes.Records) != 1 {
		t.Fatal("missing extremes")
	}
	for _, r := range []struct{ name, field string }{{"min", "mn"}, {"max", "mx"}} {
		label, _ := row.field(r.name)
		i, err := strconv.ParseInt(label, 36, 32)
		if err != nil || int(i) >= len(values) {
			t.Fatal("bad native extreme", label)
		}
		check(r.name, secondaryEqualForNative(t, org, row, extremes.Records[0].Fields[r.field].String, values[i]))
	}
	return out
}

func TestLocaleTextCollationNativeRows(t *testing.T) {
	rows := localeCollationRows(t)
	values := localeCollationValues(t)
	if len(values) != 760 {
		t.Fatal("native values", len(values))
	}
	org := collationOrg(values, "Account", map[string]bool{"Name": true})
	counts := map[string][8]int{}
	baseline := map[string]string{}
	if raw, err := os.ReadFile("testdata/locale_collation/equality-baseline.tsv"); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
			locale, value, ok := strings.Cut(line, "\t")
			if !ok {
				t.Fatal("bad equality baseline")
			}
			baseline[locale] = value
		}
	} else if os.Getenv("GLADE_COLLATION_EQUALITY_BASELINE_OUT") == "" {
		t.Fatal(err)
	}
	observedEquality := map[string]string{}
	facts, invalid, usable := 0, 0, 0
	for _, row := range rows {
		if strings.HasPrefix(row.id, "facts-") {
			facts++
			continue
		}
		locale := nativeRowLocale(row)
		if strings.Contains(row.observation, "|ERR:") {
			invalid++
			if !strings.Contains(row.observation, "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST") {
				t.Fatal("unexpected native error")
			}
			continue
		}
		usable++
		t.Run(row.id+"/"+row.route, func(t *testing.T) {
			result := checkLocaleNativeRow(t, org, values, row)
			if prior, ok := observedEquality[locale]; ok && prior != result.equality {
				t.Fatal("equality differs across native API/route twins")
			}
			observedEquality[locale] = result.equality
			if expected, ok := baseline[locale]; ok && expected != result.equality {
				t.Error("equality changed from the user-authorized baseline")
			}
			if len(baseline) != 0 {
				if _, ok := baseline[locale]; !ok {
					t.Fatal("missing equality baseline", locale)
				}
			}
			c := counts[locale]
			c[1]++
			c[2] += result.matched
			c[3] += result.checks
			c[4] += result.ordered
			c[5] += result.equalityMatched
			c[6] += result.equalityChecks
			if len(result.differences) == 0 && result.equalityMatched == result.equalityChecks {
				c[7]++
			}
			if len(result.differences) == 0 {
				c[0]++
			}
			counts[locale] = c
			if locale == "lv_LV" {
				// User-authorized fallback: newer public data differs from these native
				// rows. Keep today's comparator and keep its discrepancy visible.
				legacy := row
				legacy.observation = strings.Replace(row.observation, "loc:"+locale+"/", "loc:/", 1)
				current := collationLabels(localeQuery(t, org, row, "SELECT BillingCity FROM Account ORDER BY Name, BillingCity"), "BillingCity")
				fallback := collationLabels(localeQuery(t, org, legacy, "SELECT BillingCity FROM Account ORDER BY Name, BillingCity"), "BillingCity")
				if current != fallback || result.matched != 0 || result.ordered != 564 || len(result.differences) == 0 {
					t.Fatal("known-difference fallback changed")
				}
				t.Logf("known difference %s: %d/%d checks, %d/%d adjacent pairs; %s", locale, result.matched, result.checks, result.ordered, result.adjacent, strings.Join(result.differences, ","))
				return
			}
			if len(result.differences) != 0 {
				t.Errorf("%s: %d/%d checks, %d/%d adjacent pairs; differs: %s", locale, result.matched, result.checks, result.ordered, result.adjacent, strings.Join(result.differences, ","))
			}
		})
	}
	if facts != 2 || invalid != 20 || usable != 202 {
		t.Fatalf("accounting facts=%d invalid=%d usable=%d", facts, invalid, usable)
	}
	var locales []string
	for locale := range counts {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	var report strings.Builder
	writer := csv.NewWriter(&report)
	_ = writer.Write([]string{"locale", "matched_collation_rows", "rows", "matched_collation_checks", "collation_checks", "ordered_adjacent_pairs", "matched_equality_checks", "equality_checks", "fully_native_rows"})
	for _, locale := range locales {
		c := counts[locale]
		_ = writer.Write([]string{locale, strconv.Itoa(c[0]), strconv.Itoa(c[1]), strconv.Itoa(c[2]), strconv.Itoa(c[3]), strconv.Itoa(c[4]), strconv.Itoa(c[5]), strconv.Itoa(c[6]), strconv.Itoa(c[7])})
		t.Logf("%s rows=%d/%d checks=%d/%d adjacent=%d", locale, c[0], c[1], c[2], c[3], c[4])
	}
	writer.Flush()
	if path := os.Getenv("GLADE_COLLATION_EQUALITY_BASELINE_OUT"); path != "" {
		var b strings.Builder
		b.WriteString("locale\tresult\n")
		for _, locale := range locales {
			fmt.Fprintf(&b, "%s\t%s\n", locale, observedEquality[locale])
		}
		if err := os.WriteFile(path, []byte(b.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if path := os.Getenv("GLADE_COLLATION_REPORT"); path != "" {
		if err := os.WriteFile(path, []byte(report.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("accounted %d comparator rows, %d invalid-locale answers, %d facts rows", usable, invalid, facts)
}

func TestLocaleCollationConcurrentQueries(t *testing.T) {
	org := collationOrg([]string{"ä", "z"}, "Account", map[string]bool{"Name": true})
	query, err := Parse("SELECT Name FROM Account ORDER BY Name")
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	problems := make(chan string, 4)
	for _, locale := range []string{"de_DE", "fr_FR", "sv_SE", "sv_FI"} {
		wait.Add(1)
		go func(locale string) {
			defer wait.Done()
			want := "äz"
			if strings.HasPrefix(locale, "sv_") {
				want = "zä"
			}
			for i := 0; i < 8; i++ {
				result, err := ExecuteWithCacheAndContext(org, query, nil, ExecutionContext{Locale: locale})
				if err != nil {
					problems <- err.Error()
					return
				}
				var got strings.Builder
				for _, r := range result.Records {
					got.WriteString(r.Fields["Name"].String)
				}
				if got.String() != want {
					problems <- fmt.Sprintf("%s got %s want %s", locale, got.String(), want)
					return
				}
			}
		}(locale)
	}
	wait.Wait()
	close(problems)
	for problem := range problems {
		t.Error(problem)
	}
}
