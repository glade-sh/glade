package server

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDescribeChildRelationshipLookupOrderNativeUnitRoutes(t *testing.T) {
	rows := readNativeChildRelationshipOrderRows(t, "testdata/child_relationship_order/org.tsv")
	org := standardChildRelationshipOrderOrg(t, []string{"Contact"}, rows)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/direct_describe", func(t *testing.T) {
			var want, got []string
			for _, row := range rows {
				if row.APIVersion == api && row.ParentObject == "Contact" && row.ChildSObject == "CampaignMember" {
					want = append(want, row.Field)
				}
			}
			original := append([]storage.Relationship(nil), org.Objects["CampaignMember"].Definition.Relations...)
			for _, relationship := range describeChildRelationships("Contact", &org) {
				if relationship["childSObject"] == "CampaignMember" {
					got = append(got, relationship["field"].(string))
				}
			}
			if !reflect.DeepEqual(original, org.Objects["CampaignMember"].Definition.Relations) {
				t.Fatal("describe reordered the stored relationship definition")
			}
			if len(want) < 2 || !slices.Equal(got, want) {
				t.Fatalf("lookup order: native=%v; local=%v", want, got)
			}
		})
	}
}

func TestDescribeChildRelationshipWireAndToolingRouteTwins(t *testing.T) {
	rows := readNativeChildRelationshipOrderRows(t, "testdata/child_relationship_order/org.tsv")
	org := standardChildRelationshipOrderOrg(t, []string{"Contact"}, rows)
	t.Run("lightning_object_info", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/lightning/wire/getObjectInfo", strings.NewReader(`{"objectApiName":"Contact"}`))
		req.Header.Set("Content-Type", "application/json")
		New(&org).ServeHTTP(rec, req)
		var payload struct {
			Data struct {
				Relationships []struct {
					Child string `json:"childObjectApiName"`
					Name  string `json:"relationshipName"`
					Field string `json:"fieldName"`
				} `json:"childRelationships"`
			} `json:"data"`
		}
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &payload) != nil {
			t.Fatalf("object info response: %d %s", rec.Code, rec.Body.String())
		}
		var keys []string
		for _, relationship := range payload.Data.Relationships {
			keys = append(keys, relationship.Child+"."+relationship.Name+"."+relationship.Field)
		}
		// Object info independently sorts named relationships for its wire shape.
		if len(keys) == 0 || !slices.IsSorted(keys) {
			t.Fatalf("object info relationship ordering changed: %v", keys)
		}
	})
	t.Run("tooling_describe_without_org", func(t *testing.T) {
		payload := toolingDescribePayload(org.Objects["Contact"].Definition)
		if relationships, ok := payload["childRelationships"].([]map[string]any); !ok || len(relationships) != 0 {
			t.Fatalf("tooling relationships = %#v", payload["childRelationships"])
		}
	})
}

