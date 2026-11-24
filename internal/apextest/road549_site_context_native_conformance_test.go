package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSiteContextRoutesNativeConformance(t *testing.T) {
	f, err := os.Open("testdata/conformance/road549_site_context/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 25 {
		t.Fatalf("native rows = %d, want 24", len(rows)-1)
	}
	observations := map[string]string{}
	for _, row := range rows[1:] {
		key := fmt.Sprintf("%s/%s/%s/%s", row[0], row[1], row[2], row[3])
		if _, exists := observations[key]; exists {
			t.Fatalf("duplicate observation %s", key)
		}
		observations[key] = row[4]
	}
	for _, state := range []string{"absent", "present"} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(state+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				features := "[]"
				if state == "present" {
					features = "[Sites]"
				}
				writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: "+features+"\n")
				count := 0
				if state == "present" {
					count = 1
				}
				body := fmt.Sprintf("System.assertEquals(%d, [SELECT COUNT() FROM Site]);\n", count)
				for _, method := range []string{"getPathPrefix", "getBaseUrl", "getSiteId"} {
					var expected string
					for _, route := range []string{"anonymous", "istest"} {
						key := fmt.Sprintf("%s/%s/%s/%s", state, api, route, method)
						value, ok := observations[key]
						if !ok {
							t.Fatalf("missing native observation %s", key)
						}
						if expected != "" && expected != value {
							t.Fatalf("route mismatch for %s", key)
						}
						expected = value
					}
					switch expected {
					case "VALUE|":
						body += fmt.Sprintf("System.assert(Site.%s() != null); System.assertEquals('', Site.%s());\n", method, method)
					case "NULL":
						body += fmt.Sprintf("System.assertEquals(null, Site.%s());\n", method)
					default:
						t.Fatalf("unsupported native observation %q", expected)
					}
				}
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}
