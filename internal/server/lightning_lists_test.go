package server

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func l11ReviewControlServer(t *testing.T, api string) (*Server, storage.ID) {
	t.Helper()
	root := filepath.Join("..", "lwc", "compile", "testdata", "l11_runtime", "api"+api[:2])
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	p.LayoutFiles = append(p.LayoutFiles, filepath.Join("..", "lwc", "compile", "testdata", "l11_native_layouts", "api"+api[:2], "FamilyL11Child__c-Captured Full Edit.layout-meta.xml"))
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := apextest.OrgFromIndex(typesys.Build(p, schema))
	// Local record identities must use the prefixes assigned to this schema.
	ids := storage.NewRuntimeIDGeneratorForOrg(&org)
	parent := org.Objects["FamilyL11Parent__c"]
	parentID, err := ids.Next(parent.Definition.APIName)
	if err != nil {
		t.Fatal(err)
	}
	parent.Records[parentID] = storage.Record{ID: parentID, Object: parent.Definition.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue("Owned Parent")}}
	org.Objects[parent.Definition.APIName] = parent
	child := org.Objects["FamilyL11Child__c"]
	childID, err := ids.Next(child.Definition.APIName)
	if err != nil {
		t.Fatal(err)
	}
	child.Records[childID] = storage.Record{ID: childID, Object: child.Definition.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue("Owned Child"), "Sort__c": storage.DecimalValue("1"), "Parent__c": storage.IDValue(parentID)}}
	org.Objects[child.Definition.APIName] = child
	return NewWithSource(&org, source), parentID
}

func TestL11RelatedListRequiredFieldControls(t *testing.T) {
	data, err := os.ReadFile("testdata/l11_review_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		API        string         `json:"api"`
		ID         string         `json:"id"`
		Config     map[string]any `json:"config"`
		Error      any            `json:"error"`
		GoodConfig map[string]any `json:"good_config"`
		Oracle     string         `json:"oracle"`
	}
	if err := json.Unmarshal(data, &cases); err != nil || len(cases) != 88 {
		t.Fatalf("invalid native L11 required-field controls: %v", err)
	}
	servers := map[string]*Server{}
	parentIDs := map[string]storage.ID{}
	for _, api := range []string{"59.0", "67.0"} {
		servers[api], parentIDs[api] = l11ReviewControlServer(t, api)
	}
	for _, c := range cases {
		t.Run(c.API+"/"+c.ID, func(t *testing.T) {
			s := servers[c.API]
			if s == nil || c.Oracle == "" || c.Error == nil {
				t.Fatal("missing native API/diagnostic/provenance")
			}
			c.Config["parentRecordId"] = parentIDs[c.API].String()
			body, err := json.Marshal(c.Config)
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			s.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/lightning/wire/getRelatedListRecords", bytes.NewReader(body)))
			var payload struct {
				Error any `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(payload.Error)
			if err != nil {
				t.Fatal(err)
			}
			want, err := json.Marshal(c.Error)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("native related-list error expected <%s> actual <%s>", want, got)
			}
			// The captured Name/Sort good twin must still reach the record path.
			c.GoodConfig["parentRecordId"] = parentIDs[c.API].String()
			page, wireErr := s.relatedListRecordsData(c.GoodConfig)
			if wireErr != nil || page["count"] != 1 {
				t.Fatalf("good twin did not return its seeded child: %#v, %v", page, wireErr)
			}
		})
	}
}

func TestL11ListInlineEditControls(t *testing.T) {
	data, err := os.ReadFile("../lwc/compile/testdata/l11_native_layouts/provenance.json")
	if err != nil {
		t.Fatal(err)
	}
	var versions map[string]struct {
		Oracle     string `json:"oracle"`
		ObjectInfo struct {
			Fields map[string]struct {
				Updateable bool `json:"updateable"`
			} `json:"fields"`
		} `json:"object_info"`
		ListInfo struct {
			Columns any `json:"displayColumns"`
		} `json:"list_info"`
	}
	if err := json.Unmarshal(data, &versions); err != nil || len(versions) != 2 {
		t.Fatalf("invalid native L11 editability controls: %v", err)
	}
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			c := versions[api]
			if c.Oracle == "" || !c.ObjectInfo.Fields["Name"].Updateable || !c.ObjectInfo.Fields["Sort__c"].Updateable {
				t.Fatal("native good fields/provenance missing")
			}
			s, _ := l11ReviewControlServer(t, api)
			got, wireErr := s.listReadData("getListInfoByName", map[string]any{"objectApiName": "FamilyL11Child__c", "listViewApiName": "FamilyL11Owned"})
			if wireErr != nil {
				t.Fatal(wireErr)
			}
			actual, err := json.Marshal(got.(map[string]any)["displayColumns"])
			if err != nil {
				t.Fatal(err)
			}
			want, err := json.Marshal(c.ListInfo.Columns)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, want) {
				t.Fatalf("native inline-edit columns expected <%s> actual <%s>", want, actual)
			}
		})
	}
}

func TestRelatedListInfoCapturedVersionErrors(t *testing.T) {
	data, err := os.ReadFile("testdata/l11_related_info_version_errors.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		API    string         `json:"api"`
		Error  any            `json:"error"`
		Config map[string]any `json:"config"`
		Oracle string         `json:"oracle"`
	}
	if err := json.Unmarshal(data, &cases); err != nil || len(cases) != 9 {
		t.Fatalf("invalid captured D7 controls: %v", err)
	}
	for _, c := range cases {
		t.Run(c.API, func(t *testing.T) {
			if c.Oracle == "" || len(c.Config) == 0 {
				t.Fatal("missing native control provenance/config")
			}
			org := testOrg()
			server := New(&org)
			server.Source.Project.SourceAPIVersion = c.API
			body, err := json.Marshal(c.Config)
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/lightning/wire/getRelatedListInfo", bytes.NewReader(body)))
			var payload struct {
				Error any `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(payload.Error)
			if err != nil {
				t.Fatal(err)
			}
			want, err := json.Marshal(c.Error)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("captured D7 error expected <%s> actual <%s>", want, got)
			}
		})
	}
}