func TestDescribeChildRelationshipRecordFallbackAndExplicitRoutes(t *testing.T) {
	rows := readNativeChildRelationshipOrderRows(t, "testdata/child_relationship_order/org.tsv")
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"describe_fallback", "explicit_parent_metadata"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := standardChildRelationshipOrderOrg(t, []string{"Contact"}, rows)
				var want []string
				for _, row := range rows {
					if row.APIVersion == api && row.ParentObject == "Contact" && row.ChildSObject == "CampaignMember" {
						want = append(want, row.Field)
					}
				}
				if len(want) != 2 {
					t.Fatalf("missing native ordering pair: %v", want)
				}
				parent := org.Objects["Contact"]
				parent.Definition.Relations = nil
				if route == "explicit_parent_metadata" {
					// Explicit record metadata preserves its supplied order.
					slices.Reverse(want)
					for _, field := range want {
						parent.Definition.Relations = append(parent.Definition.Relations, storage.Relationship{Field: field, ChildRelationship: field + "Children"})
					}
				}
				const id storage.ID = "003000000000001AAA"
				parent.Records[id] = storage.Record{ID: id, Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Receiver")}}
				org.Objects["Contact"] = parent
				check := func(relationships []map[string]any) {
					t.Helper()
					var got []string
					for _, relationship := range relationships {
						if route == "explicit_parent_metadata" || relationship["childSObject"] == "CampaignMember" {
							got = append(got, relationship["field"].(string))
						}
					}
					if !slices.Equal(got, want) {
						t.Fatalf("record relationship order: got %v, want %v", got, want)
					}
				}
				check(recordChildRelationships(&org, "Contact"))
				rec := httptest.NewRecorder()
				New(&org).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/getRecord", strings.NewReader(`{"recordId":"003000000000001AAA","fields":["Contact.LastName"]}`)))
				var payload struct {
					Data struct {
						Relationships []map[string]any `json:"childRelationships"`
					} `json:"data"`
					Error any `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil || rec.Code != http.StatusOK || payload.Error != nil {
					t.Fatalf("record wire: %d %s (%v)", rec.Code, rec.Body.String(), err)
				}
				check(payload.Data.Relationships)
			})
		}
	}
}

func TestDescribeChildRelationshipRelatedListOrderingAndLookupRoutes(t *testing.T) {
	// Deliberately oppose lookup-field order and relationship-name order.
	// These preserve local wire contracts; native REST order is tested above.
	org := storage.NewOrgState()
	const parentID storage.ID = "001000000000001AAA"
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{parentID: {ID: parentID, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Parent")}}}}
	org.Objects["Link__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Link__c", KeyPrefix: "a00", Fields: map[string]storage.Field{
		"Name":     {APIName: "Name", Type: storage.FieldString},
		"Alpha__c": {APIName: "Alpha__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		"Zulu__c":  {APIName: "Zulu__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
	}, Relations: []storage.Relationship{
		{Field: "Zulu__c", ChildRelationship: "Alpha", ParentObjects: []string{"Account"}},
		{Field: "Alpha__c", ChildRelationship: "Zulu", ParentObjects: []string{"Account"}},
	}}, Records: map[storage.ID]storage.Record{
		"a00000000000001AAA": {ID: "a00000000000001AAA", Object: "Link__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Alpha child"), "Zulu__c": storage.IDValue(parentID)}},
		"a00000000000002AAA": {ID: "a00000000000002AAA", Object: "Link__c", Fields: map[string]storage.Value{"Name": storage.StringValue("Zulu child"), "Alpha__c": storage.IDValue(parentID)}},
	}}
	original := append([]storage.Relationship(nil), org.Objects["Link__c"].Definition.Relations...)
	server := New(&org)
	wire := func(t *testing.T, operation, config string) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/"+operation, strings.NewReader(config)))
		var payload struct {
			Data  map[string]any `json:"data"`
			Error any            `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil || rec.Code != http.StatusOK || payload.Error != nil {
			t.Fatalf("%s: %d %s (%v)", operation, rec.Code, rec.Body.String(), err)
		}
		return payload.Data
	}
	t.Run("related_lists_info_name_order", func(t *testing.T) {
		payload := wire(t, "getRelatedListsInfo", `{"parentObjectApiName":"Account"}`)
		lists := payload["relatedLists"].([]any)
		var got []string
		for _, entry := range lists {
			row := entry.(map[string]any)
			if row["objectApiName"] != "Link__c" {
				continue
			}
			got = append(got, row["relatedListId"].(string)+"."+row["fieldApiName"].(string))
		}
		if want := []string{"Alpha.Zulu__c", "Zulu.Alpha__c"}; !slices.Equal(got, want) {
			t.Fatalf("related lists: %v, want %v", got, want)
		}
	})
	for _, name := range []string{"Alpha", "Zulu"} {
		t.Run("related_list_child/"+name, func(t *testing.T) {
			wantField, wantID := "Zulu__c", "a00000000000001AAA"
			if name == "Zulu" {
				wantField, wantID = "Alpha__c", "a00000000000002AAA"
			}
			child, _, relation, ok := relatedListChild(&org, "Account", name)
			if !ok || child != "Link__c" || relation.Field != wantField {
				t.Fatalf("lookup: %s %+v %v", child, relation, ok)
			}
			wire(t, "getRelatedListInfo", `{"parentObjectApiName":"Account","relatedListId":"`+name+`"}`)
			config := `{"parentRecordId":"001000000000001AAA","relatedListId":"` + name + `","fields":["Link__c.Name"]}`
			payload := wire(t, "getRelatedListRecords", config)
			records := payload["records"].([]any)
			if len(records) != 1 || records[0].(map[string]any)["id"] != wantID {
				t.Fatalf("wrong lookup selected records: %v", records)
			}
			if count := wire(t, "getRelatedListCount", config)["count"]; count != float64(1) {
				t.Fatalf("related list count: %v", count)
			}
		})
	}
	if !reflect.DeepEqual(original, org.Objects["Link__c"].Definition.Relations) {
		t.Fatal("related-list reads reordered stored relationships")
	}
}

type childRelationshipOrderCases struct {
	NativeAPIVersions []string `json:"nativeAPIVersions"`
	LocalAPIVersions  []string `json:"localAPIVersions"`
	ParentObjects     []string `json:"parentObjects"`
	Resource          string   `json:"resource"`
	ObservedProperty  string   `json:"observedProperty"`
}

type nativeChildRelationshipOrderRow struct {
	APIVersion       string
	ParentObject     string
	Ordinal          int
	ChildSObject     string
	Field            string
	RelationshipName string
}

type childRelationshipOrderKey struct {
	child string
	field string
}

func TestRESTDescribeChildRelationshipOrderConformance(t *testing.T) {
	base := filepath.Join("testdata", "child_relationship_order")
	caseData, err := os.ReadFile(filepath.Join(base, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases childRelationshipOrderCases
	decoder := json.NewDecoder(strings.NewReader(string(caseData)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases.NativeAPIVersions) == 0 || len(cases.LocalAPIVersions) == 0 || len(cases.ParentObjects) == 0 || cases.ObservedProperty != "childRelationships array order" || !strings.Contains(cases.Resource, "{apiVersion}") || !strings.Contains(cases.Resource, "{parentObject}") {
		t.Fatalf("incomplete case table: %#v", cases)
	}
	for _, apiVersion := range cases.LocalAPIVersions {
		if !slices.Contains(cases.NativeAPIVersions, apiVersion) {
			t.Fatalf("local API %s has no native capture", apiVersion)
		}
	}

	rows := readNativeChildRelationshipOrderRows(t, filepath.Join(base, "org.tsv"))
	nextOrdinal := make(map[string]int)
	for _, row := range rows {
		scope := row.APIVersion + "|" + row.ParentObject
		if row.Ordinal != nextOrdinal[scope] {
			t.Fatalf("native row order for API %s %s has ordinal %d, want %d", row.APIVersion, row.ParentObject, row.Ordinal, nextOrdinal[scope])
		}
		nextOrdinal[scope]++
	}
	org := standardChildRelationshipOrderOrg(t, cases.ParentObjects, rows)
	handler := New(&org)

	for _, apiVersion := range cases.NativeAPIVersions {
		for _, parentObject := range cases.ParentObjects {
			found := false
			for _, row := range rows {
				if row.APIVersion == apiVersion && row.ParentObject == parentObject {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("case table requires native rows for API %s %s", apiVersion, parentObject)
			}
		}
	}
	for _, apiVersion := range cases.LocalAPIVersions {
		for _, parentObject := range cases.ParentObjects {
			t.Run(apiVersion+"/"+parentObject, func(t *testing.T) {
				var native []childRelationshipOrderKey
				for _, row := range rows {
					if row.APIVersion == apiVersion && row.ParentObject == parentObject {
						native = append(native, childRelationshipOrderKey{child: row.ChildSObject, field: row.Field})
					}
				}
				if len(native) == 0 {
					t.Fatalf("no native child relationship rows for API %s %s", apiVersion, parentObject)
				}

				path := strings.ReplaceAll(cases.Resource, "{apiVersion}", apiVersion)
				path = strings.ReplaceAll(path, "{parentObject}", parentObject)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
				if rec.Code != http.StatusOK {
					t.Fatalf("REST describe status = %d body=%s", rec.Code, rec.Body.String())
				}
				var payload struct {
					ChildRelationships []struct {
						ChildSObject string `json:"childSObject"`
						Field        string `json:"field"`
					} `json:"childRelationships"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}

				nativeSet := make(map[childRelationshipOrderKey]bool, len(native))
				for _, key := range native {
					nativeSet[key] = true
				}
				var localShared []childRelationshipOrderKey
				for _, relationship := range payload.ChildRelationships {
					key := childRelationshipOrderKey{child: relationship.ChildSObject, field: relationship.Field}
					if nativeSet[key] {
						localShared = append(localShared, key)
					}
				}
				localSet := make(map[childRelationshipOrderKey]bool, len(localShared))
				for _, key := range localShared {
					localSet[key] = true
				}
				var nativeShared []childRelationshipOrderKey
				for _, key := range native {
					if localSet[key] {
						nativeShared = append(nativeShared, key)
					}
				}
				if len(nativeShared) == 0 {
					t.Fatalf("no shared native relationships for API %s %s", apiVersion, parentObject)
				}
				if !slices.Equal(localShared, nativeShared) {
					limit := min(len(localShared), len(nativeShared))
					for index := range limit {
						if localShared[index] != nativeShared[index] {
							got, want := localShared[index], nativeShared[index]
							t.Fatalf("child relationship order differs at shared index %d: Salesforce has %s.%s, Glade has %s.%s (%d shared rows)", index, want.child, want.field, got.child, got.field, len(nativeShared))
						}
					}
					t.Fatalf("child relationship order has %d Glade rows and %d Salesforce rows", len(localShared), len(nativeShared))
				}

				if apiVersion == "67.0" && parentObject == "Contact" {
					want := []childRelationshipOrderKey{{child: "CampaignMember", field: "ContactId"}, {child: "CampaignMember", field: "LeadOrContactId"}}
					if !containsChildRelationshipOrderKeys(native, want) || !containsChildRelationshipOrderKeys(localShared, want) {
						t.Fatalf("native and local Contact rows must include the lookup-order reproducer %v", want)
					}
				}
			})
		}
	}
}

func readNativeChildRelationshipOrderRows(t *testing.T, path string) []nativeChildRelationshipOrderRow {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = 9
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) < 2 || strings.Join(records[0], ",") != "api_version,parent_object,ordinal,child_sobject,field,relationship_name,cascade_delete,restricted_delete,deprecated_and_hidden" {
		t.Fatalf("unexpected native table header or no data rows: %#v", records[:min(len(records), 1)])
	}
	rows := make([]nativeChildRelationshipOrderRow, 0, len(records)-1)
	for line, record := range records[1:] {
		ordinal, err := strconv.Atoi(record[2])
		if err != nil {
			t.Fatalf("native row %d ordinal: %v", line+2, err)
		}
		rows = append(rows, nativeChildRelationshipOrderRow{
			APIVersion:       record[0],
			ParentObject:     record[1],
			Ordinal:          ordinal,
			ChildSObject:     record[3],
			Field:            record[4],
			RelationshipName: record[5],
		})
	}
	return rows
}

func standardChildRelationshipOrderOrg(t *testing.T, parentObjects []string, rows []nativeChildRelationshipOrderRow) storage.OrgState {
	t.Helper()
	org := storage.NewOrgState()
	objectNames := make(map[string]bool)
	for _, name := range parentObjects {
		objectNames[name] = true
	}
	for _, row := range rows {
		objectNames[row.ChildSObject] = true
	}
	for name := range objectNames {
		definition, ok := storage.StandardObjectDefinition(name)
		if !ok {
			continue
		}
		org.Objects[definition.APIName] = storage.ObjectState{
			Definition: definition,
			Records:    make(map[storage.ID]storage.Record),
		}
	}
	for _, name := range parentObjects {
		if _, ok := org.Objects[name]; !ok {
			t.Fatalf("standard object definition is unavailable for %s", name)
		}
	}
	return org
}

func containsChildRelationshipOrderKeys(haystack, needles []childRelationshipOrderKey) bool {
	set := make(map[childRelationshipOrderKey]bool, len(haystack))
	for _, key := range haystack {
		set[key] = true
	}
	for _, needle := range needles {
		if !set[needle] {
			return false
		}
	}
	return true
}
