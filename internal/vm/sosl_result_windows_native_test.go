package vm

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sosl"
	"github.com/glade-sh/glade/internal/storage"
)

const searchWindowFixture = "testdata/conformance/sosl_result_windows/"

func readSearchWindowTable(t *testing.T, name string) [][]string {
	t.Helper()
	file, err := os.Open(searchWindowFixture + name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// NATIVE object slot allocation and OFFSET diagnostics: org.tsv records both
// APIs, two fixed ID orders, and independently indexed anonymous transactions.
func TestSOSLResultWindowsNative(t *testing.T) {
	queries := readSearchWindowTable(t, "queries.tsv")
	oracle := readSearchWindowTable(t, "org.tsv")
	if len(queries) != len(oracle) || len(queries) < 2 {
		t.Fatalf("queries=%d native rows=%d", len(queries), len(oracle))
	}
	for column, context := range oracle[0][1:] {
		t.Run(context, func(t *testing.T) {
			parts := strings.Split(context, "_")
			named := parts[0] == "istest"
			api := parts[1] + ".0"
			machine := searchWindowMachine(t, api, named, parts[2], false)
			var bulkMachine *VM
			for i, input := range queries[1:] {
				id, entry, query := input[0], input[1], input[2]
				row := oracle[i+1]
				if id != row[0] {
					t.Fatalf("query %s native row %s", id, row[0])
				}
				want := row[column+1]
				if want == "-" {
					continue
				}
				bulk := strings.HasPrefix(id, "bulk")
				selected := machine
				if bulk {
					if bulkMachine == nil {
						bulkMachine = searchWindowMachine(t, api, named, parts[2], true)
					}
					selected = bulkMachine
				}
				selected.ResetLimits()
				groups, err := searchWindowGroups(selected, entry, query)
				got := searchWindowObservation(t, groups, err, bulk)
				// The hosted index ranks before it truncates. For anonymous
				// groups without ORDER BY, only the slot count is a window
				// contract. Fixed order and explicitly ordered fields are exact.
				if !named && err == nil && strings.HasPrefix(want, "[") {
					parsed, parseErr := sosl.Parse(query)
					if parseErr != nil {
						t.Fatal(parseErr)
					}
					if bulk && len(parsed.Returning) > 1 {
						// Large indexed searches returned different ordered endpoints
						// on repeat. Preserve the observed cardinality in each group.
						got, want = searchWindowCounts(got), searchWindowCounts(want)
					} else {
						got = searchWindowComparable(got, parsed)
						want = searchWindowComparable(want, parsed)
					}
				}
				if got != want {
					t.Errorf("org.tsv:%d %s: got %s, want %s", i+2, id, got, want)
				}
			}
		})
	}
}

var searchWindowBucket = regexp.MustCompile(`\[([^\]]*)\]`)

func searchWindowCounts(text string) string {
	return searchWindowBucket.ReplaceAllStringFunc(text, func(bucket string) string {
		count, _, _ := strings.Cut(bucket, ":")
		return count + "]"
	})
}

func searchWindowComparable(text string, query sosl.Query) string {
	index := 0
	return searchWindowBucket.ReplaceAllStringFunc(text, func(bucket string) string {
		i := index
		index++
		if i >= len(query.Returning) || len(query.Returning[i].OrderBy) != 0 || bucket == "[]" {
			return bucket
		}
		return fmt.Sprintf("[count=%d]", strings.Count(bucket, ",")+1)
	})
}

func searchWindowGroups(machine *VM, entry, query string) (Value, error) {
	switch entry {
	case "inline":
		return machine.executeSOSL(strings.Replace(query, "{Copperwindow}", "'Copperwindow'", 1), nil)
	case "find":
		results, err := machine.searchFind([]Value{String(query)})
		if err != nil {
			return Null, err
		}
		parsed, err := sosl.Parse(query)
		if err != nil {
			return Null, err
		}
		groups := List()
		for _, spec := range parsed.Returning {
			rows := List()
			hits := results.Fields["results"].Map[mapKey(String(spec.Object))]
			for _, hit := range hits.List {
				rows.List = append(rows.List, hit.Fields["sObject"])
			}
			groups.List = append(groups.List, rows)
		}
		return groups, nil
	default:
		return machine.searchQuery([]Value{String(query)})
	}
}

func searchWindowObservation(t *testing.T, groups Value, err error, bulk bool) string {
	t.Helper()
	if err != nil {
		var thrown *apexThrowError
		if !errors.As(err, &thrown) {
			t.Fatal(err)
		}
		return "System." + strings.TrimPrefix(thrown.value.Type, "System.") + "|" + thrown.value.Fields["message"].Text
	}
	var result strings.Builder
	for _, bucket := range groups.List {
		var labels []string
		for _, record := range bucket.List {
			field := "LastName"
			if record.Type == "Account" {
				field = "Name"
			}
			_, label, _ := strings.Cut(record.Fields[field].Text, " ")
			labels = append(labels, label)
		}
		text := strings.Join(labels, ",")
		if bulk {
			first, last := "", ""
			if len(labels) > 0 {
				first, last = labels[0], labels[len(labels)-1]
			}
			text = fmt.Sprintf("%d:%s:%s", len(labels), first, last)
		}
		fmt.Fprintf(&result, "[%s]", text)
	}
	return result.String()
}

func searchWindowMachine(t *testing.T, api string, named bool, repeat string, bulk bool) *VM {
	t.Helper()
	org := storage.NewOrgState()
	org.APIVersion = api
	ids := make(map[string]Value)
	seed := func(object, field, prefix, term string, labels []string) {
		storage.EnsureStandardObject(&org, object)
		state := org.Objects[object]
		for i, label := range labels {
			id := storage.ID(fmt.Sprintf("%s%012d", prefix, i+1))
			ids[label] = String(string(id))
			state.Records[id] = storage.Record{Object: object, ID: id, Fields: map[string]storage.Value{
				field: storage.StringValue(term + " " + label),
			}}
		}
		org.Objects[object] = state
	}
	var fixed []string
	if bulk {
		labels := make([]string, 2002)
		for i := range labels {
			labels[i] = fmt.Sprintf("%04d", i)
		}
		seed("Account", "Name", "001", "Lindenwindow", labels)
		seed("Contact", "LastName", "003", "Lindenwindow", []string{"K", "M"})
		fixed = append(labels, "K", "M")
	} else {
		seed("Account", "Name", "001", "Copperwindow", strings.Fields("A B C D E"))
		seed("Contact", "LastName", "003", "Copperwindow", strings.Fields("K L M"))
		seed("Lead", "LastName", "00Q", "Copperwindow", strings.Fields("X Y"))
		fixed = strings.Fields("L D X B K E M Y C A")
		if repeat == "2" {
			fixed = strings.Fields("B M Y C K D X E L A")
		}
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod.APIVersion = api
	if named {
		machine.EnableTestContext()
		list := List()
		for _, label := range fixed {
			list.List = append(list.List, ids[label])
		}
		if _, err := machine.testSetFixedSearchResults([]Value{list}); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}
