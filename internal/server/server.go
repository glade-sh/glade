package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

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
	nextPath  string
}

const (
	maxQueryBatchSize = 2000
	maxQueryLocators  = 32
)

type apiVersionEntry struct {
	Version string `json:"version"`
	Label   string `json:"label"`
	URL     string `json:"url"`
}

var localAPIVersions = []apiVersionEntry{
	{Version: "61.0", Label: "Summer '24", URL: "/services/data/v61.0"},
}

const localOAuthUnsupportedMessage = "Full OAuth flows and token issuance are not implemented by the local server; use deterministic local user stubs via /services/oauth2/userinfo, /id/{org}/{user}, X-OAER-User-Id, or Authorization: Bearer <userId>"

const apexRestUnsupportedMessage = "Apex @RestResource dispatch is not implemented in the local server"

var apexRestAllowedMethods = []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete}

type resetScopeInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	NoOp        bool   `json:"noOp"`
}

var unsupportedRESTNamespaces = map[string]string{
	"actions":      "Actions",
	"analytics":    "Analytics",
	"appMenu":      "AppMenu",
	"apps":         "Apps",
	"chatter":      "Chatter",
	"connect":      "Connect",
	"metadata":     "Metadata",
	"process":      "Process",
	"quickActions": "QuickActions",
	"support":      "Support",
	"tabs":         "Tabs",
	"theme":        "Theme",
	"wave":         "Wave",
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

type fixtureLoadMode string

const (
	fixtureLoadModeMerge   fixtureLoadMode = "merge"
	fixtureLoadModeReplace fixtureLoadMode = "replace"
)

type oaerDiscoveryPayload struct {
	LocalOnly bool              `json:"localOnly"`
	URLs      map[string]string `json:"urls"`
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
	parts := splitPath(r.URL.EscapedPath())
	if len(parts) >= 2 && parts[0] == "services" && parts[1] == "oauth2" {
		s.handleOAuth(w, r, parts[2:])
		return
	}
	if len(parts) == 3 && parts[0] == "id" {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, s.identityPayload(r, storage.ID(parts[2])))
		return
	}
	if len(parts) == 2 && parts[0] == "services" && parts[1] == "data" {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, apiVersionDiscoveryPayload())
		return
	}
	if len(parts) >= 2 && parts[0] == "services" && parts[1] == "apexrest" {
		if !methodAllowed(r, apexRestAllowedMethods...) {
			writeMethodNotAllowed(w, apexRestAllowedMethods...)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, apexRestUnsupportedMessage)
		return
	}
	if len(parts) < 3 || parts[0] != "services" || parts[1] != "data" {
		writeSalesforceError(w, errUnknownEndpoint)
		return
	}
	rest := parts[3:]
	if len(parts) == 3 {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, resourceDiscoveryPayload(parts[2]))
		return
	}
	switch {
	case len(rest) == 1 && rest[0] == "sobjects":
		s.handleSObjects(w, r, parts[2])
	case len(rest) >= 2 && rest[0] == "sobjects":
		s.handleObject(w, r, parts[2], rest[1:])
	case len(rest) == 1 && rest[0] == "query":
		s.handleQuery(w, r, parts[2], "query", false)
	case len(rest) == 2 && rest[0] == "query":
		s.handleQueryMore(w, r, rest[1])
	case len(rest) == 1 && rest[0] == "queryAll":
		s.handleQuery(w, r, parts[2], "query", true)
	case len(rest) == 1 && rest[0] == "recent":
		s.handleRecent(w, r, parts[2])
	case len(rest) == 1 && rest[0] == "search":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Search/SOSL is not implemented in the local server")
	case len(rest) == 1 && rest[0] == "limits":
		s.handleLimits(w, r)
	case len(rest) >= 1 && rest[0] == "tooling":
		s.handleTooling(w, r, parts[2], rest[1:])
	case len(rest) >= 1 && rest[0] == "jobs":
		s.handleBulkJobs(w, r, parts[2], rest[1:])
	case len(rest) >= 1 && rest[0] == "metadata":
		writeUnsupportedMetadataREST(w, r, parts[2], rest[1:])
	case len(rest) >= 1 && rest[0] == "composite":
		s.handleComposite(w, r, parts[2], rest[1:])
	case len(rest) >= 1 && rest[0] == "oaer":
		s.handleOAER(w, r, rest[1:])
	case len(rest) >= 1:
		if message, ok := unsupportedRESTNamespaceMessage(rest[0]); ok {
			if r.Method != http.MethodGet {
				writeMethodNotAllowed(w, http.MethodGet)
				return
			}
			writeSalesforceError(w, errUnsupportedFeature, message)
			return
		}
		writeSalesforceError(w, errUnknownEndpoint)
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func (s *Server) handleOAuth(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) != 1 {
		writeSalesforceError(w, errUnknownEndpoint)
		return
	}
	switch parts[0] {
	case "userinfo":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, s.userInfoPayload(r))
	case "token", "revoke", "introspect":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, localOAuthUnsupportedMessage)
	case "authorize":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, localOAuthUnsupportedMessage)
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func (s *Server) handleSObjects(w http.ResponseWriter, r *http.Request, version string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	base := "/services/data/" + version + "/sobjects/"
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
			"url":         base + name,
			"describe":    base + name + "/describe",
			"recentItems": base + name + "/recent",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sobjects": objects})
}

