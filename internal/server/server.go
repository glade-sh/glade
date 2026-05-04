package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/open-aer/oaer/internal/dml"
	"github.com/open-aer/oaer/internal/soql"
	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/vm"
)

type Server struct {
	mu    sync.Mutex
	Org   *storage.OrgState
	Store interface {
		Save(storage.OrgState) error
	}
	LimitMode vm.LimitMode
	LimitCaps vm.LimitCaps

	queryLocators map[string]queryLocatorState
	queryOrder    []string
	nextQueryID   int
}

type queryLocatorState struct {
	totalSize int
	records   []storage.Record
	batchSize int
	version   string
}

const (
	maxQueryBatchSize = 2000
	maxQueryLocators  = 32
)

type resetScopeInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	NoOp        bool   `json:"noOp"`
}

var unsupportedRESTNamespaces = map[string]string{
	"actions":   "Actions",
	"analytics": "Analytics",
	"apps":      "Apps",
	"chatter":   "Chatter",
	"connect":   "Connect",
	"metadata":  "Metadata",
	"process":   "Process",
	"support":   "Support",
	"wave":      "Wave",
}

type oaerStatePayload struct {
	LocalOnly   bool                   `json:"localOnly"`
	Summary     storage.InspectSummary `json:"summary"`
	ResetScopes []resetScopeInfo       `json:"resetScopes"`
}

type oaerResetPayload struct {
	Success    bool                   `json:"success"`
	Scopes     []string               `json:"scopes"`
	NoOpScopes []string               `json:"noOpScopes,omitempty"`
	Summary    storage.InspectSummary `json:"summary"`
}

func New(org *storage.OrgState) *Server {
	return &Server{Org: org}
}

func NewWithStore(org *storage.OrgState, store interface{ Save(storage.OrgState) error }) *Server {
	return &Server{Org: org, Store: store}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	parts := splitPath(r.URL.Path)
	if len(parts) == 3 && parts[0] == "services" && parts[1] == "oauth2" && parts[2] == "userinfo" {
		writeJSON(w, http.StatusOK, s.userInfoPayload(r))
		return
	}
	if len(parts) == 3 && parts[0] == "id" {
		writeJSON(w, http.StatusOK, s.identityPayload(r, storage.ID(parts[2])))
		return
	}
	if len(parts) == 2 && parts[0] == "services" && parts[1] == "data" {
		writeJSON(w, http.StatusOK, []map[string]string{{"version": "v61.0", "url": "/services/data/v61.0"}})
		return
	}
	if len(parts) >= 2 && parts[0] == "services" && parts[1] == "apexrest" {
		writeSalesforceError(w, errUnsupportedFeature, "Apex @RestResource dispatch is not implemented in the local server")
		return
	}
	if len(parts) < 3 || parts[0] != "services" || parts[1] != "data" {
		writeSalesforceError(w, errUnknownEndpoint)
		return
	}
	rest := parts[3:]
	if len(parts) == 3 {
		writeJSON(w, http.StatusOK, resourceDiscoveryPayload(parts[2]))
		return
	}
	switch {
	case len(rest) == 1 && rest[0] == "sobjects":
		s.handleSObjects(w, r)
	case len(rest) >= 2 && rest[0] == "sobjects":
		s.handleObject(w, r, rest[1:])
	case len(rest) == 1 && rest[0] == "query":
		s.handleQuery(w, r, parts[2])
	case len(rest) == 2 && rest[0] == "query":
		s.handleQueryMore(w, r, rest[1])
	case len(rest) == 1 && rest[0] == "queryAll":
		s.handleQuery(w, r, parts[2])
	case len(rest) == 1 && rest[0] == "recent":
		s.handleRecent(w, r)
	case len(rest) == 1 && rest[0] == "search":
		writeSalesforceError(w, errUnsupportedFeature, "Search/SOSL is not implemented in the local server")
	case len(rest) == 1 && rest[0] == "limits":
		s.handleLimits(w, r)
	case len(rest) >= 1 && rest[0] == "tooling":
		s.handleTooling(w, r, parts[2], rest[1:])
	case len(rest) >= 1 && rest[0] == "jobs":
		s.handleBulkJobs(w, r, rest[1:])
	case len(rest) >= 1 && rest[0] == "composite":
		s.handleComposite(w, r, rest[1:])
	case len(rest) >= 1 && rest[0] == "oaer":
		s.handleOAER(w, r, rest[1:])
	case len(rest) >= 1:
		if message, ok := unsupportedRESTNamespaceMessage(rest[0]); ok {
			writeSalesforceError(w, errUnsupportedFeature, message)
			return
		}
		writeSalesforceError(w, errUnknownEndpoint)
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func (s *Server) handleSObjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
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

func (s *Server) handleRecent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, recentAllPayload(*s.Org))
}

