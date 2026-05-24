package server

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

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
			writeJSON(w, http.StatusOK, s.compactLayoutsPayload(object.Definition, parts))
			return
		}
		if s.hasLayoutMetadata(objectName) {
			writeJSON(w, http.StatusOK, s.layoutMetadataPayload(object.Definition, version, parts))
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
			writeJSON(w, http.StatusOK, s.listViewsPayload(object.Definition, version))
			return
		}
		s.handleListViewMetadata(w, object, version, parts)
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
		engine := s.newDMLEngine(r, &next)
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
		engine := s.newDMLEngine(r, &next)
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
		engine := s.newDMLEngine(r, &next)
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
		engine := s.newDMLEngine(r, &next)
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
		engine := s.newDMLEngine(r, &next)
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
