package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func (s *Server) handleLimits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, localAPILimits)
}

func (s *Server) handleRecordCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, s.recordCountPayload(r))
}

type recordCountObject struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (s *Server) recordCountPayload(r *http.Request) map[string]any {
	allowed := recordCountFilter(r.URL.Query().Get("sObjects"))
	names := make([]string, 0, len(s.Org.Objects))
	recordsByName := make(map[string]map[storage.ID]storage.Record, len(s.Org.Objects))
	for key, object := range s.Org.Objects {
		name := object.Definition.APIName
		if name == "" {
			name = key
		}
		if allowed != nil && !allowed[name] {
			continue
		}
		names = append(names, name)
		recordsByName[name] = object.Records
	}
	sort.Strings(names)

	objects := make([]recordCountObject, 0, len(names))
	for _, name := range names {
		objects = append(objects, recordCountObject{Name: name, Count: len(recordsByName[name])})
	}
	return map[string]any{"sObjects": objects}
}

func recordCountFilter(raw string) map[string]bool {
	if raw == "" {
		return nil
	}
	out := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		if name != "" {
			out[name] = true
		}
	}
	return out
}

func (s *Server) handleTooling(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	switch {
	case len(parts) == 0:
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, toolingDiscoveryPayload(version))
	case len(parts) == 1 && parts[0] == "executeAnonymous":
		s.handleExecuteAnonymous(w, r)
	case len(parts) == 1 && parts[0] == "query":
		s.handleToolingQuery(w, r, version, false)
	case len(parts) == 1 && parts[0] == "queryAll":
		s.handleToolingQuery(w, r, version, true)
	case len(parts) == 2 && parts[0] == "query":
		s.handleQueryMore(w, r, parts[1])
	case len(parts) == 1 && parts[0] == "search":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Tooling search is not implemented in the local server")
	case len(parts) == 1 && parts[0] == "sobjects":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		if !s.Source.hasData() {
			writeSalesforceError(w, errUnsupportedFeature, "Tooling sObject discovery is not implemented in the local server")
			return
		}
		writeJSON(w, http.StatusOK, s.toolingSObjectsPayload(version))
	case len(parts) == 3 && parts[0] == "sobjects" && parts[2] == "describe":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		object, ok := s.Source.ToolingOrg.Objects[parts[1]]
		if !ok {
			if isToolingMetadataObject(parts[1]) {
				writeSalesforceError(w, errUnsupportedFeature, "Tooling sObject describe for "+parts[1]+" is not implemented in the local server")
				return
			}
			writeSalesforceError(w, errUnknownTooling)
			return
		}
		writeJSON(w, http.StatusOK, toolingDescribePayload(object.Definition))
	case len(parts) == 2 && parts[0] == "sobjects" && s.isModeledToolingObject(parts[1]):
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		object := s.Source.ToolingOrg.Objects[parts[1]]
		writeJSON(w, http.StatusOK, toolingObjectResourcePayload(object.Definition, version))
	case len(parts) == 3 && parts[0] == "sobjects" && s.isModeledToolingObject(parts[1]):
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		object := s.Source.ToolingOrg.Objects[parts[1]]
		record, ok := object.Records[storage.ID(parts[2])]
		if !ok {
			writeSalesforceError(w, errUnknownRecord)
			return
		}
		writeJSON(w, http.StatusOK, toolingRecordPayload(record, version))
	case len(parts) == 2 && parts[0] == "sobjects" && isToolingMetadataObject(parts[1]):
		writeUnsupportedToolingMetadata(w, r, parts[1], "object collection", toolingCollectionMethods(parts[1])...)
	case len(parts) >= 3 && parts[0] == "sobjects" && isToolingMetadataObject(parts[1]):
		writeUnsupportedToolingMetadata(w, r, parts[1], "object record", toolingRecordMethods(parts[1])...)
	case len(parts) == 1 && parts[0] == "completions":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Tooling completions are not implemented in the local server")
	case len(parts) == 1 && (parts[0] == "runTestsAsynchronous" || parts[0] == "runTestsSynchronous"):
		writeUnsupportedToolingTestRun(w, r, parts[0])
	case len(parts) == 1 && parts[0] == "coverage":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Tooling ApexCodeCoverage resources are not implemented in the local server")
	default:
		writeSalesforceError(w, errUnknownTooling)
	}
}

