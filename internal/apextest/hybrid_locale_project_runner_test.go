package apextest

import (
	"encoding/csv"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE ../vm/testdata/conformance/hybrid_locale_tokens/fr/org.tsv:4 istest 62.0
// NATIVE ../vm/testdata/conformance/hybrid_locale_tokens/fr/org.tsv:5 istest 67.0
// NATIVE ../vm/testdata/conformance/hybrid_locale_tokens/de/org.tsv:4 istest 62.0
// NATIVE ../vm/testdata/conformance/hybrid_locale_tokens/de/org.tsv:5 istest 67.0
func TestHybridLocaleProjectRunner(t *testing.T) {
	for _, tc := range []struct{ fixture, locale, language string }{{"fr", "fr_FR", "fr"}, {"de", "de_DE", "de"}} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(tc.fixture+"/"+api, func(t *testing.T) {
				f, err := os.Open(filepath.Join("..", "vm", "testdata", "conformance", "hybrid_locale_tokens", tc.fixture, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				r := csv.NewReader(f)
				r.Comma = '\t'
				rows, err := r.ReadAll()
				if err != nil {
					t.Fatal(err)
				}
				want := ""
				for _, row := range rows[1:] {
					if row[1] == api && row[2] == "istest" {
						_, want, _ = strings.Cut(html.UnescapeString(row[5]), "Actual: ")
					}
				}
				if want == "" {
					t.Fatal("missing native observation")
				}
				source, err := os.ReadFile(filepath.Join("..", "vm", "testdata", "conformance", "hybrid_locale_tokens", tc.fixture, "force-app", "main", "default", "classes", "LocaleProbeTest.cls"))
				if err != nil {
					t.Fatal(err)
				}
				setup := fmt.Sprintf(`User u = new User(Alias='probe', Email='probe@example.invalid', EmailEncodingKey='UTF-8', LastName='Probe', LanguageLocaleKey='%s', LocaleSidKey='%s', ProfileId=UserInfo.getProfileId(), TimeZoneSidKey='GMT', Username='probe@example.invalid'); insert u; System.runAs(u) {`, tc.language, tc.locale)
				body := strings.Replace(string(source), "Datetime d =", setup+" Datetime d =", 1)
				body = strings.Replace(body, "System.assertEquals('ROW',", "System.assertEquals('"+want+"',", 1)
				body = strings.Replace(body, "\n }\n}", "\n }\n }\n}", 1)
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/LocaleProbeTest.cls"), body)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/LocaleProbeTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
				report := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"LocaleProbeTest"}, NoDiskCache: true, Parallelism: 1})
				if summary := report.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
					t.Fatalf("runner summary %+v; suites %+v", summary, report.Suites)
				}
			})
		}
	}
}