func (s *Server) handleObject(w http.ResponseWriter, r *http.Request, parts []string) {
	objectName := parts[0]
	switch {
	case len(parts) == 2 && parts[1] == "describe" && r.Method == http.MethodGet:
		object, ok := s.Org.Objects[objectName]
		if !ok {
			writeSalesforceError(w, errUnknownObject)
			return
		}
		writeJSON(w, http.StatusOK, describePayload(object.Definition))
	case isObjectMetadataRoute(parts) && r.Method == http.MethodGet:
		object, ok := s.Org.Objects[objectName]
		if !ok {
			writeSalesforceError(w, errUnknownObject)
			return
		}
		if isCompactLayoutsRoute(parts) {
			writeJSON(w, http.StatusOK, compactLayoutsPayload(object.Definition))
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Full SObject layout metadata is not modeled in the local server; use describe fields and compactLayouts stub data instead")
	case len(parts) == 2 && parts[1] == "recent" && r.Method == http.MethodGet:
		object, ok := s.Org.Objects[objectName]
		if !ok {
			writeSalesforceError(w, errUnknownObject)
			return
		}
		writeJSON(w, http.StatusOK, recentPayload(object))
	case len(parts) == 1 && r.Method == http.MethodGet:
		object, ok := s.Org.Objects[objectName]
		if !ok {
			writeSalesforceError(w, errUnknownObject)
			return
		}
		writeJSON(w, http.StatusOK, object.Definition)
	case len(parts) == 1 && r.Method == http.MethodPost:
		record, err := decodeRecord(r, objectName, "")
		if err != nil {
			writeSalesforceError(w, errMalformedJSON, err.Error())
			return
		}
		next := s.Org.Clone()
		engine := dml.NewEngine(&next)
		result := engine.Insert([]storage.Record{record})[0]
		if result.Success {
			if err := s.commitOrg(next); err != nil {
				writeSalesforceError(w, errStoreFailure, err.Error())
				return
			}
		}
		writeDMLResult(w, http.StatusCreated, result)
	case len(parts) == 2 && parts[1] == "describe":
		writeMethodNotAllowed(w, http.MethodGet)
	case len(parts) == 2 && parts[1] == "recent":
		writeMethodNotAllowed(w, http.MethodGet)
	case isObjectMetadataRoute(parts):
		writeMethodNotAllowed(w, http.MethodGet)
	case len(parts) == 1:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	case len(parts) == 2:
		s.handleRecord(w, r, objectName, storage.ID(parts[1]))
	default:
		writeSalesforceError(w, errUnknownSObject)
	}
}

func (s *Server) handleRecord(w http.ResponseWriter, r *http.Request, objectName string, id storage.ID) {
	object, ok := s.Org.Objects[objectName]
	if !ok {
		writeSalesforceError(w, errUnknownObject)
		return
	}
	switch r.Method {
	case http.MethodGet:
		record, ok := object.Records[id]
		if !ok || record.System.IsDeleted {
			writeSalesforceError(w, errUnknownRecord)
			return
		}
		writeJSON(w, http.StatusOK, recordPayload(record, objectName, id))
	case http.MethodPatch:
		record, err := decodeRecord(r, objectName, id)
		if err != nil {
			writeSalesforceError(w, errMalformedJSON, err.Error())
			return
		}
		next := s.Org.Clone()
		engine := dml.NewEngine(&next)
		result := engine.Update([]storage.Record{record})[0]
		if result.Success {
			if err := s.commitOrg(next); err != nil {
				writeSalesforceError(w, errStoreFailure, err.Error())
				return
			}
		}
		writeDMLResult(w, http.StatusNoContent, result)
	case http.MethodDelete:
		stored, ok := object.Records[id]
		if !ok || stored.System.IsDeleted {
			writeSalesforceError(w, errUnknownRecord)
			return
		}
		next := s.Org.Clone()
		engine := dml.NewEngine(&next)
		result := engine.Delete([]storage.Record{{Object: objectName, ID: id}})[0]
		if result.Success {
			if err := s.commitOrg(next); err != nil {
				writeSalesforceError(w, errStoreFailure, err.Error())
				return
			}
		}
		writeDMLResult(w, http.StatusNoContent, result)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPatch, http.MethodDelete)
	}
}

