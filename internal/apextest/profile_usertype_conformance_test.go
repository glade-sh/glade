package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// profileUserTypeProbe returns the Profile rows (P1) of the native capture as
// Name|UserType|UserLicense.Name|UserLicense.LicenseDefinitionKey and the
// profile-owned permission set rows (P2) as Profile.UserType.
const profileUserTypeProbe = `List<String> p1 = new List<String>();
for (Profile p : [SELECT Id, Name, UserType, UserLicenseId, UserLicense.Name, UserLicense.LicenseDefinitionKey FROM Profile ORDER BY Name]) {
    p1.add(p.Name + '|' + p.UserType + '|' + p.UserLicense.Name + '|' + p.UserLicense.LicenseDefinitionKey);
}
List<String> p2 = new List<String>();
for (PermissionSet s : [SELECT Id, Name, ProfileId, Profile.UserType, IsOwnedByProfile FROM PermissionSet WHERE IsOwnedByProfile = true ORDER BY Name]) {
    p2.add(s.Profile.UserType);
}
System.assert(false, 'OBS|' + JSON.serialize(new Map<String, Object>{'P1' => p1, 'P2' => p2}));`

// TestProfileUserTypeNativeConformance compares the seeded standard profiles,
// their UserType and their linked license (name and key), with the fresh-org
// capture, anonymous and @isTest at API 62.0 and 67.0. The
// profile inventory must match exactly: no profile the fresh org lacks.
func TestProfileUserTypeNativeConformance(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "conformance", "profile_usertype", "org.tsv"))
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
	header := map[string]int{}
	for i, name := range rows[0] {
		header[name] = i
	}
	type key struct{ api, route string }
	p1 := map[key][]string{}
	p2 := map[key][]string{}
	for _, row := range rows[1:] {
		if row[header["record_kind"]] != "ROW" {
			continue
		}
		var record struct {
			Name        string
			UserType    *string
			UserLicense *struct{ Name, LicenseDefinitionKey string }
			Profile     *struct{ UserType *string }
		}
		if err := json.Unmarshal([]byte(row[header["native_row_json"]]), &record); err != nil {
			t.Fatal(err)
		}
		k := key{row[header["api"]], row[header["route"]]}
		switch row[header["probe"]] {
		case "P1":
			p1[k] = append(p1[k], record.Name+"|"+*record.UserType+"|"+record.UserLicense.Name+"|"+record.UserLicense.LicenseDefinitionKey)
		case "P2":
			p2[k] = append(p2[k], *record.Profile.UserType)
		}
	}
	if len(p1) != 4 || len(p2) != 4 {
		t.Fatalf("native groups P1=%d P2=%d, want 4 each", len(p1), len(p2))
	}
	for k, want := range p1 {
		t.Run(k.api+"/"+k.route, func(t *testing.T) {
			if len(want) != 38 || len(p2[k]) != 38 {
				t.Fatalf("native rows P1=%d P2=%d, want 38", len(want), len(p2[k]))
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+k.api+`"}`)
			classes := filepath.Join(root, "force-app", "main", "default", "classes")
			writeFile(t, filepath.Join(classes, "ProfileUserTypeProbe.cls"), "@IsTest\nprivate class ProfileUserTypeProbe {\n    @IsTest static void probe() {\n"+profileUserTypeProbe+"\n    }\n}\n")
			writeFile(t, filepath.Join(classes, "ProfileUserTypeProbe.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+k.api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			var message string
			if k.route == "anonymous" {
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: func() *storage.OrgState {
					org := orgFromIndex(index)
					org.APIVersion = k.api
					return &org
				}()})
				_, err := runAnonymousObservation(t, runner, index, profileUserTypeProbe, k.api)
				message = anonymousAssertionMessage(t, err)
			} else {
				run := Run(index, Options{SelectedClasses: []string{"ProfileUserTypeProbe"}, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("expected one assertion transport: %+v", run.Summary())
				}
				message = run.Suites[0].Cases[0].Problem.Message
			}
			_, payload, _ := strings.Cut(message, "OBS|")
			var got struct{ P1, P2 []string }
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatalf("invalid observation %q", message)
			}
			// ORDER BY Name leaves same-name profiles (the two Authenticated
			// Website rows) in an Id order Glade does not reproduce, so the
			// name order is compared as listed and the rows as a set.
			names := func(rows []string) []string {
				out := make([]string, len(rows))
				for i, row := range rows {
					out[i], _, _ = strings.Cut(row, "|")
				}
				return out
			}
			if !reflect.DeepEqual(names(got.P1), names(want)) {
				t.Fatalf("P1 names native=%q\nGlade=%q", names(want), names(got.P1))
			}
			p1Want := append([]string(nil), want...)
			sort.Strings(p1Want)
			sort.Strings(got.P1)
			if !reflect.DeepEqual(got.P1, p1Want) {
				t.Fatalf("P1 native=%q\nGlade=%q", p1Want, got.P1)
			}
			p2Want := append([]string(nil), p2[k]...)
			sort.Strings(p2Want)
			sort.Strings(got.P2)
			if !reflect.DeepEqual(got.P2, p2Want) {
				t.Fatalf("P2 Profile.UserType native=%q\nGlade=%q", p2Want, got.P2)
			}
		})
	}
}
