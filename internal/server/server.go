package server

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type Server struct {
	mu    sync.RWMutex
	Org   *storage.OrgState
	Store interface {
		Save(storage.OrgState) error
	}
	Source       SourceMetadata
	LimitProfile string
	LimitMode    vm.LimitMode
	LimitCaps    vm.LimitCaps
	Index        *typesys.Index
	runtime      *vm.VM
	runtimeErr   error

	queryLocators          map[string]queryLocatorState
	queryOrder             []string
	nextQueryID            int
	bulkQueryJobs          map[string]bulkQueryJob
	nextBulkJobID          int
	bulkV1Jobs             map[string]bulkV1Job
	nextBulkV1JobID        int
	lightning              lightningState
	devRunEvents           []DevRunEvent
	nextDevRunSeq          int
	metadataJobs           map[string]metadataLocalJob
	nextMetadataDeployID   int
	nextMetadataRetrieveID int

	visualforceViewStateSecret []byte
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

const maxLocalRequestBodyBytes = 4 * 1000 * 1000

type apiVersionEntry struct {
	Version string `json:"version"`
	Label   string `json:"label"`
	URL     string `json:"url"`
}

const localOAuthUnsupportedMessage = "Full OAuth flows and token issuance are not implemented by the local server; use deterministic local user stubs via /services/oauth2/userinfo, /id/{org}/{user}, X-GLADE-User-Id, or Authorization: Bearer <userId>"

const apexRestUnsupportedMessage = "Apex @RestResource dispatch is not implemented in the local server"

var apexRestAllowedMethods = []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete}

const devNoStoreCacheControl = "no-store, no-cache, must-revalidate, max-age=0"

func setDevNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", devNoStoreCacheControl)
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

func (s *Server) advertisedRESTAPIVersion() (string, error) {
	if s != nil && s.Org != nil {
		version, err := storage.ResolveRESTAPIVersion(s.Org.APIVersion)
		if err != nil {
			return "", fmt.Errorf("unsupported API version %s", strings.TrimSpace(s.Org.APIVersion))
		}
		return version, nil
	}
	return storage.DefaultRESTAPIVersion, nil
}

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

type gladeStatePayload struct {
	LocalOnly   bool                   `json:"localOnly"`
	Summary     storage.InspectSummary `json:"summary"`
	ResetScopes []resetScopeInfo       `json:"resetScopes"`
}

type gladeResetPayload struct {
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

type gladeDiscoveryPayload struct {
	LocalOnly bool              `json:"localOnly"`
	URLs      map[string]string `json:"urls"`
}

func New(org *storage.OrgState) *Server {
	removeAutomatedProcessUsers(org)
	return &Server{Org: org}
}

func NewWithStore(org *storage.OrgState, store interface{ Save(storage.OrgState) error }) *Server {
	removeAutomatedProcessUsers(org)
	return &Server{Org: org, Store: store}
}

func NewWithSource(org *storage.OrgState, source SourceMetadata) *Server {
	removeAutomatedProcessUsers(org)
	return &Server{Org: org, Source: source}
}

func NewWithStoreAndSource(org *storage.OrgState, store interface{ Save(storage.OrgState) error }, source SourceMetadata) *Server {
	removeAutomatedProcessUsers(org)
	return &Server{Org: org, Store: store, Source: source}
}

func (s *Server) SetProjectIndex(index typesys.Index) {
	s.Index = &index
	machine := vm.New(nil)
	s.runtimeErr = apextest.RegisterProjectRuntimeForRequest(machine, index)
	if s.runtimeErr != nil {
		s.runtime = nil
		return
	}
	s.runtime = machine
}

// SetProjectRuntime installs a precompiled request runtime template for Apex
// REST dispatch. The server clones the template for each request.
func (s *Server) SetProjectRuntime(index typesys.Index, runtime *vm.VM, runtimeErr error) {
	s.Index = &index
	s.runtime = runtime
	s.runtimeErr = runtimeErr
}

func (s *Server) ReloadProjectState(source SourceMetadata, index typesys.Index, runtime *vm.VM, runtimeErr error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Source = source
	s.Index = &index
	s.runtime = runtime
	s.runtimeErr = runtimeErr
	s.resetLightningCacheLocked()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r.URL.EscapedPath())
	if canServeWithReadLock(r, parts) {
		s.mu.RLock()
		defer s.mu.RUnlock()
	} else {
		s.mu.Lock()
		defer s.mu.Unlock()
	}
	s.serveHTTPLocked(w, r, parts)
}

func canServeWithReadLock(r *http.Request, parts []string) bool {
	if r == nil || r.Method != http.MethodGet {
		return false
	}
	if len(parts) == 0 {
		return false
	}
	if len(parts) == 3 && parts[0] == "id" {
		return true
	}
	if len(parts) >= 1 && parts[0] == "resource" {
		return true
	}
	if len(parts) >= 1 && parts[0] == "assets" {
		return true
	}
	if len(parts) < 2 || parts[0] != "services" {
		return false
	}
	if parts[1] == "oauth2" {
		return true
	}
	if parts[1] != "data" {
		return false
	}
	if len(parts) == 2 || len(parts) == 3 {
		return true
	}
	rest := parts[3:]
	if len(rest) == 0 {
		return true
	}
	switch rest[0] {
	case "sobjects", "recent", "search", "limits":
		return true
	default:
		_, unsupported := unsupportedRESTNamespaceMessage(rest[0])
		return unsupported
	}
}