func (s *Server) handleOAER(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) >= 1 && parts[0] == "reset" && r.Method == http.MethodPost:
		scopes, err := resetScopes(r, parts[1:])
		if err != nil {
			writeSalesforceError(w, errInvalidReset, err.Error())
			return
		}
		next := s.Org.Clone()
		applyResetScopes(&next, scopes)
		if err := s.commitOrg(next); err != nil {
			writeSalesforceError(w, errStoreFailure, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, oaerResetPayload{
			Success:    true,
			Scopes:     scopes,
			NoOpScopes: noOpResetScopes(scopes),
			Summary:    storage.InspectOrg("", *s.Org),
		})
	case len(parts) == 1 && (parts[0] == "state" || parts[0] == "inspect") && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, oaerStatePayload{
			LocalOnly:   true,
			Summary:     storage.InspectOrg("", *s.Org),
			ResetScopes: resetScopeSupport(),
		})
	case len(parts) == 1 && parts[0] == "fixture" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, storage.FixtureFromOrg(*s.Org))
	case len(parts) == 1 && parts[0] == "fixture" && r.Method == http.MethodPost:
		fixture, err := storage.ReadFixture(r.Body)
		if err != nil {
			writeSalesforceError(w, errMalformedJSON, err.Error())
			return
		}
		next := s.Org.Clone()
		if err := storage.ApplyFixture(&next, fixture); err != nil {
			writeSalesforceError(w, errInvalidFixture, err.Error())
			return
		}
		if err := s.commitOrg(next); err != nil {
			writeSalesforceError(w, errStoreFailure, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "summary": storage.InspectOrg("", *s.Org)})
	case len(parts) >= 1 && parts[0] == "reset":
		writeMethodNotAllowed(w, http.MethodPost)
	case len(parts) == 1 && (parts[0] == "state" || parts[0] == "inspect"):
		writeMethodNotAllowed(w, http.MethodGet)
	case len(parts) == 1 && parts[0] == "fixture":
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	default:
		writeSalesforceError(w, errUnknownOAER)
	}
}

func resetScopeSupport() []resetScopeInfo {
	return []resetScopeInfo{
		{Name: "all", Description: "clear local data and restore deterministic platform records"},
		{Name: "data", Description: "clear non-platform object records and ID sequences"},
		{Name: "users", Description: "restore deterministic local users and related platform records"},
		{Name: "platform", Description: "restore deterministic local platform records"},
		{Name: "limits", Description: "accepted for compatibility; governor limits are per VM and have no persisted queue", NoOp: true},
		{Name: "async", Description: "accepted for compatibility; async jobs are not persisted by the local server", NoOp: true},
	}
}

func noOpResetScopes(scopes []string) []string {
	noOps := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		switch scope {
		case "limits", "async":
			noOps = append(noOps, scope)
		}
	}
	return noOps
}

