package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
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
	case "getRecords":
		s.handleLightningWireGetRecords(w, r)
	case "getRecordUi":
		s.handleLightningWireGetRecordUI(w, r)
	case "getObjectInfo":
		s.handleLightningWireGetObjectInfo(w, r)
	case "getObjectInfos":
		s.handleLightningWireGetObjectInfos(w, r)
	case "getRecordCreateDefaults":
		s.handleLightningWireGetRecordCreateDefaults(w, r)
	case "getLayout":
		s.handleLightningWireGetLayout(w, r)
	case "getPicklistValues":
		s.handleLightningWireGetPicklistValues(w, r)
	case "getPicklistValuesByRecordType":
		s.handleLightningWireGetPicklistValuesByRecordType(w, r)
	case "getRelatedListRecords":
		s.handleLightningWireGetRelatedListRecords(w, r)
	case "recordPickerSearch":
		s.handleLightningWireRecordPickerSearch(w, r)
	case "createRecord":
		s.handleLightningWireCreateRecord(w, r)
	case "updateRecord":
		s.handleLightningWireUpdateRecord(w, r)
	case "deleteRecord":
		s.handleLightningWireDeleteRecord(w, r)
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
	s.invokeLightningApex(w, r, req.ClassName, req.Method, req.Params)
}

func (s *Server) handleLightningApex(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodPost || len(parts) != 2 {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var raw any
	if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &raw); err != nil {
			writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid apex invocation request"}})
			return
		}
	}
	if envelope, ok := raw.(map[string]any); ok {
		if params, exists := envelope["params"]; exists && len(envelope) == 1 {
			raw = params
		}
	}
	s.invokeLightningApex(w, r, parts[0], parts[1], raw)
}

func (s *Server) invokeLightningApex(w http.ResponseWriter, r *http.Request, className, methodName string, rawParams any) {
	machine, err := s.visualforceRuntime()
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{
			Error: apexWireInvocationError("", "UnsupportedFeature", className, methodName, rawParams, err.Error(), http.StatusInternalServerError),
		})
		return
	}
	machine.SetCurrentUser(s.currentUser(r, ""))
	if pageURL := lightningLocalContextPageURL(r); pageURL != "" {
		machine.SetCurrentPageURL(pageURL)
	} else {
		machine.ResetApexPageState()
	}
	params, paramErr := apexWireParams(rawParams)
	if paramErr != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{
			Error: apexWireInvocationError("", "InvalidParameterValueException", className, methodName, rawParams, paramErr.Error(), http.StatusBadRequest),
		})
		return
	}
	result, err := machine.InvokeLWCMethod(strings.TrimSpace(className), strings.TrimSpace(methodName), params)
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{
			Error: apexWireInvocationError("", "RuntimeError", className, methodName, rawParams, err.Error(), http.StatusInternalServerError),
		})
		return
	}
	if !result.Success {
		out := lwcbrowser.WireResponse{}
		if result.Error != nil {
			out.Error = apexWireInvocationError(result.Error.Code, result.Error.Type, className, methodName, rawParams, result.Error.Message, http.StatusInternalServerError)
		} else {
			out.Error = apexWireInvocationError("", "ApexException", className, methodName, rawParams, "apex wire call failed", http.StatusInternalServerError)
		}
		writeWireJSON(w, out)
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: result.ReturnValue})
}

func lightningLocalContextPageURL(r *http.Request) string {
	if r == nil {
		return ""
	}
	raw := strings.TrimSpace(r.Header.Get("X-Glade-LWC-Context"))
	if raw == "" {
		return ""
	}
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		decoded = raw
	}
	var payload struct {
		URL     string `json:"url"`
		Context struct {
			RecordID      string `json:"recordId"`
			ObjectAPIName string `json:"objectApiName"`
		} `json:"context"`
	}
	if err := json.Unmarshal([]byte(decoded), &payload); err != nil {
		return ""
	}
	pageURL := strings.TrimSpace(payload.URL)
	if pageURL != "" {
		return pageURL
	}
	recordID := strings.TrimSpace(payload.Context.RecordID)
	if recordID == "" {
		return ""
	}
	objectAPIName := strings.TrimSpace(payload.Context.ObjectAPIName)
	if objectAPIName == "" {
		objectAPIName = "Record"
	}
	values := url.Values{}
	values.Set("id", recordID)
	values.Set("recordId", recordID)
	values.Set("objectApiName", objectAPIName)
	return "/lwc/preview/record/" + url.PathEscape(objectAPIName) + "/" + url.PathEscape(recordID) + "?" + values.Encode()
}

func apexWireParams(raw any) (map[string]any, error) {
	if raw == nil {
		return map[string]any{}, nil
	}
	params, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Apex params must be an object")
	}
	return params, nil
}

func apexWireError(exceptionType, message string, status int) *lwcbrowser.WireError {
	return apexWireErrorWithCode("", exceptionType, message, status)
}

func apexWireErrorWithCode(code, exceptionType, message string, status int) *lwcbrowser.WireError {
	exceptionType = strings.TrimSpace(exceptionType)
	if exceptionType == "" {
		exceptionType = "ApexException"
	}
	if status == 0 {
		status = http.StatusInternalServerError
	}
	return &lwcbrowser.WireError{
		Code:    strings.TrimSpace(code),
		Type:    exceptionType,
		Message: message,
		Status:  status,
		Body: &lwcbrowser.WireErrorBody{
			Code:          strings.TrimSpace(code),
			Message:       message,
			ExceptionType: exceptionType,
			StackTrace:    "",
		},
	}
}

