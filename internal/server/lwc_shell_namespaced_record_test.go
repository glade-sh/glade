package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// In a namespaced project the record page and the component's supported
// objects name the object without the prefix (Widget__c), while records and
// navigation carry acme__Widget__c. Navigation without objectApiName uses the
// "Record" placeholder, and the object comes from the record Id.
func TestResolveLWCShellRequestNamespacedRecordPage(t *testing.T) {
	const recordID = "a0W00000001AAAAAAA"
	cases := []struct {
		name       string
		objectPart string
		supported  string
		wantObject string
		wantCode   string
	}{
		{name: "namespaced object accepts", objectPart: "acme__Widget__c", supported: "Widget__c", wantObject: "acme__Widget__c"},
		{name: "record placeholder resolves from id", objectPart: "Record", supported: "Widget__c", wantObject: "acme__Widget__c"},
		{name: "placeholder with unknown id rejects", objectPart: "Record", supported: "Widget__c", wantCode: "GLADELWC004"},
		{name: "unsupported object rejects", objectPart: "acme__Widget__c", supported: "Contact", wantCode: "GLADELWC004"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeLWCShellServerTestFile(t, root, "sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"acme","sourceApiVersion":"64.0"}`)
			writeLWCShellServerTestFile(t, root, "force-app/main/default/lwc/contextProbe/contextProbe.js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <isExposed>true</isExposed>
  <targets><target>lightning__RecordPage</target></targets>
  <targetConfigs>
    <targetConfig targets="lightning__RecordPage">
      <objects><object>`+tc.supported+`</object></objects>
    </targetConfig>
  </targetConfigs>
</LightningComponentBundle>`)
			writeLWCShellServerTestFile(t, root, "force-app/main/default/flexipages/Widget_Record_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata">
  <sobjectType>Widget__c</sobjectType>
  <type>RecordPage</type>
  <flexiPageRegions>
    <name>main</name>
    <componentInstances>
      <componentName>contextProbe</componentName>
    </componentInstances>
  </flexiPageRegions>
</FlexiPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			id := recordID
			if strings.Contains(tc.name, "unknown id") {
				id = "a0W00000009ZZZZAAA"
			}
			org.Objects["acme__Widget__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{APIName: "acme__Widget__c", KeyPrefix: "a0W"},
				Records:    map[storage.ID]storage.Record{recordID: {ID: recordID, Object: "acme__Widget__c"}},
			}
			handler := newLightningTestServer(t, &org, SourceMetadata{Project: p})

			path := "/lwc/preview/record/" + tc.objectPart + "/" + id + "?page=Widget_Record_Page"
			shell, _, diagnostics, err := handler.resolveLWCShellRequest(httptest.NewRequest(http.MethodGet, path, nil), []string{"record", tc.objectPart, id})
			if tc.wantCode != "" {
				if err == nil || len(diagnostics) != 1 || diagnostics[0].Code != tc.wantCode {
					t.Fatalf("err = %v diagnostics = %#v, want %s", err, diagnostics, tc.wantCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve error = %v diagnostics = %#v", err, diagnostics)
			}
			if shell.Context.ObjectAPIName != tc.wantObject || shell.Context.RecordID != id {
				t.Fatalf("context = %#v, want object %q record %q", shell.Context, tc.wantObject, id)
			}
		})
	}
}
