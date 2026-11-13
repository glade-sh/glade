package vm

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// searchCapOrder returns the insertion order of the 321 labels a native
// population used: asc, desc or a seeded Fisher-Yates shuffle.
func searchCapOrder(kind string) []int {
	order := make([]int, 321)
	for i := range order {
		order[i] = i
		if kind == "desc" {
			order[i] = 320 - i
		}
	}
	if seed, ok := strings.CutPrefix(kind, "shuffled"); ok {
		s, _ := strconv.ParseInt(seed, 10, 64)
		for i := 320; i > 0; i-- {
			s = (s*1103515245 + 12345) % 2147483648
			j := int(s % int64(i+1))
			order[i], order[j] = order[j], order[i]
		}
	}
	return order
}

func searchCapRanges(t *testing.T, text string) map[int]bool {
	t.Helper()
	set := make(map[int]bool)
	for _, part := range strings.Split(text, ",") {
		lo, hi, ranged := strings.Cut(part, "-")
		first, err := strconv.Atoi(lo)
		if err != nil {
			t.Fatalf("range %q", part)
		}
		last := first
		if ranged {
			if last, err = strconv.Atoi(hi); err != nil {
				t.Fatalf("range %q", part)
			}
		}
		for i := first; i <= last; i++ {
			set[i] = true
		}
	}
	return set
}

// NATIVE which rows a capped (250) indexed group keeps: sosl_index_cap/org.tsv,
// anonymous 62.0 and 67.0, three orgs, each query twice. Where the survivors
// were identical in every cell they are the 250 earliest-inserted records;
// where they varied between orgs only the count of 250 is asserted.
func TestSOSLIndexCapSurvivorsNative(t *testing.T) {
	raw, err := os.ReadFile("testdata/conformance/sosl_index_cap/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	type population struct {
		object, field, term, order string
	}
	populations := map[string]population{
		"lead_desc_inserted":         {"Lead", "LastName", "Qleadx", "desc"},
		"opportunity_shuffled_seed7": {"Opportunity", "Name", "Qoppx", "shuffled7"},
		"campaign_asc_inserted":      {"Campaign", "Name", "Qcampx", "asc"},
		"campaign_desc_inserted":     {"Campaign", "Name", "Qcampy", "desc"},
		"case_asc_three_inserts":     {"Case", "Subject", "Qcasex", "asc"},
		"contact_asc_inserted":       {"Contact", "LastName", "Qselaa", "asc"},
		"contact_desc_inserted":      {"Contact", "LastName", "Qselab", "desc"},
		"contact_shuffled_seed7":     {"Contact", "LastName", "Qselac", "shuffled7"},
		"contact_shuffled_seed99":    {"Contact", "LastName", "Qselad", "shuffled99"},
		"contact_asc_three_inserts":  {"Contact", "LastName", "Qselae", "asc"},
	}
	for i, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("org.tsv:%d malformed", i+2)
		}
		parts := strings.Split(fields[0], ".")
		pop, ok := populations[parts[0]]
		if !ok || len(parts) != 3 {
			t.Fatalf("org.tsv:%d unknown case %s", i+2, fields[0])
		}
		group, _ := strconv.Atoi(strings.TrimPrefix(parts[2], "group"))
		objects := []string{pop.object}
		var query string
		switch parts[1] {
		case "bareFields":
			query = fmt.Sprintf("FIND {%s} RETURNING %s(Id,%s)", pop.term, pop.object, pop.field)
		case "bareObj":
			query = fmt.Sprintf("FIND {%s} RETURNING %s", pop.term, pop.object)
		case "twoPlain":
			objects = []string{"Contact", "Account"}
			query = fmt.Sprintf("FIND {%s} RETURNING Contact(Id,LastName),Account(Id,Name)", pop.term)
		case "twoOrder":
			objects = []string{"Contact", "Account"}
			query = fmt.Sprintf("FIND {%s} RETURNING Contact(Id,LastName ORDER BY LastName),Account(Id,Name ORDER BY Name)", pop.term)
		default:
			t.Fatalf("org.tsv:%d unknown query %s", i+2, parts[1])
		}
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(fmt.Sprintf("%s/%s/%s", fields[0], api, strings.ReplaceAll(api, ".", "")), func(t *testing.T) {
				order := searchCapOrder(pop.order)
				labels := make(map[string][]string)
				position := make(map[string]map[string]int)
				for _, object := range objects {
					for _, label := range order {
						labels[object] = append(labels[object], fmt.Sprintf("%03d", label))
					}
					position[object] = make(map[string]int)
				}
				machine := searchWideMachine(t, api, pop.term, labels, false, false)
				machine.ResetLimits()
				groups, err := machine.searchQuery([]Value{String(query)})
				if err != nil {
					t.Fatal(err)
				}
				records := groups.List[group].List
				if len(records) != 250 {
					t.Fatalf("org.tsv:%d returned %d rows, native 250", i+2, len(records))
				}
				if fields[3] == "varies" {
					return
				}
				want := searchCapRanges(t, fields[3])
				for _, record := range records {
					id, ok := valueIDString(record.Fields["Id"])
					if !ok || len(id) < 12 {
						t.Fatalf("record without an Id: %+v", record.Fields)
					}
					index, err := strconv.Atoi(strings.TrimLeft(id[len(id)-12:], "0"))
					if err != nil {
						t.Fatal(err)
					}
					if label := order[index-1]; !want[label] {
						t.Fatalf("org.tsv:%d kept label %03d, native survivors %s", i+2, label, fields[3])
					}
				}
			})
		}
	}
}