func apexWireInvocationError(code, exceptionType, className, methodName string, rawParams any, message string, status int) *lwcbrowser.WireError {
	qualified := strings.Trim(strings.TrimSpace(className)+"."+strings.TrimSpace(methodName), ".")
	params := apexWireParamsString(rawParams)
	detail := strings.TrimSpace(message)
	if qualified != "" {
		detail = fmt.Sprintf("%s failed: %s", qualified, detail)
	}
	if params != "" {
		detail = fmt.Sprintf("%s params=%s", detail, params)
	}
	err := apexWireErrorWithCode(code, exceptionType, detail, status)
	if err.Body != nil {
		err.Body.StackTrace = apexWireStackTrace(qualified, message)
	}
	return err
}

func apexWireParamsString(raw any) string {
	if raw == nil {
		return "{}"
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Sprint(raw)
	}
	return string(data)
}

func apexWireStackTrace(qualified, message string) string {
	if qualified == "" {
		return strings.TrimSpace(message)
	}
	return strings.TrimSpace(qualified + "\n" + message)
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
	data, wireErr := getRecordWireData(s.Org, req.RecordID, req.Fields, req.OptionalFields)
	if wireErr != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: wireErr})
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: data})
}

func (s *Server) handleLightningWireGetRecords(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireGetRecordsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid getRecords wire request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: getRecordsWireData(s.Org, req)})
}

func (s *Server) handleLightningWireGetRecordUI(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireGetRecordUIRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid getRecordUi wire request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := getRecordUIWireData(s.Org, req, s.Source)
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

func (s *Server) handleLightningWireGetObjectInfos(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireGetObjectInfosRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid getObjectInfos wire request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: getObjectInfosWireData(s.Org, req)})
}

func (s *Server) handleLightningWireGetRecordCreateDefaults(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireGetRecordCreateDefaultsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid getRecordCreateDefaults wire request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := getRecordCreateDefaultsWireData(s.Org, req, s.Source)
	if wireErr != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: wireErr})
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: data})
}

func (s *Server) handleLightningWireGetLayout(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireGetLayoutRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid getLayout wire request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := getLayoutWireData(s.Org, req, s.Source)
	if wireErr != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: wireErr})
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: data})
}

func (s *Server) handleLightningWireGetPicklistValues(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireGetPicklistValuesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid getPicklistValues wire request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := getPicklistValuesWireData(s.Org, req)
	if wireErr != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: wireErr})
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: data})
}

func (s *Server) handleLightningWireGetPicklistValuesByRecordType(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireGetPicklistValuesByRecordTypeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid getPicklistValuesByRecordType wire request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := getPicklistValuesByRecordTypeWireData(s.Org, req)
	if wireErr != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: wireErr})
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: data})
}

func (s *Server) handleLightningWireGetRelatedListRecords(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireGetRelatedListRecordsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid getRelatedListRecords wire request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := getRelatedListRecordsWireData(s.Org, req)
	if wireErr != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: wireErr})
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: data})
}

func (s *Server) handleLightningWireRecordPickerSearch(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireRecordPickerSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid recordPickerSearch request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := recordPickerSearchWireData(s.Org, req)
	if wireErr != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: wireErr})
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: data})
}

func (s *Server) handleLightningWireCreateRecord(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireCreateRecordRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid createRecord request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := createRecordWireData(s.Org, req)
	if wireErr != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: wireErr})
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: data})
}

func (s *Server) handleLightningWireUpdateRecord(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireUpdateRecordRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid updateRecord request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := updateRecordWireData(s.Org, req)
	if wireErr != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: wireErr})
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: data})
}

func (s *Server) handleLightningWireDeleteRecord(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: err.Error()}})
		return
	}
	var req lwcbrowser.WireDeleteRecordRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: &lwcbrowser.WireError{Message: "invalid deleteRecord request"}})
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := deleteRecordWireData(s.Org, req.RecordID)
	if wireErr != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{Error: wireErr})
		return
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: data})
}

func getRecordWireData(org *storage.OrgState, recordID string, fields []string, optionalFields []string) (map[string]any, *lwcbrowser.WireError) {
	recordID = strings.TrimSpace(recordID)
	if recordID == "" {
		return nil, &lwcbrowser.WireError{Message: "recordId is required"}
	}
	objectName, record, ok := findOrgRecord(org, recordID)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("record not found: %s", recordID)}
	}
	fieldNames := make([]string, 0, len(fields)+len(optionalFields))
	optionalByName := map[string]bool{}
	for _, ref := range fields {
		if name := wireFieldName(ref); name != "" {
			fieldNames = append(fieldNames, name)
		}
	}
	for _, ref := range optionalFields {
		if name := wireFieldName(ref); name != "" {
			fieldNames = append(fieldNames, name)
			optionalByName[strings.ToLower(name)] = true
		}
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
		if !hasField && !ok {
			if optionalByName[strings.ToLower(name)] {
				continue
			}
			return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("field not found: %s", name)}
		}
		label := fieldName
		if hasField {
			label = labelOrFallback(field.Label, fieldName)
		}
		if !ok {
			fieldsOut[fieldName] = recordFieldWirePayload(fieldName, field, hasField, label, nil)
			continue
		}
		jsonVal := storageValueJSON(value)
		fieldsOut[fieldName] = recordFieldWirePayload(fieldName, field, hasField, label, jsonVal)
	}
	return map[string]any{
		"id":                 recordID,
		"apiName":            objectName,
		"childRelationships": recordChildRelationships(org, objectName),
		"fields":             fieldsOut,
		"lastModifiedById":   recordLastModifiedByID(record),
		"lastModifiedDate":   recordLastModifiedDate(record),
		"recordTypeId":       recordTypeIDForRecord(org, objectName, record),
	}, nil
}

