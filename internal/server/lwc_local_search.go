package server

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type lwcLocalObjectOption struct {
	APIName     string `json:"apiName"`
	Label       string `json:"label"`
	PluralLabel string `json:"pluralLabel,omitempty"`
	KeyPrefix   string `json:"keyPrefix,omitempty"`
	RecordCount int    `json:"recordCount"`
}

type lwcLocalObjectSearchPayload struct {
	Objects []lwcLocalObjectOption `json:"objects"`
}

type lwcLocalRecordOption struct {
	ID      string         `json:"id"`
	APIName string         `json:"apiName"`
	Title   string         `json:"title"`
	Fields  map[string]any `json:"fields,omitempty"`
}

type lwcLocalRecordSearchPayload struct {
	ObjectAPIName string                 `json:"objectApiName"`
	Records       []lwcLocalRecordOption `json:"records"`
}

func (s *Server) handleLightningLocalObjects(w http.ResponseWriter, r *http.Request) {
	org := s.ensureLocalOrg()
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	writeJSON(w, http.StatusOK, lwcLocalObjectSearchPayload{Objects: lwcLocalObjectOptions(org, query, 50)})
}

func (s *Server) handleLightningLocalRecords(w http.ResponseWriter, r *http.Request) {
	org := s.ensureLocalOrg()
	objectName := strings.TrimSpace(r.URL.Query().Get("object"))
	if objectName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "object query parameter is required"})
		return
	}
	canonical, object, ok := findOrgObject(org, objectName)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": fmt.Sprintf("object not found: %s", objectName)})
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	writeJSON(w, http.StatusOK, lwcLocalRecordSearchPayload{
		ObjectAPIName: canonical,
		Records:       lwcLocalRecordOptions(canonical, object, query, 50),
	})
}

func (s *Server) ensureLocalOrg() *storage.OrgState {
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	return s.Org
}

func lwcLocalObjectOptions(org *storage.OrgState, query string, limit int) []lwcLocalObjectOption {
	if org == nil || limit <= 0 {
		return nil
	}
	search := strings.ToLower(strings.TrimSpace(query))
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		left := org.Objects[names[i]]
		right := org.Objects[names[j]]
		if len(left.Records) != len(right.Records) {
			return len(left.Records) > len(right.Records)
		}
		return strings.ToLower(lwcLocalObjectAPIName(names[i], left)) < strings.ToLower(lwcLocalObjectAPIName(names[j], right))
	})
	out := make([]lwcLocalObjectOption, 0)
	for _, name := range names {
		object := org.Objects[name]
		apiName := lwcLocalObjectAPIName(name, object)
		label := labelOrFallback(object.Definition.Label, apiName)
		if search != "" && !strings.Contains(strings.ToLower(apiName+" "+label+" "+object.Definition.PluralLabel), search) {
			continue
		}
		out = append(out, lwcLocalObjectOption{
			APIName:     apiName,
			Label:       label,
			PluralLabel: object.Definition.PluralLabel,
			KeyPrefix:   object.Definition.KeyPrefix,
			RecordCount: len(object.Records),
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func lwcLocalObjectAPIName(fallback string, object storage.ObjectState) string {
	if strings.TrimSpace(object.Definition.APIName) != "" {
		return object.Definition.APIName
	}
	return fallback
}

func lwcLocalRecordOptions(objectName string, object storage.ObjectState, query string, limit int) []lwcLocalRecordOption {
	if limit <= 0 {
		return nil
	}
	search := strings.ToLower(strings.TrimSpace(query))
	ids := make([]string, 0, len(object.Records))
	for id := range object.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	out := make([]lwcLocalRecordOption, 0)
	for _, id := range ids {
		record := object.Records[storage.ID(id)]
		record.ID = storage.ID(id)
		record.Object = objectName
		if record.System.IsDeleted || !lwcLocalRecordMatches(record, search) {
			continue
		}
		out = append(out, lwcLocalRecordOption{
			ID:      string(record.ID),
			APIName: objectName,
			Title:   lwcLocalRecordTitle(record),
			Fields:  lwcLocalRecordFields(record),
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func lwcLocalRecordMatches(record storage.Record, search string) bool {
	if search == "" {
		return true
	}
	if strings.Contains(strings.ToLower(string(record.ID)), search) {
		return true
	}
	if strings.Contains(strings.ToLower(lwcLocalRecordTitle(record)), search) {
		return true
	}
	for _, value := range record.Fields {
		if strings.Contains(strings.ToLower(fmt.Sprint(storageValueJSON(value))), search) {
			return true
		}
	}
	return false
}

func lwcLocalRecordTitle(record storage.Record) string {
	if value, ok := record.Fields["Name"]; ok {
		return fmt.Sprint(storageValueJSON(value))
	}
	return string(record.ID)
}

func lwcLocalRecordFields(record storage.Record) map[string]any {
	fields := map[string]any{}
	for _, name := range []string{"Name"} {
		if value, ok := record.Fields[name]; ok {
			fields[name] = storageValueJSON(value)
		}
	}
	return fields
}