func TestListRecordsNumericOrderingAndDistinctProjection(t *testing.T) {
	org := testOrg()
	object := org.Objects["Account"]
	object.Definition.Fields["Rank__c"] = storage.Field{APIName: "Rank__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE"}
	object.Definition.Fields["Fee__c"] = storage.Field{APIName: "Fee__c", Type: storage.FieldDecimal, DisplayType: "CURRENCY", Scale: 2}
	object.Definition.Fields["Portion__c"] = storage.Field{APIName: "Portion__c", Type: storage.FieldDecimal, DisplayType: "PERCENT"}
	object.Records = map[storage.ID]storage.Record{}
	for _, row := range []struct{ id, name, rank string }{{"001XX0000000001", "A", "10"}, {"001XX0000000002", "B", "9"}} {
		id := storage.ID(row.id)
		object.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(row.name), "Rank__c": storage.DecimalValue(row.rank), "Fee__c": storage.DecimalValue(row.rank), "Portion__c": storage.DecimalValue(row.rank)}, System: storage.SystemFields{LastModifiedDate: "2026-10-06T09:51:27.000Z", SystemModstamp: "2026-10-07T05:54:40.000Z"}}
	}
	org.Objects["Account"] = object
	server := New(&org)
	selection := []string{"Account.Name", "Account.Rank__c", "Account.Fee__c", "Account.Portion__c"}
	page, err := server.listRecordPageData(object, map[string]any{"fields": selection, "sortBy": []string{"Rank__c"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rows := page["records"].([]any)
	if len(rows) != 2 || rows[0].(map[string]any)["id"] != "001XX0000000002" {
		t.Fatalf("numeric list ordering = %#v", rows)
	}
	first := rows[0].(map[string]any)
	if relationships, ok := first["childRelationships"].(map[string]any); !ok || len(relationships) != 0 {
		t.Fatalf("list relationships = %#v", first["childRelationships"])
	}
	if value, present := first["recordTypeInfo"]; !present || value != nil {
		t.Fatalf("list recordTypeInfo = %#v, present %v", value, present)
	}
	if first["systemModstamp"] != "2026-10-07T05:54:40.000Z" {
		t.Fatalf("list systemModstamp = %#v", first["systemModstamp"])
	}
	if value := first["fields"].(map[string]any)["Rank__c"].(map[string]any)["displayValue"]; value != nil {
		t.Fatalf("list number displayValue = %#v", value)
	}
	// The list projection must not mutate the shared record resource.
	record, wireErr := getLDSRecordWireData(&org, lwcbrowser.WireGetRecordRequest{RecordID: "001XX0000000002", Fields: selection}, server.Source)
	if wireErr != nil {
		t.Fatal(wireErr)
	}
	if _, present := record["recordTypeInfo"]; present {
		t.Fatal("list projection leaked into getRecord")
	}
	if value := record["fields"].(map[string]any)["Rank__c"].(map[string]any)["displayValue"]; value != "9" {
		t.Fatalf("getRecord number displayValue changed: %#v", value)
	}
	for _, name := range []string{"Fee__c", "Portion__c"} {
		listValue := first["fields"].(map[string]any)[name].(map[string]any)["displayValue"]
		recordValue := record["fields"].(map[string]any)[name].(map[string]any)["displayValue"]
		if listValue != recordValue {
			t.Fatalf("unrelated %s display changed: list %#v record %#v", name, listValue, recordValue)
		}
	}
}

func TestListViewColumnsResolveInlineSchema(t *testing.T) {
	root := t.TempDir()
	objectPath := filepath.Join(root, "objects", "Sample__c", "Sample__c.object-meta.xml")
	viewPath := filepath.Join(root, "objects", "Sample__c", "listViews", "Owned.listView-meta.xml")
	if err := os.MkdirAll(filepath.Dir(viewPath), 0755); err != nil {
		t.Fatal(err)
	}
	object := `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Sample</label><pluralLabel>Samples</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><fields><fullName>Weight__c</fullName><label>Weight</label><type>Number</type><precision>6</precision><scale>0</scale></fields></CustomObject>`
	if err := os.WriteFile(objectPath, []byte(object), 0644); err != nil {
		t.Fatal(err)
	}
	view := `<ListView xmlns="http://soap.sforce.com/2006/04/metadata"><columns>NAME</columns><columns>Weight__c</columns><filterScope>Everything</filterScope><label>Owned</label></ListView>`
	if err := os.WriteFile(viewPath, []byte(view), 0644); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: root, ObjectFiles: []string{objectPath}, ListViewFiles: []string{viewPath}}
	if _, err := NewSourceMetadataFromProject(p); err != nil {
		t.Fatalf("inline column rejected: %v", err)
	}
	if err := os.WriteFile(viewPath, []byte(strings.ReplaceAll(view, "Weight__c", "OwnedMissing__c")), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := NewSourceMetadataFromProject(p)
	if err == nil || err.Error() != "In field: columns - no CustomField named Sample__c.OwnedMissing__c found" {
		t.Fatalf("missing column diagnostic = %v", err)
	}
}

func TestListViewUpdatePreservesUnselectedMetadata(t *testing.T) {
	original := []byte(`<ListView xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Owned</fullName><columns>NAME</columns><columns>Rank__c</columns><filterScope>Everything</filterScope><booleanFilter>1</booleanFilter><filters><field>NAME</field><operation>startsWith</operation><value>Owned </value></filters><label>Original</label><sharedTo><allInternalUsers>true</allInternalUsers></sharedTo></ListView>`)
	updated, err := updateListViewXML(original, listViewMetadata{Label: "Changed", Columns: []string{"Name"}})
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Label   string              `xml:"label"`
		Columns []string            `xml:"columns"`
		Scope   string              `xml:"filterScope"`
		Logic   string              `xml:"booleanFilter"`
		Filters []listViewFilterXML `xml:"filters"`
		Sharing struct {
			All string `xml:"allInternalUsers"`
		} `xml:"sharedTo"`
	}
	if err := xml.Unmarshal(updated, &data); err != nil {
		t.Fatal(err)
	}
	if data.Label != "Changed" || len(data.Columns) != 1 || data.Columns[0] != "Name" || data.Scope != "Everything" || data.Logic != "1" || data.Sharing.All != "true" || len(data.Filters) != 1 || data.Filters[0].Value != "Owned " {
		t.Fatalf("updated metadata lost non-selected values: %s", updated)
	}
	if !strings.Contains(string(updated), "http://soap.sforce.com/2006/04/metadata") {
		t.Fatal("metadata namespace lost")
	}
}
