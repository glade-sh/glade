package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The seeded Account matches the one-row native tabular capture; these tests
// exercise local host routing.
func TestProjectReportHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			base := filepath.Join(root, "force-app/main/default")
			writeServerTestFile(t, filepath.Join(base, "reports/B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name><folderShares><accessLevel>View</accessLevel><sharedTo>vf-test@example.test</sharedTo><sharedToType>User</sharedToType></folderShares></ReportFolder>`)
			writeServerTestFile(t, filepath.Join(base, "reports/B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><description>  Beta &amp; Gamma  </description><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns><showDetails>true</showDetails><showGrandTotal>false</showGrandTotal><showSubTotals>false</showSubTotals><timeFrameFilter><dateColumn>CREATED_DATE</dateColumn><interval>INTERVAL_CUSTOM</interval><startDate>2026-11-03</startDate><endDate>2026-11-03</endDate></timeFrameFilter></Report>`)
			writeServerTestFile(t, filepath.Join(base, "reports/B6_Reports/B6_Contacts.report-meta.xml"), `<Report><name>B6 Contacts</name><format>Tabular</format><reportType>ContactList</reportType><scope>organization</scope><columns><field>LAST_NAME</field></columns><timeFrameFilter><dateColumn>CREATED_DATE</dateColumn><interval>INTERVAL_CUSTOM</interval><startDate>2026-11-03</startDate><endDate>2026-11-03</endDate></timeFrameFilter></Report>`)
			writeServerTestFile(t, filepath.Join(base, "reports/B6_Reports/B6_Zero.report-meta.xml"), `<Report><name>B6 Zero</name><format>Summary</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT_NUMBER</field></columns><groupingsDown><field>ACCOUNT.NAME</field><sortOrder>Asc</sortOrder></groupingsDown><timeFrameFilter><dateColumn>CREATED_DATE</dateColumn><interval>INTERVAL_CUSTOM</interval><startDate>2099-01-01</startDate><endDate>2099-12-31</endDate></timeFrameFilter></Report>`)
			writeServerTestFile(t, filepath.Join(base, "reports/B6_Reports/B6_User.report-meta.xml"), `<Report><name>B6 User</name><format>Tabular</format><reportType>AccountList</reportType><scope>user</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
			writeServerTestFile(t, filepath.Join(base, "reports/B6_Reports/B6_UserContact.report-meta.xml"), `<Report><name>B6 User Contact</name><format>Tabular</format><reportType>ContactList</reportType><scope>user</scope><columns><field>LAST_NAME</field></columns></Report>`)
			writeServerTestFile(t, filepath.Join(base, "reports/B6_Reports/B6_NoScopeAccount.report-meta.xml"), `<Report><name>B6 No Scope Account</name><format>Tabular</format><reportType>AccountList</reportType><columns><field>ACCOUNT.NAME</field></columns></Report>`)
			writeServerTestFile(t, filepath.Join(base, "reports/B6_Reports/B6_NoScopeContact.report-meta.xml"), `<Report><name>B6 No Scope Contact</name><format>Tabular</format><reportType>ContactList</reportType><columns><field>LAST_NAME</field></columns></Report>`)
			writeServerTestFile(t, filepath.Join(base, "reports/B6_Reports/B6_OpportunityAbsent.report-meta.xml"), `<Report><name>B6 Opportunity Absent</name><format>Tabular</format><reportType>Opportunity</reportType><columns><field>OPPORTUNITY_NAME</field></columns></Report>`)
			writeServerTestFile(t, filepath.Join(base, "reports/B6_Reports/B6_OpportunityUser.report-meta.xml"), `<Report><name>B6 Opportunity User</name><format>Tabular</format><reportType>Opportunity</reportType><scope>user</scope><columns><field>OPPORTUNITY_NAME</field></columns></Report>`)
			writeServerTestFile(t, filepath.Join(base, "reports/B6_Reports/B6_OpportunityTimeFrame.report-meta.xml"), `<Report><name>B6 Opportunity Time Frame</name><format>Tabular</format><reportType>Opportunity</reportType><scope>organization</scope><columns><field>OPPORTUNITY_NAME</field></columns><timeFrameFilter><dateColumn>CREATED_DATE</dateColumn><interval>INTERVAL_CUSTOM</interval><startDate>2000-01-01</startDate><endDate>2100-01-01</endDate></timeFrameFilter></Report>`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6Host.cls"), `public class B6Host { @AuraEnabled(cacheable=true) public static String read() { Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Reports.ReportTypeMetadata at=Reports.ReportManager.describeReport(r.Id).getReportTypeMetadata();System.assertNotEquals(null,at);System.assertEquals(3,at.getCategories().size());System.assertEquals('Account General',at.getCategories()[0].getLabel());System.assertEquals('user',at.getScopeInfo().getDefaultValue());System.assertEquals(4,at.getScopeInfo().getValues().size());Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();System.assertEquals(true,m.getHasDetailRows());System.assertEquals('Beta & Gamma',m.getDescription());System.assertEquals(false,m.getShowGrandTotal());System.assertEquals(false,m.getShowSubtotals());System.assertEquals('CREATED_DATE',m.getStandardDateFilter().getColumn());System.assertEquals('2026-11-03',m.getStandardDateFilter().getStartDate());Reports.ReportMetadata overrideMeta=Reports.ReportManager.describeReport(r.Id).getReportMetadata();overrideMeta.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM','2000-01-01','2100-01-01'));System.assertEquals(1,Reports.ReportManager.runReport(r.Id,overrideMeta,false).getFactMap().get('T!T').getAggregates()[0].getValue());Reports.ReportMetadata bad=Reports.ReportManager.describeReport(r.Id).getReportMetadata();bad.setStandardDateFilter(new Reports.StandardDateFilter('NOT_A_DATE','CUSTOM','2000-01-01','2100-01-01'));Boolean invalid=false;try{Reports.ReportManager.runReport(r.Id,bad,false);}catch(Reports.InvalidFilterException e){invalid=true;System.assertEquals('[The column NOT_A_DATE specified for the standard date filter is invalid.]',e.getMessage());}System.assert(invalid);Report ur=[SELECT Id FROM Report WHERE DeveloperName='B6_User' LIMIT 1];Reports.ReportMetadata um=Reports.ReportManager.describeReport(ur.Id).getReportMetadata();um.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM','2099-01-01','2100-01-01'));System.assertEquals(0,Reports.ReportManager.runReport(ur.Id,um,false).getFactMap().get('T!T').getAggregates()[0].getValue());Report cr=[SELECT Id FROM Report WHERE DeveloperName='B6_Contacts' LIMIT 1];Reports.ReportTypeMetadata ct=Reports.ReportManager.describeReport(cr.Id).getReportTypeMetadata();System.assertNotEquals(null,ct);System.assertEquals(6,ct.getCategories().size());System.assertEquals('Contact: General',ct.getCategories()[0].getLabel());System.assertEquals('my',ct.getScopeInfo().getDefaultValue());System.assertEquals(5,ct.getScopeInfo().getValues().size());System.assertEquals('2026-11-03',Reports.ReportManager.describeReport(cr.Id).getReportMetadata().getStandardDateFilter().getStartDate());System.assertEquals(1,Reports.ReportManager.runReport(cr.Id,false).getFactMap().get('T!T').getAggregates()[0].getValue());Reports.ReportMetadata cm=Reports.ReportManager.describeReport(cr.Id).getReportMetadata();cm.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM','2000-01-01','2100-01-01'));System.assertEquals(1,Reports.ReportManager.runReport(cr.Id,cm,false).getFactMap().get('T!T').getAggregates()[0].getValue());Report uc=[SELECT Id FROM Report WHERE DeveloperName='B6_UserContact' LIMIT 1];Reports.ReportMetadata ucm=Reports.ReportManager.describeReport(uc.Id).getReportMetadata();ucm.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM','2099-01-01','2100-01-01'));System.assertEquals(0,Reports.ReportManager.runReport(uc.Id,ucm,false).getFactMap().get('T!T').getAggregates()[0].getValue());for(Report nr:[SELECT Id,DeveloperName FROM Report WHERE DeveloperName IN ('B6_NoScopeAccount','B6_NoScopeContact')]){Reports.ReportMetadata nm=Reports.ReportManager.describeReport(nr.Id).getReportMetadata();System.assertEquals(nr.DeveloperName=='B6_NoScopeContact'?'my':'user',nm.getScope());nm.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM','2099-01-01','2100-01-01'));System.assertEquals(0,Reports.ReportManager.runReport(nr.Id,nm,false).getFactMap().get('T!T').getAggregates()[0].getValue());}for(Report op:[SELECT Id,DeveloperName FROM Report WHERE DeveloperName IN ('B6_OpportunityAbsent','B6_OpportunityUser','B6_OpportunityTimeFrame')]){if(op.DeveloperName=='B6_OpportunityAbsent'){Reports.ReportTypeMetadata ot=Reports.ReportManager.describeReport(op.Id).getReportTypeMetadata();System.assertNotEquals(null,ot);System.assertEquals(7,ot.getCategories().size());System.assertEquals('Opportunity Information',ot.getCategories()[0].getLabel());System.assertEquals('organization',ot.getScopeInfo().getDefaultValue());System.assertEquals(4,ot.getScopeInfo().getValues().size());Object f=ot.getStandardFilterInfos().get('open');System.assertEquals(true,f instanceof Reports.StandardFilterInfoPicklist);Reports.StandardFilterInfoPicklist q=(Reports.StandardFilterInfoPicklist)f;System.assertEquals('all',q.getDefaultValue());System.assertEquals(4,q.getFilterValues().size());}System.assertEquals(op.DeveloperName=='B6_OpportunityUser'?'user':'organization',Reports.ReportManager.describeReport(op.Id).getReportMetadata().getScope());System.assertEquals(0,Reports.ReportManager.runReport(op.Id,false).getFactMap().get('T!T').getAggregates()[0].getValue());Reports.ReportMetadata om=Reports.ReportManager.describeReport(op.Id).getReportMetadata();if(op.DeveloperName=='B6_OpportunityTimeFrame')System.assertEquals('CREATED_DATE',om.getStandardDateFilter().getColumn());om.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM','2000-01-01','2100-01-01'));Reports.ReportResults orr=Reports.ReportManager.runReport(op.Id,om,false);System.assertEquals(0,orr.getFactMap().get('T!T').getAggregates()[0].getValue());System.assertEquals('CREATED_DATE',orr.getReportMetadata().getStandardDateFilter().getColumn());}Report zr=[SELECT Id FROM Report WHERE DeveloperName='B6_Zero' LIMIT 1];Reports.ReportResults zero=Reports.ReportManager.runReport(zr.Id,false);System.assertEquals('{"groupings":null}',JSON.serialize(zero.getGroupingsDown()));System.assertEquals(0,zero.getGroupingsDown().getGroupings().size());System.assertEquals(0,zero.getFactMap().get('T!T').getAggregates()[0].getValue());Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,false);System.assertEquals(false,x.getHasDetailRows());System.assertEquals(0,x.getGroupingsDown().getGroupings().size());System.assertEquals(0,x.getGroupingsAcross().getGroupings().size());System.assertEquals('{"groupings":null}',JSON.serialize(x.getGroupingsAcross()));Object v=x.getFactMap().get('T!T').getAggregates()[0].getValue();if(!(v instanceof Decimal) || v instanceof Integer)return 'wrong-count-type';System.assertEquals(1,v);System.assertEquals(1.0,v);Boolean rejected=false;try{Integer n=(Integer)v;}catch(System.TypeException e){rejected=true;}if(!rejected)return 'wrong-count-cast';return m.getName()+':'+String.valueOf(v); } @AuraEnabled public static String afterInsert(){insert new Account(Name='Transient Host Account');return read();} public String getOutcome(){return read();} }`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6Host.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6SharedHost.cls"), `public class B6SharedHost { @AuraEnabled(cacheable=true) public static String read() { Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();System.assertEquals(true,m.getHasDetailRows());System.assertEquals('Beta & Gamma',m.getDescription());System.assertEquals(false,m.getShowGrandTotal());System.assertEquals(false,m.getShowSubtotals());System.assertEquals('CREATED_DATE',m.getStandardDateFilter().getColumn());System.assertEquals('2026-11-03',m.getStandardDateFilter().getStartDate());Reports.ReportMetadata overrideMeta=Reports.ReportManager.describeReport(r.Id).getReportMetadata();overrideMeta.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM','2000-01-01','2100-01-01'));System.assertEquals(1,Reports.ReportManager.runReport(r.Id,overrideMeta,false).getFactMap().get('T!T').getAggregates()[0].getValue());Report ur=[SELECT Id FROM Report WHERE DeveloperName='B6_User' LIMIT 1];Reports.ReportMetadata um=Reports.ReportManager.describeReport(ur.Id).getReportMetadata();um.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM','2099-01-01','2100-01-01'));System.assertEquals(0,Reports.ReportManager.runReport(ur.Id,um,false).getFactMap().get('T!T').getAggregates()[0].getValue());Report cr=[SELECT Id FROM Report WHERE DeveloperName='B6_Contacts' LIMIT 1];System.assertEquals('2026-11-03',Reports.ReportManager.describeReport(cr.Id).getReportMetadata().getStandardDateFilter().getStartDate());System.assertEquals(1,Reports.ReportManager.runReport(cr.Id,false).getFactMap().get('T!T').getAggregates()[0].getValue());Reports.ReportMetadata cm=Reports.ReportManager.describeReport(cr.Id).getReportMetadata();cm.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM','2000-01-01','2100-01-01'));System.assertEquals(1,Reports.ReportManager.runReport(cr.Id,cm,false).getFactMap().get('T!T').getAggregates()[0].getValue());Report uc=[SELECT Id FROM Report WHERE DeveloperName='B6_UserContact' LIMIT 1];Reports.ReportMetadata ucm=Reports.ReportManager.describeReport(uc.Id).getReportMetadata();ucm.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM','2099-01-01','2100-01-01'));System.assertEquals(0,Reports.ReportManager.runReport(uc.Id,ucm,false).getFactMap().get('T!T').getAggregates()[0].getValue());for(Report nr:[SELECT Id,DeveloperName FROM Report WHERE DeveloperName IN ('B6_NoScopeAccount','B6_NoScopeContact')]){Reports.ReportMetadata nm=Reports.ReportManager.describeReport(nr.Id).getReportMetadata();System.assertEquals(nr.DeveloperName=='B6_NoScopeContact'?'my':'user',nm.getScope());nm.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM','2099-01-01','2100-01-01'));System.assertEquals(0,Reports.ReportManager.runReport(nr.Id,nm,false).getFactMap().get('T!T').getAggregates()[0].getValue());}for(Report op:[SELECT Id,DeveloperName FROM Report WHERE DeveloperName IN ('B6_OpportunityAbsent','B6_OpportunityUser','B6_OpportunityTimeFrame')]){System.assertEquals(op.DeveloperName=='B6_OpportunityUser'?'user':'organization',Reports.ReportManager.describeReport(op.Id).getReportMetadata().getScope());System.assertEquals(0,Reports.ReportManager.runReport(op.Id,false).getFactMap().get('T!T').getAggregates()[0].getValue());Reports.ReportMetadata om=Reports.ReportManager.describeReport(op.Id).getReportMetadata();if(op.DeveloperName=='B6_OpportunityTimeFrame')System.assertEquals('CREATED_DATE',om.getStandardDateFilter().getColumn());om.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM','2000-01-01','2100-01-01'));Reports.ReportResults orr=Reports.ReportManager.runReport(op.Id,om,false);System.assertEquals(0,orr.getFactMap().get('T!T').getAggregates()[0].getValue());System.assertEquals('CREATED_DATE',orr.getReportMetadata().getStandardDateFilter().getColumn());}Report zr=[SELECT Id FROM Report WHERE DeveloperName='B6_Zero' LIMIT 1];Reports.ReportResults zero=Reports.ReportManager.runReport(zr.Id,false);System.assertEquals('{"groupings":null}',JSON.serialize(zero.getGroupingsDown()));System.assertEquals(0,zero.getGroupingsDown().getGroupings().size());System.assertEquals(0,zero.getFactMap().get('T!T').getAggregates()[0].getValue());Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,false);System.assertEquals(false,x.getHasDetailRows());System.assertEquals(0,x.getGroupingsDown().getGroupings().size());System.assertEquals(0,x.getGroupingsAcross().getGroupings().size());System.assertEquals('{"groupings":null}',JSON.serialize(x.getGroupingsAcross()));Object v=x.getFactMap().get('T!T').getAggregates()[0].getValue();if(!(v instanceof Decimal) || v instanceof Integer)return 'wrong-count-type';System.assertEquals(1,v);System.assertEquals(1.0,v);Boolean rejected=false;try{Integer n=(Integer)v;}catch(System.TypeException e){rejected=true;}if(!rejected)return 'wrong-count-cast';return m.getName()+':'+String.valueOf(v); } @AuraEnabled public static String afterInsert(){insert new Account(Name='Transient Host Account');return read();} public String getOutcome(){return read();} }`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6SharedHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(base, "pages/B6Shared.page"), `<apex:page controller="B6SharedHost" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			writeServerTestFile(t, filepath.Join(base, "pages/B6Shared.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><label>B6 Shared</label></ApexPage>`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6CustomHost.cls"), `public class B6CustomHost { @AuraEnabled(cacheable=true) public static String read(){Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Reports.ReportTypeMetadata m=Reports.ReportManager.describeReport(r.Id).getReportTypeMetadata();System.assertNotEquals(null,m);System.assertEquals(4,m.getCategories().size());Reports.ReportTypeColumnCategory c=m.getCategories()[3];System.assertEquals('Account: Custom Info',c.getLabel());System.assertEquals(3,c.getColumns().size());System.assertEquals('B6 Text',c.getColumns().get('Account.B6_Text__c').getLabel());System.assertEquals('B6 Date',c.getColumns().get('Account.B6_Date__c').getLabel());System.assertEquals('B6 Amount',c.getColumns().get('Account.B6_Amount__c').getLabel());return 'custom-ok';} public String getOutcome(){return read();} }`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6CustomHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6CustomRest.cls"), `@RestResource(urlMapping='/b6-custom-catalog') global class B6CustomRest { @HttpGet global static String getIt(){return B6CustomHost.read();} }`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6CustomRest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(base, "pages/B6Custom.page"), `<apex:page controller="B6CustomHost" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			customVFAPI := api
			if api == "62.0" {
				customVFAPI = "59.0"
			}
			writeServerTestFile(t, filepath.Join(base, "pages/B6Custom.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+customVFAPI+`</apiVersion><label>B6 Custom</label></ApexPage>`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6StandardSecurityHost.cls"), `public class B6StandardSecurityHost { @AuraEnabled(cacheable=true) public static String read(){Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];String d='ok';String x='ok';try{Reports.ReportManager.describeReport(r.Id);}catch(Exception e){d=e.getTypeName()+'~'+e.getMessage();}try{Reports.ReportManager.runReport(r.Id,false);}catch(Exception e){x=e.getTypeName()+'~'+e.getMessage();}return d+'|'+x;} public String getOutcome(){return read();} }`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6StandardSecurityHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6StandardSecurityRest.cls"), `@RestResource(urlMapping='/b6-standard-security') global class B6StandardSecurityRest { @HttpGet global static String getIt(){return B6StandardSecurityHost.read();} }`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6StandardSecurityRest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(base, "pages/B6StandardSecurity.page"), `<apex:page controller="B6StandardSecurityHost" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			writeServerTestFile(t, filepath.Join(base, "pages/B6StandardSecurity.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+customVFAPI+`</apiVersion><label>B6 Standard Security</label></ApexPage>`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6Rest.cls"), `@RestResource(urlMapping='/b6-report') global class B6Rest { @HttpGet global static String getIt(){return B6Host.read();} }`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6Rest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6VFInsert.cls"), `public class B6VFInsert { public B6VFInsert(){insert new Account(Name='Transient Constructor Account');} public String getOutcome(){return B6Host.read();} }`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6VFInsert.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			vfAPI := api
			if api == "62.0" {
				vfAPI = "59.0"
			}
			writeServerTestFile(t, filepath.Join(base, "pages/B6HostPage.page"), `<apex:page controller="B6Host" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			writeServerTestFile(t, filepath.Join(base, "pages/B6HostPage.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+vfAPI+`</apiVersion><label>B6 Host</label></ApexPage>`)
			writeServerTestFile(t, filepath.Join(base, "pages/B6VFInsert.page"), `<apex:page controller="B6VFInsert" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			writeServerTestFile(t, filepath.Join(base, "pages/B6VFInsert.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+vfAPI+`</apiVersion><label>B6 Insert</label></ApexPage>`)
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
			storage.EnsureStandardObject(&org, "Account")
			accounts := org.Objects["Account"]
			id := storage.ID("001000000000001")
			accounts.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue("Sample Account for Entitlements")}, System: storage.SystemFields{CreatedDate: "2026-11-03T17:53:26Z"}}
			org.Objects["Account"] = accounts
			storage.EnsureStandardObject(&org, "Opportunity")
			storage.EnsureStandardObject(&org, "Contact")
			contacts := org.Objects["Contact"]
			contactID := storage.ID("003000000000001")
			contacts.Records[contactID] = storage.Record{ID: contactID, Object: "Contact", Fields: map[string]storage.Value{"Id": storage.IDValue(contactID), "LastName": storage.StringValue("B6 Boundary"), "AccountId": storage.IDValue(id)}, System: storage.SystemFields{CreatedDate: "2026-11-03T21:04:22Z"}}
			org.Objects["Contact"] = contacts
			if err = resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			constructorOrg := org.Clone()
			host := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, host)
			sharingOrg := host.Org.Clone()
			profile := host.Org.Objects["Profile"]
			principalProfile := profile.Records[visualforceTestProfileID]
			principalProfile.Fields["Name"] = storage.StringValue("System Administrator")
			profile.Records[visualforceTestProfileID] = principalProfile
			host.Org.Objects["Profile"] = profile
			host.SetProjectIndex(typesys.Build(p, sch))
			for _, route := range []struct{ name, method, path, body string }{{"vf", http.MethodGet, "/apex/B6HostPage", ""}, {"rest", http.MethodGet, "/services/apexrest/b6-report", ""}, {"lwc-wire", http.MethodPost, "/lightning/wire/apex", `{"className":"B6Host","method":"read","params":{}}`}, {"lwc-imperative", http.MethodPost, "/lightning/apex/B6Host/read", `{}`}} {
				t.Run(route.name, func(t *testing.T) {
					for _, phase := range []string{"cold", "warm"} {
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
						req.Header.Set("Content-Type", "application/json")
						host.ServeHTTP(rec, req)
						if rec.Code != http.StatusOK {
							t.Fatalf("%s HTTP %d: %s", phase, rec.Code, rec.Body.String())
						}
						if route.name == "vf" {
							if !strings.Contains(rec.Body.String(), ">B6 Accounts:1<") {
								t.Fatalf("%s: %s", phase, rec.Body.String())
							}
						} else if route.name == "rest" {
							if !strings.Contains(rec.Body.String(), "B6 Accounts:1") {
								t.Fatalf("%s: %s", phase, rec.Body.String())
							}
						} else {
							var out lwcbrowser.WireResponse
							if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
								t.Fatal(err)
							}
							if out.Error != nil || out.Data != "B6 Accounts:1" {
								t.Fatalf("%s: %+v", phase, out)
							}
						}
					}
				})
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/lightning/apex/B6Host/afterInsert", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			host.ServeHTTP(rec, req)
			var out lwcbrowser.WireResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK || out.Error != nil || out.Data != "B6 Accounts:1" {
				t.Fatalf("after uncommitted insert: status=%d result=%+v", rec.Code, out)
			}
			sharingHost := newLightningTestServer(t, &sharingOrg, source)
			configureVisualforceTestPrincipal(t, sharingHost)
			sharingHost.SetProjectIndex(typesys.Build(p, sch))
			sharingRec := httptest.NewRecorder()
			sharingHost.ServeHTTP(sharingRec, httptest.NewRequest(http.MethodGet, "/apex/B6Shared", nil))
			if sharingRec.Code != http.StatusOK || !strings.Contains(sharingRec.Body.String(), ">B6 Accounts:1<") {
				t.Fatalf("shared non-admin: HTTP %d: %s", sharingRec.Code, sharingRec.Body.String())
			}
			customOrg := sharingOrg.Clone()
			accountState := customOrg.Objects["Account"]
			accountState.Definition = accountState.Definition.Clone()
			accountState.Definition.Fields["B6_Text__c"] = storage.Field{APIName: "B6_Text__c", Label: "B6 Text", Type: storage.FieldString, Length: 30}
			accountState.Definition.Fields["B6_Date__c"] = storage.Field{APIName: "B6_Date__c", Label: "B6 Date", Type: storage.FieldDate}
			accountState.Definition.Fields["B6_Amount__c"] = storage.Field{APIName: "B6_Amount__c", Label: "B6 Amount", Type: storage.FieldDecimal}
			customOrg.Objects["Account"] = accountState
			for _, object := range []string{"PermissionSet", "PermissionSetAssignment", "FieldPermissions"} {
				storage.EnsureStandardObject(&customOrg, object)
			}
			psID := storage.ID("0PS000000000099")
			sets := customOrg.Objects["PermissionSet"]
			sets.Records[psID] = storage.Record{ID: psID, Object: "PermissionSet", Fields: map[string]storage.Value{"Name": storage.StringValue("SyntheticCatalog")}}
			customOrg.Objects["PermissionSet"] = sets
			assignments := customOrg.Objects["PermissionSetAssignment"]
			for _, entry := range []struct{ id, user storage.ID }{{"0Pa000000000098", storage.ID(storage.DefaultUserID)}, {"0Pa000000000099", visualforceTestPrincipalID}} {
				assignments.Records[entry.id] = storage.Record{ID: entry.id, Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(entry.user), "PermissionSetId": storage.IDValue(psID)}}
			}
			customOrg.Objects["PermissionSetAssignment"] = assignments
			fields := customOrg.Objects["FieldPermissions"]
			for _, entry := range []struct {
				id   storage.ID
				name string
			}{{"0FP000000000097", "B6_Text__c"}, {"0FP000000000098", "B6_Date__c"}, {"0FP000000000099", "B6_Amount__c"}} {
				fields.Records[entry.id] = storage.Record{ID: entry.id, Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(psID), "SObjectType": storage.StringValue("Account"), "Field": storage.StringValue("Account." + entry.name), "PermissionsRead": storage.BooleanValue(true)}}
			}
			customOrg.Objects["FieldPermissions"] = fields
			customHost := newLightningTestServer(t, &customOrg, source)
			configureVisualforceTestPrincipal(t, customHost)
			customProfile := customHost.Org.Objects["Profile"]
			customPrincipal := customProfile.Records[visualforceTestProfileID]
			customPrincipal.Fields["Name"] = storage.StringValue("System Administrator")
			customProfile.Records[visualforceTestProfileID] = customPrincipal
			customHost.Org.Objects["Profile"] = customProfile
			customHost.SetProjectIndex(typesys.Build(p, sch))
			for _, route := range []struct{ name, method, path, body string }{{"vf", http.MethodGet, "/apex/B6Custom", ""}, {"rest", http.MethodGet, "/services/apexrest/b6-custom-catalog", ""}, {"lwc-wire", http.MethodPost, "/lightning/wire/apex", `{"className":"B6CustomHost","method":"read","params":{}}`}, {"lwc-imperative", http.MethodPost, "/lightning/apex/B6CustomHost/read", `{}`}} {
				for _, phase := range []string{"cold", "warm"} {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
					req.Header.Set("Content-Type", "application/json")
					customHost.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "custom-ok") {
						t.Fatalf("custom %s %s: HTTP %d: %s", route.name, phase, rec.Code, rec.Body.String())
					}
				}
			}
			securityOrg := sharingOrg.Clone()
			storage.EnsureDeterministicPlatformData(&securityOrg)
			securityHost := newLightningTestServer(t, &securityOrg, source)
			configureVisualforceTestPrincipal(t, securityHost)
			securityUsers := securityHost.Org.Objects["User"]
			securityUser := securityUsers.Records[visualforceTestPrincipalID]
			securityUser.Fields["ProfileId"] = storage.IDValue("00e000000000005")
			securityUsers.Records[visualforceTestPrincipalID] = securityUser
			securityHost.Org.Objects["User"] = securityUsers
			securityHost.SetProjectIndex(typesys.Build(p, sch))
			for _, route := range []struct{ name, method, path, body string }{{"vf", http.MethodGet, "/apex/B6StandardSecurity", ""}, {"rest", http.MethodGet, "/services/apexrest/b6-standard-security", ""}, {"lwc-wire", http.MethodPost, "/lightning/wire/apex", `{"className":"B6StandardSecurityHost","method":"read","params":{}}`}, {"lwc-imperative", http.MethodPost, "/lightning/apex/B6StandardSecurityHost/read", `{}`}} {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-GLADE-User-Id", string(visualforceTestPrincipalID))
				req.Header.Set("Authorization", "Bearer "+string(visualforceTestPrincipalID))
				securityHost.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), "System.SecurityException~Required permissions are not enabled.") != 2 {
					t.Fatalf("standard security %s: HTTP %d: %s", route.name, rec.Code, rec.Body.String())
				}
			}
			constructorHost := newLightningTestServer(t, &constructorOrg, source)
			configureVisualforceTestPrincipal(t, constructorHost)
			constructorProfile := constructorHost.Org.Objects["Profile"]
			constructorPrincipal := constructorProfile.Records[visualforceTestProfileID]
			constructorPrincipal.Fields["Name"] = storage.StringValue("System Administrator")
			constructorProfile.Records[visualforceTestProfileID] = constructorPrincipal
			constructorHost.Org.Objects["Profile"] = constructorProfile
			constructorHost.SetProjectIndex(typesys.Build(p, sch))
			vfRec := httptest.NewRecorder()
			constructorHost.ServeHTTP(vfRec, httptest.NewRequest(http.MethodGet, "/apex/B6VFInsert", nil))
			if vfRec.Code != http.StatusOK || !strings.Contains(vfRec.Body.String(), ">B6 Accounts:1<") {
				t.Fatalf("constructor insert: HTTP %d", vfRec.Code)
			}
		})
	}
}

