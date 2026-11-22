package apextest

import (
	"fmt"
	"testing"
)

// NATIVE testdata/regex_java_divergence/org.tsv:754-759 anonymous 62.0
// NATIVE testdata/regex_java_divergence/org.tsv:762,770 anonymous 62.0
// NATIVE testdata/regex_java_divergence/org.tsv:2501-2506 anonymous 67.0
// NATIVE testdata/regex_java_divergence/org.tsv:2509,2517 anonymous 67.0
func TestRegexUTF16PositionsNativeRows(t *testing.T) {
	cases, rows, _ := loadRegexDivergence(t)
	selected := map[string]bool{
		"d10_split_empty": true, "d10_split_empty_list": true,
		"d10_find_count": true, "d10_find_starts": true,
		"d10_replace_empty": true, "d10_replace_empty_vis": true,
		"d10_replace_star": true, "d10_region_astral": true,
	}
	count := 0
	for _, row := range rows {
		if row.context != "anonymous" || !selected[row.id] {
			continue
		}
		count++
		t.Run(row.api+"/"+row.id, func(t *testing.T) {
			body := "String observation;\n" + fmt.Sprintf(regexJavaDivergenceWrap, cases[row.id])
			got := runRegexDivergenceAnonymous(t, body, row.api, false)
			for _, want := range row.values {
				if got == want {
					return
				}
			}
			t.Fatalf("got %q, native %q", got, row.values)
		})
	}
	if count != 16 {
		t.Fatalf("selected %d rows, want 16", count)
	}
}
