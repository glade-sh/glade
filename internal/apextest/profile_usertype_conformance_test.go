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

// profileUserTypeProbe returns the Profile rows (P1) and the profile-owned
// permission set rows (P2) of the native capture as Name|UserType pairs.
const profileUserTypeProbe = `List<String> p1 = new List<String>();
for (Profile p : [SELECT Id, Name, UserType, UserLicenseId, UserLicense.Name, UserLicense.LicenseDefinitionKey FROM Profile ORDER BY Name]) {
    p1.add(p.Name + '|' + p.UserType);
}
List<String> p2 = new List<String>();
for (PermissionSet s : [SELECT Id, Name, ProfileId, Profile.UserType, IsOwnedByProfile FROM PermissionSet WHERE IsOwnedByProfile = true ORDER BY Name]) {
    p2.add(s.Profile.UserType);
}
System.assert(false, 'OBS|' + JSON.serialize(new Map<String, Object>{'P1' => p1, 'P2' => p2}));`

// TestProfileUserTypeNativeConformance compares Profile.UserType of the seeded
// standard profiles with the fresh-org capture (Dot k2f6c90a3d18), anonymous and
// @isTest at API 62.0 and 67.0. Glade also seeds a guest profile the fresh org
// does not list; P1 compares the native profile names only.
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
			Name     string
			UserType *string
			Profile  *struct{ UserType *string }
		}
		if err := json.Unmarshal([]byte(row[header["native_row_json"]]), &record); err != nil {
			t.Fatal(err)
		}
		k := key{row[header["api"]], row[header["route"]]}
		switch row[header["probe"]] {
		case "P1":
			p1[k] = append(p1[k], record.Name+"|"+*record.UserType)
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
			native := map[string]bool{}
			for _, pair := range want {
				name, _, _ := strings.Cut(pair, "|")
				native[name] = true
			}
			var p1Got []string
			for _, pair := range got.P1 {
				if name, _, _ := strings.Cut(pair, "|"); native[name] {
					p1Got = append(p1Got, pair)
				}
			}
			if !reflect.DeepEqual(p1Got, want) {
				t.Fatalf("P1 native=%q\nGlade=%q", want, p1Got)
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
