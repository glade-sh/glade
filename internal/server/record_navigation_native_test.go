package server

import (
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Native navigation chooses the record's object from its ID, including an
// absent but valid-shaped ID; a malformed ID keeps its requested object.
func TestRecordNavigationNativeObjectResolution(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "pr163_record_navigation", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 23 {
		t.Fatalf("native rows %d: %v", len(rows), err)
	}
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	for _, object := range []string{"Account", "Contact"} {
		xml := `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata"><sobjectType>` + object + `</sobjectType><type>RecordPage</type></FlexiPage>`
		writeLWCShellServerTestFile(t, root, "force-app/main/default/flexipages/"+object+"_Page.flexipage-meta.xml", xml)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	account := storage.ID("001000000000001AAA")
	contact := storage.ID("003000000000001AAA")
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001"}, Records: map[storage.ID]storage.Record{account: {ID: account, Object: "Account"}}}
	org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", KeyPrefix: "003"}, Records: map[storage.ID]storage.Record{contact: {ID: contact, Object: "Contact"}}}
	handler := newLightningTestServer(t, &org, SourceMetadata{Project: p})
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"_"+row[1], func(t *testing.T) {
			requested, id, want := row[0], string(account), row[4]
			if strings.HasPrefix(requested, "contact_id_") {
				requested = strings.TrimPrefix(requested, "contact_id_")
				id = string(contact)
			}
			switch row[0] {
			case "absent_account":
				requested, id, want = "Account", "001000000000099AAA", "Account"
			case "absent_contact":
				requested, id, want = "Account", "003000000000099AAA", "Contact"
			case "malformed_record":
				requested, id, want = "Record", "not-an-id", "Record"
			}
			parts := []string{"record", requested, id}
			req := httptest.NewRequest(http.MethodGet, "/lwc/preview/record/"+requested+"/"+id+"?state.c__mode=probe", nil)
			shell, redirect, diagnostics, err := handler.resolveLWCShellRequest(req, parts)
			if row[0] == "malformed_record" {
				if redirect != "" || err == nil {
					t.Fatalf("malformed ID must not be canonicalized; redirect %q, err %v", redirect, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("native result %s, local error %v; diagnostics %#v", row[3], err, diagnostics)
			}
			if shell.Context.ObjectAPIName != want {
				t.Fatalf("object = %q, want %q", shell.Context.ObjectAPIName, want)
			}
			expectedRedirect := ""
			if requested != want {
				expectedRedirect = "/lwc/preview/record/" + want + "/" + id + "?state.c__mode=probe"
			}
			if redirect != expectedRedirect {
				t.Fatalf("redirect = %q, want %q", redirect, expectedRedirect)
			}
			if expectedRedirect != "" {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, req)
				if response.Code != http.StatusFound || response.Header().Get("Location") != expectedRedirect {
					t.Fatalf("HTTP navigation = %d %q, want redirect %q", response.Code, response.Header().Get("Location"), expectedRedirect)
				}
			}
		})
	}
}

// Unknown and ambiguous local prefixes retain the requested route. These are
// preservation controls for incomplete local schema, not native parity rows.
func TestRecordNavigationUnresolvedIDPreservesRoute(t *testing.T) {
	root := t.TempDir()
	writeLWCShellServerTestFile(t, root, "sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	for _, object := range []string{"Account", "Contact"} {
		writeLWCShellServerTestFile(t, root, "force-app/main/default/flexipages/"+object+"_Page.flexipage-meta.xml", `<FlexiPage xmlns="http://soap.sforce.com/2006/04/metadata"><sobjectType>`+object+`</sobjectType><type>RecordPage</type></FlexiPage>`)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	for name, prefix := range map[string]string{"Account": "001", "Contact": "003", "Other__c": "001"} {
		org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, KeyPrefix: prefix}}
	}
	handler := newLightningTestServer(t, &org, SourceMetadata{Project: p})
	for _, tc := range []struct{ name, object, id string }{
		{"unknown", "Account", "999000000000099AAA"},
		{"ambiguous", "Contact", "001000000000099AAA"},
		{"malformed", "Account", "001short"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/lwc/preview/record/"+tc.object+"/"+tc.id, nil)
			shell, redirect, diagnostics, err := handler.resolveLWCShellRequest(req, []string{"record", tc.object, tc.id})
			if err != nil || redirect != "" || shell.Context.ObjectAPIName != tc.object {
				t.Fatalf("shell object %q, redirect %q, err %v, diagnostics %#v", shell.Context.ObjectAPIName, redirect, err, diagnostics)
			}
		})
	}
}
