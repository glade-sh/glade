package server

import (
	"encoding/csv"
	"encoding/xml"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/config"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestR6FlowCapturedHostRoutes(t *testing.T) {
	cases := []struct {
		name, fixture, flows, first, second, setup string
		assigned                                   bool
	}{
		{"profile", "r6flow_profile", "R6_ProfileTrue,R6_ProfileFalse", "R6True", "R6False", "", false},
		{"user", "r6flow_user", "R6_UserTrue,R6_UserFalse", "R6UserTrue", "R6UserFalse", "", false},
		{"permissionUnassigned", "r6flow_permission", "R6_Permission", "R6Perm", "R6Perm", "", false},
		{"permissionAssigned", "r6flow_permission", "R6_Permission", "R6Perm", "R6Perm", "", true},
		{"loop", "r6flow_loop", "R6_Loop", "R6Loop", "R6Loop", "insert new Account(Name='R6 Loop Match');", false},
		{"multi", "r6flow_multi", "R6_Multi", "R6Multi", "R6Multi", "insert new List<Account>{new Account(Name='R6 Two'),new Account(Name='R6 Two')};", false},
		{"expiryFuture", "r6flow_expiry", "R6_Permission", "R6Perm", "R6Perm", "", true},
		{"expiryPast", "r6flow_expiry", "R6_Permission", "R6Perm", "R6Perm", "", true},
		{"expiryRevoked", "r6flow_expiry", "R6_Permission", "R6Perm", "R6Perm", "", true},
	}
	for _, tc := range cases {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(tc.name+"/"+api, func(t *testing.T) {
				fixture := filepath.Join("../apextest/testdata/conformance", tc.fixture)
				root := t.TempDir()
				writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				for _, name := range strings.Split(tc.flows, ",") {
					data, err := os.ReadFile(filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), "force-app/main/default/flows", name+".flow-meta.xml"))
					if err != nil {
						t.Fatal(err)
					}
					writeServerTestFile(t, filepath.Join(root, "force-app/main/default/flows", name+".flow-meta.xml"), string(data))
				}
				firstDescription, secondDescription := "R6 Loop Match", "R6 Loop Absent"
				if tc.name == "multi" {
					firstDescription, secondDescription = "R6 None", "R6 Two"
				}
				observation := `return probe('` + tc.first + `','` + firstDescription + `')+'|'+probe('` + tc.second + `','` + secondDescription + `');`
				if strings.HasPrefix(tc.name, "expiry") {
					observation = `return FeatureManagement.checkPermission('R6_Bypass')+'|'+probe('R6Perm','');`
				}
				source := `public class R6Host { private static String probe(String name,String description){try{insert new Opportunity(Name=name,Description=description,StageName='Prospecting',CloseDate=Date.today());return 'PASS';}catch(Exception e){return e.getMessage().contains('R6MATCH')?'MATCH':'OTHER';}} @AuraEnabled public static String observe(){` + tc.setup + observation + `} public String getOutput(){return observe();} }`
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/R6Host.cls"), source)
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/R6Host.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/R6Host.page"), `<apex:page controller="R6Host"><apex:outputText value="{!output}"/></apex:page>`)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				sourceMetadata, err := NewSourceMetadataFromProject(p)
				if err != nil {
					t.Fatal(err)
				}
				schema, err := gladeschema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				flows, err := automation.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				org.APIVersion = api
				for _, name := range []string{"Opportunity", "Account", "User", "Profile", "PermissionSet", "PermissionSetAssignment"} {
					storage.EnsureStandardObject(&org, name)
				}
				profile := storage.ID("00e000000000001")
				state := org.Objects["Profile"]
				state.Records[profile] = storage.Record{ID: profile, Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}}
				org.Objects["Profile"] = state
				state = org.Objects["User"]
				state.Records[storage.DefaultUserID] = storage.Record{ID: storage.DefaultUserID, Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue(profile)}}
				org.Objects["User"] = state
				if tc.assigned {
					ps := storage.ID("0PS000000000001")
					state = org.Objects["PermissionSet"]
					state.Records[ps] = storage.Record{ID: ps, Object: "PermissionSet", Fields: map[string]storage.Value{"CustomPermissions": storage.ListValue(storage.StringValue("R6_Bypass"))}}
					org.Objects["PermissionSet"] = state
					state = org.Objects["PermissionSetAssignment"]
					fields := map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetId": storage.IDValue(ps)}
					switch tc.name {
					case "expiryFuture":
						fields["ExpirationDate"] = storage.DateTimeValue(r6flowDefaultClockOffset(t, 48*time.Hour))
					case "expiryPast":
						fields["ExpirationDate"] = storage.DateTimeValue(r6flowDefaultClockOffset(t, -48*time.Hour))
					case "expiryRevoked":
						fields["IsRevoked"] = storage.BooleanValue(true)
					}
					state.Records["0Pa000000000001"] = storage.Record{ID: "0Pa000000000001", Object: "PermissionSetAssignment", Fields: fields}
					org.Objects["PermissionSetAssignment"] = state
				}
				automation.ApplyToOrg(&org, flows)
				srv := newVisualforceHTMLTestServer(t, &org, sourceMetadata)
				srv.VisualforceHTMLUserID = storage.DefaultUserID
				srv.SetProjectIndex(typesys.Build(p, schema))
				if srv.runtimeErr != nil {
					t.Fatal(srv.runtimeErr)
				}
				f, err := os.Open(filepath.Join(fixture, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				reader := csv.NewReader(f)
				reader.Comma = '\t'
				rows, err := reader.ReadAll()
				f.Close()
				if err != nil {
					t.Fatal(err)
				}
				values := map[string]string{}
				for _, row := range rows[1:] {
					if row[0] == api && row[1] == "anonymous" {
						values[row[2]] = row[3]
					}
				}
				first, second := "true", "false"
				if tc.name == "loop" {
					first, second = "found", "absent"
				}
				if strings.HasPrefix(tc.name, "permission") {
					first, second = "unassigned", "unassigned"
					if tc.assigned {
						first, second = "assigned", "assigned"
					}
				}
				if tc.name == "multi" {
					first, second = "none", "two"
				}
				if strings.HasPrefix(tc.name, "expiry") {
					first = strings.ToLower(strings.TrimPrefix(tc.name, "expiry"))
					second = first
				}
				want := values[first] + "|" + values[second]
				if strings.HasPrefix(tc.name, "expiry") {
					want = values[first]
				}
				if values[first] == "" || values[second] == "" {
					t.Fatal("missing capture row")
				}
				t.Run("vf", func(t *testing.T) {
					rec := httptest.NewRecorder()
					srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/R6Host", nil))
					if rec.Code != http.StatusOK || !strings.Contains(html.UnescapeString(rec.Body.String()), want) {
						text := html.UnescapeString(rec.Body.String())
						_, body, _ := strings.Cut(text, "<body>")
						body, _, _ = strings.Cut(body, "<input")
						t.Fatalf("VF %d: output=%q want=%q", rec.Code, body, want)
					}
				})
				t.Run("lwc", func(t *testing.T) {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, "/lightning/apex/R6Host/observe", strings.NewReader(`{}`))
					req.Header.Set("Content-Type", "application/json")
					srv.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
						t.Fatalf("LWC %d: %s", rec.Code, rec.Body.String())
					}
				})
				t.Run("server", func(t *testing.T) {
					probe := func(variable, name, description string) string {
						return `String ` + variable + `='PASS';try{insert new Opportunity(Name='` + name + `',Description='` + description + `',StageName='Prospecting',CloseDate=Date.today());}catch(Exception e){` + variable + `=e.getMessage().contains('R6MATCH')?'MATCH':'OTHER';}`
					}
					body := tc.setup + probe("a", tc.first, firstDescription) + probe("b", tc.second, secondDescription) + `System.assertEquals('` + want + `',a+'|'+b);`
					if strings.HasPrefix(tc.name, "expiry") {
						body = probe("a", tc.first, "") + `System.assertEquals('` + want + `',FeatureManagement.checkPermission('R6_Bypass')+'|'+a);`
					}
					var escaped strings.Builder
					if err := xml.EscapeText(&escaped, []byte(body)); err != nil {
						t.Fatal(err)
					}
					rec := httptest.NewRecorder()
					srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/Soap/s/"+api+"/00DLOCAL000000001", strings.NewReader(soapEnvelope("<apex:executeAnonymous><apex:String>"+escaped.String()+"</apex:String></apex:executeAnonymous>"))))
					if api == "62.0" {
						if !strings.Contains(rec.Body.String(), "INVALID_VERSION") {
							t.Fatalf("SOAP floor: %d %s", rec.Code, rec.Body.String())
						}
						return
					}
					if rec.Code != http.StatusOK {
						t.Fatalf("SOAP %d: %s", rec.Code, rec.Body.String())
					}
					result := decodeSOAPResult(t, rec.Body.String())
					if result["compiled"] != "true" || result["success"] != "true" {
						t.Fatalf("SOAP result: %v", result)
					}
				})
			})
		}
	}
}

// r6flowDefaultClockOffset returns the default test clock start plus offset,
// as an RFC3339 datetime.
func r6flowDefaultClockOffset(t *testing.T, offset time.Duration) string {
	t.Helper()
	start, err := time.Parse(time.RFC3339, config.DefaultTestClockStart)
	if err != nil {
		t.Fatal(err)
	}
	return start.Add(offset).UTC().Format(time.RFC3339)
}