func (s *Server) isModeledToolingObject(name string) bool {
	if s.Source.ToolingOrg.Objects == nil {
		return false
	}
	_, ok := s.Source.ToolingOrg.Objects[name]
	return ok
}

func (s *Server) handleToolingQuery(w http.ResponseWriter, r *http.Request, version string, allRows bool) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	queryText := r.URL.Query().Get("q")
	query, err := soql.Parse(queryText)
	if err != nil {
		writeSalesforceError(w, errMalformedQuery, err.Error())
		return
	}
	if !s.isModeledToolingObject(query.Object) {
		if _, ok := storage.ResolveObjectName(*s.Org, query.Object); ok || isToolingLocalSchemaQueryObject(query.Object) {
			s.handleQuery(w, r, version, "tooling/query", allRows)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Tooling query for "+query.Object+" is not modeled in the local server")
		return
	}
	query.AllRows = allRows
	result, err := soql.Execute(s.Source.ToolingOrg, query)
	if err != nil {
		writeSalesforceError(w, errMalformedQuery, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toolingQueryResultPayload(result.Rows, true, result.Records, version))
}

func isToolingLocalSchemaQueryObject(name string) bool {
	switch {
	case strings.EqualFold(name, "EntityDefinition"),
		strings.EqualFold(name, "EntityParticle"),
		strings.EqualFold(name, "FieldDefinition"),
		strings.EqualFold(name, "RelationshipDomain"),
		strings.EqualFold(name, "UserEntityAccess"),
		strings.EqualFold(name, "UserFieldAccess"):
		return true
	default:
		return false
	}
}

func (s *Server) toolingSObjectsPayload(version string) map[string]any {
	base := "/services/data/" + version + "/tooling/sobjects/"
	names := make([]string, 0, len(s.Source.ToolingOrg.Objects))
	for name := range s.Source.ToolingOrg.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	objects := make([]map[string]any, 0, len(names))
	for _, name := range names {
		def := s.Source.ToolingOrg.Objects[name].Definition
		objects = append(objects, map[string]any{
			"name":         name,
			"label":        labelOrFallback(def.Label, name),
			"keyPrefix":    def.KeyPrefix,
			"queryable":    true,
			"retrieveable": true,
			"createable":   false,
			"updateable":   false,
			"deletable":    false,
			"url":          base + name,
			"describe":     base + name + "/describe",
		})
	}
	return map[string]any{"sobjects": objects}
}

func toolingDescribePayload(def storage.ObjectDefinition) map[string]any {
	payload := describePayload(def, nil)
	payload["createable"] = false
	payload["updateable"] = false
	payload["deletable"] = false
	payload["custom"] = false
	payload["retrieveable"] = true
	return payload
}

func toolingObjectResourcePayload(def storage.ObjectDefinition, version string) map[string]any {
	base := "/services/data/" + version + "/tooling/sobjects/" + def.APIName
	return map[string]any{
		"name":           def.APIName,
		"label":          labelOrFallback(def.Label, def.APIName),
		"keyPrefix":      def.KeyPrefix,
		"objectDescribe": base + "/describe",
		"describe":       base + "/describe",
		"url":            base,
		"urls": map[string]string{
			"rowTemplate": base + "/{ID}",
			"describe":    base + "/describe",
		},
	}
}

func toolingQueryResultPayload(totalSize int, done bool, records []storage.Record, version string) map[string]any {
	return map[string]any{
		"totalSize": totalSize,
		"done":      done,
		"records":   toolingRecordsPayload(records, version),
	}
}

func toolingRecordsPayload(records []storage.Record, version string) []map[string]any {
	out := make([]map[string]any, 0, len(records))
	for _, record := range records {
		out = append(out, toolingRecordPayload(record, version))
	}
	return out
}

func toolingRecordPayload(record storage.Record, version string) map[string]any {
	objectName := record.Object
	out := recordPayload(record, version, objectName, record.ID)
	if attrs, ok := out["attributes"].(map[string]any); ok {
		attrs["url"] = "/services/data/" + version + "/tooling/sobjects/" + objectName + "/" + string(record.ID)
	}
	return out
}

func isToolingMetadataObject(name string) bool {
	switch name {
	case "ApexClass",
		"ApexTrigger",
		"ApexPage",
		"ApexComponent",
		"StaticResource",
		"ContainerMember",
		"ApexClassMember",
		"ApexTriggerMember",
		"ApexPageMember",
		"ApexComponentMember",
		"StaticResourceMember",
		"ApexLog",
		"TraceFlag",
		"DebugLevel",
		"ApexTestQueueItem",
		"ApexTestResult",
		"ApexCodeCoverage",
		"ApexCodeCoverageAggregate",
		"ApexOrgWideCoverage",
		"ContainerAsyncRequest",
		"MetadataContainer",
		"ApexTestRunResult",
		"ApexTestSuite",
		"ApexTestSuiteMembership":
		return true
	default:
		return false
	}
}

func toolingCollectionMethods(objectName string) []string {
	if isToolingReadOnlyObject(objectName) {
		return []string{http.MethodGet}
	}
	return []string{http.MethodGet, http.MethodPost}
}

func toolingRecordMethods(objectName string) []string {
	if isToolingReadOnlyObject(objectName) {
		return []string{http.MethodGet}
	}
	return []string{http.MethodGet, http.MethodPatch, http.MethodDelete}
}

func isToolingReadOnlyObject(name string) bool {
	switch name {
	case "ApexLog",
		"ApexTestResult",
		"ApexCodeCoverage",
		"ApexCodeCoverageAggregate",
		"ApexOrgWideCoverage",
		"ApexTestRunResult":
		return true
	default:
		return false
	}
}

func writeUnsupportedToolingMetadata(w http.ResponseWriter, r *http.Request, objectName, scope string, allowed ...string) {
	for _, method := range allowed {
		if r.Method == method {
			if !validateToolingMetadataRequest(w, r, objectName, scope) {
				return
			}
			writeSalesforceError(w, errUnsupportedFeature, "Tooling "+objectName+" "+scope+" access is not implemented in the local server")
			return
		}
	}
	writeMethodNotAllowed(w, allowed...)
}

func validateToolingMetadataRequest(w http.ResponseWriter, r *http.Request, objectName, scope string) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPatch && r.Method != http.MethodDelete {
		return true
	}
	body, ok := decodeOptionalJSONObject(w, r)
	if !ok {
		return false
	}
	if objectName == "ApexTestQueueItem" && scope == "object collection" && r.Method == http.MethodPost {
		if _, ok := body["ApexClassId"]; !ok {
			writeSalesforceError(w, errRequiredFieldMissing, "ApexTestQueueItem.ApexClassId is required")
			return false
		}
	}
	if isToolingDeployMemberObject(objectName) && scope == "object collection" && r.Method == http.MethodPost {
		if _, ok := body["MetadataContainerId"]; !ok {
			writeSalesforceError(w, errRequiredFieldMissing, objectName+".MetadataContainerId is required")
			return false
		}
	}
	return true
}