func resetScopes(r *http.Request, pathScopes []string) ([]string, error) {
	scopes := append([]string(nil), pathScopes...)
	for _, raw := range r.URL.Query()["scope"] {
		for _, part := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				scopes = append(scopes, trimmed)
			}
		}
	}
	if len(scopes) == 0 && r.Body != nil {
		var body struct {
			Scopes []string `json:"scopes"`
			Scope  string   `json:"scope"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
			return nil, err
		}
		scopes = append(scopes, body.Scopes...)
		if body.Scope != "" {
			scopes = append(scopes, body.Scope)
		}
	}
	if len(scopes) == 0 {
		scopes = []string{"all"}
	}
	normalized := make([]string, 0, len(scopes))
	seen := make(map[string]bool, len(scopes))
	for _, scope := range scopes {
		for _, part := range strings.Split(scope, ",") {
			name := strings.ToLower(strings.TrimSpace(part))
			if name == "" {
				continue
			}
			switch name {
			case "all", "data", "users", "platform", "limits", "async":
			default:
				return nil, fmt.Errorf("unknown reset scope %q", part)
			}
			if !seen[name] {
				seen[name] = true
				normalized = append(normalized, name)
			}
		}
	}
	if len(normalized) == 0 {
		return []string{"all"}, nil
	}
	return normalized, nil
}

func applyResetScopes(org *storage.OrgState, scopes []string) {
	for _, scope := range scopes {
		if scope == "all" {
			storage.ResetData(org)
			return
		}
	}
	resetPlatform := false
	for _, scope := range scopes {
		switch scope {
		case "data":
			storage.ResetNonPlatformData(org)
		case "users", "platform":
			resetPlatform = true
		case "limits", "async":
			// Limits and async queues are per-VM today; no persistent server state remains.
		}
	}
	if resetPlatform {
		storage.ResetPlatformData(org)
	}
}

func (s *Server) handleLimits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"DailyApiRequests":                  map[string]int{"Max": 15000, "Remaining": 15000},
		"DailyAsyncApexExecutions":          map[string]int{"Max": 250000, "Remaining": 250000},
		"ConcurrentAsyncGetReportInstances": map[string]int{"Max": 200, "Remaining": 200},
	})
}

func (s *Server) handleTooling(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	switch {
	case len(parts) == 1 && parts[0] == "executeAnonymous":
		s.handleExecuteAnonymous(w, r)
	case len(parts) == 1 && parts[0] == "query":
		s.handleQuery(w, r, version)
	case len(parts) == 1 && parts[0] == "sobjects":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Tooling sObject discovery is not implemented in the local server")
	case len(parts) == 3 && parts[0] == "sobjects" && parts[2] == "describe":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Tooling sObject describe for "+parts[1]+" is not implemented in the local server")
	case len(parts) == 1 && parts[0] == "completions":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Tooling completions are not implemented in the local server")
	default:
		writeSalesforceError(w, errUnknownTooling)
	}
}

func (s *Server) handleBulkJobs(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) < 1 {
		writeSalesforceError(w, errUnknownEndpoint)
		return
	}
	switch parts[0] {
	case "query":
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v2 query jobs are not implemented in the local server")
	case "ingest":
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v2 ingest jobs are not implemented in the local server")
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func (s *Server) handleExecuteAnonymous(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
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
	next := s.Org.Clone()
	machine := vm.New(nil)
	machine.SetOrg(&next)
	machine.SetCurrentUser(s.currentUser(r, ""))
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
	if err := s.commitOrg(next); err != nil {
		writeSalesforceError(w, errStoreFailure, err.Error())
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
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			writeSalesforceError(w, errUnsupportedFeature, "Composite namespace discovery is not implemented in the local server; generic REST subrequest orchestration is not modeled")
		case http.MethodPost:
			writeSalesforceError(w, errUnsupportedFeature, "Generic Composite REST subrequest orchestration is not implemented in the local server")
		default:
			writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
		return
	}
	if len(parts) == 1 && parts[0] == "sobjects" {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		var body struct {
			AllOrNone bool                         `json:"allOrNone"`
			Records   []map[string]json.RawMessage `json:"records"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeSalesforceError(w, errMalformedJSON, err.Error())
			return
		}
		records := make([]storage.Record, 0, len(body.Records))
		referenceIDs := make([]string, 0, len(body.Records))
		for _, raw := range body.Records {
			objectName := ""
			referenceID := ""
			if attrsRaw, ok := raw["attributes"]; ok {
				var attrs struct {
					Type        string `json:"type"`
					ReferenceID string `json:"referenceId"`
				}
				_ = json.Unmarshal(attrsRaw, &attrs)
				objectName = attrs.Type
				referenceID = attrs.ReferenceID
				delete(raw, "attributes")
			}
			if objectName == "" {
				writeSalesforceError(w, errRequiredFieldMissing, "attributes.type is required")
				return
			}
			record, err := recordFromRawFields(objectName, "", raw)
			if err != nil {
				writeSalesforceError(w, errMalformedJSON, err.Error())
				return
			}
			records = append(records, record)
			referenceIDs = append(referenceIDs, referenceID)
		}
		next := s.Org.Clone()
		engine := dml.NewEngine(&next)
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
			writeJSON(w, http.StatusBadRequest, compositeResults(results, referenceIDs))
			return
		}
		if hasSuccess {
			if err := s.commitOrg(next); err != nil {
				writeSalesforceError(w, errStoreFailure, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, compositeResults(results, referenceIDs))
		return
	}
	if len(parts) >= 1 && (parts[0] == "batch" || parts[0] == "tree" || parts[0] == "graph") {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Composite "+parts[0]+" is not implemented in the local server")
		return
	}
	writeSalesforceError(w, errUnknownComposite)
}