func (s *Server) handleRecent(w http.ResponseWriter, r *http.Request, version string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	limit, err := recentLimit(r)
	if err != nil {
		writeSalesforceError(w, errMalformedQuery, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, recentAllPayload(*s.Org, version, limit))
}

func (s *Server) handleObject(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	objectName := parts[0]
	switch {
	case len(parts) == 2 && parts[1] == "describe" && r.Method == http.MethodGet:
		object, ok := s.Org.Objects[objectName]
		if !ok {
			writeSalesforceError(w, errUnknownObject)
			return
		}
		writeJSON(w, http.StatusOK, describePayload(object.Definition, s.Org))
	case isObjectMetadataRoute(parts):
		object, ok := s.Org.Objects[objectName]
		if !ok {
			writeSalesforceError(w, errUnknownObject)
			return
		}
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		if isCompactLayoutsRoute(parts) {
			writeJSON(w, http.StatusOK, compactLayoutsPayload(object.Definition))
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Full SObject layout and layout-adjacent metadata is not modeled in the local server; use describe fields and compactLayouts stub data instead")
	case isDefaultValuesRoute(parts) && r.Method == http.MethodGet:
		if _, ok := s.Org.Objects[objectName]; !ok {
			writeSalesforceError(w, errUnknownObject)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "SObject default value metadata is not modeled in the local server; create records with explicit field values instead")
	case isQuickActionsRoute(parts):
		if _, ok := s.Org.Objects[objectName]; !ok {
			writeSalesforceError(w, errUnknownObject)
			return
		}
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "SObject quick action metadata and default values are not modeled in the local server")
	case isListViewsRoute(parts):
		object, ok := s.Org.Objects[objectName]
		if !ok {
			writeSalesforceError(w, errUnknownObject)
			return
		}
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		if isListViewCollectionRoute(parts) {
			writeJSON(w, http.StatusOK, listViewsPayload(object.Definition, version))
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "SObject list view describe and result execution are not modeled in the local server; collection discovery returns an empty local stub")
	case len(parts) == 2 && parts[1] == "recent" && r.Method == http.MethodGet:
		object, ok := s.Org.Objects[objectName]
		if !ok {
			writeSalesforceError(w, errUnknownObject)
			return
		}
		limit, err := recentLimit(r)
		if err != nil {
			writeSalesforceError(w, errMalformedQuery, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, recentPayload(object, version, limit))
	case len(parts) == 2 && (parts[1] == "updated" || parts[1] == "deleted"):
		object, ok := s.Org.Objects[objectName]
		if !ok {
			writeSalesforceError(w, errUnknownObject)
			return
		}
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		if parts[1] == "updated" {
			payload, err := updatedPayload(object, r)
			if err != nil {
				writeSalesforceError(w, errMalformedQuery, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, payload)
			return
		}
		payload, err := deletedPayload(object, r)
		if err != nil {
			writeSalesforceError(w, errMalformedQuery, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, payload)
	case len(parts) == 1 && r.Method == http.MethodGet:
		object, ok := s.Org.Objects[objectName]
		if !ok {
			writeSalesforceError(w, errUnknownObject)
			return
		}
		writeJSON(w, http.StatusOK, objectResourcePayload(object.Definition, version))
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
	case isDefaultValuesRoute(parts):
		writeMethodNotAllowed(w, http.MethodGet)
	case len(parts) == 1:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	case len(parts) == 2:
		id := storage.ID(parts[1])
		if isRowTemplatePlaceholder(id) {
			writeSalesforceError(w, errMalformedID, "rowTemplate placeholder {ID} is not a record id; replace it with a 15- or 18-character record id")
			return
		}
		s.handleRecord(w, r, version, objectName, id)
	case len(parts) == 3:
		s.handleExternalIDRecord(w, r, version, objectName, parts[1], parts[2])
	default:
		writeSalesforceError(w, errUnknownSObject)
	}
}

func (s *Server) handleExternalIDRecord(w http.ResponseWriter, r *http.Request, version string, objectName, externalIDField, externalIDValue string) {
	object, objectName, fieldName, field, value, ok := s.externalIDRoute(w, objectName, externalIDField, externalIDValue)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		projection, projected, ok := recordFieldProjectionFromRequest(w, r, object.Definition, s.Org.Namespace)
		if !ok {
			return
		}
		record, id, matches := findExternalIDRecord(object, fieldName, field, value)
		if !writeExternalIDLookupResult(w, objectName, fieldName, matches) {
			return
		}
		writeJSON(w, http.StatusOK, recordPayloadWithProjection(record, version, objectName, id, projection, projected))
	case http.MethodPatch:
		record, err := decodeRecord(r, objectName, "")
		if err != nil {
			writeSalesforceError(w, errMalformedJSON, err.Error())
			return
		}
		if existing, ok := record.Fields[fieldName]; ok {
			if !storageValuesEqual(field, existing, value) {
				writeSalesforceError(w, errDMLFailure, fmt.Sprintf("external id field %s value does not match path value", fieldName))
				return
			}
		} else if record.ExplicitNulls[fieldName] {
			writeSalesforceError(w, errDMLFailure, fmt.Sprintf("external id field %s value does not match path value", fieldName))
			return
		} else {
			record.Fields[fieldName] = value
		}
		delete(record.ExplicitNulls, fieldName)
		next := s.Org.Clone()
		engine := dml.NewEngine(&next)
		result := engine.UpsertWithExternalID([]storage.Record{record}, fieldName)[0]
		if result.Success {
			if err := s.commitOrg(next); err != nil {
				writeSalesforceError(w, errStoreFailure, err.Error())
				return
			}
		}
		writeExternalIDUpsertResult(w, result)
	case http.MethodDelete:
		_, id, matches := findExternalIDRecord(object, fieldName, field, value)
		if !writeExternalIDLookupResult(w, objectName, fieldName, matches) {
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

func (s *Server) externalIDRoute(w http.ResponseWriter, objectName, externalIDField, externalIDValue string) (storage.ObjectState, string, string, storage.Field, storage.Value, bool) {
	resolvedObjectName, ok := storage.ResolveObjectName(*s.Org, objectName)
	if !ok {
		writeSalesforceError(w, errUnknownObject)
		return storage.ObjectState{}, "", "", storage.Field{}, storage.Value{}, false
	}
	object := s.Org.Objects[resolvedObjectName]
	fieldName, ok := storage.ResolveFieldName(object.Definition, s.Org.Namespace, externalIDField)
	if !ok {
		writeSalesforceError(w, errInvalidField, fmt.Sprintf("external id field %s.%s does not exist", resolvedObjectName, externalIDField))
		return storage.ObjectState{}, "", "", storage.Field{}, storage.Value{}, false
	}
	field, ok := object.Definition.Fields[fieldName]
	if !ok || !field.ExternalID {
		writeSalesforceError(w, errInvalidField, fmt.Sprintf("field %s.%s is not an external id", resolvedObjectName, fieldName))
		return storage.ObjectState{}, "", "", storage.Field{}, storage.Value{}, false
	}
	if strings.TrimSpace(externalIDValue) == "" {
		writeSalesforceError(w, errRequiredFieldMissing, "external id value is required")
		return storage.ObjectState{}, "", "", storage.Field{}, storage.Value{}, false
	}
	value, err := externalIDValueFromPath(field, externalIDValue)
	if err != nil {
		writeSalesforceError(w, errInvalidField, err.Error())
		return storage.ObjectState{}, "", "", storage.Field{}, storage.Value{}, false
	}
	return object, resolvedObjectName, fieldName, field, value, true
}

func (s *Server) handleRecord(w http.ResponseWriter, r *http.Request, version string, objectName string, id storage.ID) {
	object, ok := s.Org.Objects[objectName]
	if !ok {
		writeSalesforceError(w, errUnknownObject)
		return
	}
	switch r.Method {
	case http.MethodGet:
		projection, projected, ok := recordFieldProjectionFromRequest(w, r, object.Definition, s.Org.Namespace)
		if !ok {
			return
		}
		record, ok := object.Records[id]
		if !ok || record.System.IsDeleted {
			writeSalesforceError(w, errUnknownRecord)
			return
		}
		writeJSON(w, http.StatusOK, recordPayloadWithProjection(record, version, objectName, id, projection, projected))
	case http.MethodPatch:
		stored, ok := object.Records[id]
		if !ok || stored.System.IsDeleted {
			writeSalesforceError(w, errUnknownRecord)
			return
		}
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
	case len(parts) == 0 && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, oaerDiscovery(versionFromRequest(r)))
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
		if err := validateFixtureExportRequest(r); err != nil {
			writeSalesforceError(w, errInvalidFixture, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, storage.FixtureFromOrg(*s.Org))
	case len(parts) == 1 && parts[0] == "fixture" && r.Method == http.MethodPost:
		mode, err := requestedFixtureLoadMode(r)
		if err != nil {
			writeSalesforceError(w, errInvalidFixture, err.Error())
			return
		}
		fixture, err := storage.ReadFixture(r.Body)
		if err != nil {
			var versionErr storage.UnsupportedFixtureVersionError
			if errors.As(err, &versionErr) {
				writeSalesforceError(w, errInvalidFixture, err.Error())
				return
			}
			writeSalesforceError(w, errMalformedJSON, err.Error())
			return
		}
		next := s.Org.Clone()
		if mode == fixtureLoadModeReplace {
			storage.ResetData(&next)
		}
		if err := storage.ApplyFixture(&next, fixture); err != nil {
			writeSalesforceError(w, errInvalidFixture, err.Error())
			return
		}
		if err := s.commitOrg(next); err != nil {
			writeSalesforceError(w, errStoreFailure, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "mode": mode, "summary": storage.InspectOrg("", *s.Org)})
	case len(parts) == 0:
		writeMethodNotAllowed(w, http.MethodGet)
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

func requestedFixtureLoadMode(r *http.Request) (fixtureLoadMode, error) {
	values, ok := r.URL.Query()["mode"]
	if !ok {
		return fixtureLoadModeMerge, nil
	}
	if len(values) != 1 {
		return "", fmt.Errorf("fixture load mode must be specified once")
	}
	mode := strings.ToLower(strings.TrimSpace(values[0]))
	switch mode {
	case "", "merge":
		return fixtureLoadModeMerge, nil
	case "replace":
		return fixtureLoadModeReplace, nil
	default:
		return "", fmt.Errorf("unknown fixture load mode %q; supported modes are merge and replace", values[0])
	}
}

func validateFixtureExportRequest(r *http.Request) error {
	if _, ok := r.URL.Query()["mode"]; ok {
		return fmt.Errorf("fixture export does not accept load mode; omit mode or POST the fixture with mode=merge or mode=replace")
	}
	return nil
}

func oaerDiscovery(version string) oaerDiscoveryPayload {
	base := "/services/data/" + version + "/oaer"
	return oaerDiscoveryPayload{
		LocalOnly: true,
		URLs: map[string]string{
			"fixture": base + "/fixture",
			"inspect": base + "/inspect",
			"reset":   base + "/reset",
			"state":   base + "/state",
		},
	}
}

func versionFromRequest(r *http.Request) string {
	parts := splitPath(r.URL.EscapedPath())
	if len(parts) >= 3 && parts[0] == "services" && parts[1] == "data" {
		return parts[2]
	}
	return localAPIVersions[0].Version
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
	bodyScopes, err := resetBodyScopes(r)
	if err != nil {
		return nil, err
	}
	scopes = append(scopes, bodyScopes...)
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

func resetBodyScopes(r *http.Request) ([]string, error) {
	if r.Body == nil || r.Body == http.NoBody {
		return nil, nil
	}
	var body struct {
		Scopes []string `json:"scopes"`
		Scope  string   `json:"scope"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			scopes := append([]string(nil), body.Scopes...)
			if body.Scope != "" {
				scopes = append(scopes, body.Scope)
			}
			return scopes, nil
		}
		return nil, err
	}
	return nil, fmt.Errorf("reset body must contain a single JSON object")
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

type localAPILimit struct {
	Max       int `json:"Max"`
	Remaining int `json:"Remaining"`
}

func localAPILimitValue(max int) localAPILimit {
	return localAPILimit{Max: max, Remaining: max}
}

// localAPILimits is a deterministic compatibility payload, not live org accounting.
var localAPILimits = map[string]localAPILimit{
	"ConcurrentAsyncGetReportInstances":           localAPILimitValue(200),
	"ConcurrentSyncReportRuns":                    localAPILimitValue(20),
	"DailyApiRequests":                            localAPILimitValue(15000),
	"DailyAsyncApexExecutions":                    localAPILimitValue(250000),
	"DailyBulkApiBatches":                         localAPILimitValue(15000),
	"DailyBulkV2IngestJobs":                       localAPILimitValue(10000),
	"DailyBulkV2QueryJobs":                        localAPILimitValue(10000),
	"DailyDurableGenericStreamingApiEvents":       localAPILimitValue(10000),
	"DailyStreamingApiEvents":                     localAPILimitValue(10000),
	"DataStorageMB":                               localAPILimitValue(512),
	"FileStorageMB":                               localAPILimitValue(512),
	"HourlyAsyncReportRuns":                       localAPILimitValue(1200),
	"HourlyDashboardRefreshes":                    localAPILimitValue(200),
	"HourlyDashboardResults":                      localAPILimitValue(5000),
	"HourlyDashboardStatuses":                     localAPILimitValue(1000),
	"HourlyLongTermIdMapping":                     localAPILimitValue(100000),
	"HourlyODataCallout":                          localAPILimitValue(1000),
	"HourlyPublishedPlatformEvents":               localAPILimitValue(100000),
	"HourlyPublishedStandardVolumePlatformEvents": localAPILimitValue(100000),
	"MassEmail":                                   localAPILimitValue(5000),
	"Package2VersionCreates":                      localAPILimitValue(6),
	"PermissionSets":                              localAPILimitValue(1000),
	"SingleEmail":                                 localAPILimitValue(5000),
	"StreamingApiConcurrentClients":               localAPILimitValue(20),
}

func (s *Server) handleLimits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, localAPILimits)
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
		s.handleQuery(w, r, version, "tooling/query", false)
	case len(parts) == 1 && parts[0] == "queryAll":
		s.handleQuery(w, r, version, "tooling/query", true)
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
		writeSalesforceError(w, errUnsupportedFeature, "Tooling sObject discovery is not implemented in the local server")
	case len(parts) == 3 && parts[0] == "sobjects" && parts[2] == "describe":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Tooling sObject describe for "+parts[1]+" is not implemented in the local server")
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
	writeSalesforceError(w, errUnsupportedFeature, "Tooling "+endpoint+" is not implemented in the local server; use oaer test for local Apex test execution")
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
		writeSalesforceError(w, errUnsupportedFeature, "Metadata REST component read and discovery are not implemented in the local server; use source files and oaer inspect/check for local metadata state")
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
		writeSalesforceError(w, errUnsupportedFeature, "Metadata REST deploy requests are not implemented in the local server; use source files and oaer check/test for local validation")
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

func requireWellFormedJSONBody(w http.ResponseWriter, r *http.Request) bool {
	var body any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return false
	}
	return true
}

type compositeSubrequestEnvelope struct {
	Method      string `json:"method"`
	URL         string `json:"url"`
	ReferenceID string `json:"referenceId"`
}

func requireCompositeRequestEnvelope(w http.ResponseWriter, r *http.Request) bool {
	var body struct {
		CompositeRequest *[]compositeSubrequestEnvelope `json:"compositeRequest"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return false
	}
	if body.CompositeRequest == nil || len(*body.CompositeRequest) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "compositeRequest is required and must contain at least one subrequest")
		return false
	}
	return validateCompositeSubrequests(w, *body.CompositeRequest, "compositeRequest", true)
}

func requireCompositeBatchEnvelope(w http.ResponseWriter, r *http.Request) bool {
	var body struct {
		BatchRequests *[]compositeSubrequestEnvelope `json:"batchRequests"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return false
	}
	if body.BatchRequests == nil || len(*body.BatchRequests) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "batchRequests is required and must contain at least one subrequest")
		return false
	}
	return validateCompositeSubrequests(w, *body.BatchRequests, "batchRequests", false)
}

func requireCompositeTreeEnvelope(w http.ResponseWriter, r *http.Request) bool {
	var body struct {
		Records *[]map[string]json.RawMessage `json:"records"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return false
	}
	if body.Records == nil || len(*body.Records) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "records is required and must contain at least one tree record")
		return false
	}
	for i, record := range *body.Records {
		attrsRaw, ok := record["attributes"]
		if !ok {
			writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("records[%d].attributes.referenceId is required", i))
			return false
		}
		var attrs struct {
			ReferenceID string `json:"referenceId"`
		}
		if err := json.Unmarshal(attrsRaw, &attrs); err != nil {
			writeSalesforceError(w, errMalformedJSON, "attributes must be a JSON object")
			return false
		}
		if strings.TrimSpace(attrs.ReferenceID) == "" {
			writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("records[%d].attributes.referenceId is required", i))
			return false
		}
	}
	return true
}

func requireCompositeGraphEnvelope(w http.ResponseWriter, r *http.Request) bool {
	var body struct {
		Graphs *[]struct {
			GraphID          string                         `json:"graphId"`
			CompositeRequest *[]compositeSubrequestEnvelope `json:"compositeRequest"`
		} `json:"graphs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return false
	}
	if body.Graphs == nil || len(*body.Graphs) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "graphs is required and must contain at least one graph")
		return false
	}
	for i, graph := range *body.Graphs {
		if strings.TrimSpace(graph.GraphID) == "" {
			writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("graphs[%d].graphId is required", i))
			return false
		}
		if graph.CompositeRequest == nil || len(*graph.CompositeRequest) == 0 {
			writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("graphs[%d].compositeRequest is required and must contain at least one subrequest", i))
			return false
		}
		if !validateCompositeSubrequests(w, *graph.CompositeRequest, fmt.Sprintf("graphs[%d].compositeRequest", i), true) {
			return false
		}
	}
	return true
}

func validateCompositeSubrequests(w http.ResponseWriter, requests []compositeSubrequestEnvelope, field string, requireReferenceID bool) bool {
	for i, request := range requests {
		prefix := fmt.Sprintf("%s[%d]", field, i)
		if strings.TrimSpace(request.Method) == "" {
			writeSalesforceError(w, errRequiredFieldMissing, prefix+".method is required")
			return false
		}
		if strings.TrimSpace(request.URL) == "" {
			writeSalesforceError(w, errRequiredFieldMissing, prefix+".url is required")
			return false
		}
		if requireReferenceID && strings.TrimSpace(request.ReferenceID) == "" {
			writeSalesforceError(w, errRequiredFieldMissing, prefix+".referenceId is required")
			return false
		}
	}
	return true
}

func methodAllowed(r *http.Request, allowed ...string) bool {
	for _, method := range allowed {
		if r.Method == method {
			return true
		}
	}
	return false
}

func (s *Server) handleExecuteAnonymous(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
		return
	}
	source := r.URL.Query().Get("anonymousBody")
	if source == "" && r.Method == http.MethodPost {
		var err error
		source, err = executeAnonymousBodySource(r)
		if err != nil {
			writeJSON(w, http.StatusOK, executeAnonymousFailure(false, err.Error(), nil))
			return
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

func executeAnonymousBodySource(r *http.Request) (string, error) {
	contentType := requestContentType(r)
	switch contentType {
	case "application/json":
		return executeAnonymousJSONSource(r)
	case "application/x-www-form-urlencoded":
		return executeAnonymousFormSource(r)
	default:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return "", err
		}
		if len(strings.TrimSpace(string(body))) == 0 {
			return "", nil
		}
		source, formErr := executeAnonymousFormEncodedSource(string(body))
		if source != "" {
			return source, nil
		}
		source, err = executeAnonymousJSONBytesSource(body)
		if err != nil {
			if formErr != nil {
				return "", formErr
			}
			return "", err
		}
		return source, nil
	}
}

func requestContentType(r *http.Request) string {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return ""
	}
	return strings.ToLower(contentType)
}

func executeAnonymousJSONSource(r *http.Request) (string, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	return executeAnonymousJSONBytesSource(body)
}

func executeAnonymousJSONBytesSource(body []byte) (string, error) {
	var payload struct {
		AnonymousBody string `json:"anonymousBody"`
		Source        string `json:"source"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.AnonymousBody != "" {
		return payload.AnonymousBody, nil
	}
	return payload.Source, nil
}

func executeAnonymousFormSource(r *http.Request) (string, error) {
	if err := r.ParseForm(); err != nil {
		return "", err
	}
	if source := r.PostForm.Get("anonymousBody"); source != "" {
		return source, nil
	}
	return r.PostForm.Get("source"), nil
}

func executeAnonymousFormEncodedSource(body string) (string, error) {
	form, err := url.ParseQuery(body)
	if err != nil {
		return "", err
	}
	if source := form.Get("anonymousBody"); source != "" {
		return source, nil
	}
	return form.Get("source"), nil
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

func (s *Server) handleComposite(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			writeSalesforceError(w, errUnsupportedFeature, "Composite namespace discovery is not implemented in the local server; generic REST subrequest orchestration is not modeled")
		case http.MethodPost:
			if !requireCompositeRequestEnvelope(w, r) {
				return
			}
			writeSalesforceError(w, errUnsupportedFeature, "Generic Composite REST subrequest orchestration is not implemented in the local server")
		default:
			writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
		return
	}
	if len(parts) >= 1 && parts[0] == "sobjects" {
		s.handleCompositeSObjects(w, r, version, parts)
		return
	}
	if len(parts) >= 1 && parts[0] == "batch" {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		if !requireCompositeBatchEnvelope(w, r) {
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Composite batch is not implemented in the local server")
		return
	}
	if len(parts) >= 1 && parts[0] == "tree" {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			writeSalesforceError(w, errRequiredFieldMissing, "object name is required for Composite tree")
			return
		}
		if !requireCompositeTreeEnvelope(w, r) {
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Composite tree is not implemented in the local server")
		return
	}
	if len(parts) >= 1 && parts[0] == "graph" {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		if !requireCompositeGraphEnvelope(w, r) {
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Composite graph is not implemented in the local server")
		return
	}
	writeSalesforceError(w, errUnknownComposite)
}

func (s *Server) handleCompositeSObjects(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodPost:
			s.handleCompositeSObjectInsert(w, r)
			return
		case http.MethodPatch:
			s.handleCompositeSObjectUpdate(w, r)
			return
		case http.MethodDelete:
			s.handleCompositeSObjectDelete(w, r)
			return
		default:
			writeMethodNotAllowed(w, http.MethodPost, http.MethodPatch, http.MethodDelete)
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		if len(parts) == 2 {
			s.handleCompositeSObjectTypedRetrieve(w, r, version, parts[1])
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Composite sObject typed retrieve routes beyond object collections are not implemented in the local server")
		return
	case http.MethodPost:
		writeSalesforceError(w, errUnsupportedFeature, "Composite sObject typed collection routes are not implemented in the local server")
		return
	case http.MethodPatch:
		if len(parts) == 3 {
			s.handleCompositeSObjectTypedUpsert(w, r, parts[1], parts[2])
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Composite sObject collection upsert routes are not implemented in the local server")
		return
	case http.MethodDelete:
		writeSalesforceError(w, errUnsupportedFeature, "Composite sObject collection delete routes are not implemented in the local server")
		return
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete)
		return
	}
}

func (s *Server) handleCompositeSObjectInsert(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeCompositeSObjectBody(w, r, false)
	if !ok {
		return
	}
	next := s.Org.Clone()
	engine := dml.NewEngine(&next)
	results := engine.Insert(body.Records)
	s.writeCompositeMutationResults(w, next, body.AllOrNone, results, body.ReferenceIDs)
}

func (s *Server) handleCompositeSObjectUpdate(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeCompositeSObjectBody(w, r, true)
	if !ok {
		return
	}
	next := s.Org.Clone()
	engine := dml.NewEngine(&next)
	results := engine.Update(body.Records)
	s.writeCompositeMutationResults(w, next, body.AllOrNone, results, body.ReferenceIDs)
}

func (s *Server) handleCompositeSObjectDelete(w http.ResponseWriter, r *http.Request) {
	idsParam := strings.TrimSpace(r.URL.Query().Get("ids"))
	if idsParam == "" {
		writeSalesforceError(w, errRequiredFieldMissing, "ids query parameter is required for local Composite sObject collection delete")
		return
	}
	allOrNone := strings.EqualFold(r.URL.Query().Get("allOrNone"), "true")
	ids := strings.Split(idsParam, ",")
	records := make([]storage.Record, 0, len(ids))
	for _, rawID := range ids {
		id := storage.ID(strings.TrimSpace(rawID))
		if id == "" {
			writeSalesforceError(w, errMalformedID, "ids query parameter contains an empty record id")
			return
		}
		objectName := s.objectNameForRecordID(id)
		records = append(records, storage.Record{Object: objectName, ID: id})
	}
	next := s.Org.Clone()
	engine := dml.NewEngine(&next)
	results := engine.Delete(records)
	s.writeCompositeMutationResults(w, next, allOrNone, results, nil)
}

func (s *Server) writeCompositeMutationResults(w http.ResponseWriter, next storage.OrgState, allOrNone bool, results []dml.Result, referenceIDs []string) {
	hasFailure := false
	hasSuccess := false
	for _, result := range results {
		if !result.Success {
			hasFailure = true
		} else {
			hasSuccess = true
		}
	}
	if allOrNone && hasFailure {
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
}

func (s *Server) handleCompositeSObjectTypedRetrieve(w http.ResponseWriter, r *http.Request, version string, objectName string) {
	resolvedObjectName, ok := storage.ResolveObjectName(*s.Org, objectName)
	if !ok {
		writeSalesforceError(w, errUnknownObject, "unknown object "+objectName)
		return
	}
	object := s.Org.Objects[resolvedObjectName]
	idsParam := strings.TrimSpace(r.URL.Query().Get("ids"))
	if idsParam == "" {
		writeSalesforceError(w, errRequiredFieldMissing, "ids query parameter is required for local Composite sObject collection retrieve")
		return
	}
	ids := strings.Split(idsParam, ",")
	records := make([]map[string]any, 0, len(ids))
	fields, hasProjection, ok := compositeRetrieveFields(w, object.Definition, s.Org.Namespace, r.URL.Query().Get("fields"))
	if !ok {
		return
	}
	for _, rawID := range ids {
		id := storage.ID(strings.TrimSpace(rawID))
		if id == "" {
			writeSalesforceError(w, errMalformedID, "ids query parameter contains an empty record id")
			return
		}
		record, ok := object.Records[id]
		if !ok || record.System.IsDeleted {
			writeSalesforceError(w, errUnknownRecord, "record not found: "+string(id))
			return
		}
		if hasProjection {
			records = append(records, projectedRecordPayload(record, version, resolvedObjectName, id, fields))
			continue
		}
		records = append(records, recordPayload(record, version, resolvedObjectName, id))
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": records})
}

func (s *Server) handleCompositeSObjectTypedUpsert(w http.ResponseWriter, r *http.Request, objectName string, externalIDField string) {
	objectName, ok := storage.ResolveObjectName(*s.Org, objectName)
	if !ok {
		writeSalesforceError(w, errUnknownObject, "unknown object "+objectName)
		return
	}
	object := s.Org.Objects[objectName]
	fieldName := externalIDField
	if canonical, ok := storage.ResolveFieldName(object.Definition, s.Org.Namespace, externalIDField); ok {
		fieldName = canonical
	}
	field, ok := object.Definition.Fields[fieldName]
	if !ok {
		writeSalesforceError(w, errInvalidField, "unknown external id field "+objectName+"."+externalIDField)
		return
	}
	if !field.ExternalID {
		writeSalesforceError(w, errInvalidField, "field "+objectName+"."+fieldName+" is not an external id")
		return
	}

	var body struct {
		AllOrNone bool                          `json:"allOrNone"`
		Records   *[]map[string]json.RawMessage `json:"records"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return
	}
	if body.Records == nil || len(*body.Records) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "records is required and must contain at least one row")
		return
	}
	records := make([]storage.Record, 0, len(*body.Records))
	referenceIDs := make([]string, 0, len(*body.Records))
	for _, raw := range *body.Records {
		referenceID := ""
		if attrsRaw, ok := raw["attributes"]; ok {
			var attrs struct {
				ReferenceID string `json:"referenceId"`
			}
			if err := json.Unmarshal(attrsRaw, &attrs); err != nil {
				writeSalesforceError(w, errMalformedJSON, "attributes must be a JSON object")
				return
			}
			referenceID = attrs.ReferenceID
			delete(raw, "attributes")
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
	results := engine.UpsertWithExternalID(records, fieldName)
	hasFailure := false
	hasSuccess := false
	for _, result := range results {
		if result.Success {
			hasSuccess = true
		} else {
			hasFailure = true
		}
	}
	if body.AllOrNone && hasFailure {
		writeJSON(w, http.StatusBadRequest, compositeUpsertResults(results, referenceIDs))
		return
	}
	if hasSuccess {
		if err := s.commitOrg(next); err != nil {
			writeSalesforceError(w, errStoreFailure, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, compositeUpsertResults(results, referenceIDs))
}

type compositeSObjectBody struct {
	AllOrNone    bool
	Records      []storage.Record
	ReferenceIDs []string
}

func decodeCompositeSObjectBody(w http.ResponseWriter, r *http.Request, requireID bool) (compositeSObjectBody, bool) {
	var rawBody struct {
		AllOrNone bool                          `json:"allOrNone"`
		Records   *[]map[string]json.RawMessage `json:"records"`
	}
	if err := json.NewDecoder(r.Body).Decode(&rawBody); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return compositeSObjectBody{}, false
	}
	if rawBody.Records == nil || len(*rawBody.Records) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "records is required and must contain at least one row")
		return compositeSObjectBody{}, false
	}
	body := compositeSObjectBody{AllOrNone: rawBody.AllOrNone, Records: make([]storage.Record, 0, len(*rawBody.Records)), ReferenceIDs: make([]string, 0, len(*rawBody.Records))}
	for _, raw := range *rawBody.Records {
		objectName := ""
		referenceID := ""
		if attrsRaw, ok := raw["attributes"]; ok {
			var attrs struct {
				Type        string `json:"type"`
				ReferenceID string `json:"referenceId"`
			}
			if err := json.Unmarshal(attrsRaw, &attrs); err != nil {
				writeSalesforceError(w, errMalformedJSON, "attributes must be a JSON object")
				return compositeSObjectBody{}, false
			}
			objectName = attrs.Type
			referenceID = attrs.ReferenceID
			delete(raw, "attributes")
		}
		if objectName == "" {
			writeSalesforceError(w, errRequiredFieldMissing, "attributes.type is required")
			return compositeSObjectBody{}, false
		}
		var id storage.ID
		if requireID {
			var err error
			id, err = idFromRawRecord(raw)
			if err != nil {
				writeSalesforceError(w, errMalformedJSON, err.Error())
				return compositeSObjectBody{}, false
			}
			if id == "" {
				writeSalesforceError(w, errRequiredFieldMissing, "Id is required")
				return compositeSObjectBody{}, false
			}
		}
		record, err := recordFromRawFields(objectName, id, raw)
		if err != nil {
			writeSalesforceError(w, errMalformedJSON, err.Error())
			return compositeSObjectBody{}, false
		}
		body.Records = append(body.Records, record)
		body.ReferenceIDs = append(body.ReferenceIDs, referenceID)
	}
	return body, true
}

func idFromRawRecord(raw map[string]json.RawMessage) (storage.ID, error) {
	for _, name := range []string{"Id", "id"} {
		rawID, ok := raw[name]
		if !ok {
			continue
		}
		var id string
		if err := json.Unmarshal(rawID, &id); err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		delete(raw, "Id")
		delete(raw, "id")
		return storage.ID(id), nil
	}
	return "", nil
}

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
	var result soql.Result
	var err error
	if allRows {
		query, parseErr := soql.Parse(queryText)
		if parseErr == nil {
			query.AllRows = true
			result, err = soql.Execute(*s.Org, query)
		} else {
			err = parseErr
		}
	} else {
		result, err = soql.ParseAndExecute(*s.Org, queryText)
	}
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
		writeJSON(w, http.StatusOK, queryResultPayload(result.Rows, true, result.Records, version, ""))
		return
	}
	locator := s.storeQueryLocator(queryLocatorState{
		totalSize: result.Rows,
		records:   cloneQueryRecords(result.Records),
		batchSize: batchSize,
		version:   version,
		nextPath:  nextPath,
	})
	writeJSON(w, http.StatusOK, queryResultPayload(result.Rows, false, result.Records[:batchSize], version, queryNextURL(version, nextPath, locator, batchSize)))
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
		errors = append(errors, salesforceError{ErrorCode: code, Message: message})
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
			"enterprise": base + "/services/Soap/c/61.0/" + orgID,
			"metadata":   base + "/services/Soap/m/61.0/" + orgID,
			"partner":    base + "/services/Soap/u/61.0/" + orgID,
			"rest":       base + "/services/data/v61.0/",
			"sobjects":   base + "/services/data/v61.0/sobjects/",
			"search":     base + "/services/data/v61.0/search/",
			"query":      base + "/services/data/v61.0/query/",
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

func isDefaultValuesRoute(parts []string) bool {
	return len(parts) == 2 && parts[1] == "defaultValues"
}

func isQuickActionsRoute(parts []string) bool {
	if len(parts) == 2 {
		return parts[1] == "quickActions"
	}
	if len(parts) == 3 {
		return parts[1] == "quickActions"
	}
	return len(parts) == 4 && parts[1] == "quickActions" && parts[3] == "defaultValues"
}

func isListViewsRoute(parts []string) bool {
	return isListViewCollectionRoute(parts) || (len(parts) == 4 && parts[1] == "listviews" && (parts[3] == "describe" || parts[3] == "results"))
}

func isListViewCollectionRoute(parts []string) bool {
	return len(parts) == 2 && parts[1] == "listviews"
}

func isRowTemplatePlaceholder(id storage.ID) bool {
	text := string(id)
	return strings.Contains(text, "{") || strings.Contains(text, "}")
}

func isObjectMetadataRoute(parts []string) bool {
	if len(parts) == 2 {
		return parts[1] == "layouts" || parts[1] == "compactLayouts"
	}
	if len(parts) == 3 && parts[1] == "describe" {
		return parts[2] == "layouts" || parts[2] == "approvalLayouts" || parts[2] == "namedLayouts" || parts[2] == "compactLayouts"
	}
	if len(parts) == 4 && parts[1] == "describe" && parts[2] == "compactLayouts" {
		return parts[3] != ""
	}
	return len(parts) == 3 && parts[1] == "namedLayouts"
}

func isCompactLayoutsRoute(parts []string) bool {
	return (len(parts) == 2 && parts[1] == "compactLayouts") ||
		(len(parts) == 3 && parts[1] == "describe" && parts[2] == "compactLayouts") ||
		(len(parts) == 4 && parts[1] == "describe" && parts[2] == "compactLayouts" && parts[3] != "")
}

func describePayload(def storage.ObjectDefinition, org *storage.OrgState) map[string]any {
	fields := make([]map[string]any, 0, len(def.Fields))
	names := make([]string, 0, len(def.Fields))
	for name := range def.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		field := def.Fields[name]
		fields = append(fields, describeFieldPayload(field))
	}
	return map[string]any{
		"name":               def.APIName,
		"label":              labelOrFallback(def.Label, def.APIName),
		"labelPlural":        labelOrFallback(def.PluralLabel, labelOrFallback(def.Label, def.APIName)),
		"custom":             strings.HasSuffix(def.APIName, "__c") || strings.HasSuffix(def.APIName, "__mdt"),
		"keyPrefix":          def.KeyPrefix,
		"fields":             fields,
		"searchable":         true,
		"queryable":          true,
		"createable":         true,
		"updateable":         true,
		"deletable":          true,
		"recordTypeInfos":    describeRecordTypeInfos(def.RecordTypes),
		"childRelationships": describeChildRelationships(def.APIName, org),
	}
}

func describeFieldPayload(field storage.Field) map[string]any {
	createable := field.Type != storage.FieldID && field.Type != storage.FieldCalculated
	updateable := createable
	referenceTo := append([]string(nil), field.ReferenceTo...)
	sort.Strings(referenceTo)
	nillable := !field.Required && field.Type != storage.FieldID && !strings.EqualFold(field.APIName, "Id")
	return map[string]any{
		"name":             field.APIName,
		"label":            labelOrFallback(field.Label, field.APIName),
		"type":             string(field.Type),
		"nillable":         nillable,
		"createable":       createable,
		"updateable":       updateable,
		"filterable":       true,
		"sortable":         field.Type != storage.FieldAddress && field.Type != storage.FieldLocation,
		"externalId":       field.ExternalID,
		"unique":           field.Unique,
		"idLookup":         field.Type == storage.FieldID || strings.EqualFold(field.APIName, "Id") || field.ExternalID,
		"referenceTo":      referenceTo,
		"relationshipName": field.RelationshipName,
		"picklistValues":   describePicklistValues(field.PicklistValues),
	}
}

func describePicklistValues(values []storage.PicklistValue) []map[string]any {
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"value":        value.Value,
			"label":        labelOrFallback(value.Label, value.Value),
			"active":       value.Active,
			"defaultValue": value.Default,
		})
	}
	return out
}

func describeRecordTypeInfos(recordTypes []storage.RecordTypeInfo) []map[string]any {
	sorted := append([]storage.RecordTypeInfo(nil), recordTypes...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].DeveloperName == sorted[j].DeveloperName {
			if sorted[i].Name == sorted[j].Name {
				return sorted[i].ID < sorted[j].ID
			}
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].DeveloperName < sorted[j].DeveloperName
	})
	out := make([]map[string]any, 0, len(sorted))
	for _, recordType := range sorted {
		out = append(out, map[string]any{
			"recordTypeId":             recordType.ID.String(),
			"developerName":            recordType.DeveloperName,
			"name":                     labelOrFallback(recordType.Name, recordType.DeveloperName),
			"active":                   recordType.Active,
			"available":                recordType.Available,
			"defaultRecordTypeMapping": recordType.Default,
		})
	}
	return out
}

func describeChildRelationships(objectName string, org *storage.OrgState) []map[string]any {
	if org == nil {
		return []map[string]any{}
	}
	childNames := make([]string, 0, len(org.Objects))
	for childName := range org.Objects {
		childNames = append(childNames, childName)
	}
	sort.Strings(childNames)
	out := make([]map[string]any, 0)
	for _, childName := range childNames {
		relationships := append([]storage.Relationship(nil), org.Objects[childName].Definition.Relations...)
		sort.Slice(relationships, func(i, j int) bool {
			if relationships[i].ChildRelationship == relationships[j].ChildRelationship {
				return relationships[i].Field < relationships[j].Field
			}
			return relationships[i].ChildRelationship < relationships[j].ChildRelationship
		})
		for _, relationship := range relationships {
			if !relationshipTargetsObject(relationship, objectName) {
				continue
			}
			out = append(out, map[string]any{
				"cascadeDelete":       relationship.CascadeDelete,
				"childSObject":        childName,
				"deprecatedAndHidden": false,
				"field":               relationship.Field,
				"relationshipName":    relationship.ChildRelationship,
				"restrictedDelete":    relationship.RestrictedDelete,
			})
		}
	}
	return out
}

func relationshipTargetsObject(relationship storage.Relationship, objectName string) bool {
	for _, parent := range relationship.ParentObjects {
		if strings.EqualFold(parent, objectName) {
			return true
		}
	}
	return false
}

func labelOrFallback(label, fallback string) string {
	if label != "" {
		return label
	}
	return fallback
}

func listViewsPayload(def storage.ObjectDefinition, version string) map[string]any {
	return map[string]any{
		"done":       true,
		"size":       0,
		"listviews":  []map[string]any{},
		"objectType": def.APIName,
		"url":        "/services/data/" + version + "/sobjects/" + def.APIName + "/listviews",
		"message":    "List view metadata is not modeled; returning an empty local stub.",
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

func objectResourcePayload(def storage.ObjectDefinition, version string) map[string]any {
	name := def.APIName
	label := def.Label
	if label == "" {
		label = name
	}
	base := "/services/data/" + version + "/sobjects/" + name
	describe := base + "/describe"
	recent := base + "/recent"
	return map[string]any{
		"name":           name,
		"label":          label,
		"keyPrefix":      def.KeyPrefix,
		"custom":         strings.HasSuffix(name, "__c"),
		"objectDescribe": describe,
		"recentItems":    recent,
		"describe":       describe,
		"url":            base,
		"urls": map[string]string{
			"rowTemplate":     base + "/{ID}",
			"defaultValues":   base + "/defaultValues?recordTypeId&fields",
			"describe":        describe,
			"recent":          recent,
			"updated":         base + "/updated",
			"deleted":         base + "/deleted",
			"items":           base,
			"layouts":         describe + "/layouts",
			"approvalLayouts": describe + "/approvalLayouts",
			"compactLayouts":  base + "/compactLayouts",
			"namedLayouts":    base + "/namedLayouts/{LAYOUT_NAME}",
			"quickActions":    base + "/quickActions",
			"listviews":       base + "/listviews",
		},
	}
}

func findExternalIDRecord(object storage.ObjectState, fieldName string, field storage.Field, value storage.Value) (storage.Record, storage.ID, int) {
	matches := make([]storage.ID, 0, 1)
	for id, record := range object.Records {
		if record.System.IsDeleted {
			continue
		}
		storedValue, ok := record.Fields[fieldName]
		if !ok || !storageValuesEqual(field, storedValue, value) {
			continue
		}
		matches = append(matches, id)
	}
	if len(matches) != 1 {
		return storage.Record{}, "", len(matches)
	}
	id := matches[0]
	record := object.Records[id]
	if record.ID == "" {
		record.ID = id
	}
	if record.Object == "" {
		record.Object = object.Definition.APIName
	}
	return record, id, 1
}

func writeExternalIDLookupResult(w http.ResponseWriter, objectName, fieldName string, matches int) bool {
	switch matches {
	case 1:
		return true
	case 0:
		writeSalesforceError(w, errUnknownRecord)
	default:
		writeSalesforceError(w, errDuplicateValue, fmt.Sprintf("external id %s.%s matched multiple records", objectName, fieldName))
	}
	return false
}

func externalIDValueFromPath(field storage.Field, raw string) (storage.Value, error) {
	switch field.Type {
	case storage.FieldID, storage.FieldReference:
		return storage.IDValue(storage.ID(raw)), nil
	case storage.FieldInteger:
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return storage.Value{}, fmt.Errorf("invalid external id integer value %q", raw)
		}
		return storage.IntegerValue(value), nil
	case storage.FieldBoolean:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return storage.Value{}, fmt.Errorf("invalid external id boolean value %q", raw)
		}
		return storage.BooleanValue(value), nil
	case storage.FieldDecimal:
		return storage.DecimalValue(raw), nil
	case storage.FieldDate:
		return storage.DateValue(raw), nil
	case storage.FieldDateTime:
		return storage.DateTimeValue(raw), nil
	default:
		return storage.StringValue(raw), nil
	}
}

func storageValuesEqual(field storage.Field, left, right storage.Value) bool {
	if left.Kind == storage.ValueString && right.Kind == storage.ValueString && !field.CaseSensitive {
		return strings.EqualFold(left.String, right.String)
	}
	if left.Kind != right.Kind {
		if left.Kind == storage.ValueID && right.Kind == storage.ValueString {
			if !field.CaseSensitive {
				return strings.EqualFold(string(left.ID), right.String)
			}
			return string(left.ID) == right.String
		}
		if left.Kind == storage.ValueString && right.Kind == storage.ValueID {
			if !field.CaseSensitive {
				return strings.EqualFold(left.String, string(right.ID))
			}
			return left.String == string(right.ID)
		}
		return false
	}
	switch left.Kind {
	case storage.ValueNull:
		return true
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime:
		return left.String == right.String
	case storage.ValueDecimal:
		return left.Decimal == right.Decimal
	case storage.ValueInteger:
		return left.Integer == right.Integer
	case storage.ValueBoolean:
		return left.Boolean == right.Boolean
	case storage.ValueID:
		return left.ID == right.ID
	default:
		return false
	}
}

func recordPayload(record storage.Record, version string, objectName string, id storage.ID) map[string]any {
	return recordPayloadWithProjection(record, version, objectName, id, nil, false)
}

func recordFieldProjectionFromRequest(w http.ResponseWriter, r *http.Request, definition storage.ObjectDefinition, namespace string) ([]string, bool, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("fields"))
	if raw == "" {
		return nil, false, true
	}
	fields := make([]string, 0)
	seen := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		requested := strings.TrimSpace(part)
		if requested == "" {
			continue
		}
		canonical, ok := storage.ResolveFieldName(definition, namespace, requested)
		if !ok {
			writeSalesforceError(w, errInvalidField, fmt.Sprintf("No such column %q on entity %q", requested, definition.APIName))
			return nil, false, false
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		fields = append(fields, canonical)
	}
	if len(fields) == 0 {
		return nil, false, true
	}
	return fields, true, true
}

func recordPayloadWithProjection(record storage.Record, version string, objectName string, id storage.ID, projection []string, projected bool) map[string]any {
	if record.Object != "" {
		objectName = record.Object
	}
	if record.ID != "" {
		id = record.ID
	}
	out := map[string]any{
		"attributes": map[string]any{
			"type": objectName,
			"url":  "/services/data/" + version + "/sobjects/" + objectName + "/" + string(id),
		},
		"Id": string(id),
	}
	if projected {
		for _, name := range projection {
			if name == "Id" {
				continue
			}
			if value, ok := record.Fields[name]; ok {
				out[name] = storageValueJSON(value)
				continue
			}
			out[name] = nil
		}
		return out
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

func compositeRetrieveFields(w http.ResponseWriter, definition storage.ObjectDefinition, namespace, rawFields string) ([]string, bool, bool) {
	if strings.TrimSpace(rawFields) == "" {
		return nil, false, true
	}
	parts := strings.Split(rawFields, ",")
	fields := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, raw := range parts {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		canonical, ok := storage.ResolveFieldName(definition, namespace, name)
		if !ok {
			writeSalesforceError(w, errInvalidField, fmt.Sprintf("unknown field %s.%s", definition.APIName, name))
			return nil, false, false
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		fields = append(fields, canonical)
	}
	if len(fields) == 0 {
		return nil, false, true
	}
	return fields, true, true
}

func projectedRecordPayload(record storage.Record, version string, objectName string, id storage.ID, fields []string) map[string]any {
	if record.Object != "" {
		objectName = record.Object
	}
	if record.ID != "" {
		id = record.ID
	}
	out := map[string]any{
		"attributes": map[string]any{
			"type": objectName,
			"url":  "/services/data/" + version + "/sobjects/" + objectName + "/" + string(id),
		},
		"Id": string(id),
	}
	for _, name := range fields {
		if name == "Id" {
			continue
		}
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

const (
	defaultRecentLimit = 25
	maxRecentLimit     = 200
)

func recentLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultRecentLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 || limit > maxRecentLimit {
		return 0, fmt.Errorf("limit must be a positive integer no greater than 200")
	}
	return limit, nil
}

func recentPayload(object storage.ObjectState, version string, limit int) []map[string]any {
	ids := make([]string, 0, len(object.Records))
	for id, record := range object.Records {
		if record.System.IsDeleted {
			continue
		}
		ids = append(ids, string(id))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	if len(ids) > limit {
		ids = ids[:limit]
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
		out = append(out, map[string]any{"Id": id, "Name": name, "attributes": map[string]any{"type": objectName, "url": "/services/data/" + version + "/sobjects/" + objectName + "/" + id}})
	}
	return out
}

type updatedResourcePayload struct {
	LatestDateCovered string   `json:"latestDateCovered"`
	IDs               []string `json:"ids"`
}

type deletedResourcePayload struct {
	EarliestDateAvailable string                 `json:"earliestDateAvailable"`
	LatestDateCovered     string                 `json:"latestDateCovered"`
	DeletedRecords        []deletedResourceEntry `json:"deletedRecords"`
}

type deletedResourceEntry struct {
	ID          string `json:"id"`
	DeletedDate string `json:"deletedDate"`
}

func updatedPayload(object storage.ObjectState, r *http.Request) (updatedResourcePayload, error) {
	bounds, err := queryDateBounds(r)
	if err != nil {
		return updatedResourcePayload{}, err
	}
	ids := make([]string, 0, len(object.Records))
	latest := ""
	for id, record := range object.Records {
		if record.System.IsDeleted {
			continue
		}
		stamp := recordChangeTimestamp(record.System)
		if stamp == "" || !timestampInBounds(stamp, bounds) {
			continue
		}
		ids = append(ids, string(id))
		if compareTimestamps(stamp, latest) > 0 {
			latest = stamp
		}
	}
	sort.Strings(ids)
	return updatedResourcePayload{LatestDateCovered: latest, IDs: ids}, nil
}

func deletedPayload(object storage.ObjectState, r *http.Request) (deletedResourcePayload, error) {
	bounds, err := queryDateBounds(r)
	if err != nil {
		return deletedResourcePayload{}, err
	}
	ids := make([]string, 0, len(object.Records))
	deletedDates := make(map[string]string, len(object.Records))
	earliest := ""
	latest := ""
	for id, record := range object.Records {
		if !record.System.IsDeleted {
			continue
		}
		stamp := recordChangeTimestamp(record.System)
		if stamp == "" || !timestampInBounds(stamp, bounds) {
			continue
		}
		stringID := string(id)
		ids = append(ids, stringID)
		deletedDates[stringID] = stamp
		if earliest == "" || compareTimestamps(stamp, earliest) < 0 {
			earliest = stamp
		}
		if compareTimestamps(stamp, latest) > 0 {
			latest = stamp
		}
	}
	sort.Strings(ids)
	records := make([]deletedResourceEntry, 0, len(ids))
	for _, id := range ids {
		records = append(records, deletedResourceEntry{ID: id, DeletedDate: deletedDates[id]})
	}
	return deletedResourcePayload{EarliestDateAvailable: earliest, LatestDateCovered: latest, DeletedRecords: records}, nil
}

type dateBounds struct {
	hasStart bool
	start    time.Time
	hasEnd   bool
	end      time.Time
}

func queryDateBounds(r *http.Request) (dateBounds, error) {
	query := r.URL.Query()
	var bounds dateBounds
	if raw := firstQueryValue(query, "start", "startDate"); raw != "" {
		start, err := parseRESTTimestamp(raw)
		if err != nil {
			return bounds, fmt.Errorf("malformed start date %q", raw)
		}
		bounds.hasStart = true
		bounds.start = start
	}
	if raw := firstQueryValue(query, "end", "endDate"); raw != "" {
		end, err := parseRESTTimestamp(raw)
		if err != nil {
			return bounds, fmt.Errorf("malformed end date %q", raw)
		}
		bounds.hasEnd = true
		bounds.end = end
	}
	return bounds, nil
}

func firstQueryValue(query map[string][]string, names ...string) string {
	for _, name := range names {
		for _, value := range query[name] {
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func recordChangeTimestamp(system storage.SystemFields) string {
	if system.LastModifiedDate != "" {
		return system.LastModifiedDate
	}
	if system.SystemModstamp != "" {
		return system.SystemModstamp
	}
	return system.CreatedDate
}

func timestampInBounds(stamp string, bounds dateBounds) bool {
	if !bounds.hasStart && !bounds.hasEnd {
		return true
	}
	parsed, err := parseRESTTimestamp(stamp)
	if err != nil {
		return false
	}
	if bounds.hasStart && parsed.Before(bounds.start) {
		return false
	}
	if bounds.hasEnd && !parsed.Before(bounds.end) {
		return false
	}
	return true
}

func compareTimestamps(left, right string) int {
	if right == "" {
		if left == "" {
			return 0
		}
		return 1
	}
	leftTime, leftErr := parseRESTTimestamp(left)
	rightTime, rightErr := parseRESTTimestamp(right)
	if leftErr == nil && rightErr == nil {
		switch {
		case leftTime.Before(rightTime):
			return -1
		case leftTime.After(rightTime):
			return 1
		default:
			return 0
		}
	}
	return strings.Compare(left, right)
}

func parseRESTTimestamp(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z0700",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05Z0700",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("malformed date")
}

func recentAllPayload(org storage.OrgState, version string, limit int) []map[string]any {
	out := make([]map[string]any, 0)
	for _, object := range org.Objects {
		out = append(out, recentPayload(object, version, limit)...)
	}
	sort.Slice(out, func(i, j int) bool {
		left, _ := out[i]["Id"].(string)
		right, _ := out[j]["Id"].(string)
		return left > right
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func toolingDiscoveryPayload(version string) map[string]string {
	base := "/services/data/" + version + "/tooling"
	return map[string]string{
		"completions":          base + "/completions",
		"executeAnonymous":     base + "/executeAnonymous",
		"query":                base + "/query",
		"queryAll":             base + "/queryAll",
		"runTestsAsynchronous": base + "/runTestsAsynchronous",
		"runTestsSynchronous":  base + "/runTestsSynchronous",
		"search":               base + "/search",
		"sobjects":             base + "/sobjects",
	}
}

func metadataRESTDiscoveryPayload(version string) map[string]string {
	base := "/services/data/" + version + "/metadata"
	return map[string]string{
		"components":       base + "/components",
		"deployRequest":    base + "/deployRequest",
		"describe":         base + "/describe",
		"describeMetadata": base + "/describeMetadata",
		"listMetadata":     base + "/listMetadata",
		"retrieveRequest":  base + "/retrieveRequest",
	}
}

func isMetadataReadDiscoveryRoute(name string) bool {
	switch name {
	case "components", "describe", "describeMetadata", "listMetadata":
		return true
	default:
		return false
	}
}

func metadataReadDiscoveryUnsupportedMessage(name string) string {
	switch name {
	case "components":
		return "Metadata REST component discovery is not implemented in the local server; use source files and oaer inspect/check for local metadata state"
	case "describe", "describeMetadata":
		return "Metadata REST describeMetadata is not implemented in the local server; use SObject describe and project metadata files for local shape information"
	case "listMetadata":
		return "Metadata REST listMetadata is not implemented in the local server; use source file discovery for local metadata listings"
	default:
		return "Metadata REST read/discovery is not implemented in the local server"
	}
}

func bulkJobsDiscoveryPayload(version string) map[string]string {
	base := "/services/data/" + version + "/jobs"
	return map[string]string{
		"query":  base + "/query",
		"ingest": base + "/ingest",
	}
}

func resourceDiscoveryPayload(version string) map[string]string {
	base := "/services/data/" + version
	return map[string]string{
		"actions":      base + "/actions",
		"analytics":    base + "/analytics",
		"appMenu":      base + "/appMenu",
		"apps":         base + "/apps",
		"chatter":      base + "/chatter",
		"composite":    base + "/composite",
		"connect":      base + "/connect",
		"jobs":         base + "/jobs",
		"limits":       base + "/limits",
		"metadata":     base + "/metadata",
		"oaer":         base + "/oaer",
		"process":      base + "/process",
		"query":        base + "/query",
		"queryAll":     base + "/queryAll",
		"quickActions": base + "/quickActions",
		"recent":       base + "/recent",
		"search":       base + "/search",
		"sobjects":     base + "/sobjects",
		"support":      base + "/support",
		"tabs":         base + "/tabs",
		"theme":        base + "/theme",
		"tooling":      base + "/tooling",
		"wave":         base + "/wave",
	}
}

func apiVersionDiscoveryPayload() []apiVersionEntry {
	out := make([]apiVersionEntry, len(localAPIVersions))
	copy(out, localAPIVersions)
	return out
}

func unsupportedRESTNamespaceMessage(namespace string) (string, bool) {
	display, ok := unsupportedRESTNamespaces[namespace]
	if !ok {
		return "", false
	}
	return display + " REST namespace is not implemented in the local server", true
}

func compositeResults(results []dml.Result, referenceIDs []string) []map[string]any {
	return compositeResultRows(results, referenceIDs, false)
}

func compositeUpsertResults(results []dml.Result, referenceIDs []string) []map[string]any {
	return compositeResultRows(results, referenceIDs, true)
}

func compositeResultRows(results []dml.Result, referenceIDs []string, includeCreated bool) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for i, result := range results {
		row := map[string]any{"id": result.ID, "success": result.Success, "errors": []map[string]any{}}
		if includeCreated {
			row["created"] = result.Created
		}
		if i < len(referenceIDs) && referenceIDs[i] != "" {
			row["referenceId"] = referenceIDs[i]
		}
		if !result.Success {
			row["errors"] = compositeErrorRows(result)
		}
		out = append(out, row)
	}
	return out
}

func compositeErrorRows(result dml.Result) []map[string]any {
	if len(result.Errors) == 0 {
		statusCode := result.StatusCode
		if statusCode == "" {
			statusCode = salesforceErrorCode(errDMLFailure)
		}
		return []map[string]any{{"statusCode": statusCode, "message": result.Error, "fields": result.Fields}}
	}
	errors := make([]map[string]any, 0, len(result.Errors))
	for _, err := range result.Errors {
		statusCode := err.StatusCode
		if statusCode == "" {
			statusCode = salesforceErrorCode(errDMLFailure)
		}
		errors = append(errors, map[string]any{"statusCode": statusCode, "message": err.Message, "fields": err.Fields})
	}
	return errors
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
			unescaped, err := url.PathUnescape(part)
			if err != nil {
				unescaped = part
			}
			out = append(out, unescaped)
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
