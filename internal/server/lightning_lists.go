package server

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/storage"
)

// List reads share the same transport, but preserve their distinct native
// resource errors and batch envelopes. No fixture identities enter this path.
// Captured resource URLs use the latest platform API, including floor callers.
func latestListRESTAPIVersion() string {
	latest, major := storage.DefaultRESTAPIVersion, 0
	for _, version := range storage.SupportedRESTAPIVersions {
		candidate, err := strconv.Atoi(strings.SplitN(version, ".", 2)[0])
		if err == nil && candidate > major {
			latest, major = version, candidate
		}
	}
	return latest
}

func (s *Server) handleLightningListRead(w http.ResponseWriter, r *http.Request, operation string) {
	var config map[string]any
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&config); err != nil {
		writeLDSRecordResponse(w, nil, &lwcbrowser.WireError{Message: "invalid list wire request"})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := s.listReadData(operation, config)
	writeListReadResponse(w, data, wireErr)
}

func (s *Server) handleLightningListDelete(w http.ResponseWriter, r *http.Request) {
	var config map[string]any
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&config); err != nil {
		writeListReadResponse(w, nil, &lwcbrowser.WireError{Message: "invalid list deletion request"})
		return
	}
	writeListReadResponse(w, nil, s.listDeleteData(config))
}

func (s *Server) listDeleteData(config map[string]any) *lwcbrowser.WireError {
	objectName, _, ok := findOrgObject(s.Org, listConfigString(config, "objectApiName"))
	if !ok {
		return &lwcbrowser.WireError{Type: "INSUFFICIENT_ACCESS", Status: 403, RecordErrorID: "777159533", Message: "You don't have access to this record. Ask your administrator for help or to request access."}
	}
	view, ok := s.findListView(objectName, listConfigString(config, "listViewApiName"))
	if !ok {
		return &lwcbrowser.WireError{Type: "NOT_FOUND", Status: 404, RecordErrorID: "732739174", Message: "The requested resource does not exist"}
	}
	if view.FileName != "" {
		if err := os.Remove(view.FileName); err != nil && !os.IsNotExist(err) {
			return &lwcbrowser.WireError{Status: 500, Message: "unable to delete list view metadata"}
		}
	}
	remaining := make([]listViewMetadata, 0, len(s.Source.ListViews[objectName])-1)
	for _, candidate := range s.Source.ListViews[objectName] {
		if candidate.DeveloperName != view.DeveloperName {
			remaining = append(remaining, candidate)
		}
	}
	s.Source.ListViews[objectName] = remaining
	paths := make([]string, 0, len(s.Source.Project.ListViewFiles))
	for _, path := range s.Source.Project.ListViewFiles {
		if path != view.FileName {
			paths = append(paths, path)
		}
	}
	s.Source.Project.ListViewFiles = paths
	components := make([]metadataComponent, 0, len(s.Source.Components))
	for _, component := range s.Source.Components {
		if component.Type != "ListView" || component.FullName != objectName+"."+view.DeveloperName {
			components = append(components, component)
		}
	}
	s.Source.Components = components
	if tooling, ok := s.Source.ToolingOrg.Objects["ListView"]; ok {
		delete(tooling.Records, storage.ID(view.ID))
	}
	s.Source.sortAndIndexComponents()
	return nil
}

func (s *Server) handleLightningListWrite(w http.ResponseWriter, r *http.Request, operation string) {
	var config map[string]any
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&config); err != nil {
		writeListReadResponse(w, nil, &lwcbrowser.WireError{Message: "invalid list mutation request"})
		return
	}
	data, err := s.listWriteData(operation, config)
	writeListReadResponse(w, data, err)
}

