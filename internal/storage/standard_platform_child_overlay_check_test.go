package storage

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"
)

// Native field facets must drive both the current and unmemoized write-check
// routes. A persisted overlay marker cannot hide a missing or stale edge field.
func TestStandardParentPlatformChildNativeOverlayCheckRoutes(t *testing.T) {
	f, err := os.Open("../../testdata/conformance/standard_platform_child_fields/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	table, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range table[1:] {
		if record[0] != "67.0" || record[1] != "anonymous" {
			continue
		}
		row := strings.Split(record[2], "|")
		for _, route := range []string{"current-write", "current-refresh", "unmemoized-write", "unmemoized-refresh"} {
			t.Run(row[0]+"/"+row[1]+"/"+route, func(t *testing.T) {
				check := map[string]func(ObjectDefinition, string) bool{
					"current-write":      standardObjectFieldsNeedWrite,
					"current-refresh":    standardFieldsOverlayNeedsRefresh,
					"unmemoized-write":   legacyStandardObjectFieldsNeedWrite,
					"unmemoized-refresh": legacyStandardFieldsOverlayNeedsRefresh,
				}[route]
				definition := enrichedStandardOverlayDefinition(row[2], nil)
				if check(definition, "") {
					t.Fatal("fresh native overlay requests repair")
				}
				name, ok := ResolveFieldName(definition, "", row[3])
				if !ok {
					t.Fatalf("native field %s.%s is missing", row[2], row[3])
				}
				field := definition.Fields[name]
				field.Type = ""
				definition.Fields[name] = field
				if !check(definition, "") {
					t.Fatal("native edge with cleared type does not request repair")
				}
			})
		}
	}
}
