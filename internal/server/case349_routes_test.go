package server

import (
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestCase349ServerAnonymousRoutes(t *testing.T) {
	source := `EqualityProbe__c a=new EqualityProbe__c(Key__c=1.25); insert a;
EqualityProbe__c b=new EqualityProbe__c(Key__c=2.50); Database.UpsertResult r=Database.upsert(b,EqualityProbe__c.Key__c,false);
System.assertEquals(true,r.isSuccess()); System.assertEquals(true,r.isCreated()); System.assertNotEquals(a.Id,b.Id);
EqualityProbe__c c=new EqualityProbe__c(Key__c=1.250); Database.UpsertResult eq=Database.upsert(c,EqualityProbe__c.Key__c,false);
System.assertEquals(false,eq.isCreated()); System.assertEquals(a.Id,c.Id);
Account business=new Account(Name='Business probe',PersonEmail='   '); Database.SaveResult s=Database.insert(business,false);
System.assertEquals(true,s.isSuccess()); System.assertEquals(false,[SELECT IsPersonAccount FROM Account WHERE Id=:business.Id].IsPersonAccount);`
	// Queried Name rejects; Id-only succeeds: case349_query_trigger/org.tsv:12,24,55,67.
	source += `
Account querySeed=new Account(LastName='Synthetic',RecordTypeId='012000000000002'); insert querySeed; Account queryName=[SELECT Id,Name FROM Account WHERE Id=:querySeed.Id]; queryName.PersonHomePhone='4155550123'; Database.SaveResult queryReject=Database.update(queryName,false); System.assertEquals(false,queryReject.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(queryReject.getErrors()[0].getStatusCode())); Account queryId=[SELECT Id FROM Account WHERE Id=:querySeed.Id]; queryId.PersonHomePhone='4155550123'; Database.SaveResult queryAccept=Database.update(queryId,false); System.assertEquals(true,queryAccept.isSuccess());`
	// Component query rows: case349_component_query/org.tsv:2-5,7,11-14,16.
	source += `
Account componentFirst=[SELECT Id,Name FROM Account WHERE Id=:querySeed.Id]; componentFirst.FirstName='Changed'; System.assertEquals(false,Database.update(componentFirst,false).isSuccess()); Account componentLast=[SELECT Id,Name FROM Account WHERE Id=:querySeed.Id]; componentLast.LastName='Changed'; System.assertEquals(false,Database.upsert(componentLast,false).isSuccess()); Account componentControl=[SELECT Id FROM Account WHERE Id=:querySeed.Id]; componentControl.FirstName='Changed'; System.assertEquals(true,Database.update(componentControl,false).isSuccess());`

	source += `
Account phone=new Account(Name='Phone probe',RecordTypeId='012000000000001',PersonHomePhone='2125550100');
Database.SaveResult pr=Database.insert(phone,false); System.assertEquals(false,pr.isSuccess());
System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(pr.getErrors()[0].getStatusCode()));
Account named=new Account(Name='Name probe',RecordTypeId='012000000000001'); named.put('FirstName',null);
Database.SaveResult nr=Database.insert(named,false); System.assertEquals(false,nr.isSuccess());
System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(nr.getErrors()[0].getStatusCode()));`
	source += `
Account defaultPhone=new Account(Name='Default phone',PersonMobilePhone='2125550100');
Database.SaveResult dr=Database.insert(defaultPhone,false); System.assertEquals(false,dr.isSuccess());
System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(dr.getErrors()[0].getStatusCode()));
Account malformed=new Account(Name='Malformed email',PersonEmail='x');
Database.SaveResult er=Database.insert(malformed,false); System.assertEquals(false,er.isSuccess());
System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(er.getErrors()[0].getStatusCode()));`
	source += `
Account mixedOnly=new Account(Name='Mixed only',PersonMemo__c='synthetic'); insert mixedOnly; System.assertEquals(false,[SELECT IsPersonAccount FROM Account WHERE Id=:mixedOnly.Id].IsPersonAccount);
Account mixedPhone=new Account(Name='Mixed phone',PersonMemo__c='synthetic',PersonHomePhone='4155550123'); Database.SaveResult mixResult=Database.insert(mixedPhone,false); System.assertEquals(false,mixResult.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(mixResult.getErrors()[0].getStatusCode()));
Account mixedTitle=new Account(Name='Mixed title',RecordTypeId='012000000000001',PersonTitle='Synthetic',PersonHomePhone='4155550123'); Database.SaveResult titleResult=Database.insert(mixedTitle,false); System.assertEquals(false,titleResult.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(titleResult.getErrors()[0].getStatusCode()));
Account extBusiness=new Account(Name='External business',ProbeKey__c='business'); insert extBusiness;
Account extFirst=new Account(ProbeKey__c='business',FirstName='Synthetic'); Database.UpsertResult extFirstResult=Database.upsert(extFirst,Account.Fields.ProbeKey__c,false);
System.assertEquals(false,extFirstResult.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(extFirstResult.getErrors()[0].getStatusCode()));
Account extEmail=new Account(ProbeKey__c='business',PersonEmail='invalid'); Database.UpsertResult extEmailResult=Database.upsert(extEmail,Account.Fields.ProbeKey__c,false);
System.assertEquals(false,extEmailResult.isSuccess()); System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(extEmailResult.getErrors()[0].getStatusCode()));
Account extNew=new Account(Name='Unmatched',ProbeKey__c='unmatched',FirstName='Synthetic'); Database.UpsertResult extNewResult=Database.upsert(extNew,Account.Fields.ProbeKey__c,false);
System.assertEquals(false,extNewResult.isSuccess()); System.assertEquals(true,extNewResult.isCreated());
Account nativeBatchSecond=new Account(Name='Batch second',ProbeKey__c='batch-second'); insert nativeBatchSecond;
Id nativeBatchType=[SELECT RecordTypeId FROM Account WHERE Id=:extBusiness.Id].RecordTypeId;
for(String batchOp:new List<String>{'insert','update','upsert'}) {
 List<Account> mixedBatch;
 if(batchOp=='insert') mixedBatch=new List<Account>{new Account(Name='First',PersonEmail='invalid'),new Account(Name='Second',RecordTypeId=nativeBatchType,FirstName='Synthetic')};
 else if(batchOp=='update') mixedBatch=new List<Account>{new Account(Id=extBusiness.Id,PersonEmail='invalid'),new Account(Id=nativeBatchSecond.Id,FirstName='Synthetic')};
 else mixedBatch=new List<Account>{new Account(ProbeKey__c='business',PersonEmail='invalid'),new Account(ProbeKey__c='batch-second',FirstName='Synthetic')};
 Boolean mixedCaught=false;
 try { if(batchOp=='insert') Database.insert(mixedBatch,true); else if(batchOp=='update') Database.update(mixedBatch,true); else Database.upsert(mixedBatch,Account.Fields.ProbeKey__c,true); }
 catch(DmlException e) { mixedCaught=true; System.assertEquals(2,e.getNumDml()); System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(e.getDmlStatusCode(0))); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(e.getDmlStatusCode(1))); }
 System.assertEquals(true,mixedCaught);
}
Account personSeed=new Account(LastName='Synthetic Person'); insert personSeed;
Account personUpdate=new Account(Id=personSeed.Id,PersonHomePhone='4155550123');
System.assertEquals(true,Database.update(personUpdate,false).isSuccess());
Account idSeed=new Account(Name='Id seed'); insert idSeed;
Account idFirst=new Account(Id=idSeed.Id,FirstName='Synthetic'); Database.UpsertResult idFirstResult=Database.upsert(idFirst,Account.Fields.Id,false);
System.assertEquals(false,idFirstResult.isSuccess()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(idFirstResult.getErrors()[0].getStatusCode()));
Account idBad=new Account(Id=idSeed.Id,PersonEmail='bad'); Database.UpsertResult idBadResult=Database.upsert(idBad,Account.Fields.Id,false);
System.assertEquals(false,idBadResult.isSuccess()); System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(idBadResult.getErrors()[0].getStatusCode()));
Account idBlank=new Account(Id=idSeed.Id,PersonEmail='   '); System.assertEquals(true,Database.upsert(idBlank,Account.Fields.Id,false).isSuccess());
Account reading=new Account(Name='Read probe',Balance__c=1.250); insert reading;
Account partial=[SELECT Id FROM Account WHERE Id=:reading.Id]; Boolean hidden=false;
try { Decimal ignored=partial.Balance__c; } catch(SObjectException ex) { hidden=true; }
System.assertEquals(true,hidden);`
	// Native decimal equality rows: case349_review/org.tsv:2-4,10-12.
	source += `Account pw=new Account(Name='Parent wrong',Decimal_Key__c=1.25); Account pright=new Account(Name='Parent right',Decimal_Key__c=2.500); Contact pc=new Contact(LastName='Stored'); pc.putSObject('Account',new Account(Decimal_Key__c=2.5)); insert new List<Account>{pw,pright}; upsert pc; System.assertEquals(pright.Id,[SELECT AccountId FROM Contact WHERE Id=:pc.Id].AccountId);
Account qw=new Account(Name='Pending wrong',Decimal_Key__c=3.75); Account qright=new Account(Name='Pending right',Decimal_Key__c=4.500); Contact qc=new Contact(LastName='Pending'); qc.putSObject('Account',new Account(Decimal_Key__c=4.5)); insert new List<SObject>{qw,qright,qc}; System.assertEquals(qright.Id,[SELECT AccountId FROM Contact WHERE Id=:qc.Id].AccountId);`
	for _, tc := range []struct{ route, api string }{{"tooling", "62.0"}, {"tooling", "67.0"}, {"soap", "65.0"}, {"soap", "67.0"}} {
		t.Run(tc.route+"/"+tc.api, func(t *testing.T) {
			org := testOrg()
			org.APIVersion = tc.api
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureStandardObject(&org, "Contact")
			account := org.Objects["Account"]
			account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Person_Commerce", Name: "Person Commerce", Active: true, Available: true, Default: true}, {ID: "012000000000002", DeveloperName: "Synthetic_Person", Name: "Synthetic Person", Active: true, Available: true, PersonAccount: true}}
			account.Definition.Fields["Decimal_Key__c"] = storage.Field{APIName: "Decimal_Key__c", Type: storage.FieldDecimal, ExternalID: true}
			account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
			account.Definition.Fields["PersonMemo__c"] = storage.Field{APIName: "PersonMemo__c", Type: storage.FieldString}
			account.Definition.Fields["Balance__c"] = storage.Field{APIName: "Balance__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", DefaultValue: "1.25", Scale: 3}
			org.Objects["Account"] = account
			org.Objects["RecordType"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "RecordType", KeyPrefix: "012"}, Records: map[storage.ID]storage.Record{"012000000000001": {Object: "RecordType", ID: "012000000000001"}, "012000000000002": {Object: "RecordType", ID: "012000000000002"}}}
			org.Objects["EqualityProbe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "EqualityProbe__c", KeyPrefix: "a01", Fields: map[string]storage.Field{"Key__c": {APIName: "Key__c", Type: storage.FieldDecimal, ExternalID: true}}}, Records: map[storage.ID]storage.Record{}}
			handler := New(&org)
			handler.SetProjectIndex(typesys.Index{Objects: []schema.Object{
				{Name: "Account", Partial: true, Fields: []schema.Field{{Name: "RecordTypeId", Type: "Lookup", ReferenceTo: []string{"RecordType"}}, {Name: "FirstName", Type: "Text"}, {Name: "LastName", Type: "Text"}, {Name: "PersonEmail", Type: "Email"}, {Name: "PersonHomePhone", Type: "Phone"}, {Name: "PersonMobilePhone", Type: "Phone"}, {Name: "IsPersonAccount", Type: "Checkbox"}, {Name: "Balance__c", Type: "Number", DefaultValue: "1.25"}, {Name: "Decimal_Key__c", Type: "Number", ExternalID: true}, {Name: "ProbeKey__c", Type: "Text", ExternalID: true, Unique: true}, {Name: "PersonMemo__c", Type: "Text"}, {Name: "PersonTitle", Type: "Text"}}},
				{Name: "EqualityProbe__c", Fields: []schema.Field{{Name: "Key__c", Type: "Number", ExternalID: true}}},
			}})
			rec := httptest.NewRecorder()
			if tc.route == "tooling" {
				query := url.Values{"anonymousBody": []string{source}}
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v"+tc.api+"/tooling/executeAnonymous?"+query.Encode(), nil))
				if tc.api == "62.0" {
					if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "unsupported API version") {
						t.Fatalf("version boundary: %d %s", rec.Code, rec.Body.String())
					}
					return
				}
				var result struct {
					Compiled         bool   `json:"compiled"`
					Success          bool   `json:"success"`
					CompileProblem   string `json:"compileProblem"`
					ExceptionMessage string `json:"exceptionMessage"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
					t.Fatalf("status=%d: %v", rec.Code, err)
				}
				if rec.Code != http.StatusOK || !result.Compiled || !result.Success {
					t.Fatalf("status=%d result=%+v", rec.Code, result)
				}
			} else {
				body := soapEnvelope(`<apex:executeAnonymous><apex:String>` + html.EscapeString(source) + `</apex:String></apex:executeAnonymous>`)
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, strings.Replace(soapApexTestPath, "65.0", tc.api, 1), strings.NewReader(body)))
				result := decodeSOAPResult(t, rec.Body.String())
				if rec.Code != http.StatusOK || result["compiled"] != "true" || result["success"] != "true" {
					t.Fatalf("status=%d result=%v", rec.Code, result)
				}
			}
		})
	}
}