// The queued callback observes the committed Account from the captured test
// route, after the anonymous enqueue transaction has completed.
func TestProjectReportQueueableDrainRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			base := filepath.Join(root, "force-app/main/default")
			writeServerTestFile(t, filepath.Join(base, "reports/B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
			writeServerTestFile(t, filepath.Join(base, "reports/B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6QueueWorker.cls"), `public class B6QueueWorker implements Queueable {public void execute(QueueableContext ctx){Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Decimal n=(Decimal)Reports.ReportManager.runReport(r.Id,false).getFactMap().get('T!T').getAggregates()[0].getValue();System.assertEquals(1,n);}}`)
			writeServerTestFile(t, filepath.Join(base, "classes/B6QueueWorker.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			accounts := org.Objects["Account"]
			id := storage.ID("001000000000001")
			accounts.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue("Sample Account for Entitlements")}}
			org.Objects["Account"] = accounts
			if err = resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			m := vm.New(nil)
			m.SetOrg(&org)
			m.SetCurrentUser(storage.Record{ID: storage.ID(storage.DefaultUserID), Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(storage.DefaultUserID)}})
			if err = apextest.RegisterProjectRuntimeForRequest(m, typesys.Build(p, sch)); err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(`System.enqueueJob(new B6QueueWorker());`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := m.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			if err = m.DrainAsync(&result); err != nil {
				t.Fatal(err)
			}
			testOrg := org.Clone()
			testMachine := vm.New(nil)
			testMachine.SetOrg(&testOrg)
			testMachine.SetCurrentUser(storage.Record{ID: storage.ID(storage.DefaultUserID), Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(storage.DefaultUserID)}})
			testMachine.SetTestReportBaseline(&testOrg)
			testMachine.EnableTestContext()
			testMachine.SetTestSeeAllData(true)
			if err = apextest.RegisterProjectRuntimeForRequest(testMachine, typesys.Build(p, sch)); err != nil {
				t.Fatal(err)
			}
			testProgram, err := vm.CompileAnonymousWithOptions(`insert new Account(Name='B6 Parent Only');Test.startTest();System.enqueueJob(new B6QueueWorker());Test.stopTest();`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = testMachine.Execute(testProgram); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProjectReportDefaultAdminPrincipal(t *testing.T) {
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	base := filepath.Join(root, "force-app/main/default/reports")
	writeServerTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
	writeServerTestFile(t, filepath.Join(base, "B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "User")
	storage.EnsureStandardObject(&org, "Profile")
	profileID := storage.ID("00e000000000001")
	users := org.Objects["User"]
	users.Records[storage.ID(storage.DefaultUserID)] = storage.Record{ID: storage.ID(storage.DefaultUserID), Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(storage.DefaultUserID), "ProfileId": storage.IDValue(profileID)}}
	org.Objects["User"] = users
	profiles := org.Objects["Profile"]
	profiles.Records[profileID] = storage.Record{ID: profileID, Object: "Profile", Fields: map[string]storage.Value{"Id": storage.IDValue(profileID), "Name": storage.StringValue("System Administrator")}}
	org.Objects["Profile"] = profiles
	if err = resource.ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	m := vm.New(nil)
	m.SetOrg(&org)
	program, err := vm.CompileAnonymous(`Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];System.assertEquals('B6 Accounts',Reports.ReportManager.describeReport(r.Id).getReportMetadata().getName());`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Execute(program); err != nil {
		t.Fatal(err)
	}
}
