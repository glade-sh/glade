package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/dbmanager"
)

func (s *Server) handleDBManagerShell(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	if len(parts) == 0 {
		s.serveDBManagerAsset(w, "web/index.html", "text/html; charset=utf-8")
		return
	}
	if len(parts) == 2 && parts[0] == "assets" {
		switch parts[1] {
		case "app.js":
			s.serveDBManagerAsset(w, "web/app.js", "text/javascript; charset=utf-8")
		case "styles.css":
			s.serveDBManagerAsset(w, "web/styles.css", "text/css; charset=utf-8")
		default:
			writeSalesforceError(w, errUnknownEndpoint, "unknown DB manager asset")
		}
		return
	}
	writeSalesforceError(w, errUnknownEndpoint, "unknown DB manager endpoint")
}

func (s *Server) serveDBManagerAsset(w http.ResponseWriter, name, contentType string) {
	data, err := fs.ReadFile(dbmanager.Assets, name)
	if err != nil {
		writeSalesforceError(w, errUnknownEndpoint, "DB manager asset not found")
		return
	}
	w.Header().Set("Content-Type", contentType)
	setDevNoStore(w)
	_, _ = w.Write(data)
}

func (s *Server) handleDBManagerAPI(w http.ResponseWriter, r *http.Request, parts []string) {
	manager := dbmanager.New(s.Org)
	switch {
	case len(parts) == 0:
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"localOnly": true,
			"urls": map[string]string{
				"objects": "/services/data/" + versionFromRequest(r) + "/glade/db-manager/objects",
				"lookup":  "/services/data/" + versionFromRequest(r) + "/glade/db-manager/lookup",
			},
		})
	case len(parts) == 1 && parts[0] == "objects":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, manager.ListObjects(dbmanager.ListObjectsOptions{Query: r.URL.Query().Get("q")}))
	case len(parts) == 2 && parts[0] == "objects":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		detail, err := manager.ObjectDetail(parts[1])
		if err != nil {
			writeSalesforceError(w, errUnknownObject, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, detail)
	case len(parts) == 3 && parts[0] == "objects" && parts[2] == "records":
		s.handleDBManagerRecords(w, r, parts[1])
	case len(parts) == 4 && parts[0] == "objects" && parts[2] == "records":
		s.handleDBManagerRecord(w, r, parts[1], parts[3])
	case len(parts) == 5 && parts[0] == "objects" && parts[2] == "records" && parts[4] == "undelete":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		s.handleDBManagerMutation(w, func(m dbmanager.Manager) dbmanager.MutationResult {
			return m.UndeleteRecord(parts[1], parts[3])
		})
	case len(parts) == 1 && parts[0] == "lookup":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		result, err := manager.Lookup(dbmanager.LookupOptions{
			Object: r.URL.Query().Get("object"),
			Field:  r.URL.Query().Get("field"),
			Query:  r.URL.Query().Get("q"),
			Limit:  dbManagerIntQuery(r, "limit", 10),
		})
		if err != nil {
			writeSalesforceError(w, errInvalidField, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeSalesforceError(w, errUnknownGLADE, "unknown DB manager endpoint")
	}
}

func (s *Server) handleDBManagerRecords(w http.ResponseWriter, r *http.Request, objectName string) {
	manager := dbmanager.New(s.Org)
	switch r.Method {
	case http.MethodGet:
		records, err := manager.ListRecords(objectName, dbmanager.ListRecordsOptions{
			Query:          r.URL.Query().Get("q"),
			Limit:          dbManagerIntQuery(r, "limit", 50),
			Offset:         dbManagerIntQuery(r, "offset", 0),
			IncludeDeleted: dbManagerBoolQuery(r, "includeDeleted"),
		})
		if err != nil {
			writeSalesforceError(w, errUnknownObject, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, records)
	case http.MethodPost:
		var payload dbmanager.MutationPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeSalesforceError(w, errMalformedJSON, err.Error())
			return
		}
		s.handleDBManagerMutation(w, func(m dbmanager.Manager) dbmanager.MutationResult {
			return m.CreateRecord(objectName, payload)
		})
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleDBManagerRecord(w http.ResponseWriter, r *http.Request, objectName, id string) {
	manager := dbmanager.New(s.Org)
	switch r.Method {
	case http.MethodGet:
		record, err := manager.RecordDetail(objectName, id)
		if err != nil {
			writeSalesforceError(w, errUnknownRecord, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, record)
	case http.MethodPatch:
		var payload dbmanager.MutationPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeSalesforceError(w, errMalformedJSON, err.Error())
			return
		}
		s.handleDBManagerMutation(w, func(m dbmanager.Manager) dbmanager.MutationResult {
			return m.UpdateRecord(objectName, id, payload)
		})
	case http.MethodDelete:
		s.handleDBManagerMutation(w, func(m dbmanager.Manager) dbmanager.MutationResult {
			return m.DeleteRecord(objectName, id)
		})
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPatch, http.MethodDelete)
	}
}

func (s *Server) handleDBManagerMutation(w http.ResponseWriter, mutate func(dbmanager.Manager) dbmanager.MutationResult) {
	next := s.Org.Clone()
	result := mutate(dbmanager.New(&next))
	if !result.Success {
		writeJSON(w, http.StatusBadRequest, result)
		return
	}
	if err := s.commitOrg(next); err != nil {
		writeSalesforceError(w, errStoreFailure, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func dbManagerIntQuery(r *http.Request, name string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func dbManagerBoolQuery(r *http.Request, name string) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(r.URL.Query().Get(name)))
	return err == nil && value
}