func recordFieldWirePayload(fieldName string, field storage.Field, hasField bool, label string, value any) map[string]any {
	out := map[string]any{
		"value":        value,
		"displayValue": nil,
		"label":        label,
	}
	if value != nil {
		out["displayValue"] = fmt.Sprint(value)
	}
	if hasField {
		out["dataType"] = lightningFieldDataType(field)
		out["relationshipName"] = field.RelationshipName
		out["referenceToInfos"] = describeReferenceToInfos(field.ReferenceTo)
		out["apiName"] = fieldName
	}
	return out
}

func recordChildRelationships(org *storage.OrgState, objectName string) []map[string]any {
	if object, ok := org.Objects[objectName]; ok && len(object.Definition.Relations) > 0 {
		out := make([]map[string]any, 0, len(object.Definition.Relations))
		for _, relationship := range object.Definition.Relations {
			out = append(out, map[string]any{
				"field":            relationship.Field,
				"relationshipName": relationship.ChildRelationship,
			})
		}
		return out
	}
	return describeChildRelationships(objectName, org)
}

func recordLastModifiedByID(record storage.Record) string {
	if record.System.LastModifiedByID != "" {
		return string(record.System.LastModifiedByID)
	}
	if value, ok := record.Fields["LastModifiedById"]; ok {
		return fmt.Sprint(storageValueJSON(value))
	}
	return "005000000000000AAA"
}

func recordLastModifiedDate(record storage.Record) string {
	if record.System.LastModifiedDate != "" {
		return record.System.LastModifiedDate
	}
	if value, ok := record.Fields["LastModifiedDate"]; ok {
		return fmt.Sprint(storageValueJSON(value))
	}
	return "2000-01-01T00:00:00.000Z"
}

func recordTypeIDForRecord(org *storage.OrgState, objectName string, record storage.Record) string {
	if value, ok := record.Fields["RecordTypeId"]; ok {
		return fmt.Sprint(storageValueJSON(value))
	}
	if object, ok := org.Objects[objectName]; ok {
		return createDefaultsRecordTypeID(object.Definition, "")
	}
	return ""
}

func wireFieldName(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if dot := strings.LastIndex(ref, "."); dot >= 0 && dot < len(ref)-1 {
		return ref[dot+1:]
	}
	return ref
}

func getRecordsWireData(org *storage.OrgState, req lwcbrowser.WireGetRecordsRequest) map[string]any {
	results := make([]map[string]any, 0)
	for _, item := range req.Records {
		for _, recordID := range item.RecordIDs {
			data, wireErr := getRecordWireData(org, recordID, item.Fields, item.OptionalFields)
			results = append(results, batchWireResult(data, wireErr))
		}
	}
	return map[string]any{"results": results}
}

func getRecordUIWireData(org *storage.OrgState, req lwcbrowser.WireGetRecordUIRequest, source SourceMetadata) (map[string]any, *lwcbrowser.WireError) {
	records := map[string]any{}
	objectInfos := map[string]any{}
	layouts := map[string]any{}
	objectNames := map[string]bool{}
	recordTypeIDs := map[string]string{}
	for _, recordID := range req.RecordIDs {
		recordID = strings.TrimSpace(recordID)
		if recordID == "" {
			continue
		}
		record, wireErr := getRecordWireData(org, recordID, req.Fields, req.OptionalFields)
		if wireErr != nil {
			return nil, wireErr
		}
		records[recordID] = record
		objectName, _ := record["apiName"].(string)
		if objectName == "" {
			continue
		}
		objectNames[objectName] = true
		recordTypeIDs[objectName] = recordTypeIDForRecordUI(org, objectName, recordID, req.RecordTypeID)
	}
	for objectName := range objectNames {
		objectInfo, wireErr := getObjectInfoWireData(org, objectName)
		if wireErr != nil {
			return nil, wireErr
		}
		objectInfos[objectName] = objectInfo
		recordTypeID := recordTypeIDs[objectName]
		layouts[objectName] = recordUILayoutsForObject(org, objectName, recordTypeID, req, source)
	}
	return map[string]any{
		"records":     records,
		"objectInfos": objectInfos,
		"layouts":     layouts,
	}, nil
}

func recordTypeIDForRecordUI(org *storage.OrgState, objectName string, recordID string, explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	object, ok := org.Objects[objectName]
	if !ok {
		return ""
	}
	_, record, ok := storage.LookupRecordByID(object.Records, storage.ID(recordID))
	if ok {
		if value, hasValue := record.Fields["RecordTypeId"]; hasValue && value.ID != "" {
			return string(value.ID)
		}
	}
	return createDefaultsRecordTypeID(object.Definition, "")
}

func recordUILayoutsForObject(org *storage.OrgState, objectName string, recordTypeID string, req lwcbrowser.WireGetRecordUIRequest, source SourceMetadata) map[string]any {
	byType := map[string]any{}
	layoutTypes := req.LayoutTypes
	if len(layoutTypes) == 0 {
		layoutTypes = []string{"Full"}
	}
	modes := req.Modes
	if len(modes) == 0 {
		modes = []string{"View"}
	}
	for _, layoutType := range layoutTypes {
		normalizedType := layoutTypeOrDefault(layoutType)
		if byType[normalizedType] == nil {
			byType[normalizedType] = map[string]any{}
		}
		modeMap := byType[normalizedType].(map[string]any)
		for _, mode := range modes {
			normalizedMode := layoutModeOrDefault(mode)
			layout, wireErr := getLayoutWireData(org, lwcbrowser.WireGetLayoutRequest{
				ObjectAPIName: objectName,
				RecordTypeID:  recordTypeID,
				LayoutType:    normalizedType,
				Mode:          normalizedMode,
				FormFactor:    req.FormFactor,
			}, source)
			if wireErr != nil {
				continue
			}
			modeMap[normalizedMode] = layout
		}
	}
	return map[string]any{recordTypeID: byType}
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
			uiField := make(map[string]any, len(field))
			for key, value := range field {
				uiField[key] = value
			}
			delete(uiField, "type")
			fields[name] = uiField
		}
	}
	payload["fields"] = fields
	payload["recordTypeInfos"] = recordTypeInfosByID(payload["recordTypeInfos"])
	payload["themeInfo"] = map[string]any{
		"color":   "747474",
		"iconUrl": "",
	}
	return payload, nil
}

