package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/dml"
	"github.com/open-aer/oaer/internal/soql"
	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/vm"
)

type Server struct {
	Org   *storage.OrgState
	Store interface {
		Save(storage.OrgState) error
	}
	LimitMode vm.LimitMode
	LimitCaps vm.LimitCaps
}

func New(org *storage.OrgState) *Server {
	return &Server{Org: org}
}

func NewWithStore(org *storage.OrgState, store interface{ Save(storage.OrgState) error }) *Server {
	return &Server{Org: org, Store: store}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	parts := splitPath(r.URL.Path)
	if len(parts) == 3 && parts[0] == "services" && parts[1] == "oauth2" && parts[2] == "userinfo" {
		writeJSON(w, http.StatusOK, s.userInfoPayload(r))
		return
	}
	if len(parts) == 3 && parts[0] == "id" {
		writeJSON(w, http.StatusOK, s.identityPayload(r))
		return
	}
	if len(parts) == 2 && parts[0] == "services" && parts[1] == "data" {
		writeJSON(w, http.StatusOK, []map[string]string{{"version": "v61.0", "url": "/services/data/v61.0"}})
		return
	}
	if len(parts) < 3 || parts[0] != "services" || parts[1] != "data" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "unknown endpoint")
		return
	}
	rest := parts[3:]
	if len(parts) == 3 {
		writeJSON(w, http.StatusOK, map[string]string{
			"sobjects": "/services/data/" + parts[2] + "/sobjects",
			"query":    "/services/data/" + parts[2] + "/query",
			"queryAll": "/services/data/" + parts[2] + "/queryAll",
			"tooling":  "/services/data/" + parts[2] + "/tooling",
			"limits":   "/services/data/" + parts[2] + "/limits",
		})
		return
	}
	switch {
	case len(rest) == 1 && rest[0] == "sobjects":
		s.handleSObjects(w, r)
	case len(rest) >= 2 && rest[0] == "sobjects":
		s.handleObject(w, r, rest[1:])
	case len(rest) == 1 && rest[0] == "query":
		s.handleQuery(w, r)
	case len(rest) == 1 && rest[0] == "queryAll":
		s.handleQuery(w, r)
	case len(rest) == 1 && rest[0] == "limits":
		s.handleLimits(w, r)
	case len(rest) >= 1 && rest[0] == "tooling":
		s.handleTooling(w, r, rest[1:])
	case len(rest) >= 1 && rest[0] == "composite":
		s.handleComposite(w, r, rest[1:])
	case len(rest) >= 1 && rest[0] == "oaer":
		s.handleOAER(w, r, rest[1:])
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "unknown endpoint")
	}
}

func (s *Server) handleSObjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	objects := make([]map[string]string, 0, len(s.Org.Objects))
	names := make([]string, 0, len(s.Org.Objects))
	for name := range s.Org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		object := s.Org.Objects[name]
		objects = append(objects, map[string]string{
			"name":        name,
			"label":       object.Definition.Label,
			"keyPrefix":   object.Definition.KeyPrefix,
			"url":         "/services/data/v61.0/sobjects/" + name,
			"describe":    "/services/data/v61.0/sobjects/" + name + "/describe",
			"recentItems": "/services/data/v61.0/sobjects/" + name + "/recent",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sobjects": objects})
}

func (s *Server) handleObject(w http.ResponseWriter, r *http.Request, parts []string) {
	objectName := parts[0]
	switch {
	case len(parts) == 2 && parts[1] == "describe" && r.Method == http.MethodGet:
		object, ok := s.Org.Objects[objectName]
		if !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "unknown object")
			return
		}
		writeJSON(w, http.StatusOK, describePayload(object.Definition))
	case len(parts) == 2 && parts[1] == "recent" && r.Method == http.MethodGet:
		object, ok := s.Org.Objects[objectName]
		if !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "unknown object")
			return
		}
		writeJSON(w, http.StatusOK, recentPayload(object))
	case len(parts) == 1 && r.Method == http.MethodGet:
		object, ok := s.Org.Objects[objectName]
		if !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "unknown object")
			return
		}
		writeJSON(w, http.StatusOK, object.Definition)
	case len(parts) == 1 && r.Method == http.MethodPost:
		record, err := decodeRecord(r, objectName, "")
		if err != nil {
			writeError(w, http.StatusBadRequest, "JSON_PARSER_ERROR", err.Error())
			return
		}
		engine := dml.NewEngine(s.Org)
		result := engine.Insert([]storage.Record{record})[0]
		if result.Success {
			if err := s.persist(); err != nil {
				writeError(w, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
				return
			}
		}
		writeDMLResult(w, http.StatusCreated, result)
	case len(parts) == 2:
		s.handleRecord(w, r, objectName, storage.ID(parts[1]))
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "unknown sobject endpoint")
	}
}

