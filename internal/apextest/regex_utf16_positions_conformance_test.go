package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// NATIVE testdata/regex_java_divergence/org.tsv:1690-1695,1698,1706 isTest 62.0
// NATIVE testdata/regex_java_divergence/org.tsv:3487-3492,3495,3503 isTest 67.0
func TestRegexUTF16PositionsNativeIsTestRows(t *testing.T) {
	_, rows, _ := loadRegexDivergence(t)
	selected := map[string]string{
		"d10_split_empty": "00", "d10_split_empty_list": "00",
		"d10_find_count": "00", "d10_find_starts": "00",
		"d10_replace_empty": "00", "d10_replace_empty_vis": "00",
		"d10_replace_star": "01", "d10_region_astral": "01",
	}
	count := 0
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for _, suffix := range []string{"00", "01"} {
				class := "Rxd10" + suffix + api[:2]
				for _, extension := range []string{".cls", ".cls-meta.xml"} {
					rel := filepath.Join("force-app", "main", "default", "classes", class+extension)
					data, err := os.ReadFile(filepath.Join(regexJavaDivergenceFixture, "project", rel))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, rel), string(data))
				}
			}
			index := loadTestIndex(t, root)
			for _, row := range rows {
				suffix, ok := selected[row.id]
				if row.api != api || row.context != "isTest" || !ok {
					continue
				}
				count++
				row := row
				t.Run(row.id, func(t *testing.T) {
					class := "Rxd10" + suffix + api[:2]
					result := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: row.id, NoDiskCache: true, Parallelism: 1})
					if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("missing observation: %+v", result.Suites)
					}
					message := result.Suites[0].Cases[0].Problem.Message
					_, got, ok := strings.Cut(message, ", Actual: ")
					if !ok {
						got = "TESTOUTCOME " + message
					}
					for _, want := range row.values {
						if got == want {
							return
						}
					}
					t.Fatalf("got %q, native %q", got, row.values)
				})
			}
		})
	}
	if count != 16 {
		t.Fatalf("selected %d rows, want 16", count)
	}
}