func recordTypeInfosByID(raw any) map[string]any {
	out := map[string]any{}
	items, _ := raw.([]map[string]any)
	for _, item := range items {
		id, _ := item["recordTypeId"].(string)
		if id == "" {
			continue
		}
		out[id] = item
	}
	return out
}

func getObjectInfosWireData(org *storage.OrgState, req lwcbrowser.WireGetObjectInfosRequest) map[string]any {
	results := make([]map[string]any, 0, len(req.ObjectAPINames))
	for _, objectName := range req.ObjectAPINames {
		data, wireErr := getObjectInfoWireData(org, objectName)
		results = append(results, batchWireResult(data, wireErr))
	}
	return map[string]any{"results": results}
}

func getRecordCreateDefaultsWireData(org *storage.OrgState, req lwcbrowser.WireGetRecordCreateDefaultsRequest, source SourceMetadata) (map[string]any, *lwcbrowser.WireError) {
	objectName, object, ok := findOrgObject(org, req.ObjectAPIName)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("object not found: %s", req.ObjectAPIName)}
	}
	objectInfo, wireErr := getObjectInfoWireData(org, objectName)
	if wireErr != nil {
		return nil, wireErr
	}
	recordTypeID := createDefaultsRecordTypeID(object.Definition, req.RecordTypeID)
	record := storage.Record{Object: objectName, Fields: map[string]storage.Value{}}
	if recordTypeID != "" {
		record.Fields["RecordTypeId"] = storage.IDValue(storage.ID(recordTypeID))
	}
	fieldNames := createDefaultFieldNames(object.Definition, org.Namespace, req.OptionalFields)
	layout, hasSourceLayout := sourceCreateLayout(source, objectName)
	if hasSourceLayout {
		fieldNames = appendUniqueFieldNames(fieldNames, sourceLayoutFieldNames(layout, object.Definition, org.Namespace)...)
	} else {
		fieldNames = appendUniqueFieldNames(fieldNames, createableFieldNames(object.Definition)...)
	}
	fields := make(map[string]any, len(fieldNames))
	for _, fieldName := range fieldNames {
		field := object.Definition.Fields[fieldName]
		if !fieldCreateable(field) {
			continue
		}
		value, hasValue := storage.DefaultValueForRecordField(object.Definition, record, field)
		if strings.EqualFold(fieldName, "RecordTypeId") && recordTypeID != "" {
			value, hasValue = storage.IDValue(storage.ID(recordTypeID)), true
		}
		jsonValue := any(nil)
		displayValue := any(nil)
		if hasValue {
			jsonValue = storageValueJSON(value)
			displayValue = fmt.Sprint(jsonValue)
		}
		fields[fieldName] = map[string]any{
			"value":        jsonValue,
			"displayValue": displayValue,
			"label":        labelOrFallback(field.Label, fieldName),
		}
	}
	return map[string]any{
		"apiName":      objectName,
		"recordTypeId": recordTypeID,
		"objectInfos":  map[string]any{objectName: objectInfo},
		"layout":       recordCreateDefaultsLayout(objectName, object.Definition, org.Namespace, recordTypeID, fieldNames, layout, hasSourceLayout),
		"record": map[string]any{
			"id":           nil,
			"apiName":      objectName,
			"recordTypeId": recordTypeID,
			"fields":       fields,
		},
	}, nil
}

func getLayoutWireData(org *storage.OrgState, req lwcbrowser.WireGetLayoutRequest, source SourceMetadata) (map[string]any, *lwcbrowser.WireError) {
	objectName, object, ok := findOrgObject(org, req.ObjectAPIName)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("object not found: %s", req.ObjectAPIName)}
	}
	recordTypeID := createDefaultsRecordTypeID(object.Definition, req.RecordTypeID)
	layout, hasSourceLayout := sourceCreateLayout(source, objectName)
	fieldNames := createableFieldNames(object.Definition)
	out := recordCreateDefaultsLayout(objectName, object.Definition, org.Namespace, recordTypeID, fieldNames, layout, hasSourceLayout)
	out["layoutType"] = layoutTypeOrDefault(req.LayoutType)
	out["mode"] = layoutModeOrDefault(req.Mode)
	return out, nil
}

func layoutTypeOrDefault(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "compact":
		return "Compact"
	case "full", "":
		return "Full"
	default:
		return strings.TrimSpace(value)
	}
}

func layoutModeOrDefault(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "create":
		return "Create"
	case "edit":
		return "Edit"
	case "view", "":
		return "View"
	default:
		return strings.TrimSpace(value)
	}
}

func sourceCreateLayout(source SourceMetadata, objectName string) (layoutMetadata, bool) {
	layouts := source.Layouts[objectName]
	if len(layouts) == 0 {
		return layoutMetadata{}, false
	}
	for _, layout := range layouts {
		if len(layout.Sections) > 0 {
			return layout, true
		}
	}
	return layouts[0], true
}

