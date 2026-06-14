package server

import (
	"net/http"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (s *Server) handleObjectBreadth(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	if len(parts) >= 2 && isDefaultValuesRoute(parts) {
		s.handleObjectDefaultValues(w, r, version, parts[0])
		return
	}
	s.handleObject(w, r, version, parts)
}

func (s *Server) handleObjectDefaultValues(w http.ResponseWriter, r *http.Request, version string, objectName string) {
	object, ok := s.Org.Objects[objectName]
	if !ok {
		writeSalesforceError(w, errUnknownObject)
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	fields, ok := defaultValueFieldsFromRequest(w, r, object.Definition, s.Org.Namespace)
	if !ok {
		return
	}
	record := storage.Record{Object: objectName, Fields: map[string]storage.Value{}}
	if recordTypeID := strings.TrimSpace(r.URL.Query().Get("recordTypeId")); recordTypeID != "" {
		record.Fields["RecordTypeId"] = storage.IDValue(storage.ID(recordTypeID))
	}
	defaults := make(map[string]any)
	for _, fieldName := range fields {
		field := object.Definition.Fields[fieldName]
		value, ok := storage.DefaultValueForRecordField(object.Definition, record, field)
		if !ok {
			continue
		}
		defaults[fieldName] = storageValueJSON(value)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"objectType":    objectName,
		"recordTypeId":  strings.TrimSpace(r.URL.Query().Get("recordTypeId")),
		"defaultValues": defaults,
		"url":           "/services/data/" + version + "/sobjects/" + objectName + "/defaultValues",
	})
}

func defaultValueFieldsFromRequest(w http.ResponseWriter, r *http.Request, definition storage.ObjectDefinition, namespace string) ([]string, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("fields"))
	if raw == "" {
		names := make([]string, 0, len(definition.Fields))
		for name := range definition.Fields {
			names = append(names, name)
		}
		sort.Strings(names)
		return names, true
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
			writeSalesforceError(w, errInvalidField, "unknown default value field "+definition.APIName+"."+requested)
			return nil, false
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		fields = append(fields, canonical)
	}
	return fields, true
}