func (s *Server) handleRecord(w http.ResponseWriter, r *http.Request, objectName string, id storage.ID) {
	object, ok := s.Org.Objects[objectName]
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "unknown object")
		return
	}
	switch r.Method {
	case http.MethodGet:
		record, ok := object.Records[id]
		if !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "record not found")
			return
		}
		writeJSON(w, http.StatusOK, record)
	case http.MethodPatch:
		record, err := decodeRecord(r, objectName, id)
		if err != nil {
			writeError(w, http.StatusBadRequest, "JSON_PARSER_ERROR", err.Error())
			return
		}
		engine := dml.NewEngine(s.Org)
		result := engine.Update([]storage.Record{record})[0]
		if result.Success {
			if err := s.persist(); err != nil {
				writeError(w, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
				return
			}
		}
		writeDMLResult(w, http.StatusNoContent, result)
	case http.MethodDelete:
		engine := dml.NewEngine(s.Org)
		result := engine.Delete([]storage.Record{{Object: objectName, ID: id}})[0]
		if result.Success {
			if err := s.persist(); err != nil {
				writeError(w, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
				return
			}
		}
		writeDMLResult(w, http.StatusNoContent, result)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (s *Server) handleOAER(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) == 1 && parts[0] == "reset" && r.Method == http.MethodPost:
		storage.ResetData(s.Org)
		if err := s.persist(); err != nil {
			writeError(w, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "summary": storage.InspectOrg("", *s.Org)})
	case len(parts) == 1 && parts[0] == "fixture" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, storage.FixtureFromOrg(*s.Org))
	case len(parts) == 1 && parts[0] == "fixture" && r.Method == http.MethodPost:
		fixture, err := storage.ReadFixture(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "JSON_PARSER_ERROR", err.Error())
			return
		}
		if err := storage.ApplyFixture(s.Org, fixture); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_FIXTURE", err.Error())
			return
		}
		if err := s.persist(); err != nil {
			writeError(w, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "summary": storage.InspectOrg("", *s.Org)})
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "unknown oaer endpoint")
	}
}

func (s *Server) handleLimits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"DailyApiRequests":                  map[string]int{"Max": 15000, "Remaining": 15000},
		"DailyAsyncApexExecutions":          map[string]int{"Max": 250000, "Remaining": 250000},
		"ConcurrentAsyncGetReportInstances": map[string]int{"Max": 200, "Remaining": 200},
	})
}

func (s *Server) handleTooling(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) == 1 && parts[0] == "executeAnonymous":
		s.handleExecuteAnonymous(w, r)
	case len(parts) == 1 && parts[0] == "query":
		s.handleQuery(w, r)
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "unknown tooling endpoint")
	}
}

