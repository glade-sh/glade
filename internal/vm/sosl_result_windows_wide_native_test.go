package vm

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

var searchWideObjects = []struct{ object, field, prefix string }{
	{"Account", "Name", "001"}, {"Contact", "LastName", "003"}, {"Lead", "LastName", "00Q"},
	{"Opportunity", "Name", "006"}, {"Case", "Subject", "500"}, {"Campaign", "Name", "701"},
	{"Product2", "Name", "01t"}, {"Solution", "SolutionName", "501"}, {"Pricebook2", "Name", "01s"},
	{"Asset", "Name", "02i"}, {"Task", "Subject", "00T"},
}

// searchWideMachine seeds labels[object] as "<term> <label>" on each object's
// label field. A named machine fixes every seeded Id, interleaved by object,
// in forward or reversed order.
func searchWideMachine(t *testing.T, api, term string, labels map[string][]string, named, reversed bool) *VM {
	t.Helper()
	org := storage.NewOrgState()
	org.APIVersion = api
	var fixed []Value
	for _, spec := range searchWideObjects {
		storage.EnsureStandardObject(&org, spec.object)
		state := org.Objects[spec.object]
		if _, ok := state.Definition.Fields[spec.field]; !ok {
			t.Fatalf("%s.%s is not in the standard schema", spec.object, spec.field)
		}
		for i, label := range labels[spec.object] {
			id := storage.ID(fmt.Sprintf("%s%012d", spec.prefix, i+1))
			state.Records[id] = storage.Record{Object: spec.object, ID: id, Fields: map[string]storage.Value{
				spec.field: storage.StringValue(term + " " + label),
			}}
			fixed = append(fixed, String(string(id)))
		}
		org.Objects[spec.object] = state
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod.APIVersion = api
	if named {
		// Spread the objects through the list as the capture did.
		mixed := List()
		for stride := 0; stride < 7; stride++ {
			for i := stride; i < len(fixed); i += 7 {
				mixed.List = append(mixed.List, fixed[i])
			}
		}
		if reversed {
			for i, j := 0, len(mixed.List)-1; i < j; i, j = i+1, j-1 {
				mixed.List[i], mixed.List[j] = mixed.List[j], mixed.List[i]
			}
		}
		machine.EnableTestContext()
		if _, err := machine.testSetFixedSearchResults([]Value{mixed}); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}

func searchWideLabels(count int, objects ...string) map[string][]string {
	labels := make(map[string][]string)
	for _, object := range objects {
		for i := range count {
			labels[object] = append(labels[object], fmt.Sprintf("%04d", i))
		}
	}
	return labels
}

func searchWideObservation(t *testing.T, groups Value, err error, endpoints bool) string {
	t.Helper()
	if err != nil {
		var thrown *apexThrowError
		if !errors.As(err, &thrown) {
			t.Fatal(err)
		}
		return "System." + strings.TrimPrefix(thrown.value.Type, "System.") + "|" + thrown.value.Fields["message"].Text
	}
	fields := make(map[string]string)
	for _, spec := range searchWideObjects {
		fields[spec.object] = spec.field
	}
	var result strings.Builder
	for _, bucket := range groups.List {
		var labels []string
		for _, record := range bucket.List {
			_, label, _ := strings.Cut(record.Fields[fields[record.Type]].Text, " ")
			labels = append(labels, label)
		}
		text := strings.Join(labels, ",")
		if endpoints && len(labels) > 0 {
			text = fmt.Sprintf("%d:%s:%s", len(labels), labels[0], labels[len(labels)-1])
		}
		fmt.Fprintf(&result, "[%s]", text)
	}
	return result.String()
}

// NATIVE fixed-result allocation across four, five and ten RETURNING objects,
// and the 2,000-row default budget across three, four and five:
// sosl_result_windows_wide/org.tsv, istest 62.0 and 67.0, two fixed-Id orders.
func TestSOSLResultWindowsWideNative(t *testing.T) {
	const fixture = "sosl_result_windows_wide/"
	queries := readSearchWindowTable(t, "../"+fixture+"queries.tsv")
	oracle := readSearchWindowTable(t, "../"+fixture+"org.tsv")
	if len(queries) != len(oracle) || len(queries) < 2 {
		t.Fatalf("queries=%d native rows=%d", len(queries), len(oracle))
	}
	small := map[string][]string{
		"Account": strings.Fields("A B C D E"), "Contact": strings.Fields("K L M N O"), "Lead": strings.Fields("P Q R S T"),
		"Opportunity": strings.Fields("U V W X Y"), "Case": strings.Fields("1 2 3 4 5"), "Campaign": strings.Fields("m1 m2 m3 m4 m5"),
		"Product2": strings.Fields("n1 n2 n3 n4 n5"), "Solution": strings.Fields("o1 o2 o3 o4 o5"),
		"Pricebook2": strings.Fields("p1 p2 p3 p4 p5"), "Asset": strings.Fields("q1 q2 q3 q4 q5"),
	}
	large := map[string]map[string][]string{
		"B3": searchWideLabels(700, "Account", "Contact", "Lead"),
		"B4": searchWideLabels(510, "Account", "Contact", "Lead", "Opportunity"),
		"B5": searchWideLabels(410, "Account", "Contact", "Lead", "Opportunity", "Case"),
	}
	for column, context := range oracle[0][1:] {
		t.Run(context, func(t *testing.T) {
			parts := strings.Split(context, "_")
			api := parts[1] + ".0"
			machines := make(map[string]*VM)
			for i, input := range queries[1:] {
				id, query := input[0], input[2]
				row := oracle[i+1]
				if id != row[0] {
					t.Fatalf("query %s native row %s", id, row[0])
				}
				want := row[column+1]
				if want == "-" {
					continue
				}
				population, labels := "small", small
				if bulk, ok := large[id[:2]]; ok {
					population, labels = id[:2], bulk
				}
				machine := machines[population]
				if machine == nil {
					machine = searchWideMachine(t, api, "Wideterm", labels, true, parts[2] == "2")
					machines[population] = machine
				}
				machine.ResetLimits()
				groups, err := machine.searchQuery([]Value{String(query)})
				got := searchWideObservation(t, groups, err, population != "small")
				if strings.Contains(want, "@sosl") {
					got += fmt.Sprintf("@sosl+%d", machine.limits.SOSLQueries)
				}
				if got != want {
					t.Errorf("org.tsv:%d %s: got %s, want %s", i+2, id, got, want)
				}
			}
		})
	}
}

// NATIVE indexed ceilings for one, two, four, five and ten RETURNING objects:
// sosl_result_windows_dense/org.tsv, anonymous 62.0 and 67.0, each twice.
// The index chose which rows filled a capped group, so counts are compared.
func TestSOSLResultWindowsDenseNative(t *testing.T) {
	const fixture = "sosl_result_windows_dense/"
	queries := readSearchWindowTable(t, "../"+fixture+"queries.tsv")
	oracle := readSearchWindowTable(t, "../"+fixture+"org.tsv")
	if len(queries) != len(oracle) || len(queries) < 2 {
		t.Fatalf("queries=%d native rows=%d", len(queries), len(oracle))
	}
	labels := make(map[string][]string)
	for _, spec := range searchWideObjects {
		if spec.object == "Pricebook2" {
			continue
		}
		for i := range 321 {
			labels[spec.object] = append(labels[spec.object], fmt.Sprintf("%03d", i))
		}
	}
	for column, context := range oracle[0][1:] {
		t.Run(context, func(t *testing.T) {
			api := strings.Split(context, "_")[1] + ".0"
			machine := searchWideMachine(t, api, "Quartzwindow", labels, false, false)
			for i, input := range queries[1:] {
				id, query := input[0], input[2]
				row := oracle[i+1]
				if id != row[0] {
					t.Fatalf("query %s native row %s", id, row[0])
				}
				machine.ResetLimits()
				groups, err := machine.searchQuery([]Value{String(query)})
				got := searchDenseCounts(searchWideObservation(t, groups, err, true))
				if want := searchDenseCounts(row[column+1]); got != want {
					t.Errorf("org.tsv:%d %s: got %s, want %s", i+2, id, got, want)
				}
			}
		})
	}
}

// searchDenseCounts keeps each bucket's leading row count; "[]" counts zero.
func searchDenseCounts(text string) string {
	return searchWindowBucket.ReplaceAllStringFunc(text, func(bucket string) string {
		count, _, _ := strings.Cut(strings.Trim(bucket, "[]"), ":")
		if count == "" {
			count = "0"
		}
		return "[" + count + "]"
	})
}