func sourceLayoutFieldNames(layout layoutMetadata, def storage.ObjectDefinition, namespace string) []string {
	names := []string{}
	for _, section := range layout.Sections {
		for _, column := range section.Columns {
			for _, item := range column.Items {
				canonical, ok := storage.ResolveFieldName(def, namespace, item.Field)
				if !ok || !fieldCreateable(def.Fields[canonical]) {
					continue
				}
				names = append(names, canonical)
			}
		}
	}
	return names
}

func appendUniqueFieldNames(names []string, more ...string) []string {
	seen := make(map[string]bool, len(names)+len(more))
	for _, name := range names {
		seen[name] = true
	}
	for _, name := range more {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func createableFieldNames(def storage.ObjectDefinition) []string {
	names := make([]string, 0, len(def.Fields))
	for name, field := range def.Fields {
		if fieldCreateable(field) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func recordCreateDefaultsLayout(objectName string, def storage.ObjectDefinition, namespace string, recordTypeID string, fieldNames []string, sourceLayout layoutMetadata, hasSourceLayout bool) map[string]any {
	if hasSourceLayout && len(sourceLayout.Sections) > 0 {
		return map[string]any{
			"id":            sourceLayout.ID,
			"layoutType":    "Full",
			"mode":          "Create",
			"objectApiName": objectName,
			"recordTypeId":  recordTypeID,
			"saveOptions":   []map[string]any{},
			"sections":      sourceLayoutSectionsPayload(sourceLayout, def, namespace),
		}
	}
	return map[string]any{
		"id":            "local-" + objectName + "-create-layout",
		"layoutType":    "Full",
		"mode":          "Create",
		"objectApiName": objectName,
		"recordTypeId":  recordTypeID,
		"saveOptions":   []map[string]any{},
		"sections":      fallbackLayoutSectionsPayload(objectName, def, fieldNames),
	}
}

func sourceLayoutSectionsPayload(layout layoutMetadata, def storage.ObjectDefinition, namespace string) []map[string]any {
	sections := make([]map[string]any, 0, len(layout.Sections))
	for index, section := range layout.Sections {
		columnCount := len(section.Columns)
		if columnCount == 0 {
			columnCount = 1
		}
		maxRows := 0
		for _, column := range section.Columns {
			if len(column.Items) > maxRows {
				maxRows = len(column.Items)
			}
		}
		rows := make([]map[string]any, 0, maxRows)
		for rowIndex := 0; rowIndex < maxRows; rowIndex++ {
			items := make([]map[string]any, 0, columnCount)
			for _, column := range section.Columns {
				if rowIndex >= len(column.Items) {
					continue
				}
				item := column.Items[rowIndex]
				canonical, ok := storage.ResolveFieldName(def, namespace, item.Field)
				if !ok {
					continue
				}
				field := def.Fields[canonical]
				if !fieldCreateable(field) {
					continue
				}
				items = append(items, recordLayoutFieldItem(canonical, field, item.Behavior))
			}
			if len(items) > 0 {
				rows = append(rows, map[string]any{"layoutItems": items})
			}
		}
		id := strings.TrimSpace(section.ID)
		if id == "" {
			id = fmt.Sprintf("section-%d", index+1)
		}
		heading := strings.TrimSpace(section.Label)
		if heading == "" {
			heading = labelOrFallback(def.Label, def.APIName)
		}
		sections = append(sections, map[string]any{
			"id":          id,
			"heading":     heading,
			"columns":     columnCount,
			"rows":        len(rows),
			"layoutRows":  rows,
			"collapsible": false,
			"useHeading":  section.UseHeading,
			"tabOrder":    layoutSectionTabOrder(section.Style),
		})
	}
	return sections
}

func fallbackLayoutSectionsPayload(objectName string, def storage.ObjectDefinition, fieldNames []string) []map[string]any {
	columns := 1
	if len(fieldNames) > 1 {
		columns = 2
	}
	rows := make([]map[string]any, 0, (len(fieldNames)+columns-1)/columns)
	for i := 0; i < len(fieldNames); i += columns {
		items := make([]map[string]any, 0, columns)
		for j := 0; j < columns && i+j < len(fieldNames); j++ {
			fieldName := fieldNames[i+j]
			field := def.Fields[fieldName]
			if !fieldCreateable(field) {
				continue
			}
			items = append(items, recordLayoutFieldItem(fieldName, field, ""))
		}
		if len(items) > 0 {
			rows = append(rows, map[string]any{"layoutItems": items})
		}
	}
	return []map[string]any{{
		"id":          "main",
		"heading":     labelOrFallback(def.Label, objectName),
		"columns":     columns,
		"rows":        len(rows),
		"layoutRows":  rows,
		"collapsible": false,
		"useHeading":  true,
		"tabOrder":    "TopDown",
	}}
}

func recordLayoutFieldItem(fieldName string, field storage.Field, behavior string) map[string]any {
	required, editableForNew, editableForUpdate, uiBehavior := recordLayoutItemBehavior(field, behavior)
	label := labelOrFallback(field.Label, fieldName)
	return map[string]any{
		"fieldApiName":      fieldName,
		"label":             label,
		"required":          required,
		"editableForNew":    editableForNew,
		"editableForUpdate": editableForUpdate,
		"sortable":          false,
		"uiBehavior":        uiBehavior,
		"layoutComponents": []map[string]any{{
			"apiName":       fieldName,
			"componentType": "Field",
			"label":         label,
		}},
	}
}

func recordLayoutItemBehavior(field storage.Field, behavior string) (bool, bool, bool, string) {
	switch strings.ToLower(strings.TrimSpace(behavior)) {
	case "required":
		return true, true, fieldUpdateable(field), "Required"
	case "readonly", "read only", "read-only":
		return false, false, false, "Readonly"
	case "edit":
		if field.Required {
			return true, true, fieldUpdateable(field), "Required"
		}
		return false, true, fieldUpdateable(field), "Edit"
	default:
		if field.Required {
			return true, true, fieldUpdateable(field), "Required"
		}
		return false, true, fieldUpdateable(field), "Edit"
	}
}

func layoutSectionTabOrder(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "twocolumnslefttoright":
		return "LeftRight"
	default:
		return "TopDown"
	}
}

func createDefaultsRecordTypeID(def storage.ObjectDefinition, requested string) string {
	if trimmed := strings.TrimSpace(requested); trimmed != "" {
		return trimmed
	}
	for _, recordType := range def.RecordTypes {
		if recordType.Default && recordType.ID != "" {
			return string(recordType.ID)
		}
	}
	for _, recordType := range def.RecordTypes {
		if recordType.ID != "" && (recordType.Active || recordType.Available) {
			return string(recordType.ID)
		}
	}
	return "012000000000000AAA"
}

func createDefaultFieldNames(def storage.ObjectDefinition, namespace string, optionalFields []string) []string {
	names := make([]string, 0, len(def.Fields)+len(optionalFields))
	seen := make(map[string]bool, len(def.Fields)+len(optionalFields))
	for name := range def.Fields {
		field := def.Fields[name]
		if !fieldCreateable(field) {
			continue
		}
		if _, ok := storage.DefaultValueForField(field); ok || field.Required || strings.EqualFold(name, "RecordTypeId") {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	for _, ref := range optionalFields {
		fieldName := strings.TrimSpace(ref)
		if objectName, name, ok := splitFieldRef(fieldName); ok {
			if !strings.EqualFold(objectName, def.APIName) {
				continue
			}
			fieldName = name
		}
		canonical, ok := storage.ResolveFieldName(def, namespace, fieldName)
		if !ok || seen[canonical] {
			continue
		}
		seen[canonical] = true
		names = append(names, canonical)
	}
	return names
}

func fieldCreateable(field storage.Field) bool {
	createable := field.Type != storage.FieldID && field.Type != storage.FieldCalculated
	if field.Createable != nil {
		createable = *field.Createable
	}
	return createable
}

func fieldUpdateable(field storage.Field) bool {
	updateable := field.Type != storage.FieldID && field.Type != storage.FieldCalculated
	if field.Updateable != nil {
		updateable = *field.Updateable
	}
	return updateable
}

func batchWireResult(data map[string]any, wireErr *lwcbrowser.WireError) map[string]any {
	if wireErr != nil {
		status := wireErr.Status
		if status == 0 {
			status = http.StatusNotFound
		}
		errorCode := strings.TrimSpace(wireErr.Type)
		if errorCode == "" {
			errorCode = "NOT_FOUND"
		}
		return map[string]any{
			"statusCode": status,
			"result": map[string]any{
				"errorCode": errorCode,
				"message":   wireErr.Message,
			},
		}
	}
	return map[string]any{
		"statusCode": http.StatusOK,
		"result":     data,
	}
}

func getPicklistValuesWireData(org *storage.OrgState, req lwcbrowser.WireGetPicklistValuesRequest) (map[string]any, *lwcbrowser.WireError) {
	objectName, fieldName, ok := splitFieldRef(req.FieldAPIName)
	if !ok {
		objectName = strings.TrimSpace(req.ObjectAPIName)
		fieldName = strings.TrimSpace(req.FieldAPIName)
	}
	objectName, object, ok := findOrgObject(org, objectName)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("object not found: %s", req.ObjectAPIName)}
	}
	canonical, ok := storage.ResolveFieldName(object.Definition, org.Namespace, fieldName)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("field not found: %s", fieldName)}
	}
	field := object.Definition.Fields[canonical]
	return map[string]any{
		"controllerValues": map[string]any{},
		"defaultValue":     defaultPicklistValue(field, req.RecordTypeID),
		"url":              fmt.Sprintf("/lightning/wire/getPicklistValues/%s.%s", objectName, canonical),
		"values":           picklistValuesPayload(field, req.RecordTypeID),
	}, nil
}

func getPicklistValuesByRecordTypeWireData(org *storage.OrgState, req lwcbrowser.WireGetPicklistValuesByRecordTypeRequest) (map[string]any, *lwcbrowser.WireError) {
	objectName, object, ok := findOrgObject(org, req.ObjectAPIName)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("object not found: %s", req.ObjectAPIName)}
	}
	out := map[string]any{}
	for name, field := range object.Definition.Fields {
		if field.Type != storage.FieldPicklist && field.Type != storage.FieldMultiPicklist {
			continue
		}
		out[name] = map[string]any{
			"controllerValues": map[string]any{},
			"defaultValue":     defaultPicklistValue(field, req.RecordTypeID),
			"values":           picklistValuesPayload(field, req.RecordTypeID),
		}
	}
	return map[string]any{
		"objectApiName":       objectName,
		"recordTypeId":        strings.TrimSpace(req.RecordTypeID),
		"picklistFieldValues": out,
	}, nil
}

