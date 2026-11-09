package apextest

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"
)

// The reference-facets packet replays these describe rows on both routes.
func TestB5B9FeatureReferenceFacetsNativeRoutes(t *testing.T) {
	f, err := os.Open("testdata/conformance/standard_describe_profiles/b5b9_feature_objects/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			body := `Map<String,Schema.SObjectField> fields=Schema.getGlobalDescribe().get('DocumentTemplateContentDoc').getDescribe().fields.getMap();`
			count := 0
			for _, row := range rows[1:] {
				if len(row) != 5 || row[0] != api || row[1] != "docgen-isolated" || row[2] != "anonymous" || !strings.HasSuffix(row[3], "-facets") {
					continue
				}
				field := strings.TrimSuffix(strings.TrimPrefix(row[3], "DocumentTemplateContentDoc."), "-facets")
				values := strings.Split(row[4], "|")
				if len(values) != 5 || values[0] != "REFERENCE" {
					t.Fatalf("bad native facet row: %v", row)
				}
				body += `{Schema.DescribeFieldResult f=fields.get('` + field + `').getDescribe();System.assertEquals('` + values[2] + `',f.getRelationshipName());System.assertEquals('` + values[1] + `',f.getReferenceTo()[0].getDescribe().getName());System.assertEquals(` + values[3] + `,f.isCreateable());System.assertEquals(` + values[4] + `,f.isUpdateable());}`
				count++
			}
			if count != 3 {
				t.Fatalf("native facet count=%d", count)
			}
			body += `System.assertEquals(false,fields.containsKey('OwnedMissingField'));`
			runStandardFieldEvidenceRoutes(t, b5b9AccessProject(t, "doc", api), api, body)
		})
	}
}