func (s *Server) listWriteData(operation string, config map[string]any) (any, *lwcbrowser.WireError) {
	objectName, object, ok := findOrgObject(s.Org, listConfigString(config, "objectApiName"))
	if !ok {
		id := "1356115655"
		if operation == "createListInfo" {
			id = "-959475431"
		}
		return nil, &lwcbrowser.WireError{Type: "INSUFFICIENT_ACCESS", Status: 403, RecordErrorID: id, Message: "You don't have access to this record. Ask your administrator for help or to request access."}
	}
	name := listConfigString(config, "listViewApiName")
	view, exists := s.findListView(objectName, name)
	if operation != "createListInfo" && !exists {
		return nil, &lwcbrowser.WireError{Type: "NOT_FOUND", Status: 404, RecordErrorID: "-1892118122", Message: "The requested resource does not exist"}
	}
	if operation == "updateListPreferences" {
		preferences := listPreferencesData(view, canonicalListColumns(object.Definition, s.Org.Namespace, view.Columns))
		for _, key := range []string{"columnWidths", "columnWrap", "orderedBy"} {
			if value, present := config[key]; present {
				if key == "columnWrap" {
					// Native preference writes ignore unknown columns and retain
					// the current defaults for columns omitted from the request.
					current, currentMap := preferences[key].(map[string]any)
					if updates, ok := value.(map[string]any); ok && currentMap {
						merged := cloneListConfig(current)
						for column, wrap := range updates {
							if _, selected := current[column]; selected {
								merged[column] = wrap
							}
						}
						value = merged
					}
				}
				preferences[key] = value
			}
		}
		view.Preferences = preferences
		for index, existing := range s.Source.ListViews[objectName] {
			if existing.ID == view.ID {
				s.Source.ListViews[objectName][index] = view
			}
		}
		return preferences, nil
	}
	creating := operation == "createListInfo"
	label, hasLabel := config["label"].(string)
	if creating && !hasLabel {
		label = name
	}
	if creating {
		errors := []any{}
		for _, input := range []struct{ name, value string }{{"label", label}, {"developerName", name}} {
			if len([]rune(input.value)) > 40 {
				errors = append(errors, map[string]any{"fieldApiName": input.name, "errorMessage": fmt.Sprintf("The value %q for the %s field exceeds the 40 character limit.", input.value, input.name)})
			}
		}
		if len(errors) > 0 {
			return nil, &lwcbrowser.WireError{Status: 400, RecordErrorID: "-1047347708", RecordBody: map[string]any{"message": "Something's not right with your input parameters. See the errors below and try again.", "statusCode": 400, "output": map[string]any{"fieldErrors": errors}}}
		}
		// Identity becomes a selected metadata filename, never a relative path.
		if !listDeveloperNamePattern.MatchString(name) || exists {
			return nil, &lwcbrowser.WireError{Status: 400, Message: "invalid or existing list view identity"}
		}
		directory := ""
		for _, path := range s.Source.Project.ObjectFiles {
			if filepath.Base(path) == objectName+".object-meta.xml" {
				directory = filepath.Join(filepath.Dir(path), "listViews")
				break
			}
		}
		if directory == "" {
			return nil, &lwcbrowser.WireError{Status: 400, Message: "list view object has no source metadata"}
		}
		used := map[string]bool{}
		for _, views := range s.Source.ListViews {
			for _, existing := range views {
				used[existing.ID] = true
			}
		}
		ordinal := 1
		for used[string(sequenceID("00B", ordinal))] {
			ordinal++
		}
		view = listViewMetadata{ID: string(sequenceID("00B", ordinal)), ObjectName: objectName, DeveloperName: name, Label: label, FilterScope: "Mine", FileName: filepath.Join(directory, name+".listView-meta.xml")}
	} else if hasLabel {
		view.Label = label
	}
	if _, present := config["displayColumns"]; present {
		view.Columns = canonicalListColumns(object.Definition, s.Org.Namespace, listStrings(config["displayColumns"]))
	}
	if visibility, ok := config["visibility"].(string); ok {
		view.Visibility = visibility
	}
	if view.FileName == "" {
		return nil, &lwcbrowser.WireError{Status: 400, Message: "list view has no source metadata"}
	}
	if err := persistListView(view); err != nil {
		return nil, &lwcbrowser.WireError{Status: 500, Message: "unable to save list view metadata"}
	}
	if creating {
		if s.Source.ListViews == nil {
			s.Source.ListViews = map[string][]listViewMetadata{}
		}
		s.Source.ListViews[objectName] = append(s.Source.ListViews[objectName], view)
		s.Source.Project.ListViewFiles = append(s.Source.Project.ListViewFiles, view.FileName)
		s.Source.Components = append(s.Source.Components, metadataComponent{Type: "ListView", FullName: objectName + "." + name, FileName: view.FileName, ID: storage.ID(view.ID)})
	} else {
		for index, existing := range s.Source.ListViews[objectName] {
			if existing.ID == view.ID {
				s.Source.ListViews[objectName][index] = view
			}
		}
	}
	s.Source.sortAndIndexComponents()
	return s.listInfoData(view, object.Definition, canonicalListColumns(object.Definition, s.Org.Namespace, view.Columns)), nil
}

var listDeveloperNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

func persistListView(view listViewMetadata) error {
	var data []byte
	original, err := os.ReadFile(view.FileName)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		data, err = updateListViewXML(original, view)
		if err != nil {
			return err
		}
	} else {
		raw := struct {
			XMLName   xml.Name `xml:"ListView"`
			Namespace string   `xml:"xmlns,attr"`
			FullName  string   `xml:"fullName"`
			listViewXML
		}{Namespace: "http://soap.sforce.com/2006/04/metadata", FullName: view.DeveloperName, listViewXML: listViewXML{Label: view.Label, Columns: view.Columns, FilterScope: view.FilterScope, Filters: view.Filters}}
		data, err = xml.MarshalIndent(raw, "", "    ")
		if err != nil {
			return err
		}
		data = append([]byte(xml.Header), data...)
	}
	if err := os.MkdirAll(filepath.Dir(view.FileName), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(view.FileName), ".list-view-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0644); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), view.FileName)
}

// Replace only the two mutation-selected sections. Unknown metadata, sharing,
// filters and scope survive an update; a lossy struct round-trip would drop them.
func updateListViewXML(original []byte, view listViewMetadata) ([]byte, error) {
	decoder := xml.NewDecoder(bytes.NewReader(original))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	depth := 0
	written := map[string]bool{}
	var namespace string
	emit := func(name string) error {
		if written[name] {
			return nil
		}
		written[name] = true
		values := view.Columns
		if name == "label" {
			values = []string{view.Label}
		}
		for _, value := range values {
			if err := encoder.EncodeElement(value, xml.StartElement{Name: xml.Name{Space: namespace, Local: name}}); err != nil {
				return err
			}
		}
		return nil
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				namespace = value.Name.Space
			}
			if depth == 1 && (value.Name.Local == "columns" || value.Name.Local == "label") {
				if err := decoder.Skip(); err != nil {
					return nil, err
				}
				if err := emit(value.Name.Local); err != nil {
					return nil, err
				}
				continue
			}
			// Encoder declares resolved namespaces; copying decoder namespace
			// declarations would duplicate the default xmlns attribute.
			attributes := []xml.Attr{}
			for _, attribute := range value.Attr {
				if attribute.Name.Local != "xmlns" && attribute.Name.Space != "xmlns" {
					attributes = append(attributes, attribute)
				}
			}
			value.Attr = attributes
			token = value
			depth++
		case xml.EndElement:
			if depth == 1 {
				for _, name := range []string{"columns", "label"} {
					if err := emit(name); err != nil {
						return nil, err
					}
				}
			}
			depth--
		}
		if err := encoder.EncodeToken(token); err != nil {
			return nil, err
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func writeListReadResponse(w http.ResponseWriter, data any, err *lwcbrowser.WireError) {
	var responseError any
	if err != nil {
		status := err.Status
		if status == 0 {
			status = 400
		}
		body := err.RecordBody
		if body == nil {
			body = map[string]any{"errorCode": err.Type, "message": err.Message, "statusCode": status}
		}
		if err.RecordErrorID != "" {
			body["id"] = err.RecordErrorID
		}
		text := http.StatusText(status)
		if status == 403 {
			text = "Unexpected HTTP Status Code: 403"
		}
		if status == 500 {
			text = "Server Error"
		}
		responseError = map[string]any{"body": body, "errorType": "fetchResponse", "headers": map[string]any{}, "ok": false, "status": status, "statusText": text}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "error": responseError})
}