func (s *Server) serveHTTPLocked(w http.ResponseWriter, r *http.Request, parts []string) {
	w.Header().Set("Content-Type", "application/json")
	if len(parts) == 0 && s.hasLWCWorkbenchProject() {
		s.handleLWCShell(w, r, nil)
		return
	}
	if len(parts) >= 2 && parts[0] == "apex" {
		s.handleVisualforcePage(w, r, parts[1:])
		return
	}
	if len(parts) >= 1 && parts[0] == "lightning" {
		s.handleLightning(w, r, parts[1:])
		return
	}
	if len(parts) >= 1 && parts[0] == "assets" {
		s.handleLightningAssets(w, r, parts[1:])
		return
	}
	if len(parts) >= 1 && parts[0] == "lwc" {
		s.handleLWCShell(w, r, parts[1:])
		return
	}
	if len(parts) >= 1 && parts[0] == "db" {
		s.handleDBManagerShell(w, r, parts[1:])
		return
	}
	if len(parts) >= 1 && parts[0] == "resource" {
		s.handleStaticResource(w, r, parts[1:])
		return
	}
	if len(parts) >= 2 && parts[0] == "services" && parts[1] == "oauth2" {
		s.handleOAuth(w, r, parts[2:])
		return
	}
	if len(parts) == 3 && parts[0] == "id" {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		payload, err := s.identityPayload(r, storage.ID(parts[2]))
		if err != nil {
			writeSalesforceError(w, errUnknownEndpoint, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, payload)
		return
	}
	if len(parts) == 2 && parts[0] == "services" && parts[1] == "data" {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, s.apiVersionDiscoveryPayload())
		return
	}
	if requested, ok := requestAPIVersion(parts); ok {
		version, err := storage.ResolveRESTAPIVersion(requested)
		if err != nil {
			message := "unsupported API version " + requested
			if parts[1] == "Soap" {
				writeSOAPFault(w, http.StatusInternalServerError, "sf:INVALID_VERSION", message)
			} else {
				writeSalesforceError(w, errUnknownEndpoint, message)
			}
			return
		}
		switch parts[1] {
		case "data":
			parts[2] = "v" + version
		case "Soap":
			parts[3] = version
		case "async":
			parts[2] = version
		}
	}
	if len(parts) >= 3 && parts[0] == "services" && parts[1] == "Soap" {
		s.handleSOAP(w, r, parts[2:])
		return
	}
	if len(parts) >= 2 && parts[0] == "services" && parts[1] == "async" {
		s.handleBulkV1(w, r, parts[2:])
		return
	}
	if len(parts) >= 2 && parts[0] == "services" && parts[1] == "apexrest" {
		if !methodAllowed(r, apexRestAllowedMethods...) {
			writeMethodNotAllowed(w, apexRestAllowedMethods...)
			return
		}
		s.handleApexRest(w, r)
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
		s.handleObjectBreadth(w, r, parts[2], rest[1:])
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
	case len(rest) == 2 && rest[0] == "limits" && rest[1] == "recordCount":
		s.handleRecordCount(w, r)
	case len(rest) >= 1 && rest[0] == "tooling":
		s.handleTooling(w, r, parts[2], rest[1:])
	case len(rest) >= 1 && rest[0] == "jobs":
		s.handleBulkJobsWithQueryPaging(w, r, parts[2], rest[1:])
	case len(rest) >= 1 && rest[0] == "metadata":
		s.handleMetadataRESTWithJobs(w, r, parts[2], rest[1:])
	case len(rest) >= 1 && rest[0] == "composite":
		s.handleCompositeBreadth(w, r, parts[2], rest[1:])
	case len(rest) >= 3 && rest[0] == "actions" && rest[1] == "custom" && rest[2] == "apex":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Invocable Apex REST actions are not implemented in the local server")
	case len(rest) >= 3 && rest[0] == "async" && rest[1] == "specifications" && rest[2] == "oas3":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "OpenAPI specification generation is not implemented in the local server")
	case len(rest) >= 1 && rest[0] == "glade":
		s.handleGLADE(w, r, rest[1:])
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

func requestAPIVersion(parts []string) (string, bool) {
	if len(parts) < 3 || parts[0] != "services" {
		return "", false
	}
	switch parts[1] {
	case "data", "async":
		return parts[2], true
	case "Soap":
		if len(parts) >= 4 {
			return parts[3], true
		}
	}
	return "", false
}

func (s *Server) hasLWCWorkbenchProject() bool {
	if s == nil {
		return false
	}
	p := s.Source.Project
	return len(p.LWCFiles) > 0 || len(p.LWCHTMLFiles) > 0 || len(p.LWCMetaFiles) > 0 || len(p.FlexiPageFiles) > 0 || len(p.TabFiles) > 0
}