func getRelatedListRecordsWireData(org *storage.OrgState, req lwcbrowser.WireGetRelatedListRecordsRequest) (map[string]any, *lwcbrowser.WireError) {
	parentObjectName, _, ok := findOrgRecord(org, req.ParentRecordID)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("parent record not found: %s", req.ParentRecordID)}
	}
	parentObject, ok := org.Objects[parentObjectName]
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("parent object not found: %s", parentObjectName)}
	}
	relationshipName := strings.TrimSpace(req.RelatedListID)
	var relation storage.Relationship
	for _, candidate := range parentObject.Definition.Relations {
		if strings.EqualFold(candidate.ChildRelationship, relationshipName) {
			relation = candidate
			break
		}
	}
	if relation.ChildRelationship == "" || relation.Field == "" {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("related list not found: %s", relationshipName)}
	}
	childObjectName, childObject, ok := findChildObjectForRelationship(org, parentObjectName, relation)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("related list child object not found: %s", relationshipName)}
	}
	records := make([]map[string]any, 0)
	for id, record := range childObject.Records {
		if record.System.IsDeleted {
			continue
		}
		value, ok := record.Fields[relation.Field]
		if !ok || fmt.Sprint(storageValueJSON(value)) != strings.TrimSpace(req.ParentRecordID) {
			continue
		}
		record.ID = id
		record.Object = childObjectName
		row, _ := getRecordWireData(org, string(id), req.Fields, req.OptionalFields)
		if row != nil {
			records = append(records, row)
		}
	}
	return map[string]any{
		"count":              len(records),
		"currentPageToken":   nil,
		"nextPageToken":      nil,
		"previousPageToken":  nil,
		"records":            records,
		"relatedListId":      relationshipName,
		"parentRecordId":     strings.TrimSpace(req.ParentRecordID),
		"childObjectApiName": childObjectName,
	}, nil
}