func listResourceError(operation string) *lwcbrowser.WireError {
	id := "926117961"
	if operation == "getListRecordsByName" {
		id = "-1910126468"
	}
	if operation == "getListPreferences" {
		id = "1109617167"
	}
	if operation == "getListUi" {
		id = "564804945"
	}
	return &lwcbrowser.WireError{Type: "NOT_FOUND", Status: 404, RecordErrorID: id, Message: "The requested resource does not exist"}
}

func listConfigString(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return value
}
func listStrings(value any) []string {
	out := []string{}
	if items, ok := value.([]string); ok {
		out = append(out, items...)
	} else if items, ok := value.([]any); ok {
		for _, item := range items {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
	} else if text, ok := value.(string); ok {
		out = append(out, text)
	}
	return out
}
func listNumber(config map[string]any, key string, fallback int) int {
	if number, ok := config[key].(float64); ok {
		return int(number)
	}
	return fallback
}
func listReference(view listViewMetadata) map[string]any {
	return map[string]any{"id": view.ID, "objectApiName": view.ObjectName, "listViewApiName": view.DeveloperName, "type": "listView"}
}
func relatedListReference(parentObject, relationship string, parentID any) map[string]any {
	return map[string]any{"id": nil, "inContextOfRecordId": parentID, "listViewApiName": nil, "objectApiName": nil, "parentObjectApiName": parentObject, "recordTypeId": nil, "relatedListId": relationship, "type": "relatedList"}
}
func columnPreferences(columns []string) map[string]any {
	widths, wrap := map[string]any{}, map[string]any{}
	for _, name := range columns {
		widths[name] = -1
		wrap[name] = false
	}
	return map[string]any{"columnWidths": widths, "columnWrap": wrap}
}
func listPreferencesData(view listViewMetadata, columns []string) map[string]any {
	data := columnPreferences(columns)
	data["orderedBy"] = []any{}
	for key, value := range view.Preferences {
		data[key] = value
	}
	data["listReference"] = listReference(view)
	return data
}
func (s *Server) listReadData(operation string, config map[string]any) (any, *lwcbrowser.WireError) {
	switch operation {
	case "getRelatedListRecords":
		return s.relatedListRecordsData(config)
	case "getRelatedListCount":
		req := lwcbrowser.WireGetRelatedListRecordsRequest{ParentRecordID: listConfigString(config, "parentRecordId"), RelatedListID: listConfigString(config, "relatedListId")}
		records, err := getRelatedListRecordsWireData(s.Org, req)
		if err != nil {
			return nil, err
		}
		maximum := listNumber(config, "maxCount", 1999)
		if maximum < 1 || maximum > 1999 {
			return nil, &lwcbrowser.WireError{Type: "NUMBER_OUTSIDE_VALID_RANGE", Status: 400, RecordErrorID: "242608947", Message: "maxSize parameter must be between 1 and 1999"}
		}
		count := records["count"].(int)
		parent, _, _ := findOrgRecord(s.Org, req.ParentRecordID)
		hasMore := count > maximum
		if hasMore {
			count = maximum
		}
		return map[string]any{"count": count, "hasMore": hasMore, "listReference": relatedListReference(parent, req.RelatedListID, req.ParentRecordID)}, nil
	case "getRelatedListInfo":
		return s.relatedListInfoData(config)
	case "getRelatedListsInfo":
		return s.relatedListsInfoData(config)
	case "getRelatedListInfoBatch", "getRelatedListRecordsBatch", "getListInfosByName":
		results := []any{}
		var inputs []map[string]any
		if operation == "getRelatedListInfoBatch" {
			for _, name := range listStrings(config["relatedListNames"]) {
				input := cloneListConfig(config)
				input["relatedListId"] = name
				inputs = append(inputs, input)
			}
		} else if operation == "getRelatedListRecordsBatch" {
			if items, ok := config["relatedListParameters"].([]any); ok {
				for _, item := range items {
					if input, ok := item.(map[string]any); ok {
						input = cloneListConfig(input)
						input["parentRecordId"] = config["parentRecordId"]
						inputs = append(inputs, input)
					}
				}
			}
		} else {
			for _, name := range listStrings(config["names"]) {
				object, view, _ := strings.Cut(name, ".")
				inputs = append(inputs, map[string]any{"objectApiName": object, "listViewApiName": view})
			}
		}
		for _, input := range inputs {
			var data any
			var err *lwcbrowser.WireError
			switch operation {
			case "getRelatedListInfoBatch":
				data, err = s.relatedListInfoData(input)
			case "getRelatedListRecordsBatch":
				data, err = s.relatedListRecordsData(input)
			default:
				data, err = s.listReadData("getListInfoByName", input)
			}
			status := 200
			if err != nil {
				status = err.Status
				if status == 0 {
					status = 400
				}
				code, message := err.Type, err.Message
				if operation == "getListInfosByName" {
					code, message = "NOT_FOUND", "Resource not found."
				}
				data = []any{map[string]any{"errorCode": code, "message": message}}
			}
			// The native batch list-info projection omits the single-read object list.
			if operation == "getListInfosByName" && err == nil {
				data.(map[string]any)["objectApiNames"] = []string{}
			}
			results = append(results, map[string]any{"result": data, "statusCode": status})
		}
		return map[string]any{"results": results}, nil
	}
	objectName := listConfigString(config, "objectApiName")
	if operation == "getListInfosByObjectName" && objectName == "" {
		return nil, &lwcbrowser.WireError{Type: "ILLEGAL_QUERY_PARAMETER_VALUE", Status: 400, RecordErrorID: "-174780500", Message: "entityApiName should be not null or empty"}
	}
	objectName, object, ok := findOrgObject(s.Org, objectName)
	if !ok {
		if operation == "getListInfosByObjectName" {
			return nil, &lwcbrowser.WireError{Type: "INSUFFICIENT_ACCESS", Status: 403, RecordErrorID: "-630308365", Message: "You don't have access to this record. Ask your administrator for help or to request access."}
		}
		return nil, listResourceError(operation)
	}
	if operation == "getListInfosByObjectName" || operation == "getListUi" && listConfigString(config, "listViewApiName") == "" {
		return s.listCollectionData(operation, objectName, config)
	}
	view, ok := s.findListView(objectName, listConfigString(config, "listViewApiName"))
	if !ok {
		return nil, listResourceError(operation)
	}
	columns := canonicalListColumns(object.Definition, s.Org.Namespace, view.Columns)
	switch operation {
	case "getListPreferences":
		return listPreferencesData(view, columns), nil
	case "getListInfoByName":
		return s.listInfoData(view, object.Definition, columns), nil
	case "getListRecordsByName":
		return s.listRecordsData(view, object, config)
	case "getListUi":
		pageSize := listNumber(config, "pageSize", 50)
		if pageSize < 1 || pageSize > 2000 {
			return nil, &lwcbrowser.WireError{Type: "NUMBER_OUTSIDE_VALID_RANGE", Status: 400, RecordErrorID: "572175235", Message: "pageSize parameter must be between 1 and 2000"}
		}
		if token := listConfigString(config, "pageToken"); token != "" {
			if _, err := strconv.Atoi(token); err != nil {
				return nil, &lwcbrowser.WireError{Type: "ILLEGAL_QUERY_PARAMETER_VALUE", Status: 400, RecordErrorID: "-1318348436", Message: fmt.Sprintf("For input string: %q", token)}
			}
		}
		selection := cloneListConfig(config)
		fields := append([]string{}, columns...)
		fields = append(fields, "Id", "CreatedDate", "LastModifiedDate", "LastModifiedById", "SystemModstamp")
		for index, name := range fields {
			fields[index] = objectName + "." + name
		}
		selection["fields"] = fields
		records, err := s.listRecordsData(view, object, selection)
		if err != nil {
			return nil, err
		}
		records["fields"] = []string{}
		records["listInfoETag"] = view.ID
		return map[string]any{"info": s.listInfoData(view, object.Definition, columns), "records": records}, nil
	}
	return nil, &lwcbrowser.WireError{Message: "unsupported list read"}
}

func (s *Server) listCollectionData(operation, objectName string, config map[string]any) (any, *lwcbrowser.WireError) {
	pageSize := listNumber(config, "pageSize", 20)
	if pageSize < 1 || pageSize > 2000 {
		id := "-2103616905"
		if operation == "getListUi" {
			id = "572175235"
		}
		return nil, &lwcbrowser.WireError{Type: "NUMBER_OUTSIDE_VALID_RANGE", Status: 400, RecordErrorID: id, Message: "pageSize parameter must be between 1 and 2000"}
	}
	offset := 0
	if token := listConfigString(config, "pageToken"); token != "" {
		var err error
		offset, err = strconv.Atoi(token)
		if err != nil || offset < 0 {
			return nil, &lwcbrowser.WireError{Message: "invalid list collection page token", Status: 400}
		}
	}
	requestedObject := listConfigString(config, "objectApiName")
	version := latestListRESTAPIVersion()
	lists := []any{}
	query := strings.ToLower(listConfigString(config, "q"))
	for _, view := range s.Source.ListViews[objectName] {
		// Source metadata has no per-user recently used list state. The captured
		// untouched collection likewise has no recently used entries.
		if config["recentListsOnly"] == true || !strings.Contains(strings.ToLower(view.Label), query) {
			continue
		}
		urlKey, resource := "url", "list-info"
		if operation == "getListUi" {
			urlKey, resource = "listUiUrl", "list-ui"
		}
		lists = append(lists, map[string]any{"apiName": view.DeveloperName, "id": view.ID, "label": view.Label, urlKey: fmt.Sprintf("/services/data/v%s/ui-api/%s/%s/%s", version, resource, requestedObject, view.DeveloperName)})
	}
	offset = min(offset, len(lists))
	end := min(offset+pageSize, len(lists))
	var next, previous any
	if end < len(lists) {
		next = strconv.Itoa(end)
	}
	if offset > 0 {
		previous = strconv.Itoa(max(0, offset-pageSize))
	}
	lists = lists[offset:end]
	return map[string]any{"count": len(lists), "lists": lists, "objectApiName": requestedObject, "currentPageToken": strconv.Itoa(offset), "nextPageToken": next, "previousPageToken": previous, "pageSize": pageSize, "queryString": config["q"], "recentListsOnly": config["recentListsOnly"] == true}, nil
}

// Related lists are incoming references declared on child objects. Reuse the
// product describe traversal rather than interpreting a parent's own lookups
// as child relationships. Keep support for older preassembled parent metadata.
func relatedListChild(org *storage.OrgState, parentName, relationship string) (string, storage.ObjectState, storage.Relationship, bool) {
	for _, descriptor := range describeChildRelationships(parentName, org) {
		name, _ := descriptor["relationshipName"].(string)
		if name == "" || !strings.EqualFold(name, relationship) {
			continue
		}
		childName, _ := descriptor["childSObject"].(string)
		field, _ := descriptor["field"].(string)
		child := org.Objects[childName]
		lookup, ok := child.Definition.Fields[field]
		if !ok || lookup.Type != storage.FieldReference ||
			!relationshipTargetsObject(storage.Relationship{ParentObjects: lookup.ReferenceTo}, parentName) {
			continue
		}
		return childName, child, storage.Relationship{Field: field, ChildRelationship: name, ParentObjects: []string{parentName}}, true
	}
	if parent, ok := org.Objects[parentName]; ok {
		for _, relation := range parent.Definition.Relations {
			if !strings.EqualFold(relation.ChildRelationship, relationship) || relation.Field == "" {
				continue
			}
			if name, child, ok := findChildObjectForRelationship(org, parentName, relation); ok {
				return name, child, relation, true
			}
		}
	}
	return "", storage.ObjectState{}, storage.Relationship{}, false
}

func cloneListConfig(config map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range config {
		out[key] = value
	}
	return out
}
func canonicalListColumns(def storage.ObjectDefinition, namespace string, columns []string) []string {
	out := []string{}
	for _, name := range columns {
		if canonical, ok := storage.ResolveFieldName(def, namespace, name); ok {
			out = append(out, canonical)
		}
	}
	return out
}
func (s *Server) listInfoData(view listViewMetadata, def storage.ObjectDefinition, names []string) map[string]any {
	columns, filters := []any{}, []any{}
	for _, name := range names {
		field := def.Fields[name]
		var lookup any
		if name == "Name" {
			lookup = "Id"
		}
		columns = append(columns, map[string]any{"fieldApiName": name, "label": labelOrFallback(field.Label, name), "lookupId": lookup, "searchable": name == "Name", "sortable": storage.FieldFlagValue(field.Sortable, true), "inlineEditAttributes": map[string]any{"012000000000000AAA": s.listInlineEditAttributes(def, name)}})
	}
	for _, filter := range view.Filters {
		name, ok := storage.ResolveFieldName(def, "", filter.Field)
		if !ok {
			continue
		}
		operator := filter.Operation
		if len(operator) > 0 {
			operator = strings.ToUpper(operator[:1]) + operator[1:]
		}
		filters = append(filters, map[string]any{"fieldApiName": name, "label": labelOrFallback(def.Fields[name].Label, name), "operandLabels": []string{strings.TrimSpace(filter.Value)}, "operator": operator})
	}
	visibility := view.Visibility
	if visibility == "" {
		visibility = "Public"
	}
	scopeLabel := "All " + strings.ToLower(def.PluralLabel)
	if view.FilterScope == "Mine" {
		scopeLabel = "My " + strings.ToLower(def.PluralLabel)
	}
	preferences := listPreferencesData(view, names)
	delete(preferences, "listReference")
	delete(preferences, "orderedBy")
	return map[string]any{"cloneable": true, "createable": true, "deletable": true, "displayColumns": columns, "filterLogicString": nil, "filteredByInfo": filters, "hasMassActions": true, "inlineEditDetails": map[string]any{"message": nil, "state": "Enabled"}, "label": view.Label, "listReference": listReference(view), "listShares": []any{}, "objectApiNames": []string{view.ObjectName}, "orderedByInfo": []any{map[string]any{"fieldApiName": "Name", "isAscending": true, "label": labelOrFallback(def.Fields["Name"].Label, "Name")}}, "scope": map[string]any{"apiName": strings.ToLower(view.FilterScope), "entity": nil, "label": scopeLabel, "relatedEntity": nil}, "searchable": false, "updateable": true, "userPreferences": preferences, "visibility": visibility, "visibilityEditable": true}
}

// Captured Full Edit controls distinguish field permissions from membership
// and behavior on the edit layout. Keep the existing fallback when no layout
// metadata is available; do not infer a layout from field names or types.
func (s *Server) listInlineEditAttributes(def storage.ObjectDefinition, name string) map[string]any {
	field := def.Fields[name]
	layout, ok := sourceCreateLayout(s.Source, def.APIName)
	if !ok {
		return map[string]any{"editable": fieldUpdateable(field), "required": field.Required}
	}
	for _, section := range layout.Sections {
		for _, column := range section.Columns {
			for _, item := range column.Items {
				canonical, ok := storage.ResolveFieldName(def, s.Org.Namespace, item.Field)
				if ok && canonical == name {
					required, _, editable, _ := recordLayoutItemBehavior(field, item.Behavior)
					return map[string]any{"editable": editable, "required": required}
				}
			}
		}
	}
	return map[string]any{"editable": false, "required": field.Required}
}

func (s *Server) relatedListInfoData(config map[string]any) (any, *lwcbrowser.WireError) {
	parentName, _, ok := findOrgObject(s.Org, listConfigString(config, "parentObjectApiName"))
	if !ok {
		id := "-1365054019"
		// Native controls cover every version. This opaque resource ID changes
		// non-monotonically; it is not a minimum-version admission gate.
		switch sourceAPIVersion(s.Source.Project.SourceAPIVersion) {
		case "59.0", "63.0", "64.0":
			id = "2121206592"
		}
		return nil, &lwcbrowser.WireError{Type: "INSUFFICIENT_ACCESS", Status: 403, RecordErrorID: id, Message: "You don't have access to this record. Ask your administrator for help or to request access."}
	}
	relationship := listConfigString(config, "relatedListId")
	childName, child, relation, ok := relatedListChild(s.Org, parentName, relationship)
	if !ok || relation.ChildRelationship == "" {
		return nil, &lwcbrowser.WireError{Type: "INVALID_TYPE", Status: 400, RecordErrorID: "-1287131655", Message: "The related lists UI API does not currently support this entity"}
	}
	layoutColumns := s.relatedLayoutColumns(parentName, childName, relation.Field)
	names := canonicalListColumns(child.Definition, s.Org.Namespace, layoutColumns)
	restrict := config["restrictColumnsToLayout"] != false
	if !restrict {
		// UI list columns expand the audit/owner relationships, and omit the
		// context lookup and internal system selectors from the business list.
		names = []string{"Id", "Name"}
		business := []string{}
		for name := range child.Definition.Fields {
			if name == relation.Field || relatedListSystemSelector(name) {
				continue
			}
			business = append(business, name)
		}
		sort.Strings(business)
		names = append(names, business...)
		for _, group := range []struct {
			field   string
			members []string
		}{
			{"OwnerId", []string{"NameOrAlias", "FirstName", "LastName"}},
			{"CreatedById", []string{"Alias", "Name"}},
			{"CreatedDate", nil},
			{"LastModifiedById", []string{"Alias", "Name"}},
			{"LastModifiedDate", nil},
		} {
			field, exists := child.Definition.Fields[group.field]
			if !exists {
				continue
			}
			if group.members == nil {
				names = append(names, group.field)
			} else if field.RelationshipName != "" {
				for _, member := range group.members {
					names = append(names, field.RelationshipName+"."+member)
				}
			}
		}
	}
	fields, optional := listStrings(config["fields"]), listStrings(config["optionalFields"])
	if len(fields) > 0 {
		names = canonicalListColumns(child.Definition, s.Org.Namespace, append(fieldsToListColumns(fields), fieldsToListColumns(optional)...))
	}
	columns := []any{}
	objects := []string{childName}
	for _, name := range names {
		field := child.Definition.Fields[name]
		var lookup any
		if relationship, member, nested := strings.Cut(name, "."); nested {
			for _, reference := range child.Definition.Fields {
				if reference.RelationshipName != relationship {
					continue
				}
				for _, target := range reference.ReferenceTo {
					if target == "User" {
						objects = appendUniqueFieldNames(objects, target)
					}
				}
				label := strings.TrimSuffix(reference.Label, " ID")
				suffix := member
				if member == "NameOrAlias" {
					suffix = "Alias"
				} else if member == "FirstName" {
					suffix = "First Name"
				} else if member == "LastName" {
					suffix = "Last Name"
				}
				if member != "Name" {
					label += " " + suffix
				}
				field = storage.Field{APIName: name, Type: storage.FieldString, Label: label}
				lookup = relationship + ".Id"
				break
			}
		}
		dataType, quick := strings.ToLower(string(field.Type)), any(nil)
		if field.Type == storage.FieldString {
			quick = "contains"
		}
		if field.Type == storage.FieldDecimal {
			dataType, quick = "double", "within"
		}
		if field.Type == storage.FieldDateTime {
			quick = "within"
		}
		if name == "Name" {
			lookup = "Id"
		}
		columns = append(columns, map[string]any{"dataType": dataType, "fieldApiName": name, "filterable": storage.FieldFlagValue(field.Filterable, field.Type != storage.FieldID), "label": labelOrFallback(field.Label, name), "lookupId": lookup, "picklistValues": []any{}, "quickFilterOperator": quick, "sortable": storage.FieldFlagValue(field.Sortable, true)})
	}
	// A child relationship API name is not its display label. The lookup's
	// source metadata supplies the captured related-list label.
	label := relationLabel(s.Source.Project.FieldFiles, childName, relation.Field, relationship)
	return map[string]any{"cloneable": false, "createable": false, "deletable": false, "displayColumns": columns, "fieldApiName": relation.Field, "fields": fields, "filterLogicString": "", "filterable": true, "filteredByInfo": []any{}, "label": label, "listReference": relatedListReference(parentName, relationship, nil), "objectApiNames": objects, "optionalFields": optional, "orderedByInfo": []any{}, "restrictColumnsToLayout": restrict, "updateable": false, "userPreferences": columnPreferences(canonicalListColumns(child.Definition, s.Org.Namespace, layoutColumns)), "visibility": "Public", "visibilityEditable": false}, nil
}

func relatedListSystemSelector(name string) bool {
	switch name {
	case "Id", "Name", "OwnerId", "CreatedById", "CreatedDate", "LastModifiedById", "LastModifiedDate", "SystemModstamp", "IsDeleted", "RecordTypeId",
		"LastActivityDate", "LastReferencedDate", "LastViewedDate":
		return true
	}
	return false
}
func fieldsToListColumns(fields []string) []string {
	out := []string{}
	for _, field := range fields {
		_, name, found := strings.Cut(field, ".")
		if found {
			out = append(out, name)
		} else {
			out = append(out, field)
		}
	}
	return out
}
func relationLabel(paths []string, objectName, fieldName, fallback string) string {
	for _, path := range paths {
		if !strings.Contains(path, "/objects/"+objectName+"/fields/"+fieldName+".field-meta.xml") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var raw struct {
			Label string `xml:"relationshipLabel"`
		}
		if xml.Unmarshal(data, &raw) == nil && raw.Label != "" {
			return raw.Label
		}
	}
	return fallback
}
func (s *Server) relatedLayoutColumns(parent, child, field string) []string {
	for _, layout := range s.Source.Layouts[parent] {
		data, err := os.ReadFile(layout.FileName)
		if err != nil {
			continue
		}
		var raw struct {
			Lists []struct {
				Fields      []string `xml:"fields"`
				RelatedList string   `xml:"relatedList"`
			} `xml:"relatedLists"`
		}
		if xml.Unmarshal(data, &raw) != nil {
			continue
		}
		for _, list := range raw.Lists {
			if list.RelatedList == child+"."+field {
				return list.Fields
			}
		}
	}
	return []string{}
}
func (s *Server) relatedListsInfoData(config map[string]any) (any, *lwcbrowser.WireError) {
	if recordType := listConfigString(config, "recordTypeId"); recordType != "" && len(recordType) != 15 && len(recordType) != 18 {
		return nil, &lwcbrowser.WireError{Status: 500, RecordBody: map[string]any{"error": fmt.Sprintf("id %s must be 15 characters", recordType)}}
	}
	parentName, _, ok := findOrgObject(s.Org, listConfigString(config, "parentObjectApiName"))
	if !ok {
		return nil, &lwcbrowser.WireError{Type: "INSUFFICIENT_ACCESS", Status: 403, RecordErrorID: "1679941703", Message: "You don't have access to this record. Ask your administrator for help or to request access."}
	}
	version := latestListRESTAPIVersion()
	lists := []any{}
	for _, descriptor := range describeChildRelationships(parentName, s.Org) {
		name, _ := descriptor["relationshipName"].(string)
		childName, child, relation, ok := relatedListChild(s.Org, parentName, name)
		if !ok || name == "" {
			continue
		}
		lists = append(lists, map[string]any{"entityLabel": child.Definition.Label, "entityPluralLabel": child.Definition.PluralLabel, "fieldApiName": relation.Field, "keyPrefix": child.Definition.KeyPrefix, "label": relationLabel(s.Source.Project.FieldFiles, childName, relation.Field, relation.ChildRelationship), "objectApiName": childName, "parentFieldApiName": "Id", "relatedListId": relation.ChildRelationship, "relatedListInfoUrl": fmt.Sprintf("/services/data/v%s/ui-api/related-list-info/%s/%s?recordTypeId=012000000000000AAA", version, parentName, relation.ChildRelationship), "themeInfo": map[string]any{"color": "747E96", "iconUrl": "/img/icon/t4v35/standard/custom_120.png"}, "uiApiEnabledLayout": len(s.relatedLayoutColumns(parentName, childName, relation.Field)) > 0})
	}
	sort.Slice(lists, func(i, j int) bool {
		return lists[i].(map[string]any)["relatedListId"].(string) < lists[j].(map[string]any)["relatedListId"].(string)
	})
	return map[string]any{"parentObjectApiName": parentName, "parentRecordTypeId": config["recordTypeId"], "relatedLists": lists}, nil
}

func (s *Server) listRecordsData(view listViewMetadata, object storage.ObjectState, config map[string]any) (map[string]any, *lwcbrowser.WireError) {
	data, err := s.listRecordPageData(object, config, view.Filters)
	if err != nil {
		return nil, err
	}
	data["listReference"] = listReference(view)
	data["searchTerm"] = config["searchTerm"]
	return data, nil
}

func (s *Server) relatedListRecordsData(config map[string]any) (map[string]any, *lwcbrowser.WireError) {
	pageSize := listNumber(config, "pageSize", 50)
	if pageSize < 1 || pageSize > 1999 {
		return nil, &lwcbrowser.WireError{Type: "NUMBER_OUTSIDE_VALID_RANGE", Status: 400, RecordErrorID: "-32372111", Message: "pageSize parameter must be between 1 and 1999"}
	}
	parentName, parent, ok := findOrgRecord(s.Org, listConfigString(config, "parentRecordId"))
	if !ok {
		return nil, &lwcbrowser.WireError{Message: "parent record not found"}
	}
	relationship := listConfigString(config, "relatedListId")
	_, child, relation, ok := relatedListChild(s.Org, parentName, relationship)
	if !ok {
		return nil, &lwcbrowser.WireError{Type: "INVALID_TYPE", Status: 400, RecordErrorID: "531283287", Message: "The related lists UI API currently does not support this related list"}
	}
	where := listConfigString(config, "where")
	if listWhereUnterminatedKeyPattern.MatchString(where) {
		return nil, &lwcbrowser.WireError{Type: "UNKNOWN_EXCEPTION", Status: 400, RecordErrorID: "531283287", Message: fmt.Sprintf("org.json.JSONException: Expected a ':' after a key at character %d of %s", len(where), where)}
	}
	if err := s.relatedListRequiredFieldError(parentName, child.Definition, relation.ChildRelationship, config); err != nil {
		return nil, err
	}
	selected := child
	selected.Records = make(map[storage.ID]storage.Record)
	for id, record := range child.Records {
		if value, ok := record.Fields[relation.Field]; ok && storage.IDsEqual(storage.ID(fmt.Sprint(storageValueJSON(value))), parent.ID) {
			selected.Records[id] = record
		}
	}
	data, err := s.listRecordPageData(selected, config, nil)
	if err != nil {
		return nil, err
	}
	data["listReference"] = relatedListReference(parentName, relation.ChildRelationship, string(parent.ID))
	// Native related reads echo an absent sort selection as an empty array.
	data["sortBy"] = listStrings(config["sortBy"])
	return data, nil
}

// Native required-field controls at both APIs retain the same sorted nested
// selection after cold, related, list, record and legacy reads. This projection
// belongs to the related-list error DTO; it does not expand successful reads or
// change the shared getRecord diagnostic.
func (s *Server) relatedListRequiredFieldError(parentName string, def storage.ObjectDefinition, relationship string, config map[string]any) *lwcbrowser.WireError {
	fields := listStrings(config["fields"])
	missing := []string{}
	terms := []string{}
	add := func(selector string, required bool) {
		name := strings.TrimPrefix(strings.TrimPrefix(selector, "-"), def.APIName+".")
		if strings.Contains(name, ".") {
			return // Relationship selections retain their shared record validation.
		}
		if canonical, ok := storage.ResolveFieldName(def, s.Org.Namespace, name); ok {
			terms = appendUniqueFieldNames(terms, canonical)
		} else if required && name != "" {
			missing = appendUniqueFieldNames(missing, name)
			terms = appendUniqueFieldNames(terms, name)
		}
	}
	for _, name := range []string{"CreatedDate", "Id", "LastModifiedById", "LastModifiedDate", "Name", "SystemModstamp"} {
		add(name, false)
	}
	for _, name := range fields {
		add(name, true)
	}
	for _, name := range listStrings(config["optionalFields"]) {
		add(name, false)
	}
	for _, name := range listStrings(config["sortBy"]) {
		add(name, false)
	}
	if len(missing) == 0 {
		return nil
	}
	// Order diagnostic selectors without case sensitivity, retaining their
	// original spelling for the query excerpt and caret position.
	sort.SliceStable(terms, func(i, j int) bool {
		return strings.ToLower(terms[i]) < strings.ToLower(terms[j])
	})
	sort.Strings(missing)
	parentFields := "Id"
	if _, ok := s.Org.Objects[parentName].Definition.Fields["Name"]; ok {
		parentFields += ", Name"
	}
	prefix := "SELECT " + parentFields + ", (SELECT "
	// Only this captured portion of the query is exposed by the error window.
	query := prefix + strings.Join(terms, ", ") + " FROM " + relationship + " ORDER"
	position := len(prefix)
	for _, term := range terms {
		if term == missing[0] {
			break
		}
		position += len(term) + 2
	}
	start, end := max(0, position-30), min(len(query), position+31)
	for start > 0 && query[start-1] != ' ' {
		start--
	}
	for end < len(query) && query[end] != ' ' && query[end] != ',' {
		end++
	}
	excerpt := query[start:end]
	message := "INVALID_FIELD: \n" + excerpt + "\n" + strings.Repeat(" ", position-start) + "^\n" +
		fmt.Sprintf("ERROR at Row:1:Column:%d\nNo such column '%s' on entity '%s'. If you are attempting to use a custom field, be sure to append the '__c' after the custom field name. Please reference your WSDL or the describe call for the appropriate names.", position+1, missing[0], def.APIName)
	return &lwcbrowser.WireError{Type: "INVALID_FIELD", Status: 400, RecordErrorID: "382714451", Message: message}
}

// List views and related lists use the same record selection, ordering and
// paging path; their callers supply the list reference and view-only facets.
func (s *Server) listRecordPageData(object storage.ObjectState, config map[string]any, filters []listViewFilterXML) (map[string]any, *lwcbrowser.WireError) {
	pageSize := listNumber(config, "pageSize", 50)
	if pageSize < 1 || pageSize > 2000 {
		return nil, &lwcbrowser.WireError{Status: 400, Message: "invalid list page size"}
	}
	offset := 0
	if token := listConfigString(config, "pageToken"); token != "" {
		var err error
		offset, err = strconv.Atoi(token)
		if err != nil {
			return nil, &lwcbrowser.WireError{Type: "ILLEGAL_QUERY_PARAMETER_VALUE", Status: 400, RecordErrorID: "-510054387", Message: fmt.Sprintf("For input string: %q", token)}
		}
	}
	fields, optional, sortBy := listStrings(config["fields"]), listStrings(config["optionalFields"]), listStrings(config["sortBy"])
	if len(sortBy) == 0 {
		sortBy = []string{"Name", "Id"}
	}
	ids := []string{}
	where := listConfigString(config, "where")
	var whereField string
	var whereMinimum *big.Rat
	whereInclusive := true
	if where != "" {
		parts := listWhereMinimumPattern.FindStringSubmatch(where)
		if len(parts) != 4 {
			return nil, &lwcbrowser.WireError{Status: 400, Message: "unsupported list where expression"}
		}
		var ok bool
		whereField, ok = storage.ResolveFieldName(object.Definition, s.Org.Namespace, parts[1])
		if !ok {
			return nil, &lwcbrowser.WireError{Status: 400, Message: "unknown list where field"}
		}
		whereInclusive = parts[2] == "gte"
		whereMinimum, _ = new(big.Rat).SetString(parts[3])
	}
	for id, record := range object.Records {
		if record.System.IsDeleted {
			continue
		}
		match := true
		for _, filter := range filters {
			value, _ := record.GetField(filter.Field)
			text := fmt.Sprint(storageValueJSON(value))
			switch filter.Operation {
			case "startsWith":
				match = match && strings.HasPrefix(text, filter.Value)
			case "equals":
				match = match && text == filter.Value
			}
		}
		if whereMinimum != nil {
			value, _ := record.GetField(whereField)
			number, valid := new(big.Rat).SetString(fmt.Sprint(storageValueJSON(value)))
			match = match && valid && (number.Cmp(whereMinimum) > 0 || whereInclusive && number.Cmp(whereMinimum) == 0)
		}
		if match {
			ids = append(ids, string(id))
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		for _, sortField := range sortBy {
			descending := strings.HasPrefix(sortField, "-")
			name := strings.TrimPrefix(sortField, "-")
			name = wireFieldName(name)
			left, _ := object.Records[storage.ID(ids[i])].GetField(name)
			right, _ := object.Records[storage.ID(ids[j])].GetField(name)
			a, b := fmt.Sprint(storageValueJSON(left)), fmt.Sprint(storageValueJSON(right))
			if name == "Id" {
				a, b = ids[i], ids[j]
			}
			comparison := strings.Compare(a, b)
			field := object.Definition.Fields[name]
			if field.Type == storage.FieldDecimal || field.Type == storage.FieldInteger {
				leftNumber, leftOK := new(big.Rat).SetString(a)
				rightNumber, rightOK := new(big.Rat).SetString(b)
				if leftOK && rightOK {
					comparison = leftNumber.Cmp(rightNumber)
				}
			}
			if comparison != 0 {
				if descending {
					return comparison > 0
				}
				return comparison < 0
			}
		}
		return ids[i] < ids[j]
	})
	records := []any{}
	if offset < 0 {
		offset = 0
	}
	if offset > len(ids) {
		offset = len(ids)
	}
	end := offset + pageSize
	if end > len(ids) {
		end = len(ids)
	}
	for _, id := range ids[offset:end] {
		row, err := getLDSRecordWireData(s.Org, lwcbrowser.WireGetRecordRequest{RecordID: id, Fields: fields, OptionalFields: optional}, s.Source)
		if err != nil {
			return nil, err
		}
		// Unlike an omitted getRecord selection, a captured empty list fields
		// selection returns record envelopes with no selected business fields.
		if len(fields) == 0 && len(optional) == 0 {
			row["fields"] = map[string]any{}
		}
		s.projectListRecord(row, object.Records[storage.ID(id)], object.Definition)
		records = append(records, row)
	}
	var next, previous any
	if end < len(ids) {
		next = strconv.Itoa(end)
	}
	if offset > 0 {
		previous = strconv.Itoa(max(0, offset-pageSize))
	}
	return map[string]any{"count": len(records), "currentPageToken": strconv.Itoa(offset), "nextPageToken": next, "previousPageToken": previous, "fields": fields, "optionalFields": optional, "listInfoETag": nil, "pageSize": pageSize, "records": records, "sortBy": sortBy, "where": config["where"]}, nil
}

// List resources have their own projection; getRecord's relationship array and
// field display defaults are not the list-record representation.
func (s *Server) projectListRecord(row map[string]any, record storage.Record, definition storage.ObjectDefinition) {
	row["childRelationships"] = map[string]any{}
	row["recordTypeInfo"] = nil
	row["systemModstamp"] = record.System.SystemModstamp
	fields := row["fields"].(map[string]any)
	for name, raw := range fields {
		payload, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		field := definition.Fields[name]
		if field.Type == storage.FieldDecimal && strings.EqualFold(field.DisplayType, "DOUBLE") {
			payload["displayValue"] = nil
		}
		if name == "SystemModstamp" {
			payload["value"] = record.System.SystemModstamp
		}
		if field.Type == storage.FieldDateTime {
			if value, ok := payload["value"].(string); ok {
				payload["displayValue"] = s.listDateTimeDisplay(value)
			}
		}
	}
}

func (s *Server) listDateTimeDisplay(value string) any {
	user := s.currentUser(nil, "")
	// Unsupported locales or unavailable zone data do not panic or silently
	// apply another user's zone. The raw timestamp remains in value.
	if userString(user, "LocaleSidKey", "") != "en_US" {
		return nil
	}
	zoneName := userString(user, "TimeZoneSidKey", "")
	if zoneName == "" {
		return nil
	}
	zone, err := time.LoadLocation(zoneName)
	if err != nil {
		return nil
	}
	stamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	return stamp.In(zone).Format("1/2/2006, 3:04 PM")
}

// Native list-view and related-list controls supply numeric gte/gt predicates.
// Other expression forms remain explicit local gaps until captured.
var listWhereMinimumPattern = regexp.MustCompile(`^\s*\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*:\s*\{\s*(gte|gt)\s*:\s*(-?[0-9]+(?:\.[0-9]+)?)\s*\}\s*\}\s*$`)

// An unquoted ASCII key ending at EOF is the malformed-input class captured
// by r_relatedRecords_filter_invalid. Other syntax errors remain local gaps.
var listWhereUnterminatedKeyPattern = regexp.MustCompile(`^\s*\{\s*[A-Za-z_][A-Za-z0-9_]*\s*$`)
