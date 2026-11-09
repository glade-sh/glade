package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// The unlicensed capture is an Apex exception; these host routes establish
// that the same outcome survives VF and Lightning dispatch and warm reuse.
func TestKnowledgeB4UnlicensedHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeServerTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","settings":{"knowledgeSettings":{"enableKnowledge":true}}}`)
			base := filepath.Join(root, "force-app/main/default")
			writeServerTestFile(t, filepath.Join(base, "classes/KnowledgeHost.cls"), `public class KnowledgeHost { public String getOutcome() { return run(); } @AuraEnabled public static String run() { try { KbManagement.PublishingService.publishArticle(null,false); return 'returned'; } catch (Exception e) { return e.getTypeName(); } } }`)
			writeServerTestFile(t, filepath.Join(base, "classes/KnowledgeHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(base, "pages/KnowledgeHost.page"), `<apex:page controller="KnowledgeHost" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			vfAPI := api
			if api == "62.0" {
				vfAPI = "59.0"
			}
			writeServerTestFile(t, filepath.Join(base, "pages/KnowledgeHost.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+vfAPI+`</apiVersion><label>Knowledge Host</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			storage.ApplyOrgShape(&org, []string{"Knowledge", "KnowledgeEnabled"})
			host := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, host)
			host.SetProjectIndex(typesys.Build(p, sch))
			for _, route := range []struct{ name, method, path, body string }{{"vf", http.MethodGet, "/apex/KnowledgeHost", ""}, {"lwc", http.MethodPost, "/lightning/apex/KnowledgeHost/run", `{}`}, {"wire", http.MethodPost, "/lightning/wire/apex", `{"className":"KnowledgeHost","method":"run","params":{}}`}} {
				t.Run(route.name, func(t *testing.T) {
					for pass := 0; pass < 2; pass++ {
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
						req.Header.Set("Content-Type", "application/json")
						host.ServeHTTP(rec, req)
						if rec.Code != http.StatusOK {
							t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
						}
						if route.name == "vf" {
							if !strings.Contains(rec.Body.String(), ">System.NoAccessException<") {
								t.Fatal(rec.Body.String())
							}
						} else {
							var out lwcbrowser.WireResponse
							if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
								t.Fatal(err)
							}
							if out.Error != nil || out.Data != "System.NoAccessException" {
								t.Fatalf("%+v", out)
							}
						}
					}
				})
			}
		})
	}
}

