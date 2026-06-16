package server

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func (s *Server) objectNameForRecordID(id storage.ID) string {
	for objectName, object := range s.Org.Objects {
		if stored, ok := object.Records[id]; ok && !stored.System.IsDeleted {
			return objectName
		}
	}
	prefix := ""
	if len(id) >= 3 {
		prefix = string(id[:3])
	}
	match := ""
	for objectName, object := range s.Org.Objects {
		if object.Definition.KeyPrefix != prefix {
			continue
		}
		if match != "" {
			return ""
		}
		match = objectName
	}
	return match
}

func (s *Server) commitOrg(org storage.OrgState) error {
	if err := s.persistOrg(org); err != nil {
		return err
	}
	*s.Org = org
	return nil
}

func (s *Server) newDMLEngine(r *http.Request, org *storage.OrgState) dml.Engine {
	engine := dml.NewEngine(org)
	if user := s.currentUser(r, ""); user.ID != "" {
		engine.UserID = user.ID
	}
	return engine
}

func (s *Server) persistOrg(org storage.OrgState) error {
	if s.Store == nil {
		return nil
	}
	return s.Store.Save(org)
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request, version string, nextPath string, allRows bool) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	if _, ok := r.URL.Query()["explain"]; ok {
		writeSalesforceError(w, errUnsupportedFeature, "SOQL query plan explain is not implemented in the local server")
		return
	}
	queryText := r.URL.Query().Get("q")
	query, err := soql.ParseAtWithFiscalYearStartMonth(queryText, time.Now().UTC(), soql.FiscalYearStartMonth(*s.Org))
	if err != nil {
		writeSalesforceError(w, errMalformedQuery, err.Error())
		return
	}
	if allRows {
		query.AllRows = true
	}
	result, err := soql.Execute(*s.Org, query)
	if err != nil {
		writeSalesforceError(w, errMalformedQuery, err.Error())
		return
	}
	batchSize, paginated, ok := queryBatchSize(r)
	if !ok {
		writeSalesforceError(w, errMalformedQuery, "batchSize must be a positive integer no greater than 2000")
		return
	}
	if !paginated || result.Rows <= batchSize {
		payload := queryResultPayload(result.Rows, true, result.Records, version, "")
		addQueryColumnMetadata(payload, r, query)
		writeJSON(w, http.StatusOK, payload)
		return
	}
	locator := s.storeQueryLocator(queryLocatorState{
		totalSize: result.Rows,
		records:   cloneQueryRecords(result.Records),
		batchSize: batchSize,
		version:   version,
		nextPath:  nextPath,
	})
	payload := queryResultPayload(result.Rows, false, result.Records[:batchSize], version, queryNextURL(version, nextPath, locator, batchSize))
	addQueryColumnMetadata(payload, r, query)
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleQueryMore(w http.ResponseWriter, r *http.Request, token string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	locator, offset, ok := parseQueryLocatorToken(token)
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "query locator not found or expired")
		return
	}
	state, ok := s.queryLocators[locator]
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "query locator not found or expired")
		return
	}
	if offset < 0 || offset >= len(state.records) {
		writeSalesforceError(w, errUnknownEndpoint, "query locator not found or expired")
		return
	}
	end := offset + state.batchSize
	if end > len(state.records) {
		end = len(state.records)
	}
	done := end >= len(state.records)
	nextURL := ""
	if done {
		s.deleteQueryLocator(locator)
	} else {
		nextPath := state.nextPath
		if nextPath == "" {
			nextPath = "query"
		}
		nextURL = queryNextURL(state.version, nextPath, locator, end)
	}
	writeJSON(w, http.StatusOK, queryResultPayload(state.totalSize, done, state.records[offset:end], state.version, nextURL))
}

func queryBatchSize(r *http.Request) (int, bool, bool) {
	values, exists := r.URL.Query()["batchSize"]
	if !exists {
		return maxQueryBatchSize, true, true
	}
	raw := ""
	if len(values) > 0 {
		raw = values[0]
	}
	size, err := strconv.Atoi(raw)
	if err != nil || size <= 0 || size > maxQueryBatchSize {
		return 0, true, false
	}
	return size, true, true
}