func recordPickerSearchWireData(org *storage.OrgState, req lwcbrowser.WireRecordPickerSearchRequest) (map[string]any, *lwcbrowser.WireError) {
	objectName, object, ok := findOrgObject(org, req.ObjectAPIName)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("object not found: %s", req.ObjectAPIName)}
	}
	fields := normalizeRecordPickerFields(req.Fields)
	matchingFields := normalizeRecordPickerFields(req.MatchingFields)
	if len(matchingFields) == 0 {
		matchingFields = fields
	}
	term := strings.ToLower(strings.TrimSpace(req.SearchTerm))
	ids := make([]string, 0, len(object.Records))
	for id := range object.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	limit := req.PageSize
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	records := make([]map[string]any, 0)
	for _, id := range ids {
		record := object.Records[storage.ID(id)]
		record.ID = storage.ID(id)
		record.Object = objectName
		if record.System.IsDeleted || !recordPickerMatches(record, matchingFields, term) {
			continue
		}
		records = append(records, recordPickerRow(objectName, object.Definition, record, fields))
		if len(records) >= limit {
			break
		}
	}
	return map[string]any{
		"objectApiName": objectName,
		"records":       records,
	}, nil
}

func normalizeRecordPickerFields(fields []string) []string {
	out := make([]string, 0, len(fields)+1)
	seen := map[string]bool{}
	add := func(name string) {
		name = wireFieldName(name)
		if name == "" || seen[strings.ToLower(name)] {
			return
		}
		seen[strings.ToLower(name)] = true
		out = append(out, name)
	}
	add("Name")
	for _, field := range fields {
		add(field)
	}
	return out
}

func recordPickerMatches(record storage.Record, fields []string, term string) bool {
	if term == "" {
		return true
	}
	for _, field := range fields {
		value, ok := record.Fields[field]
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(fmt.Sprint(storageValueJSON(value))), term) {
			return true
		}
	}
	return false
}

func recordPickerRow(objectName string, def storage.ObjectDefinition, record storage.Record, fields []string) map[string]any {
	fieldPayload := map[string]any{}
	for _, fieldName := range fields {
		value, ok := record.Fields[fieldName]
		if !ok {
			continue
		}
		field := def.Fields[fieldName]
		fieldPayload[fieldName] = recordFieldWirePayload(fieldName, field, field.APIName != "", labelOrFallback(field.Label, fieldName), storageValueJSON(value))
	}
	title := string(record.ID)
	if value, ok := record.Fields["Name"]; ok {
		title = fmt.Sprint(storageValueJSON(value))
	}
	return map[string]any{
		"id":      string(record.ID),
		"apiName": objectName,
		"title":   title,
		"fields":  fieldPayload,
	}
}

func createRecordWireData(org *storage.OrgState, req lwcbrowser.WireCreateRecordRequest) (map[string]any, *lwcbrowser.WireError) {
	objectName, _, ok := findOrgObject(org, req.APIName)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("object not found: %s", req.APIName)}
	}
	record := storage.Record{
		Object: objectName,
		Fields: map[string]storage.Value{},
	}
	for fieldName, raw := range req.Fields {
		if strings.EqualFold(fieldName, "Id") {
			continue
		}
		record.Fields[fieldName] = storageValueFromAny(raw)
	}
	engine := dml.NewEngine(org)
	results := engine.Insert([]storage.Record{record})
	if len(results) != 1 || !results[0].Success {
		return nil, wireErrorFromDMLResult(firstDMLResult(results, "create failed"))
	}
	stored := org.Objects[objectName].Records[results[0].ID]
	stored.ID = results[0].ID
	return recordWireMutationPayload(objectName, stored), nil
}

func updateRecordWireData(org *storage.OrgState, req lwcbrowser.WireUpdateRecordRequest) (map[string]any, *lwcbrowser.WireError) {
	rawID, ok := req.Fields["Id"]
	if !ok {
		return nil, &lwcbrowser.WireError{Message: "fields.Id is required"}
	}
	recordID := strings.TrimSpace(fmt.Sprint(rawID))
	objectName, record, ok := findOrgRecord(org, recordID)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("record not found: %s", recordID)}
	}
	updates := storage.Record{
		ID:            record.ID,
		Object:        objectName,
		Fields:        map[string]storage.Value{},
		ExplicitNulls: map[string]bool{},
	}
	for fieldName, raw := range req.Fields {
		if strings.EqualFold(fieldName, "Id") {
			continue
		}
		if raw == nil {
			updates.ExplicitNulls[fieldName] = true
			continue
		}
		updates.Fields[fieldName] = storageValueFromAny(raw)
	}
	engine := dml.NewEngine(org)
	results := engine.Update([]storage.Record{updates})
	if len(results) != 1 || !results[0].Success {
		return nil, wireErrorFromDMLResult(firstDMLResult(results, "update failed"))
	}
	stored := org.Objects[objectName].Records[record.ID]
	stored.ID = record.ID
	return recordWireMutationPayload(objectName, stored), nil
}

