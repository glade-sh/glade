package vm

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE Site.getPathPrefix without a Site request context
// ../apextest/testdata/conformance/site_path_prefix/org.tsv anonymous+istest 62.0+67.0.
// Native returns ” in a plain org and in an org with Communities declared and
// no active Site. Rows that Glade still answers from its seeded Site record
// are listed in siteSeedDivergence and keep their origin/main behavior.
var siteSeedDivergence = map[string]bool{
	"getSiteId": true, "getMasterLabel": true, "getAdminEmail": true,
	"getOriginalUrl": true, "getAnalyticsTrackingCode": true,
}

func TestExecSitePathPrefixNativeConformance(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/site_path_prefix/"
	read := func(name string) [][]string {
		t.Helper()
		f, err := os.Open(fixture + name)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma = '\t'
		r.LazyQuotes = true
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	expressions := map[string]string{}
	for _, row := range read("cases.tsv")[1:] {
		expressions[row[0]] = row[1]
	}
	covered := 0
	for _, row := range read("org.tsv")[1:] {
		config, api, route, name, want := row[0], row[1], row[2], row[3], row[4]
		if strings.HasPrefix(want, "COMPILE|") || siteSeedDivergence[name] {
			continue
		}
		covered++
		t.Run(strings.Join(row[:4], "/"), func(t *testing.T) {
			body := `String observed;
try {
 Object result = ` + expressions[name] + `;
 observed = result == null ? 'NULL' : 'VALUE|' + String.valueOf(result);
} catch (Exception e) { observed = 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage(); }
System.assertEquals('` + strings.ReplaceAll(want, "'", "\\'") + `', observed);`
			program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			if config == "sites_communities" {
				storage.ApplyOrgShape(&org, []string{"Sites", "Communities", "NetworksEnabled"})
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if route == "istest" {
				machine.EnableTestContext()
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	if covered != 64 {
		t.Fatalf("native matrix: %d cells, want 64", covered)
	}
}