func (s *Server) commitOrg(org storage.OrgState) error {
	if err := s.persistOrg(org); err != nil {
		return err
	}
	*s.Org = org
	return nil
}

func (s *Server) persistOrg(org storage.OrgState) error {
	if s.Store == nil {
		return nil
	}
	return s.Store.Save(org)
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request, version string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	result, err := soql.ParseAndExecute(*s.Org, r.URL.Query().Get("q"))
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
		writeJSON(w, http.StatusOK, queryResultPayload(result.Rows, true, result.Records, ""))
		return
	}
	locator := s.storeQueryLocator(queryLocatorState{
		totalSize: result.Rows,
		records:   append([]storage.Record(nil), result.Records...),
		batchSize: batchSize,
		version:   version,
	})
	writeJSON(w, http.StatusOK, queryResultPayload(result.Rows, false, result.Records[:batchSize], queryNextURL(version, locator, batchSize)))
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
		nextURL = queryNextURL(state.version, locator, end)
	}
	writeJSON(w, http.StatusOK, queryResultPayload(state.totalSize, done, state.records[offset:end], nextURL))
}

func queryBatchSize(r *http.Request) (int, bool, bool) {
	raw := r.URL.Query().Get("batchSize")
	if raw == "" {
		return 0, false, true
	}
	size, err := strconv.Atoi(raw)
	if err != nil || size <= 0 || size > maxQueryBatchSize {
		return 0, true, false
	}
	return size, true, true
}

func queryResultPayload(totalSize int, done bool, records []storage.Record, nextURL string) map[string]any {
	payload := map[string]any{
		"totalSize": totalSize,
		"done":      done,
		"records":   records,
	}
	if nextURL != "" {
		payload["nextRecordsUrl"] = nextURL
	}
	return payload
}

func (s *Server) storeQueryLocator(state queryLocatorState) string {
	if s.queryLocators == nil {
		s.queryLocators = make(map[string]queryLocatorState)
	}
	s.nextQueryID++
	locator := fmt.Sprintf("oaerql%06d", s.nextQueryID)
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

func queryNextURL(version, locator string, offset int) string {
	return fmt.Sprintf("/services/data/%s/query/%s-%d", version, locator, offset)
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
		writeSalesforceError(w, errDMLFailure, result.Error)
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, map[string]any{"id": result.ID, "success": true, "errors": []string{}})
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
	for _, id := range ids {
		if record, ok := userRecord(object, storage.ID(id)); ok {
			return record
		}
	}
	return storage.Record{ID: defaultLocalUserID, Object: "User", Fields: map[string]storage.Value{"Username": storage.StringValue("system@example.invalid")}}
}