func deleteRecordWireData(org *storage.OrgState, recordID string) (map[string]any, *lwcbrowser.WireError) {
	recordID = strings.TrimSpace(recordID)
	objectName, record, ok := findOrgRecord(org, recordID)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("record not found: %s", recordID)}
	}
	engine := dml.NewEngine(org)
	results := engine.Delete([]storage.Record{{ID: record.ID, Object: objectName}})
	if len(results) != 1 || !results[0].Success {
		return nil, wireErrorFromDMLResult(firstDMLResult(results, "delete failed"))
	}
	return map[string]any{"id": string(record.ID), "apiName": objectName, "deleted": true}, nil
}

func firstDMLResult(results []dml.Result, fallback string) dml.Result {
	if len(results) > 0 {
		return results[0]
	}
	return dml.Result{Error: fallback, StatusCode: "UNKNOWN_EXCEPTION"}
}

func wireErrorFromDMLResult(result dml.Result) *lwcbrowser.WireError {
	if len(result.Errors) > 0 {
		err := result.Errors[0]
		return &lwcbrowser.WireError{Type: err.StatusCode, Message: err.Message}
	}
	if strings.TrimSpace(result.StatusCode) != "" || strings.TrimSpace(result.Error) != "" {
		return &lwcbrowser.WireError{Type: result.StatusCode, Message: result.Error}
	}
	return &lwcbrowser.WireError{Message: "DML operation failed"}
}

func recordWireMutationPayload(objectName string, record storage.Record) map[string]any {
	fields := map[string]any{}
	for name, value := range record.Fields {
		fields[name] = map[string]any{"value": storageValueJSON(value)}
	}
	return map[string]any{
		"id":      string(record.ID),
		"apiName": objectName,
		"fields":  fields,
	}
}

func splitFieldRef(ref string) (objectName, fieldName string, ok bool) {
	ref = strings.TrimSpace(ref)
	if dot := strings.LastIndex(ref, "."); dot > 0 && dot < len(ref)-1 {
		return ref[:dot], ref[dot+1:], true
	}
	return "", "", false
}

func picklistValuesPayload(field storage.Field, recordTypeID string) []map[string]any {
	defaultValue := defaultPicklistValueName(field, recordTypeID)
	values := make([]map[string]any, 0, len(field.PicklistValues))
	for _, value := range field.PicklistValues {
		active := value.Active
		if !active && value.Value == "" && value.Label == "" {
			continue
		}
		values = append(values, map[string]any{
			"attributes":   nil,
			"label":        labelOrFallback(value.Label, value.Value),
			"value":        value.Value,
			"validFor":     []string{},
			"defaultValue": value.Default || value.Value == defaultValue,
			"active":       active || value.Active,
		})
	}
	return values
}

func defaultPicklistValue(field storage.Field, recordTypeID string) map[string]any {
	defaultName := defaultPicklistValueName(field, recordTypeID)
	if defaultName == "" {
		return nil
	}
	for _, value := range field.PicklistValues {
		if value.Value == defaultName {
			return map[string]any{
				"attributes":   nil,
				"label":        labelOrFallback(value.Label, value.Value),
				"value":        value.Value,
				"validFor":     []string{},
				"defaultValue": true,
				"active":       value.Active,
			}
		}
	}
	return nil
}

func defaultPicklistValueName(field storage.Field, _ string) string {
	for _, value := range field.PicklistValues {
		if value.Default {
			return value.Value
		}
	}
	return ""
}

func findChildObjectForRelationship(org *storage.OrgState, parentObjectName string, relation storage.Relationship) (string, storage.ObjectState, bool) {
	for name, object := range org.Objects {
		field, ok := object.Definition.Fields[relation.Field]
		if !ok || field.Type != storage.FieldReference {
			continue
		}
		for _, parent := range field.ReferenceTo {
			if strings.EqualFold(parent, parentObjectName) {
				return name, object, true
			}
		}
	}
	return "", storage.ObjectState{}, false
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
		if storedID, rec, found := storage.LookupRecordByID(object.Records, id); found {
			if rec.System.IsDeleted {
				return "", storage.Record{}, false
			}
			rec.ID = storedID
			return name, rec, true
		}
	}
	return "", storage.Record{}, false
}

func storageValueFromAny(raw any) storage.Value {
	switch value := raw.(type) {
	case nil:
		return storage.NullValue()
	case bool:
		return storage.BooleanValue(value)
	case float64:
		return storage.DecimalValue(fmt.Sprint(value))
	case string:
		return storage.StringValue(value)
	default:
		return storage.StringValue(fmt.Sprint(value))
	}
}

func findOrgObject(org *storage.OrgState, objectAPIName string) (objectName string, object storage.ObjectState, ok bool) {
	if org == nil {
		return "", storage.ObjectState{}, false
	}
	objectAPIName = strings.TrimSpace(objectAPIName)
	if canonical, known := storage.ResolveKnownStandardObjectName(objectAPIName); known {
		storage.EnsureStandardObject(org, canonical)
		objectAPIName = canonical
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