func queryResultPayload(totalSize int, done bool, records []storage.Record, version string, nextURL string) map[string]any {
	payload := map[string]any{
		"totalSize": totalSize,
		"done":      done,
		"records":   queryRecordsPayload(records, version),
	}
	if nextURL != "" {
		payload["nextRecordsUrl"] = nextURL
	}
	return payload
}

func addQueryColumnMetadata(payload map[string]any, r *http.Request, query soql.Query) {
	if _, ok := r.URL.Query()["columns"]; !ok {
		return
	}
	payload["columnMetadata"] = queryColumnMetadata(query)
}

func queryColumnMetadata(query soql.Query) []map[string]any {
	columns := make([]map[string]any, 0, len(query.Fields)+len(query.ChildQueries)+len(query.Typeofs))
	for _, field := range query.Fields {
		columns = append(columns, queryFieldColumnMetadata(field))
	}
	for _, childQuery := range query.ChildQueries {
		columns = append(columns, map[string]any{
			"columnName":  childQuery.Relationship,
			"aggregate":   true,
			"joinColumns": queryColumnMetadata(childQuery.Query),
		})
	}
	for _, spec := range query.Typeofs {
		seen := make(map[string]bool)
		for _, fields := range spec.When {
			for _, field := range fields {
				name := spec.Relationship + "." + queryFieldOutputName(field)
				if !seen[strings.ToLower(name)] {
					columns = append(columns, map[string]any{"columnName": name})
					seen[strings.ToLower(name)] = true
				}
			}
		}
		for _, field := range spec.Else {
			name := spec.Relationship + "." + queryFieldOutputName(field)
			if !seen[strings.ToLower(name)] {
				columns = append(columns, map[string]any{"columnName": name})
				seen[strings.ToLower(name)] = true
			}
		}
	}
	return columns
}

func queryFieldColumnMetadata(field string) map[string]any {
	name := queryFieldOutputName(field)
	if queryFieldIsAggregate(field) {
		return map[string]any{
			"columnName":  name,
			"displayName": name,
			"aggregate":   true,
		}
	}
	return map[string]any{"columnName": name}
}

func queryFieldOutputName(field string) string {
	parts := strings.Fields(field)
	if len(parts) == 2 && !strings.Contains(parts[0], "(") {
		return parts[1]
	}
	if len(parts) == 2 && strings.Contains(parts[0], "(") {
		return parts[1]
	}
	return field
}

func queryFieldIsAggregate(field string) bool {
	field = strings.TrimSpace(field)
	open := strings.Index(field, "(")
	if open <= 0 {
		return false
	}
	fn := strings.ToUpper(strings.TrimSpace(field[:open]))
	switch fn {
	case "AVG", "COUNT", "COUNT_DISTINCT", "GROUPING", "MAX", "MIN", "SUM":
		return true
	default:
		return false
	}
}

func queryRecordsPayload(records []storage.Record, version string) []map[string]any {
	out := make([]map[string]any, 0, len(records))
	for _, record := range records {
		row := recordPayload(record, version, record.Object, record.ID)
		for relationship, children := range record.Children {
			row[relationship] = queryResultPayload(len(children), true, children, version, "")
		}
		out = append(out, row)
	}
	return out
}

func cloneQueryRecords(records []storage.Record) []storage.Record {
	if records == nil {
		return nil
	}
	cloned := make([]storage.Record, len(records))
	for i, record := range records {
		cloned[i] = cloneQueryRecord(record)
	}
	return cloned
}

func cloneQueryRecord(record storage.Record) storage.Record {
	record.Fields = cloneStorageValues(record.Fields)
	record.ExplicitNulls = cloneBoolMap(record.ExplicitNulls)
	if record.Children != nil {
		children := make(map[string][]storage.Record, len(record.Children))
		for relationship, records := range record.Children {
			children[relationship] = cloneQueryRecords(records)
		}
		record.Children = children
	}
	return record
}

func cloneStorageValues(values map[string]storage.Value) map[string]storage.Value {
	if values == nil {
		return nil
	}
	cloned := make(map[string]storage.Value, len(values))
	for name, value := range values {
		cloned[name] = cloneStorageValue(value)
	}
	return cloned
}

func cloneStorageValue(value storage.Value) storage.Value {
	if value.List != nil {
		value.List = append([]storage.Value(nil), value.List...)
		for i, item := range value.List {
			value.List[i] = cloneStorageValue(item)
		}
	}
	return value
}