func selectedUserID(r *http.Request, pathUserID storage.ID) storage.ID {
	if value := strings.TrimSpace(r.Header.Get("X-OAER-User-Id")); value != "" {
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
	orgID := nonEmpty(s.Org.OrgID, "00D000000000001")
	return map[string]any{
		"id":              base + "/id/" + orgID + "/" + string(user.ID),
		"organization_id": orgID,
		"user_id":         user.ID,
		"username":        username,
		"display_name":    displayName,
		"active":          active,
		"user_type":       userType,
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
	}
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

func isObjectMetadataRoute(parts []string) bool {
	if len(parts) == 2 {
		return parts[1] == "layouts" || parts[1] == "compactLayouts"
	}
	return len(parts) == 3 && parts[1] == "describe" && parts[2] == "layouts"
}

func isCompactLayoutsRoute(parts []string) bool {
	return len(parts) == 2 && parts[1] == "compactLayouts"
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

func compactLayoutsPayload(def storage.ObjectDefinition) map[string]any {
	return map[string]any{
		"compactLayouts":         []map[string]any{},
		"defaultCompactLayoutId": nil,
		"objectType":             def.APIName,
		"message":                "Compact layout metadata is not modeled; returning an empty local stub.",
	}
}

func recordPayload(record storage.Record, objectName string, id storage.ID) map[string]any {
	if record.Object != "" {
		objectName = record.Object
	}
	if record.ID != "" {
		id = record.ID
	}
	out := map[string]any{
		"attributes": map[string]any{
			"type": objectName,
			"url":  "/services/data/v61.0/sobjects/" + objectName + "/" + string(id),
		},
		"Id": string(id),
	}
	fieldNames := make([]string, 0, len(record.Fields)+len(record.ExplicitNulls))
	seen := make(map[string]bool, len(record.Fields)+len(record.ExplicitNulls))
	for name := range record.Fields {
		if name == "Id" || name == "attributes" {
			continue
		}
		fieldNames = append(fieldNames, name)
		seen[name] = true
	}
	for name, isNull := range record.ExplicitNulls {
		if !isNull || name == "Id" || name == "attributes" || seen[name] {
			continue
		}
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)
	for _, name := range fieldNames {
		if value, ok := record.Fields[name]; ok {
			out[name] = storageValueJSON(value)
			continue
		}
		out[name] = nil
	}
	return out
}

func storageValueJSON(value storage.Value) any {
	switch value.Kind {
	case storage.ValueNull:
		return nil
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime:
		return value.String
	case storage.ValueInteger:
		return value.Integer
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueList:
		items := make([]any, 0, len(value.List))
		for _, item := range value.List {
			items = append(items, storageValueJSON(item))
		}
		return items
	default:
		return nil
	}
}

func recentPayload(object storage.ObjectState) []map[string]any {
	ids := make([]string, 0, len(object.Records))
	for id, record := range object.Records {
		if record.System.IsDeleted {
			continue
		}
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
		objectName := record.Object
		if objectName == "" {
			objectName = object.Definition.APIName
		}
		out = append(out, map[string]any{"Id": id, "Name": name, "attributes": map[string]any{"type": objectName, "url": "/services/data/v61.0/sobjects/" + objectName + "/" + id}})
	}
	return out
}

func recentAllPayload(org storage.OrgState) []map[string]any {
	out := make([]map[string]any, 0)
	for _, object := range org.Objects {
		out = append(out, recentPayload(object)...)
	}
	sort.Slice(out, func(i, j int) bool {
		left, _ := out[i]["Id"].(string)
		right, _ := out[j]["Id"].(string)
		return left > right
	})
	if len(out) > 25 {
		out = out[:25]
	}
	return out
}

func resourceDiscoveryPayload(version string) map[string]string {
	base := "/services/data/" + version
	return map[string]string{
		"composite": base + "/composite",
		"jobs":      base + "/jobs",
		"limits":    base + "/limits",
		"oaer":      base + "/oaer",
		"query":     base + "/query",
		"queryAll":  base + "/queryAll",
		"recent":    base + "/recent",
		"search":    base + "/search",
		"sobjects":  base + "/sobjects",
		"tooling":   base + "/tooling",
	}
}

func unsupportedRESTNamespaceMessage(namespace string) (string, bool) {
	display, ok := unsupportedRESTNamespaces[namespace]
	if !ok {
		return "", false
	}
	return display + " REST namespace is not implemented in the local server", true
}

func compositeResults(results []dml.Result, referenceIDs []string) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for i, result := range results {
		row := map[string]any{"id": result.ID, "success": result.Success, "errors": []map[string]string{}}
		if i < len(referenceIDs) && referenceIDs[i] != "" {
			row["referenceId"] = referenceIDs[i]
		}
		if !result.Success {
			row["errors"] = []map[string]string{{"statusCode": salesforceErrorCode(errDMLFailure), "message": result.Error}}
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
