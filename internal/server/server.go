package server

import (
	"net/http"
	"sync"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type Server struct {
	mu    sync.Mutex
	Org   *storage.OrgState
	Store interface {
		Save(storage.OrgState) error
	}
	Source     SourceMetadata
	LimitMode  vm.LimitMode
	LimitCaps  vm.LimitCaps
	Index      *typesys.Index
	runtime    *vm.VM
	runtimeErr error

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

const localOAuthUnsupportedMessage = "Full OAuth flows and token issuance are not implemented by the local server; use deterministic local user stubs via /services/oauth2/userinfo, /id/{org}/{user}, X-GLADE-User-Id, or Authorization: Bearer <userId>"

const apexRestUnsupportedMessage = "Apex @RestResource dispatch is not implemented in the local server"

var apexRestAllowedMethods = []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete}

func (s *Server) advertisedRESTAPIVersion() string {
	if s != nil && s.Org != nil {
		return storage.EffectiveRESTAPIVersion(s.Org.APIVersion)
	}
	return storage.DefaultRESTAPIVersion
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

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serveHTTPLocked(w, r)
}

func (s *Server) serveHTTPLocked(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusOK, s.apiVersionDiscoveryPayload())
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
		s.handleMetadataREST(w, r, parts[2], rest[1:])
	case len(rest) >= 1 && rest[0] == "composite":
		s.handleComposite(w, r, parts[2], rest[1:])
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