func cloneBoolMap(values map[string]bool) map[string]bool {
	if values == nil {
		return nil
	}
	cloned := make(map[string]bool, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func (s *Server) storeQueryLocator(state queryLocatorState) string {
	if s.queryLocators == nil {
		s.queryLocators = make(map[string]queryLocatorState)
	}
	s.nextQueryID++
	locator := fmt.Sprintf("gladeql%06d", s.nextQueryID)
	s.queryLocators[locator] = state
	s.queryOrder = append(s.queryOrder, locator)
	for len(s.queryOrder) > maxQueryLocators {
		oldest := s.queryOrder[0]
		s.queryOrder = s.queryOrder[1:]
		delete(s.queryLocators, oldest)
	}
	return locator
}

func (s *Server) deleteQueryLocator(locator string) {
	delete(s.queryLocators, locator)
	for i, id := range s.queryOrder {
		if id == locator {
			s.queryOrder = append(s.queryOrder[:i], s.queryOrder[i+1:]...)
			return
		}
	}
}

func queryNextURL(version, nextPath, locator string, offset int) string {
	if nextPath == "" {
		nextPath = "query"
	}
	return fmt.Sprintf("/services/data/%s/%s/%s-%d", version, nextPath, locator, offset)
}

func parseQueryLocatorToken(token string) (string, int, bool) {
	idx := strings.LastIndex(token, "-")
	if idx <= 0 || idx == len(token)-1 {
		return "", 0, false
	}
	offset, err := strconv.Atoi(token[idx+1:])
	if err != nil || offset < 0 {
		return "", 0, false
	}
	return token[:idx], offset, true
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
		if name == "Id" || name == "id" || name == "attributes" {
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
		if math.Trunc(v) != v {
			return storage.DecimalValue(strconv.FormatFloat(v, 'f', -1, 64))
		}
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
		writeDMLFailure(w, result)
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, map[string]any{"id": result.ID, "success": true, "errors": []string{}})
}

func writeExternalIDUpsertResult(w http.ResponseWriter, result dml.Result) {
	if !result.Success {
		writeDMLFailure(w, result)
		return
	}
	if !result.Created {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": result.ID, "success": true, "errors": []string{}, "created": true})
}

func writeDMLFailure(w http.ResponseWriter, result dml.Result) {
	if len(result.Errors) == 0 {
		writeSalesforceError(w, errDMLFailure, result.Error)
		return
	}
	errors := make([]salesforceError, 0, len(result.Errors))
	for _, err := range result.Errors {
		code := err.StatusCode
		if code == "" {
			code = salesforceErrorCode(errDMLFailure)
		}
		message := err.Message
		if message == "" {
			message = result.Error
		}
		errors = append(errors, salesforceError{ErrorCode: code, Message: message, Fields: append([]string(nil), err.Fields...)})
	}
	writeJSON(w, http.StatusBadRequest, errors)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

const defaultLocalUserID = storage.ID("005000000000001")

func (s *Server) currentUserID() storage.ID {
	if user := s.currentUser(nil, ""); user.ID != "" {
		return user.ID
	}
	return defaultLocalUserID
}

func (s *Server) currentUser(r *http.Request, pathUserID storage.ID) storage.Record {
	object := s.Org.Objects["User"]
	if r != nil {
		if record, ok := userRecord(object, selectedUserID(r, pathUserID)); ok {
			return record
		}
	}
	if record, ok := userRecord(object, defaultLocalUserID); ok {
		return record
	}
	ids := make([]string, 0, len(object.Records))
	for id := range object.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	var fallback storage.Record
	for _, id := range ids {
		if record, ok := userRecord(object, storage.ID(id)); ok {
			if strings.EqualFold(userString(record, "UserType", ""), "AutomatedProcess") {
				if fallback.ID == "" {
					fallback = record
				}
				continue
			}
			return record
		}
	}
	if fallback.ID != "" {
		return fallback
	}
	return storage.Record{ID: defaultLocalUserID, Object: "User", Fields: map[string]storage.Value{"Username": storage.StringValue("system@example.invalid")}}
}

func selectedUserID(r *http.Request, pathUserID storage.ID) storage.ID {
	if value := strings.TrimSpace(r.Header.Get("X-GLADE-User-Id")); value != "" {
		return storage.ID(value)
	}
	if value := bearerUserID(r.Header.Get("Authorization")); value != "" {
		return value
	}
	return pathUserID
}

func bearerUserID(header string) storage.ID {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return storage.ID(parts[1])
}

func userRecord(object storage.ObjectState, id storage.ID) (storage.Record, bool) {
	if id == "" {
		return storage.Record{}, false
	}
	record, ok := object.Records[id]
	if !ok {
		return storage.Record{}, false
	}
	if record.ID == "" {
		record.ID = id
	}
	if record.Object == "" {
		record.Object = "User"
	}
	if record.Fields == nil {
		record.Fields = map[string]storage.Value{}
	}
	return record, true
}

func (s *Server) identityPayload(r *http.Request, pathUserID storage.ID) map[string]any {
	user := s.currentUser(r, pathUserID)
	username := userString(user, "Username", "system@example.invalid")
	displayName := userDisplayName(user, username)
	userType := strings.ToUpper(userString(user, "UserType", "STANDARD"))
	active := true
	if value, ok := user.Fields["IsActive"]; ok && value.Kind == storage.ValueBoolean {
		active = value.Boolean
	}
	base := "http://" + r.Host
	version := s.advertisedRESTAPIVersion()
	orgID := nonEmpty(s.Org.OrgID, "00D000000000001")
	userID := string(user.ID)
	identityURL := base + "/id/" + orgID + "/" + userID
	return map[string]any{
		"id":              identityURL,
		"organization_id": orgID,
		"user_id":         user.ID,
		"username":        username,
		"display_name":    displayName,
		"active":          active,
		"user_type":       userType,
		"urls": map[string]any{
			"enterprise": base + "/services/Soap/c/" + version + "/" + orgID,
			"metadata":   base + "/services/Soap/m/" + version + "/" + orgID,
			"partner":    base + "/services/Soap/u/" + version + "/" + orgID,
			"rest":       base + "/services/data/v" + version + "/",
			"sobjects":   base + "/services/data/v" + version + "/sobjects/",
			"search":     base + "/services/data/v" + version + "/search/",
			"query":      base + "/services/data/v" + version + "/query/",
		},
	}
}

func (s *Server) userInfoPayload(r *http.Request) map[string]any {
	user := s.currentUser(r, "")
	identity := s.identityPayload(r, user.ID)
	username := userString(user, "Username", "system@example.invalid")
	return map[string]any{
		"sub":                identity["user_id"],
		"user_id":            identity["user_id"],
		"organization_id":    identity["organization_id"],
		"preferred_username": identity["username"],
		"name":               identity["display_name"],
		"email":              userString(user, "Email", username),
		"email_verified":     true,
		"profile":            identity["id"],
		"picture":            nil,
		"website":            nil,
		"zoneinfo":           "UTC",
		"locale":             "en_US",
		"updated_at":         nil,
		"urls":               identity["urls"],
	}
}

func (s *Server) localTokenPayload(r *http.Request) map[string]any {
	user := s.currentUser(r, "")
	identity := s.identityPayload(r, user.ID)
	base := "http://" + r.Host
	userID := string(user.ID)
	return map[string]any{
		"access_token": localAccessToken(userID),
		"instance_url": base,
		"id":           identity["id"],
		"issued_at":    "0",
		"signature":    "local",
		"token_type":   "Bearer",
		"scope":        "api refresh_token",
	}
}

func localAccessToken(userID string) string {
	return "local-" + userID
}

func userString(user storage.Record, field, fallback string) string {
	value, ok := user.Fields[field]
	if !ok {
		return fallback
	}
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime:
		if value.String != "" {
			return value.String
		}
	case storage.ValueDecimal:
		if value.Decimal != "" {
			return value.Decimal
		}
	case storage.ValueID:
		if value.ID != "" {
			return string(value.ID)
		}
	}
	return fallback
}

func userDisplayName(user storage.Record, username string) string {
	if name := userString(user, "Name", ""); name != "" {
		return name
	}
	first := userString(user, "FirstName", "")
	last := userString(user, "LastName", "")
	if full := strings.TrimSpace(first + " " + last); full != "" {
		return full
	}
	return username
}