// Native lifecycle, assignment and category rows are replayed through the
// same cold and warm VF/Lightning entry points used by controllers.
func TestKnowledgeB4LicensedHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeServerTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["Knowledge"],"settings":{"knowledgeSettings":{"enableKnowledge":true}}}`)
			base := filepath.Join(root, "force-app/main/default")
			controller := `public class KnowledgeLifecycleHost {
 public String getOutcome() { return run(); }
 @AuraEnabled public static String run() {
  Savepoint sp=Database.setSavepoint();
  Knowledge__kav k=new Knowledge__kav(Title='Synthetic host',UrlName='synthetic-host',Language='en_US'); insert k;
  k=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:k.Id]; Id parent=k.KnowledgeArticleId;
  KbManagement.PublishingService.publishArticle(parent,true); Id edited=KbManagement.PublishingService.editOnlineArticle(parent,false);
  KbManagement.PublishingService.assignDraftArticleTask(parent,UserInfo.getUserId(),'Review',Datetime.now(),true);
  Knowledge__kav assigned=[SELECT AssignedToId FROM Knowledge__kav WHERE Id=:edited]; Boolean currentAssigned=assigned.AssignedToId==UserInfo.getUserId();
  Id target='005000000000083AAA'; KbManagement.PublishingService.assignDraftArticleTask(parent,target,'Review',null,false);
  Boolean invalidRejected=false; try { KbManagement.PublishingService.assignDraftArticleTask(parent,'005000000000082AAA','Review',null,false); } catch(System.InvalidParameterValueException expected) { invalidRejected=true; }
  Knowledge__DataCategorySelection selection=new Knowledge__DataCategorySelection(ParentId=edited,DataCategoryGroupName='Guides',DataCategoryName='General'); insert selection;
  Boolean invalidGroup=false; try { insert new Knowledge__DataCategorySelection(ParentId=edited,DataCategoryGroupName='Guides__c',DataCategoryName='General__c'); } catch(DmlException expected) { invalidGroup=String.valueOf(expected.getDmlStatusCode(0))=='INVALID_DATA_CATEGORY_GROUP_REFERENCE'; }
  Knowledge__kav draft=[SELECT PublishStatus,VersionNumber,AssignedToId,AssignmentNote,AssignmentDueDate FROM Knowledge__kav WHERE Id=:edited];
  Integer categories=[SELECT COUNT() FROM Knowledge__kav WHERE PublishStatus='Draft' AND Id=:edited WITH DATA CATEGORY Guides__c AT General__c];
  delete selection;
  Integer afterDelete=[SELECT COUNT() FROM Knowledge__kav WHERE PublishStatus='Draft' AND Id=:edited WITH DATA CATEGORY Guides__c AT General__c];
  Integer allRows=[SELECT COUNT() FROM Knowledge__DataCategorySelection WHERE Id=:selection.Id ALL ROWS];
  KbManagement.PublishingService.publishArticle(parent,true);
  Knowledge__kav original=[SELECT PublishStatus,IsLatestVersion FROM Knowledge__kav WHERE Id=:k.Id];
  Integer hiddenArchive=[SELECT COUNT() FROM Knowledge__kav WHERE KnowledgeArticleId=:parent AND PublishStatus='Archived'];
  Integer visibleArchive=[SELECT COUNT() FROM Knowledge__kav WHERE Id=:k.Id AND PublishStatus='Archived'];
  Knowledge__kav major=[SELECT VersionNumber FROM Knowledge__kav WHERE Id=:edited];
  Id minor=KbManagement.PublishingService.editOnlineArticle(parent,false); KbManagement.PublishingService.publishArticle(parent,false);
  Knowledge__kav current=[SELECT VersionNumber FROM Knowledge__kav WHERE Id=:minor];
  Integer replaced=[SELECT COUNT() FROM Knowledge__kav WHERE Id=:edited ALL ROWS];
  Knowledge__kav archived=new Knowledge__kav(Title='Synthetic archive',UrlName='synthetic-archive',Language='en_US'); insert archived;
  archived=[SELECT KnowledgeArticleId FROM Knowledge__kav WHERE Id=:archived.Id]; KbManagement.PublishingService.publishArticle(archived.KnowledgeArticleId,true);
  KbManagement.PublishingService.archiveOnlineArticle(archived.KnowledgeArticleId,null); archived=[SELECT PublishStatus FROM Knowledge__kav WHERE Id=:archived.Id];
  String result=draft.PublishStatus+':'+draft.VersionNumber+':'+String.valueOf(currentAssigned)+':'+String.valueOf(draft.AssignedToId==target)+':'+String.valueOf(draft.AssignmentDueDate==null)+':'+String.valueOf(invalidRejected)+':'+draft.AssignmentNote+':'+categories+':'+afterDelete+':'+allRows+':'+archived.PublishStatus+':'+original.PublishStatus+':'+String.valueOf(original.IsLatestVersion)+':'+major.VersionNumber+':'+current.VersionNumber+':'+replaced+':'+String.valueOf(invalidGroup)+':'+hiddenArchive+':'+visibleArchive;
  Database.rollback(sp); return result;
 }
}`
			writeServerTestFile(t, filepath.Join(base, "classes/KnowledgeLifecycleHost.cls"), controller)
			writeServerTestFile(t, filepath.Join(base, "classes/KnowledgeLifecycleHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(base, "pages/KnowledgeLifecycle.page"), `<apex:page controller="KnowledgeLifecycleHost" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			vfAPI := api
			if api == "62.0" {
				vfAPI = "59.0"
			}
			writeServerTestFile(t, filepath.Join(base, "pages/KnowledgeLifecycle.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+vfAPI+`</apiVersion><label>Knowledge Lifecycle</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			storage.ApplyOrgShape(&org, []string{"Knowledge", "KnowledgeEnabled"})
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "Knowledge__DataCategorySelection")
			org.Metadata.DataCategoryGroups = []storage.DataCategoryGroup{{Name: "Guides", Categories: []storage.DataCategory{{Name: "All", Children: []storage.DataCategory{{Name: "General"}}}}}}
			users := org.Objects["User"]
			user := users.Records[storage.DefaultUserID]
			user.Fields["UserPermissionsKnowledgeUser"] = storage.BooleanValue(true)
			users.Records[storage.DefaultUserID] = user
			users.Records["005000000000083AAA"] = storage.Record{ID: "005000000000083AAA", Object: "User", Fields: map[string]storage.Value{"UserPermissionsKnowledgeUser": storage.BooleanValue(true)}}
			users.Records["005000000000082AAA"] = storage.Record{ID: "005000000000082AAA", Object: "User", Fields: map[string]storage.Value{"UserPermissionsKnowledgeUser": storage.BooleanValue(false)}}
			org.Objects["User"] = users
			host := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, host)
			users = host.Org.Objects["User"]
			user = users.Records[visualforceTestPrincipalID]
			user.Fields["UserPermissionsKnowledgeUser"] = storage.BooleanValue(true)
			users.Records[visualforceTestPrincipalID] = user
			host.Org.Objects["User"] = users
			host.SetProjectIndex(typesys.Build(p, sch))
			for _, route := range []struct{ name, method, path, body string }{{"vf", http.MethodGet, "/apex/KnowledgeLifecycle", ""}, {"lwc", http.MethodPost, "/lightning/apex/KnowledgeLifecycleHost/run", `{}`}, {"wire", http.MethodPost, "/lightning/wire/apex", `{"className":"KnowledgeLifecycleHost","method":"run","params":{}}`}} {
				t.Run(route.name, func(t *testing.T) {
					for pass := 0; pass < 2; pass++ {
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
						req.Header.Set("Content-Type", "application/json")
						host.ServeHTTP(rec, req)
						if rec.Code != http.StatusOK {
							t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
						}
						const want = "Draft:0:true:true:true:true:Review:1:0:0:Archived:Archived:false:2:2:1:true:0:1"
						if route.name == "vf" {
							if !strings.Contains(rec.Body.String(), ">"+want+"<") {
								t.Fatal(rec.Body.String())
							}
						} else {
							var out lwcbrowser.WireResponse
							if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
								t.Fatal(err)
							}
							if out.Error != nil || out.Data != want {
								t.Fatalf("%+v", out)
							}
						}
					}
				})
			}
		})
	}
}
