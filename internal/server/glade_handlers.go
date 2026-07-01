package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/visualforce"
)

func (s *Server) handleGLADE(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) == 0 && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, gladeDiscovery(versionFromRequest(r)))
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
		writeJSON(w, http.StatusOK, gladeResetPayload{
			Success:    true,
			Scopes:     scopes,
			NoOpScopes: noOpResetScopes(scopes),
			Summary:    storage.InspectOrg("", *s.Org),
		})
	case len(parts) == 1 && (parts[0] == "state" || parts[0] == "inspect") && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, gladeStatePayload{
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
	case len(parts) == 2 && parts[0] == "visualforce" && parts[1] == "support" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, visualforceSupportPayload())
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
	case len(parts) >= 1 && parts[0] == "db-manager":
		s.handleDBManagerAPI(w, r, parts[1:])
	case len(parts) == 0:
		writeMethodNotAllowed(w, http.MethodGet)
	case len(parts) >= 1 && parts[0] == "reset":
		writeMethodNotAllowed(w, http.MethodPost)
	case len(parts) == 1 && (parts[0] == "state" || parts[0] == "inspect"):
		writeMethodNotAllowed(w, http.MethodGet)
	case len(parts) == 1 && parts[0] == "fixture":
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	case len(parts) == 2 && parts[0] == "visualforce" && parts[1] == "support":
		writeMethodNotAllowed(w, http.MethodGet)
	default:
		writeSalesforceError(w, errUnknownGLADE)
	}
}

type visualforceSupportComponent struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type visualforceSupportResponse struct {
	Components []visualforceSupportComponent `json:"components"`
}

func visualforceSupportPayload() visualforceSupportResponse {
	specs := visualforce.StandardComponentSpecs()
	components := make([]visualforceSupportComponent, 0, len(specs))
	for _, spec := range specs {
		name := visualforceSupportName(spec.Name)
		if name == "" {
			continue
		}
		components = append(components, visualforceSupportComponent{
			Name:   name,
			Status: string(spec.Status),
			Reason: strings.TrimSpace(spec.Reason),
		})
	}
	sort.Slice(components, func(i, j int) bool {
		return components[i].Name < components[j].Name
	})
	return visualforceSupportResponse{Components: components}
}

func visualforceSupportName(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(strings.ToLower(name), "apex:") {
		return strings.TrimSpace(name[len("apex:"):])
	}
	return name
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

func gladeDiscovery(version string) gladeDiscoveryPayload {
	base := "/services/data/" + version + "/glade"
	return gladeDiscoveryPayload{
		LocalOnly: true,
		URLs: map[string]string{
			"dbManager": base + "/db-manager",
			"fixture":   base + "/fixture",
			"inspect":   base + "/inspect",
			"reset":     base + "/reset",
			"state":     base + "/state",
		},
	}
}

func versionFromRequest(r *http.Request) string {
	parts := splitPath(r.URL.EscapedPath())
	if len(parts) >= 3 && parts[0] == "services" && parts[1] == "data" {
		return parts[2]
	}
	return "v" + storage.DefaultRESTAPIVersion
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
			removeAutomatedProcessUsers(org)
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
	removeAutomatedProcessUsers(org)
}

func removeAutomatedProcessUsers(org *storage.OrgState) {
	users, ok := org.Objects["User"]
	if !ok || len(users.Records) == 0 {
		return
	}
	for id, record := range users.Records {
		userType := ""
		if value, ok := record.Fields["UserType"]; ok && value.Kind == storage.ValueString {
			userType = value.String
		}
		if strings.EqualFold(userType, "AutomatedProcess") {
			delete(users.Records, id)
		}
	}
	org.Objects["User"] = users
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
