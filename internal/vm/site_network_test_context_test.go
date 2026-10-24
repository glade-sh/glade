package vm

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecSiteNetworkTestContextNativeRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/site_network_test_context/"
	read := func(name string) [][]string {
		t.Helper()
		f, err := os.Open(fixture + name)
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
		return rows
	}
	expressions := map[string]string{}
	for _, row := range read("cases.tsv")[1:] {
		expressions[row[0]] = row[1]
	}
	covered := 0
	for _, row := range read("org.tsv")[1:] {
		expression, ok := expressions[row[3]]
		if !ok || row[2] != "istest" {
			continue
		}
		covered++
		t.Run(strings.Join(row[:4], "/"), func(t *testing.T) {
			// Anonymous execution enters the same static dispatch used by class tests.
			body := `User externalUser = new User(Username='visitor@example.invalid', Email='visitor@example.invalid', LastName='Visitor', Alias='visit');
User portalUser = externalUser.clone();
portalUser.ProfileId = '00e000000000001';
String observed;
try {
 Object result = ` + expression + `;
 observed = result == null ? 'NULL' : (result instanceof PageReference ? 'PAGE|' + ((PageReference)result).getUrl() : 'VALUE|' + String.valueOf(result));
} catch (Exception e) { observed = 'EXCEPTION|' + e.getTypeName() + '|' + e.getMessage(); }
System.assertEquals('` + strings.ReplaceAll(row[4], "'", "\\'") + `', observed);`
			program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			if row[0] != "networks_disabled" {
				storage.ApplyOrgShape(&org, []string{"Sites", "Communities"})
			}
			if row[0] == "networks_enabled" {
				for _, name := range []string{"Site", "Network"} {
					object := org.Objects[name]
					object.Records = map[storage.ID]storage.Record{}
					org.Objects[name] = object
				}
			}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	if covered != 78 {
		t.Fatalf("native test matrix: %d cells, want 78", covered)
	}
}

// Hosted anonymous DTO behavior is retained independently of native test rows.
func TestExecSiteNetworkAnonymousDTOsRemainAvailable(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
System.assertEquals('/home', Site.login('visitor@example.invalid', 'invalid', '/home').getUrl());
System.assertEquals('/home', Network.forwardToAuthPage('/home').getUrl());
System.assertEquals('/home', Network.forwardToAuthPage('/home', 'page').getUrl());
System.assertEquals('707000000000001', Network.createExternalUserAsync(new User(), new Contact(), new Account()));
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.ApplyOrgShape(&org, []string{"Sites", "Communities"})
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
