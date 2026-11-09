package apextest_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// The hosts replay the named Apex dynamic-get observations in the feature
// packet. This is a local host bridge test, not a native browser capture.
func TestB5B9FeatureVisualforceAndLWCHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			pageAPI := api
			if api == "62.0" {
				pageAPI = "59.0"
			}
			root := t.TempDir()
			write := func(name, body string) {
				t.Helper()
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("sfdx-project.json", `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			write("config/project-scratch-def.json", `{"edition":"Developer","features":["DocGen","B2BCommerce","Communities","Fundraising","PublicSectorAccess"],"settings":{"communitiesSettings":{"enableNetworksEnabled":true},"commerceSettings":{"commerceEnabled":true},"industriesSettings":{"enableFundraising":true}}}`)
			assignment := ""
			if api == "67.0" {
				assignment = `((DocumentGenerationProcess)r).IsLocalizationEnabled=true; GiftCommitmentSchedule schedule=new GiftCommitmentSchedule(); schedule.GenerationalCohort=null; result+='P';`
			}
			write("force-app/main/default/classes/OwnedFeatureHost.cls", `public class OwnedFeatureHost {
 @AuraEnabled(cacheable=true) public static String observe() {
  SObject r=new DocumentGenerationProcess(); String result='';
  try { r.get('AdditionalMetadata'); result+='P'; } catch(SObjectException e) { result+='A'; }
  try { r.get('IsLocalizationEnabled'); result+='P'; } catch(SObjectException e) { result+='A'; }
  try { r.get(OwnedFeatureTokens.field()); result+='P'; } catch(SObjectException e) { result+='A'; }
  try { Database.countQuery('SELECT count() FROM DocumentGenerationProcess WHERE IsLocalizationEnabled = true'); result+='P'; } catch(QueryException e) { result+='A'; }
  Map<String,Schema.SObjectField> f=PricebookEntry.SObjectType.getDescribe().fields.getMap();
  result+=f.containsKey('ProductSellingModelId') ? 'P' : 'A';
  result+=f.containsKey('ActivePriceAdjustmentQuantity') ? 'P' : 'A';
  try { r.get('DocGenerationBatchProcessId'); result+='P'; } catch(SObjectException e) { result+='A'; }
  SObject gift=new GiftTransaction();
  try { gift.put(OwnedFeatureTokens.gift(),1); result+='P'; } catch(SObjectException e) { result+='A'; }
  Map<String,Schema.SObjectField> df=DocumentTemplateContentDoc.SObjectType.getDescribe().fields.getMap();
  result+=String.valueOf(df.get('ContentDocumentId').getDescribe().getReferenceTo()[0])=='ContentDocument' ? 'P' : 'A';
  Map<String,Schema.SObjectField> gf=GiftTransaction.SObjectType.getDescribe().fields.getMap();
  result+=String.valueOf(gf.get('GiftCommitmentScheduleId').getDescribe().getReferenceTo()[0])=='GiftCommitmentSchedule' ? 'P' : 'A';
  result+=gf.get('GiftCommitmentScheduleId').getDescribe().getReferenceTo()[0].getDescribe().getName()=='GiftCommitmentSchedule' ? 'P' : 'A';
  SObject scheduleRecord=Schema.getGlobalDescribe().get('GiftCommitmentSchedule').newSObject();
  try { scheduleRecord.get('GenerationalCohort'); result+='P'; } catch(SObjectException e) { result+='A'; }
  result+=RegulatoryCode.SObjectType.getDescribe().fields.getMap().containsKey('IsActive') ? 'P' : 'A';
  `+assignment+`
  return result;
 }
 public String getObservation() { return observe(); }
}`)
			write("force-app/main/default/classes/OwnedFeatureHost.cls-meta.xml", `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			write("force-app/main/default/classes/OwnedFeatureTokens.cls", `public class OwnedFeatureTokens { public static Schema.SObjectField field() { return DocumentGenerationProcess.IsLocalizationEnabled; } public static Schema.SObjectField gift() { return GiftTransaction.TaxDeductionAmount; } }`)
			write("force-app/main/default/classes/OwnedFeatureTokens.cls-meta.xml", `<ApexClass><apiVersion>67.0</apiVersion></ApexClass>`)
			write("force-app/main/default/pages/OwnedFeature.page", `<apex:page controller="OwnedFeatureHost" showHeader="false" sidebar="false"><pre id="feature-observation"><apex:outputText value="{!observation}"/></pre></apex:page>`)
			write("force-app/main/default/pages/OwnedFeature.page-meta.xml", `<ApexPage><apiVersion>`+pageAPI+`</apiVersion><label>Owned feature</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			source, err := server.NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			storage.ApplyOrgShape(&org, []string{"DocGen", "B2BCommerce", "Communities", "CommerceEnabled", "Fundraising", "FundraisingEnabled", "PublicSectorAccess"})
			storage.EnsureStandardObject(&org, "PricebookEntry")
			const user storage.ID = "005000000000091"
			storage.EnsureStandardObject(&org, "User")
			users := org.Objects["User"]
			users.Records[user] = storage.Record{ID: user, Object: "User", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true)}}
			org.Objects["User"] = users
			handler := server.NewWithSource(&org, source)
			handler.VisualforceHTMLUserID = user
			handler.SetProjectIndex(typesys.Build(p, loaded))
			want := "AAAAPPPAPPPAP"
			if api == "67.0" {
				want = "PPPPPPPAPPPPPP"
			}
			for _, route := range []string{"vf", "imperative", "wire"} {
				t.Run(route, func(t *testing.T) {
					method, path, body := http.MethodGet, "/apex/OwnedFeature", ""
					if route == "imperative" {
						method, path, body = http.MethodPost, "/lightning/apex/OwnedFeatureHost/observe", `{}`
					}
					if route == "wire" {
						method, path, body = http.MethodPost, "/lightning/wire/apex", `{"className":"OwnedFeatureHost","method":"observe","params":{}}`
					}
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(method, path, strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					handler.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK {
						t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
					}
					if route == "vf" {
						if !strings.Contains(rec.Body.String(), `<pre id="feature-observation">`+want+`</pre>`) {
							t.Fatalf("render=%s want=%s", rec.Body.String(), want)
						}
						return
					}
					var response lwcbrowser.WireResponse
					if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
						t.Fatal(err)
					}
					observed, ok := response.Data.(string)
					if response.Error != nil || !ok || observed != want {
						t.Fatalf("data=%v error=%v want=%s", response.Data, response.Error, want)
					}
				})
			}
		})
	}
}
