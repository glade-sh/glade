package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestToolingSourceMetadataComponentsReadQueryAndDescribe(t *testing.T) {
	source := testToolingMetadataComponentSource(t)
	org := testOrg()
	handler := NewWithSource(&org, source)

	cases := []struct {
		objectName string
		query      string
		id         string
		want       string
	}{
		{
			objectName: "CustomObject",
			query:      "SELECT Id, DeveloperName, FullName FROM CustomObject WHERE DeveloperName = 'Account'",
			id:         "01I000000000001",
			want:       `"FullName":"Account"`,
		},
		{
			objectName: "CustomField",
			query:      "SELECT Id, DeveloperName, TableEnumOrId, FullName FROM CustomField WHERE FullName = 'Account.Rating__c'",
			id:         "00N000000000001",
			want:       `"TableEnumOrId":"Account"`,
		},
		{
			objectName: "Layout",
			query:      "SELECT Id, Name, TableEnumOrId FROM Layout WHERE Name = 'Account-Account Layout'",
			id:         "00h000000000001",
			want:       `"TableEnumOrId":"Account"`,
		},
		{
			objectName: "CompactLayout",
			query:      "SELECT Id, DeveloperName, SobjectType, FullName FROM CompactLayout WHERE DeveloperName = 'AccountCard'",
			id:         "0CL000000000001",
			want:       `"SobjectType":"Account"`,
		},
		{
			objectName: "RecordType",
			query:      "SELECT Id, DeveloperName, SobjectType, FullName FROM RecordType WHERE DeveloperName = 'Business'",
			id:         "012000000000001",
			want:       `"SobjectType":"Account"`,
		},
		{
			objectName: "ValidationRule",
			query:      "SELECT Id, ValidationName, EntityDefinitionId, Active, FullName FROM ValidationRule WHERE ValidationName = 'Require_Rating'",
			id:         "03d000000000001",
			want:       `"Active":true`,
		},
	}

	discovery := httptest.NewRecorder()
	handler.ServeHTTP(discovery, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/tooling/sobjects", nil))
	if discovery.Code != http.StatusOK {
		t.Fatalf("tooling discovery status = %d body=%s", discovery.Code, discovery.Body.String())
	}
	for _, tc := range cases {
		if !strings.Contains(discovery.Body.String(), `"name":"`+tc.objectName+`"`) {
			t.Fatalf("tooling discovery missing %s: %s", tc.objectName, discovery.Body.String())
		}
	}

	for _, tc := range cases {
		t.Run(tc.objectName, func(t *testing.T) {
			describe := httptest.NewRecorder()
			handler.ServeHTTP(describe, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/tooling/sobjects/"+tc.objectName+"/describe", nil))
			if describe.Code != http.StatusOK || !strings.Contains(describe.Body.String(), `"name":"Id"`) {
				t.Fatalf("%s describe status=%d body=%s", tc.objectName, describe.Code, describe.Body.String())
			}

			query := httptest.NewRecorder()
			handler.ServeHTTP(query, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/tooling/query?q="+url.QueryEscape(tc.query), nil))
			if query.Code != http.StatusOK || !strings.Contains(query.Body.String(), `"totalSize":1`) || !strings.Contains(query.Body.String(), tc.want) {
				t.Fatalf("%s query status=%d body=%s", tc.objectName, query.Code, query.Body.String())
			}
			var payload struct {
				Records []map[string]any `json:"records"`
			}
			if err := json.Unmarshal(query.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Records) != 1 {
				t.Fatalf("%s records = %#v", tc.objectName, payload.Records)
			}
			assertQueryRecordShape(t, payload.Records[0], tc.objectName, tc.id, serverTestDataPath+"/tooling/sobjects/"+tc.objectName+"/"+tc.id)

			record := httptest.NewRecorder()
			handler.ServeHTTP(record, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/tooling/sobjects/"+tc.objectName+"/"+tc.id, nil))
			if record.Code != http.StatusOK || !strings.Contains(record.Body.String(), tc.want) {
				t.Fatalf("%s record status=%d body=%s", tc.objectName, record.Code, record.Body.String())
			}
		})
	}
}

func TestToolingSourceMetadataUsesVisualforceSidecarVersionAndFallback(t *testing.T) {
	root := filepath.Join(".testdata-generated", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	pagePath := filepath.Join(root, "force-app/main/default/pages/Sidecar.page")
	fallbackPath := filepath.Join(root, "force-app/main/default/pages/Fallback.page")
	componentPath := filepath.Join(root, "force-app/main/default/components/Sidecar.component")
	metadataOnlyPath := filepath.Join(root, "force-app/main/default/pages/MetadataOnly.page-meta.xml")
	writeServerTestFile(t, pagePath, `<apex:page/>`)
	writeServerTestFile(t, pagePath+"-meta.xml", `<ApexPage><apiVersion>61.0</apiVersion></ApexPage>`)
	writeServerTestFile(t, fallbackPath, `<apex:page/>`)
	writeServerTestFile(t, componentPath, `<apex:component/>`)
	writeServerTestFile(t, componentPath+"-meta.xml", `<ApexComponent><apiVersion>62.0</apiVersion></ApexComponent>`)
	writeServerTestFile(t, metadataOnlyPath, `<ApexPage><apiVersion>60.0</apiVersion></ApexPage>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	assertToolingVisualforceVersion(t, source, "ApexPage", "Sidecar", "61.0")
	assertToolingVisualforceVersion(t, source, "ApexPage", "Fallback", "65.0")
	assertToolingVisualforceVersion(t, source, "ApexPage", "MetadataOnly", "60.0")
	assertToolingVisualforceVersion(t, source, "ApexComponent", "Sidecar", "62.0")

	if got := source.ToolingOrg.Objects["ApexPage"].Records[findToolingVisualforceRecord(t, source, "ApexPage", "MetadataOnly")].Fields["Markup"].String; got != "" {
		t.Fatalf("metadata-only page markup = %q, want empty", got)
	}
}

func TestToolingSourceMetadataRejectsInvalidVisualforceSidecarVersion(t *testing.T) {
	root := filepath.Join(".testdata-generated", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	path := filepath.Join(root, "force-app/main/default/pages/Probe.page")
	writeServerTestFile(t, path, `<apex:page/>`)
	writeServerTestFile(t, path+"-meta.xml", `<ApexPage><apiVersion>67.1</apiVersion></ApexPage>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewSourceMetadataFromProject(p)
	if err == nil || !strings.Contains(err.Error(), "unsupported Visualforce source API version") {
		t.Fatalf("NewSourceMetadataFromProject error = %v, want invalid sidecar version", err)
	}
}

func assertToolingVisualforceVersion(t *testing.T, source SourceMetadata, objectName, name, want string) {
	t.Helper()
	if got := source.ToolingOrg.Objects[objectName].Records[findToolingVisualforceRecord(t, source, objectName, name)].Fields["ApiVersion"].Decimal; got != want {
		t.Fatalf("%s %s API version = %q, want %q", objectName, name, got, want)
	}
}

func findToolingVisualforceRecord(t *testing.T, source SourceMetadata, objectName, name string) storage.ID {
	t.Helper()
	for id, record := range source.ToolingOrg.Objects[objectName].Records {
		if record.Fields["Name"].String == name {
			return id
		}
	}
	t.Fatalf("missing %s %s record", objectName, name)
	return ""
}

func testToolingMetadataComponentSource(t *testing.T) SourceMetadata {
	t.Helper()
	root := filepath.Join(".testdata-generated", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject><label>Account</label><pluralLabel>Accounts</pluralLabel></CustomObject>`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Rating__c.field-meta.xml"), `<CustomField><fullName>Rating__c</fullName><label>Rating</label><type>Text</type><length>40</length></CustomField>`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/layouts/Account-Account Layout.layout-meta.xml"), `<Layout/>`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/objects/Account/compactLayouts/AccountCard.compactLayout-meta.xml"), `<CompactLayout><label>Account Card</label><fields>Name</fields><fields>Rating__c</fields></CompactLayout>`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/objects/Account/recordTypes/Business.recordType-meta.xml"), `<RecordType><label>Business Account</label><active>true</active></RecordType>`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/objects/Account/validationRules/Require_Rating.validationRule-meta.xml"), `<ValidationRule><active>true</active><errorConditionFormula>ISBLANK(Rating__c)</errorConditionFormula><errorMessage>Rating required</errorMessage></ValidationRule>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return source
}
