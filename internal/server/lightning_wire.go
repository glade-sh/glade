package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/storage"
)

func (s *Server) handleLightningWire(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodPost || len(parts) == 0 {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	switch parts[0] {
	case "apex":
		s.handleLightningWireApex(w, r)
	case "getRecord":
		s.handleLightningWireGetRecord(w, r)
	case "getObjectInfo":
		s.handleLightningWireGetObjectInfo(w, r)
	default:
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning wire endpoint")
	}
}

func (s *Server) handleLightningWireApex(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireApexRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid wire apex request"}})
		return
	}
	machine, err := s.visualforceRuntime()
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{
			Error: &lwcbrowser.WireError{Type: "UnsupportedFeature", Message: err.Error()},
		})
		return
	}
	params := req.Params
	if params == nil {
		params = map[string]any{}
	}
	result, err := machine.InvokeLWCMethod(strings.TrimSpace(req.ClassName), strings.TrimSpace(req.Method), params)
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{
			Error: &lwcbrowser.WireError{Type: "RuntimeError", Message: err.Error()},
		})
		return
	}
	if !result.Success {
		out := lwcbrowser.WireResponse{}
		if result.Error != nil {
			out.Error = &lwcbrowser.WireError{Type: result.Error.Type, Message: result.Error.Message}
		} else {
			out.Error = &lwcbrowser.WireError{Message: "apex wire call failed"}
		}
		writeWireJSON(w, out)
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: result.ReturnValue})
}

func (s *Server) handleLightningWireGetRecord(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireGetRecordRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid getRecord wire request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := getRecordWireData(s.Org, req.RecordID, req.Fields)
	if wireErr != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: wireErr})
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: data})
}

func (s *Server) handleLightningWireGetObjectInfo(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireGetObjectInfoRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid getObjectInfo wire request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := getObjectInfoWireData(s.Org, req.ObjectAPIName)
	if wireErr != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: wireErr})
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: data})
}

func getRecordWireData(org *storage.OrgState, recordID string, fields []string) (map[string]any, *lwcbrowser.WireError) {
	recordID = strings.TrimSpace(recordID)
	if recordID == "" {
		return nil, &lwcbrowser.WireError{Message: "recordId is required"}
	}
	objectName, record, ok := findOrgRecord(org, recordID)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("record not found: %s", recordID)}
	}
	fieldNames := make([]string, 0, len(fields))
	for _, ref := range fields {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if dot := strings.LastIndex(ref, "."); dot >= 0 && dot < len(ref)-1 {
			fieldNames = append(fieldNames, ref[dot+1:])
			continue
		}
		fieldNames = append(fieldNames, ref)
	}
	if len(fieldNames) == 0 {
		for name := range record.Fields {
			if name != "Id" && name != "attributes" {
				fieldNames = append(fieldNames, name)
			}
		}
	}
	fieldsOut := make(map[string]any, len(fieldNames))
	for _, name := range fieldNames {
		fieldName := name
		field, hasField := storage.Field{}, false
		if object, ok := org.Objects[objectName]; ok {
			if canonical, ok := storage.ResolveFieldName(object.Definition, org.Namespace, name); ok {
				fieldName = canonical
				field = object.Definition.Fields[canonical]
				hasField = true
			}
		}
		value, ok := record.Fields[name]
		if !ok && fieldName != name {
			value, ok = record.Fields[fieldName]
		}
		label := fieldName
		if hasField {
			label = labelOrFallback(field.Label, fieldName)
		}
		if !ok {
			fieldsOut[fieldName] = map[string]any{"value": nil, "displayValue": nil, "label": label}
			continue
		}
		jsonVal := storageValueJSON(value)
		fieldsOut[fieldName] = map[string]any{
			"value":        jsonVal,
			"displayValue": fmt.Sprint(jsonVal),
			"label":        label,
		}
	}
	return map[string]any{
		"id":      recordID,
		"apiName": objectName,
		"fields":  fieldsOut,
	}, nil
}

func getObjectInfoWireData(org *storage.OrgState, objectAPIName string) (map[string]any, *lwcbrowser.WireError) {
	objectAPIName = strings.TrimSpace(objectAPIName)
	if objectAPIName == "" {
		return nil, &lwcbrowser.WireError{Message: "objectApiName is required"}
	}
	objectName, object, ok := findOrgObject(org, objectAPIName)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("object not found: %s", objectAPIName)}
	}
	payload := describePayload(object.Definition, org)
	payload["apiName"] = objectName
	fieldList, _ := payload["fields"].([]map[string]any)
	fields := make(map[string]any, len(fieldList))
	for _, field := range fieldList {
		name, _ := field["name"].(string)
		if name != "" {
			fields[name] = field
		}
	}
	payload["fields"] = fields
	return payload, nil
}

func findOrgRecord(org *storage.OrgState, recordID string) (objectName string, record storage.Record, ok bool) {
	if org == nil || len(recordID) < 3 {
		return "", storage.Record{}, false
	}
	prefix := recordID[:3]
	id := storage.ID(recordID)
	for name, object := range org.Objects {
		if strings.TrimSpace(object.Definition.KeyPrefix) != prefix {
			continue
		}
		if rec, found := object.Records[id]; found {
			return name, rec, true
		}
	}
	return "", storage.Record{}, false
}

func findOrgObject(org *storage.OrgState, objectAPIName string) (objectName string, object storage.ObjectState, ok bool) {
	if org == nil {
		return "", storage.ObjectState{}, false
	}
	for name, candidate := range org.Objects {
		if strings.EqualFold(name, objectAPIName) || strings.EqualFold(candidate.Definition.APIName, objectAPIName) {
			return name, candidate, true
		}
	}
	return "", storage.ObjectState{}, false
}

func writeWireJSON(w http.ResponseWriter, payload lwcbrowser.WireResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(payload)
}