func isToolingDeployMemberObject(name string) bool {
	switch name {
	case "ContainerMember",
		"ApexClassMember",
		"ApexTriggerMember",
		"ApexPageMember",
		"ApexComponentMember",
		"StaticResourceMember":
		return true
	default:
		return false
	}
}

func writeUnsupportedToolingTestRun(w http.ResponseWriter, r *http.Request, endpoint string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	if _, ok := decodeOptionalJSONObject(w, r); !ok {
		return
	}
	writeSalesforceError(w, errUnsupportedFeature, "Tooling "+endpoint+" is not implemented in the local server; use glade test for local Apex test execution")
}

func decodeOptionalJSONObject(w http.ResponseWriter, r *http.Request) (map[string]json.RawMessage, bool) {
	if r.Body == nil || r.Body == http.NoBody {
		return map[string]json.RawMessage{}, true
	}
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if err == io.EOF {
			return map[string]json.RawMessage{}, true
		}
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return nil, false
	}
	if body == nil {
		body = map[string]json.RawMessage{}
	}
	return body, true
}

func writeUnsupportedMetadataREST(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	switch {
	case len(parts) == 0:
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, metadataRESTDiscoveryPayload(version))
	case len(parts) == 1 && isMetadataReadDiscoveryRoute(parts[0]):
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, metadataReadDiscoveryUnsupportedMessage(parts[0]))
	case len(parts) >= 2 && parts[0] == "components":
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Metadata REST component read and discovery are not implemented in the local server; use source files and glade inspect/check for local metadata state")
	case len(parts) == 1 && parts[0] == "retrieveRequest":
		if !methodAllowed(r, http.MethodPost) {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		if _, ok := decodeOptionalJSONObject(w, r); !ok {
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Metadata REST retrieve requests are not implemented in the local server; no retrieve jobs are created locally")
	case len(parts) == 2 && parts[0] == "retrieveRequest":
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Metadata REST retrieve status is not implemented in the local server; no retrieve jobs are created locally")
	case len(parts) == 3 && parts[0] == "retrieveRequest" && parts[2] == "results":
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Metadata REST retrieve results are not implemented in the local server; no retrieve jobs are created locally")
	case len(parts) == 1 && parts[0] == "deployRequest":
		if !methodAllowed(r, http.MethodPost) {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		if _, ok := decodeOptionalJSONObject(w, r); !ok {
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Metadata REST deploy requests are not implemented in the local server; use source files and glade check/test for local validation")
	case len(parts) == 2 && parts[0] == "deployRequest":
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Metadata REST deploy status is not implemented in the local server; no deploy jobs are created locally")
	case len(parts) == 3 && parts[0] == "deployRequest" && (parts[2] == "results" || parts[2] == "deployDetails"):
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		resource := "results"
		if parts[2] == "deployDetails" {
			resource = "details"
		}
		writeSalesforceError(w, errUnsupportedFeature, "Metadata REST deploy "+resource+" retrieval is not implemented in the local server; no deploy jobs are created locally")
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func (s *Server) handleMetadataREST(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	switch {
	case len(parts) == 0:
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, metadataRESTDiscoveryPayload(version))
	case len(parts) == 1 && (parts[0] == "describe" || parts[0] == "describeMetadata"):
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		if !s.Source.hasData() {
			writeSalesforceError(w, errUnsupportedFeature, metadataReadDiscoveryUnsupportedMessage(parts[0]))
			return
		}
		writeJSON(w, http.StatusOK, s.describeMetadataPayload(version))
	case len(parts) == 1 && parts[0] == "listMetadata":
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		if !s.Source.hasData() {
			writeSalesforceError(w, errUnsupportedFeature, metadataReadDiscoveryUnsupportedMessage(parts[0]))
			return
		}
		writeJSON(w, http.StatusOK, s.listMetadataPayload(r))
	case len(parts) == 1 && parts[0] == "components":
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		if !s.Source.hasData() {
			writeSalesforceError(w, errUnsupportedFeature, metadataReadDiscoveryUnsupportedMessage(parts[0]))
			return
		}
		writeJSON(w, http.StatusOK, s.metadataComponentsPayload("", ""))
	case len(parts) == 2 && parts[0] == "components":
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		if !s.Source.hasData() {
			writeSalesforceError(w, errUnsupportedFeature, "Metadata REST component read and discovery are not implemented in the local server; use source files and glade inspect/check for local metadata state")
			return
		}
		writeJSON(w, http.StatusOK, s.metadataComponentsPayload(parts[1], ""))
	case len(parts) == 3 && parts[0] == "components":
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		if !s.Source.hasData() {
			writeSalesforceError(w, errUnsupportedFeature, "Metadata REST component read and discovery are not implemented in the local server; use source files and glade inspect/check for local metadata state")
			return
		}
		component, ok := s.Source.componentBy[metadataComponentKey(parts[1], parts[2])]
		if !ok {
			writeSalesforceError(w, errUnknownEndpoint, "metadata component not found")
			return
		}
		writeJSON(w, http.StatusOK, s.metadataComponentPayload(component, true))
	default:
		writeUnsupportedMetadataREST(w, r, version, parts)
	}
}

func (s *Server) describeMetadataPayload(version string) map[string]any {
	counts := make(map[string]int)
	for _, component := range s.Source.Components {
		counts[component.Type]++
	}
	types := make([]string, 0, len(counts))
	for typ := range counts {
		types = append(types, typ)
	}
	sort.Strings(types)
	objects := make([]map[string]any, 0, len(types))
	for _, typ := range types {
		objects = append(objects, map[string]any{
			"xmlName":        typ,
			"directoryName":  metadataDirectoryName(typ),
			"inFolder":       false,
			"metaFile":       metadataTypeHasMetaFile(typ),
			"childXmlNames":  []string{},
			"suffix":         metadataSuffix(typ),
			"localFileCount": counts[typ],
		})
	}
	return map[string]any{
		"metadataObjects":       objects,
		"organizationNamespace": s.Source.Project.Namespace,
		"partialSaveAllowed":    false,
		"testRequired":          false,
		"version":               version,
	}
}

func (s *Server) listMetadataPayload(r *http.Request) map[string]any {
	typ := strings.TrimSpace(firstNonEmptyQuery(r.URL.Query(), "type", "typeName", "metadataType"))
	components := make([]map[string]any, 0)
	for _, component := range s.Source.Components {
		if typ != "" && !strings.EqualFold(component.Type, typ) {
			continue
		}
		components = append(components, metadataFileProperties(component))
	}
	return map[string]any{
		"result": components,
		"size":   len(components),
	}
}

func (s *Server) metadataComponentsPayload(typ, fullName string) map[string]any {
	components := make([]map[string]any, 0)
	for _, component := range s.Source.Components {
		if typ != "" && !strings.EqualFold(component.Type, typ) {
			continue
		}
		if fullName != "" && !strings.EqualFold(component.FullName, fullName) {
			continue
		}
		components = append(components, s.metadataComponentPayload(component, false))
	}
	return map[string]any{"components": components, "size": len(components)}
}

func (s *Server) metadataComponentPayload(component metadataComponent, includeContent bool) map[string]any {
	payload := metadataFileProperties(component)
	payload["id"] = string(component.ID)
	if includeContent {
		if component.Content != "" {
			payload["content"] = component.Content
		} else if data, err := os.ReadFile(component.FileName); err == nil {
			payload["content"] = string(data)
		}
	}
	return payload
}

func metadataFileProperties(component metadataComponent) map[string]any {
	return map[string]any{
		"type":            component.Type,
		"fullName":        component.FullName,
		"fileName":        filepath.ToSlash(component.FileName),
		"manageableState": "unmanaged",
	}
}

func firstNonEmptyQuery(values url.Values, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(values.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func metadataDirectoryName(typ string) string {
	switch typ {
	case "ApexClass":
		return "classes"
	case "ApexTrigger":
		return "triggers"
	case "ApexPage":
		return "pages"
	case "ApexComponent":
		return "components"
	case "CustomObject", "CustomField", "RecordType", "ValidationRule", "ListView", "CompactLayout":
		return "objects"
	case "Layout":
		return "layouts"
	case "StaticResource":
		return "staticresources"
	case "Workflow":
		return "workflows"
	default:
		return strings.ToLower(typ)
	}
}

func metadataSuffix(typ string) string {
	switch typ {
	case "ApexClass":
		return "cls"
	case "ApexTrigger":
		return "trigger"
	case "ApexPage":
		return "page"
	case "ApexComponent":
		return "component"
	case "Layout":
		return "layout"
	case "ListView":
		return "listView"
	case "CompactLayout":
		return "compactLayout"
	case "StaticResource":
		return "resource"
	default:
		return ""
	}
}

func metadataTypeHasMetaFile(typ string) bool {
	switch typ {
	case "ApexClass", "ApexTrigger", "ApexPage", "ApexComponent", "StaticResource":
		return true
	default:
		return strings.HasSuffix(typ, "Layout") || typ == "ListView"
	}
}

func (s *Server) handleBulkJobs(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	if len(parts) < 1 {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, bulkJobsDiscoveryPayload(version))
		return
	}
	switch parts[0] {
	case "query":
		writeUnsupportedBulkQueryJob(w, r, parts[1:])
	case "ingest":
		writeUnsupportedBulkIngestJob(w, r, parts[1:])
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func writeUnsupportedBulkQueryJob(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) == 0:
		if !methodAllowed(r, http.MethodGet, http.MethodPost) {
			writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
			return
		}
		if r.Method == http.MethodPost {
			if _, ok := decodeOptionalJSONObject(w, r); !ok {
				return
			}
		}
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v2 query jobs are not implemented in the local server")
	case len(parts) == 1:
		if !methodAllowed(r, http.MethodGet, http.MethodPatch, http.MethodDelete) {
			writeMethodNotAllowed(w, http.MethodGet, http.MethodPatch, http.MethodDelete)
			return
		}
		if r.Method == http.MethodPatch {
			if _, ok := decodeOptionalJSONObject(w, r); !ok {
				return
			}
		}
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v2 query job records are not implemented in the local server")
	case len(parts) == 2 && parts[1] == "results":
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v2 query job results are not implemented in the local server")
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func writeUnsupportedBulkIngestJob(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) == 0:
		if !methodAllowed(r, http.MethodGet, http.MethodPost) {
			writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
			return
		}
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, localBulkJobCollectionPayload())
			return
		}
		if r.Method == http.MethodPost {
			if _, ok := decodeOptionalJSONObject(w, r); !ok {
				return
			}
		}
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v2 ingest jobs are not implemented in the local server")
	case len(parts) == 1:
		if !methodAllowed(r, http.MethodGet, http.MethodPatch, http.MethodDelete) {
			writeMethodNotAllowed(w, http.MethodGet, http.MethodPatch, http.MethodDelete)
			return
		}
		if r.Method == http.MethodPatch {
			if _, ok := decodeOptionalJSONObject(w, r); !ok {
				return
			}
		}
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v2 ingest job records are not implemented in the local server")
	case len(parts) == 2 && parts[1] == "batches":
		if !methodAllowed(r, http.MethodPut) {
			writeMethodNotAllowed(w, http.MethodPut)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v2 ingest job batches are not implemented in the local server")
	case len(parts) == 2 && parts[1] == "successfulResults":
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v2 ingest successful results are not implemented in the local server")
	case len(parts) == 2 && parts[1] == "failedResults":
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v2 ingest failed results are not implemented in the local server")
	case len(parts) == 2 && parts[1] == "unprocessedrecords":
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v2 ingest unprocessed records are not implemented in the local server")
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func localBulkJobCollectionPayload() map[string]any {
	return map[string]any{
		"done":           true,
		"records":        []any{},
		"nextRecordsUrl": nil,
	}
}