func (s *Server) handleExecuteAnonymous(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	source := r.URL.Query().Get("anonymousBody")
	if source == "" {
		var body struct {
			AnonymousBody string `json:"anonymousBody"`
			Source        string `json:"source"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			source = body.AnonymousBody
			if source == "" {
				source = body.Source
			}
		}
	}
	if source == "" {
		writeJSON(w, http.StatusOK, executeAnonymousFailure(false, "anonymousBody is required", nil))
		return
	}
	program, err := vm.CompileAnonymous(source)
	if err != nil {
		writeJSON(w, http.StatusOK, executeAnonymousFailure(false, err.Error(), nil))
		return
	}
	machine := vm.New(nil)
	machine.SetOrg(s.Org)
	if s.LimitMode != "" {
		machine.SetLimitMode(s.LimitMode)
	}
	if s.LimitCaps != (vm.LimitCaps{}) {
		machine.SetLimitCaps(s.LimitCaps)
	}
	result, err := machine.Execute(program)
	if err != nil {
		writeJSON(w, http.StatusOK, executeAnonymousFailure(true, err.Error(), result.Debug))
		return
	}
	if err := s.persist(); err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"compiled":            true,
		"success":             true,
		"compileProblem":      nil,
		"exceptionMessage":    nil,
		"exceptionStackTrace": nil,
		"line":                -1,
		"column":              -1,
		"logs":                strings.Join(result.Debug, "\n"),
	})
}

func executeAnonymousFailure(compiled bool, message string, logs []string) map[string]any {
	payload := map[string]any{
		"compiled":            compiled,
		"success":             false,
		"line":                1,
		"column":              1,
		"logs":                strings.Join(logs, "\n"),
		"exceptionStackTrace": nil,
	}
	if compiled {
		payload["compileProblem"] = nil
		payload["exceptionMessage"] = message
	} else {
		payload["compileProblem"] = message
		payload["exceptionMessage"] = nil
	}
	return payload
}

func (s *Server) handleComposite(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 1 && parts[0] == "sobjects" && r.Method == http.MethodPost {
		var body struct {
			AllOrNone bool                         `json:"allOrNone"`
			Records   []map[string]json.RawMessage `json:"records"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "JSON_PARSER_ERROR", err.Error())
			return
		}
		records := make([]storage.Record, 0, len(body.Records))
		for _, raw := range body.Records {
			objectName := ""
			if attrsRaw, ok := raw["attributes"]; ok {
				var attrs struct {
					Type string `json:"type"`
				}
				_ = json.Unmarshal(attrsRaw, &attrs)
				objectName = attrs.Type
				delete(raw, "attributes")
			}
			if objectName == "" {
				writeError(w, http.StatusBadRequest, "REQUIRED_FIELD_MISSING", "attributes.type is required")
				return
			}
			record, err := recordFromRawFields(objectName, "", raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "JSON_PARSER_ERROR", err.Error())
				return
			}
			records = append(records, record)
		}
		engine := dml.NewEngine(s.Org)
		backup := s.Org.Clone()
		results := engine.Insert(records)
		hasFailure := false
		hasSuccess := false
		for _, result := range results {
			if !result.Success {
				hasFailure = true
			} else {
				hasSuccess = true
			}
		}
		if body.AllOrNone && hasFailure {
			*s.Org = backup
			writeJSON(w, http.StatusBadRequest, compositeResults(results))
			return
		}
		if hasSuccess {
			if err := s.persist(); err != nil {
				writeError(w, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, compositeResults(results))
		return
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", "unknown composite endpoint")
}

func (s *Server) persist() error {
	if s.Store == nil {
		return nil
	}
	return s.Store.Save(*s.Org)
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	result, err := soql.ParseAndExecute(*s.Org, r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "MALFORMED_QUERY", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"totalSize": result.Rows,
		"done":      true,
		"records":   result.Records,
	})
}

func decodeRecord(r *http.Request, objectName string, id storage.ID) (storage.Record, error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return storage.Record{}, err
	}
	return recordFromRawFields(objectName, id, raw)
}

func recordFromRawFields(objectName string, id storage.ID, raw map[string]json.RawMessage) (storage.Record, error) {
	record := storage.Record{
		ID:            id,
		Object:        objectName,
		Fields:        make(map[string]storage.Value),
		ExplicitNulls: make(map[string]bool),
	}
	for name, rawValue := range raw {
		if name == "Id" || name == "attributes" {
			continue
		}
		value, err := decodeStorageValue(rawValue)
		if err != nil {
			return storage.Record{}, fmt.Errorf("%s: %w", name, err)
		}
		if value.Kind == storage.ValueNull {
			record.ExplicitNulls[name] = true
			continue
		}
		record.Fields[name] = value
	}
	return record, nil
}

func decodeStorageValue(raw json.RawMessage) (storage.Value, error) {
	var value storage.Value
	if err := json.Unmarshal(raw, &value); err == nil && value.Kind != "" {
		return value, nil
	}
	var anyValue any
	if err := json.Unmarshal(raw, &anyValue); err != nil {
		return storage.Value{}, err
	}
	return storageValueFromJSON(anyValue), nil
}

