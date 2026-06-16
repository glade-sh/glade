package server

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPartnerSOAPDescribeSObjectsReturnsNimbleForceFields(t *testing.T) {
	org := partnerSOAPTestOrg()
	handler := New(&org)

	rec := partnerSOAPPost(handler, partnerSOAPEnvelope(`
		<urn:describeSObjects>
			<urn:sObjectType>Account</urn:sObjectType>
		</urn:describeSObjects>`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	assertSOAPText(t, body, "name", "Account")
	assertSOAPText(t, body, "label", "Account")
	assertSOAPText(t, body, "custom", "false")
	assertSOAPText(t, body, "customSetting", "false")
	assertSOAPText(t, body, "keyPrefix", "001")
	assertSOAPText(t, body, "name", "External_Id__c")
	assertSOAPText(t, body, "externalId", "true")
	assertSOAPText(t, body, "unique", "true")
	assertSOAPText(t, body, "relationshipName", "Account__r")
	assertSOAPText(t, body, "referenceTo", "Account")
	assertSOAPText(t, body, "recordTypeId", "012000000000001AAA")
}

func TestPartnerSOAPDescribeSObjectsUsesFieldKeyWhenAPINameMissing(t *testing.T) {
	org := partnerSOAPTestOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Sparse__c"] = storage.Field{Type: storage.FieldString}
	org.Objects["Account"] = account
	handler := New(&org)

	rec := partnerSOAPPost(handler, partnerSOAPEnvelope(`
		<urn:describeSObjects>
			<urn:sObjectType>Account</urn:sObjectType>
		</urn:describeSObjects>`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	assertSOAPText(t, rec.Body.String(), "name", "Sparse__c")
	assertSOAPText(t, rec.Body.String(), "label", "Sparse__c")
}

func TestPartnerSOAPUpsertCreatesAndUpdatesByExternalID(t *testing.T) {
	org := partnerSOAPTestOrg()
	handler := New(&org)

	create := partnerSOAPPost(handler, partnerSOAPEnvelope(`
		<urn:upsert>
			<urn:externalIDFieldName>External_Id__c</urn:externalIDFieldName>
			<urn:sObjects xsi:type="sf:Account">
				<sf:type>Account</sf:type>
				<sf:External_Id__c>EXT-1</sf:External_Id__c>
				<sf:Name>First Name</sf:Name>
			</urn:sObjects>
		</urn:upsert>`))
	if create.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", create.Code, create.Body.String())
	}
	assertSOAPText(t, create.Body.String(), "success", "true")
	assertSOAPText(t, create.Body.String(), "created", "true")

	update := partnerSOAPPost(handler, partnerSOAPEnvelope(`
		<urn:upsert>
			<urn:externalIDFieldName>External_Id__c</urn:externalIDFieldName>
			<urn:sObjects xsi:type="sf:Account">
				<sf:External_Id__c>EXT-1</sf:External_Id__c>
				<sf:Name>Second Name</sf:Name>
			</urn:sObjects>
		</urn:upsert>`))
	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", update.Code, update.Body.String())
	}
	assertSOAPText(t, update.Body.String(), "success", "true")
	assertSOAPText(t, update.Body.String(), "created", "false")

	account := singlePartnerSOAPRecord(t, org.Objects["Account"])
	if got := account.Fields["Name"].String; got != "Second Name" {
		t.Fatalf("Name = %q, want Second Name", got)
	}
}

func TestPartnerSOAPUpsertResolvesRelationshipExternalID(t *testing.T) {
	org := partnerSOAPTestOrg()
	account := org.Objects["Account"]
	account.Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Parent"),
			"External_Id__c": storage.StringValue("PARENT-1"),
		},
	}
	org.Objects["Account"] = account
	handler := New(&org)

	rec := partnerSOAPPost(handler, partnerSOAPEnvelope(`
		<urn:upsert>
			<urn:externalIDFieldName>External_Id__c</urn:externalIDFieldName>
			<urn:sObjects>
				<sf:type>Membership__c</sf:type>
				<sf:External_Id__c>MEM-1</sf:External_Id__c>
				<sf:Name>Membership</sf:Name>
				<sf:Account__r>
					<sf:type>Account</sf:type>
					<sf:External_Id__c>PARENT-1</sf:External_Id__c>
				</sf:Account__r>
			</urn:sObjects>
		</urn:upsert>`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	assertSOAPText(t, rec.Body.String(), "success", "true")

	membership := singlePartnerSOAPRecord(t, org.Objects["Membership__c"])
	if got := membership.Fields["Account__c"].ID; got != "001000000000001" {
		t.Fatalf("Account__c = %q, want parent id", got)
	}
}

func TestPartnerSOAPUpsertXsiNilClearsField(t *testing.T) {
	org := partnerSOAPTestOrg()
	account := org.Objects["Account"]
	account.Records["001000000000001"] = storage.Record{
		ID:     "001000000000001",
		Object: "Account",
		Fields: map[string]storage.Value{
			"Name":           storage.StringValue("Existing"),
			"External_Id__c": storage.StringValue("EXT-1"),
			"Description":    storage.StringValue("clear me"),
		},
	}
	org.Objects["Account"] = account
	handler := New(&org)

	rec := partnerSOAPPost(handler, partnerSOAPEnvelope(`
		<urn:upsert>
			<urn:externalIDFieldName>External_Id__c</urn:externalIDFieldName>
			<urn:sObjects xsi:type="sf:Account">
				<sf:External_Id__c>EXT-1</sf:External_Id__c>
				<sf:Description xsi:nil="true"/>
			</urn:sObjects>
		</urn:upsert>`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	assertSOAPText(t, rec.Body.String(), "success", "true")

	stored := org.Objects["Account"].Records["001000000000001"]
	if _, ok := stored.Fields["Description"]; ok {
		t.Fatalf("Description was not cleared: %#v", stored.Fields["Description"])
	}
}

func TestPartnerSOAPUpsertPartialFailureReturnsRowResults(t *testing.T) {
	org := partnerSOAPTestOrg()
	handler := New(&org)

	rec := partnerSOAPPost(handler, partnerSOAPEnvelope(`
		<urn:upsert>
			<urn:externalIDFieldName>External_Id__c</urn:externalIDFieldName>
			<urn:sObjects xsi:type="sf:Account">
				<sf:External_Id__c>GOOD-1</sf:External_Id__c>
				<sf:Name>Good Row</sf:Name>
			</urn:sObjects>
			<urn:sObjects xsi:type="sf:Account">
				<sf:External_Id__c>BAD-1</sf:External_Id__c>
			</urn:sObjects>
		</urn:upsert>`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if got := strings.Count(body, "<success>true</success>"); got != 1 {
		t.Fatalf("true success count = %d, body = %s", got, body)
	}
	if got := strings.Count(body, "<success>false</success>"); got != 1 {
		t.Fatalf("false success count = %d, body = %s", got, body)
	}
	assertSOAPText(t, body, "statusCode", "REQUIRED_FIELD_MISSING")
	assertSOAPText(t, body, "fields", "Name")
}

func partnerSOAPTestOrg() storage.OrgState {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Label = "Account"
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{
		ID:            "012000000000001AAA",
		Name:          "Business Account",
		DeveloperName: "Business",
		Active:        true,
		Available:     true,
		Default:       true,
	}}
	storage.EnsureRecordTypeIDField(&account.Definition)
	account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Account Name", Type: storage.FieldString, Required: true, Nillable: storage.BoolFlag(false), Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	account.Definition.Fields["Description"] = storage.Field{APIName: "Description", Label: "Description", Type: storage.FieldString, Nillable: storage.BoolFlag(true), Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	account.Definition.Fields["External_Id__c"] = storage.Field{APIName: "External_Id__c", Label: "External ID", Type: storage.FieldString, ExternalID: true, Unique: true, Nillable: storage.BoolFlag(false), Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	account.Definition.Fields["Account__c"] = storage.Field{APIName: "Account__c", Label: "Parent Account", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r", Nillable: storage.BoolFlag(true), Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)}
	org.Objects["Account"] = account
	org.Objects["RecordType"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "RecordType",
			KeyPrefix: "012",
			Fields: map[string]storage.Field{
				"Id":          {APIName: "Id", Type: storage.FieldID},
				"SobjectType": {APIName: "SobjectType", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"012000000000001AAA": {
				ID:     "012000000000001AAA",
				Object: "RecordType",
				Fields: map[string]storage.Value{
					"SobjectType": storage.StringValue("Account"),
				},
			},
		},
	}
	org.Objects["Membership__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Membership__c",
			Label:     "Membership",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Name":           {APIName: "Name", Label: "Membership Name", Type: storage.FieldString, Required: true, Nillable: storage.BoolFlag(false), Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)},
				"External_Id__c": {APIName: "External_Id__c", Label: "External ID", Type: storage.FieldString, ExternalID: true, Unique: true, Nillable: storage.BoolFlag(false), Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)},
				"Account__c":     {APIName: "Account__c", Label: "Account", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r", Nillable: storage.BoolFlag(true), Createable: storage.BoolFlag(true), Updateable: storage.BoolFlag(true)},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	return org
}

func partnerSOAPEnvelope(body string) string {
	return `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:urn="urn:partner.soap.sforce.com" xmlns:sf="urn:sobject.partner.soap.sforce.com" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><soapenv:Body>` + body + `</soapenv:Body></soapenv:Envelope>`
}

func partnerSOAPPost(handler http.Handler, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/services/Soap/u/65.0/00D000000000001", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	handler.ServeHTTP(rec, req)
	return rec
}

func singlePartnerSOAPRecord(t *testing.T, object storage.ObjectState) storage.Record {
	t.Helper()
	if len(object.Records) != 1 {
		t.Fatalf("%s record count = %d, want 1", object.Definition.APIName, len(object.Records))
	}
	for _, record := range object.Records {
		return record
	}
	return storage.Record{}
}

func assertSOAPText(t *testing.T, body, element, want string) {
	t.Helper()
	decoder := xml.NewDecoder(strings.NewReader(body))
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != element {
			continue
		}
		var text string
		if err := decoder.DecodeElement(&text, &start); err != nil {
			t.Fatalf("decode %s: %v", element, err)
		}
		if strings.TrimSpace(text) == want {
			return
		}
	}
	t.Fatalf("missing <%s>%s</%s> in %s", element, want, element, body)
}