func storageValueFromJSON(value any) storage.Value {
	switch v := value.(type) {
	case nil:
		return storage.NullValue()
	case string:
		return storage.StringValue(v)
	case bool:
		return storage.BooleanValue(v)
	case float64:
		return storage.IntegerValue(int64(v))
	case []any:
		values := make([]storage.Value, 0, len(v))
		for _, item := range v {
			values = append(values, storageValueFromJSON(item))
		}
		return storage.ListValue(values...)
	default:
		return storage.StringValue(fmt.Sprintf("%v", v))
	}
}

func writeDMLResult(w http.ResponseWriter, status int, result dml.Result) {
	if !result.Success {
		writeError(w, http.StatusBadRequest, "DML_EXCEPTION", result.Error)
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, map[string]any{"id": result.ID, "success": true, "errors": []string{}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, []map[string]string{{"errorCode": code, "message": message}})
}

func (s *Server) currentUserID() storage.ID {
	if user := s.currentUser(); user.ID != "" {
		return user.ID
	}
	return "005000000000001"
}

func (s *Server) currentUser() storage.Record {
	object := s.Org.Objects["User"]
	for _, record := range object.Records {
		return record
	}
	return storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"Username": storage.StringValue("system@example.invalid")}}
}

func (s *Server) identityPayload(r *http.Request) map[string]any {
	user := s.currentUser()
	username := "system@example.invalid"
	if value, ok := user.Fields["Username"]; ok && value.Kind == storage.ValueString {
		username = value.String
	}
	base := "http://" + r.Host
	return map[string]any{
		"id":              base + "/id/" + nonEmpty(s.Org.OrgID, "00D000000000001") + "/" + string(user.ID),
		"organization_id": nonEmpty(s.Org.OrgID, "00D000000000001"),
		"user_id":         user.ID,
		"username":        username,
		"display_name":    username,
		"active":          true,
		"user_type":       "STANDARD",
	}
}

func (s *Server) userInfoPayload(r *http.Request) map[string]any {
	identity := s.identityPayload(r)
	return map[string]any{
		"sub":                identity["user_id"],
		"user_id":            identity["user_id"],
		"organization_id":    identity["organization_id"],
		"preferred_username": identity["username"],
		"name":               identity["display_name"],
		"email":              identity["username"],
	}
}

func describePayload(def storage.ObjectDefinition) map[string]any {
	fields := make([]map[string]any, 0, len(def.Fields))
	names := make([]string, 0, len(def.Fields))
	for name := range def.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		field := def.Fields[name]
		fields = append(fields, map[string]any{
			"name":             field.APIName,
			"label":            field.APIName,
			"type":             string(field.Type),
			"nillable":         !field.Required,
			"referenceTo":      field.ReferenceTo,
			"relationshipName": field.RelationshipName,
		})
	}
	return map[string]any{
		"name":       def.APIName,
		"label":      def.Label,
		"keyPrefix":  def.KeyPrefix,
		"fields":     fields,
		"queryable":  true,
		"createable": true,
		"updateable": true,
		"deletable":  true,
	}
}

func recentPayload(object storage.ObjectState) []map[string]any {
	ids := make([]string, 0, len(object.Records))
	for id := range object.Records {
		ids = append(ids, string(id))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	if len(ids) > 25 {
		ids = ids[:25]
	}
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		record := object.Records[storage.ID(id)]
		name := id
		if value, ok := record.Fields["Name"]; ok {
			name = value.String
		}
		out = append(out, map[string]any{"Id": id, "Name": name, "attributes": map[string]any{"type": record.Object}})
	}
	return out
}

func compositeResults(results []dml.Result) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, result := range results {
		row := map[string]any{"id": result.ID, "success": result.Success, "errors": []map[string]string{}}
		if !result.Success {
			row["errors"] = []map[string]string{{"statusCode": "DML_EXCEPTION", "message": result.Error}}
		}
		out = append(out, row)
	}
	return out
}

func nonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func splitPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	out := raw[:0]
	for _, part := range raw {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func URL(addr string) string {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	return fmt.Sprintf("http://%s", addr)
}
