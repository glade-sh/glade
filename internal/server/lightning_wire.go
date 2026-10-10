package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
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
	case "getRelatedListCount", "getRelatedListInfo", "getRelatedListsInfo", "getRelatedListRecordsBatch", "getRelatedListInfoBatch",
		"getListInfoByName", "getListInfosByName", "getListInfosByObjectName", "getListRecordsByName", "getListPreferences", "getListUi":
		s.handleLightningListRead(w, r, parts[0])
	case "deleteListInfo":
		s.handleLightningListDelete(w, r)
	case "createListInfo", "updateListInfoByName", "updateListPreferences":
		s.handleLightningListWrite(w, r, parts[0])
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
	s.invokeLightningApex(w, r, req.ClassName, req.Method, req.Params, req.Cacheable)
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
	s.invokeLightningApex(w, r, parts[0], parts[1], raw, false)
}

func (s *Server) invokeLightningApex(w http.ResponseWriter, r *http.Request, className, methodName string, rawParams any, cacheable bool) {
	machine, err := s.visualforceRuntime()
	if err != nil {
		writeWireJSON(w, lwcbrowser.WireResponse{
			Error: apexWireInvocationError("", "UnsupportedFeature", className, methodName, rawParams, err.Error(), http.StatusInternalServerError),
		})
		return
	}
	// LWC Apex actions are mutating request boundaries. Execute against an
	// isolated org and publish it only after the action succeeds and the
	// backing store accepts the result. Using s.Org directly would leak DML
	// from a later Apex exception and make persistence failures irreversible.
	workingOrg := s.Org.Clone()
	machine.SetOrg(&workingOrg)
	machine.SetSynchronousActionBoundary(true)
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
	if cacheable && !s.lightningApexCacheable(className, methodName) {
		action := &vm.UIActionError{Type: "InvalidActionParameter", Message: "Apex methods that are to be cached must be marked as @AuraEnabled(cacheable=true)"}
		writeWireJSON(w, lwcbrowser.WireResponse{Error: apexWireActionError(action, className, methodName, rawParams)})
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
			out.Error = apexWireActionError(result.Error, className, methodName, rawParams)
		} else {
			out.Error = apexWireInvocationError("", "ApexException", className, methodName, rawParams, "apex wire call failed", http.StatusInternalServerError)
		}
		writeWireJSON(w, out)
		return
	}
	if machine.HasRejectedAsyncAction() {
		writeWireJSON(w, lwcbrowser.WireResponse{
			Error: apexWireInvocationError("", "UnsupportedFeature", className, methodName, rawParams, "asynchronous Apex work cannot be committed by the local synchronous LWC action boundary", http.StatusNotImplemented),
		})
		return
	}
	if machine.HasPendingAsyncWork() {
		writeWireJSON(w, lwcbrowser.WireResponse{
			Error: apexWireInvocationError("", "UnsupportedFeature", className, methodName, rawParams, "asynchronous Apex work cannot be committed by the local synchronous LWC action boundary", http.StatusNotImplemented),
		})
		return
	}
	if !orgStateEqual(workingOrg, *s.Org) {
		if err := s.commitOrg(workingOrg); err != nil {
			writeWireJSON(w, lwcbrowser.WireResponse{
				Error: apexWireInvocationError("", "StoreFailure", className, methodName, rawParams, err.Error(), http.StatusInternalServerError),
			})
			return
		}
	}
	writeWireJSON(w, lwcbrowser.WireResponse{Data: result.ReturnValue})
}

func (s *Server) lightningApexCacheable(className, methodName string) bool {
	if s.Index == nil {
		return false
	}
	for _, typ := range s.Index.Types {
		if !strings.EqualFold(typ.Name, strings.TrimSpace(className)) {
			continue
		}
		for _, member := range typ.Members {
			if !strings.EqualFold(member.Name, strings.TrimSpace(methodName)) {
				continue
			}
			for _, annotation := range member.Annotations {
				if !strings.EqualFold(annotation.Name, "AuraEnabled") {
					continue
				}
				for _, argument := range annotation.Arguments {
					if strings.EqualFold(argument.Name, "cacheable") && argument.Value == "true" {
						return true
					}
				}
			}
		}
	}
	return false
}

// Action failures expose the captured Salesforce body. Local unsupported
// boundaries keep their explicit diagnostic and invocation context.
func apexWireActionError(action *vm.UIActionError, className, methodName string, params any) *lwcbrowser.WireError {
	if action.Type == "AuraHandledException" || action.Type == "InvalidActionParameter" {
		return &lwcbrowser.WireError{Type: action.Type, Message: action.Message, Status: http.StatusInternalServerError, Body: &lwcbrowser.WireErrorBody{Message: action.Message}}
	}
	if strings.HasSuffix(action.Type, "Exception") {
		exceptionType := action.ExceptionTypeName()
		userDefined := !strings.HasPrefix(exceptionType, "System.")
		message := action.Message
		if exceptionType == "System.NullPointerException" {
			// The VM's receiver context belongs in local diagnostics, not the
			// captured action exception message (r_error_null_pointer_*).
			if start := strings.Index(message, " (context:"); start >= 0 {
				message = message[:start]
			}
		}
		return &lwcbrowser.WireError{Type: action.Type, Message: message, Status: http.StatusInternalServerError, Body: &lwcbrowser.WireErrorBody{
			Message: message, ExceptionType: exceptionType, IsUserDefinedException: &userDefined,
			StackTrace: apexWireStackTrace(className+"."+methodName, action.Message),
		}}
	}
	return apexWireInvocationError(action.Code, action.Type, className, methodName, params, action.Message, http.StatusInternalServerError)
}

func orgStateEqual(left, right storage.OrgState) bool {
	leftBytes, leftErr := json.Marshal(left)
	rightBytes, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return bytes.Equal(leftBytes, rightBytes)
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
	data, wireErr := getLDSRecordWireData(s.Org, req, s.Source)
	writeLDSRecordResponse(w, data, wireErr)
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
	if strings.Contains(req.ObjectAPIName, ".") {
		writeObjectMetadataWireError(w, http.StatusForbidden, "INSUFFICIENT_ACCESS", "391411223", "You don't have access to this record. Ask your administrator for help or to request access.")
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
	// Unlike REST describe, this adapter requires the canonical API-name case.
	// An empty envelope leaves both wire values undefined.
	if data["apiName"] != req.ObjectAPIName {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct{}{})
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
		if writeObjectMetadataDataError(w, capturedCreateDefaultsErrorEnvelope(req, wireErr.Code), wireErr.Code, wireErr.Message) {
			return
		}
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
	if req.ObjectAPIName == "" && !strings.Contains(req.FieldAPIName, ".") {
		writeObjectMetadataWireError(w, http.StatusBadRequest, "MISSING_ARGUMENT", "16767885", "Parameter required: fieldApiName")
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	data, wireErr := getPicklistValuesWireData(s.Org, req)
	if wireErr != nil {
		if writeObjectMetadataDataError(w, capturedPicklistErrorEnvelope(req, wireErr.Code), wireErr.Code, wireErr.Message) {
			return
		}
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
	s.handleLightningListRead(w, r, "getRelatedListRecords")
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
	// LDS creation provisions the Full layout, including its nullable fields
	// and related identities. Keep the direct mutation helper independent of
	// source metadata for its existing callers.
	if wireErr == nil {
		if _, hasLayout := sourceCreateLayout(s.Source, req.APIName); hasLayout {
			data, wireErr = getLDSRecordWireData(s.Org, lwcbrowser.WireGetRecordRequest{
				RecordID: fmt.Sprint(data["id"]), LayoutTypes: []string{"Full"}, Modes: []string{"View"},
			}, s.Source)
		}
	}
	writeLDSRecordResponse(w, data, wireErr)
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
	writeLDSRecordResponse(w, data, wireErr)
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
	writeLDSRecordResponse(w, data, wireErr)
}

func getRecordWireData(org *storage.OrgState, recordID string, fields []string, optionalFields []string) (map[string]any, *lwcbrowser.WireError) {
	recordID = strings.TrimSpace(recordID)
	if recordID == "" {
		return nil, &lwcbrowser.WireError{Message: "recordId is required"}
	}
	objectName, record, ok := findOrgRecord(org, recordID)
	if !ok {
		return nil, &lwcbrowser.WireError{Type: "NOT_FOUND", Status: http.StatusNotFound, RecordErrorID: "-857233874", Message: "The requested resource does not exist"}
	}
	fieldNames := make([]string, 0, len(fields)+len(optionalFields))
	optionalByName := map[string]bool{}
	for _, ref := range fields {
		requestedObject, name, qualified := strings.Cut(ref, ".")
		if !qualified || name == "" {
			continue
		}
		if !strings.EqualFold(requestedObject, objectName) {
			return nil, &lwcbrowser.WireError{Type: "INVALID_INPUT", Status: http.StatusBadRequest, RecordErrorID: "945406546", Message: fmt.Sprintf(`The "fields" query string parameter contained object api names that do not correspond to the api names of any of the requested record ids. The requested object api names were: [%s], while the requested records had object types: [%s]`, requestedObject, objectName)}
		}
		if name != "" {
			fieldNames = append(fieldNames, name)
		}
	}
	for _, ref := range optionalFields {
		if requestedObject, name, qualified := strings.Cut(ref, "."); qualified && strings.EqualFold(requestedObject, objectName) && name != "" {
			fieldNames = append(fieldNames, name)
			optionalByName[strings.ToLower(name)] = true
		}
	}
	if len(fields) == 0 && len(optionalFields) == 0 {
		for name := range record.Fields {
			if name != "Id" && name != "attributes" {
				fieldNames = append(fieldNames, name)
			}
		}
	}
	fieldsOut := make(map[string]any, len(fieldNames))
	for _, name := range fieldNames {
		if relationship, rest, nested := strings.Cut(name, "."); nested {
			object := org.Objects[objectName]
			for key, field := range object.Definition.Fields {
				if field.RelationshipName != relationship {
					continue
				}
				value, _ := ldsRecordValue(record, key)
				parentID := fmt.Sprint(storageValueJSON(value))
				parentName, parentRecord, found := findOrgRecord(org, parentID)
				if !found {
					fieldsOut[relationship] = map[string]any{"value": nil, "displayValue": nil}
					break
				}
				parent, wireErr := getRecordWireData(org, parentID, []string{parentName + "." + rest}, nil)
				if wireErr != nil {
					return nil, wireErr
				}
				if previous, ok := fieldsOut[relationship].(map[string]any); ok {
					if data, ok := previous["value"].(map[string]any); ok {
						for key, field := range data["fields"].(map[string]any) {
							parent["fields"].(map[string]any)[key] = field
						}
					}
				}
				var display any
				if label, ok := parentRecord.GetField("Name"); ok {
					display = storageValueJSON(label)
				}
				fieldsOut[relationship] = map[string]any{"value": parent, "displayValue": display}
				break
			}
			if _, found := fieldsOut[relationship]; found {
				continue
			}
		}
		fieldName := name
		field, hasField := storage.Field{}, false
		if object, ok := org.Objects[objectName]; ok {
			if canonical, ok := storage.ResolveFieldName(object.Definition, org.Namespace, name); ok {
				fieldName = canonical
				field = object.Definition.Fields[canonical]
				hasField = true
			}
		}
		value, ok := ldsRecordValue(record, fieldName)
		if !hasField && !ok {
			if optionalByName[strings.ToLower(name)] {
				continue
			}
			return nil, &lwcbrowser.WireError{Type: "INVALID_FIELD", Status: http.StatusBadRequest, RecordErrorID: "-92527422", Message: fmt.Sprintf("field not found: %s", name)}
		}
		if !ok {
			fieldsOut[fieldName] = ldsRecordFieldPayload(field, nil)
			continue
		}
		if !hasField && value.Kind == storage.ValueDateTime {
			field.Type = storage.FieldDateTime
		}
		jsonVal := recordWireValueJSON(value)
		fieldsOut[fieldName] = ldsRecordFieldPayload(field, jsonVal)
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

func ldsRecordValue(record storage.Record, name string) (storage.Value, bool) {
	// LDS selects audit fields from the shared system-field storage, not just
	// the explicit business-field map. Captured conditional-update controls
	// read these values and LastModifiedBy.Name before issuing the update.
	switch strings.ToLower(name) {
	case "id":
		return storage.IDValue(record.ID), true
	case "ownerid":
		if record.System.OwnerID != "" {
			return storage.IDValue(record.System.OwnerID), true
		}
		return record.GetField(name)
	case "createddate":
		return storage.DateTimeValue(record.System.CreatedDate), true
	case "createdbyid":
		return storage.IDValue(record.System.CreatedByID), true
	case "lastmodifieddate":
		return storage.DateTimeValue(recordLastModifiedDate(record)), true
	case "lastmodifiedbyid":
		return storage.IDValue(storage.ID(recordLastModifiedByID(record))), true
	default:
		return record.GetField(name)
	}
}

func ldsRecordFieldPayload(field storage.Field, value any) map[string]any {
	out := map[string]any{
		"value":        value,
		"displayValue": nil,
	}
	if value != nil && field.Type != storage.FieldString && field.Type != storage.FieldInteger && field.Type != storage.FieldID && field.Type != storage.FieldReference {
		out["displayValue"] = fmt.Sprint(value)
	}
	if value != nil && field.Type == storage.FieldDateTime {
		if stamp, err := time.Parse(time.RFC3339Nano, fmt.Sprint(value)); err == nil {
			// The default local LDS context is en-US/UTC. Keep the raw value;
			// native audit-field displays use numeric dates and minute precision.
			out["displayValue"] = stamp.UTC().Format("1/2/2006, 3:04 PM")
		}
	}
	if value != nil && strings.EqualFold(field.DisplayType, "CURRENCY") {
		if number, err := strconv.ParseFloat(fmt.Sprint(value), 64); err == nil {
			// The local i18n context uses en-US/USD. UI API displays currency
			// at the field's scale without rounding the underlying JSON value.
			factor := math.Pow10(field.Scale)
			text := strconv.FormatFloat(math.Round(number*factor)/factor, 'f', field.Scale, 64)
			sign := ""
			if strings.HasPrefix(text, "-") {
				sign, text = "-", text[1:]
			}
			integer, fraction, fractional := strings.Cut(text, ".")
			for index := len(integer) - 3; index > 0; index -= 3 {
				integer = integer[:index] + "," + integer[index:]
			}
			text = sign + "$" + integer
			if fractional {
				text += "." + fraction
			}
			out["displayValue"] = text
		}
	}
	return out
}

// UI API record values are JSON numbers, unlike the string decimal transport
// used by the REST describe/query paths. Field metadata belongs to objectInfo,
// not to each record's value/displayValue wrapper.
func recordWireValueJSON(value storage.Value) any {
	if value.Kind == storage.ValueDecimal {
		return json.Number(value.Decimal)
	}
	return storageValueJSON(value)
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
		seen := map[storage.ID]bool{}
		for _, recordID := range item.RecordIDs {
			id := storage.ID(recordID)
			if _, record, ok := findOrgRecord(org, recordID); ok {
				id = record.ID
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			data, wireErr := getRecordWireData(org, recordID, item.Fields, item.OptionalFields)
			result := batchWireResult(data, wireErr)
			if wireErr != nil && wireErr.Type == "INVALID_FIELD" {
				message := wireErr.Message
				if objectName, _, ok := findOrgRecord(org, recordID); ok {
					message = recordBatchFieldDiagnostic(objectName, strings.TrimPrefix(message, "field not found: "))
				}
				result["result"] = []map[string]any{{"errorCode": wireErr.Type, "message": message}}
			}
			results = append(results, result)
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
			fields[name] = objectInfoFieldPayload(field, object.Definition.Fields[name])
		}
	}
	payload["fields"] = fields
	payload["childRelationships"] = objectInfoRelationships(org, objectName, payload["childRelationships"])
	payload["recordTypeInfos"] = recordTypeInfosByID(payload["recordTypeInfos"])
	if objectName == "User" || objectName == "Group" || objectName == "Name" {
		payload["recordTypeInfos"] = map[string]any{}
	}
	addObjectInfoUIProperties(payload, objectName)
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
		out[id] = map[string]any{
			"available":                item["available"],
			"defaultRecordTypeMapping": item["defaultRecordTypeMapping"],
			"master":                   id == "012000000000000AAA" || id == "012000000000000",
			"name":                     item["name"],
			"recordTypeId":             id,
		}
	}
	return out
}

func getObjectInfosWireData(org *storage.OrgState, req lwcbrowser.WireGetObjectInfosRequest) map[string]any {
	results := make([]map[string]any, 0, len(req.ObjectAPINames))
	seen := make(map[string]bool, len(req.ObjectAPINames))
	for _, objectName := range req.ObjectAPINames {
		if seen[objectName] {
			continue
		}
		seen[objectName] = true
		data, wireErr := getObjectInfoWireData(org, objectName)
		results = append(results, batchWireResult(data, wireErr))
	}
	return map[string]any{"results": results}
}

func getRecordCreateDefaultsWireData(org *storage.OrgState, req lwcbrowser.WireGetRecordCreateDefaultsRequest, source SourceMetadata) (map[string]any, *lwcbrowser.WireError) {
	for _, ref := range req.OptionalFields {
		if !strings.Contains(ref, ".") {
			return nil, &lwcbrowser.WireError{Code: "ILLEGAL_QUERY_PARAMETER_VALUE", Message: fmt.Sprintf("Expected '.' in all qualified names: %s is invalid", ref)}
		}
	}
	objectName, object, ok := findOrgObject(org, req.ObjectAPIName)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("object not found: %s", req.ObjectAPIName)}
	}
	objectInfo, wireErr := getObjectInfoWireData(org, objectName)
	if wireErr != nil {
		return nil, wireErr
	}
	recordTypeID := objectMetadataRecordTypeID(object.Definition, req.RecordTypeID)
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
	layoutData := recordCreateDefaultsLayout(objectName, object.Definition, org.Namespace, recordTypeID, fieldNames, layout, hasSourceLayout)
	if metadataLayout, ok := objectMetadataLayout(objectName, object.Definition, org.Namespace, recordTypeID, "Full", "Create", source); ok {
		layoutData = metadataLayout
	}
	return map[string]any{
		"apiName":      objectName,
		"recordTypeId": recordTypeID,
		"objectInfos":  createDefaultsObjectInfos(org, objectName, object.Definition, source, org.Namespace, objectInfo),
		"layout":       layoutData,
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
	recordTypeID := objectMetadataRecordTypeID(object.Definition, req.RecordTypeID)
	if layout, ok := objectMetadataLayout(objectName, object.Definition, org.Namespace, recordTypeID, layoutTypeOrDefault(req.LayoutType), layoutModeOrDefault(req.Mode), source); ok {
		return layout, nil
	}
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
				items = append(items, recordLayoutFieldItem(def.APIName, canonical, field, item.Behavior))
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
			items = append(items, recordLayoutFieldItem(objectName, fieldName, field, ""))
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

func recordLayoutFieldItem(objectName, fieldName string, field storage.Field, behavior string) map[string]any {
	required, editableForNew, editableForUpdate, uiBehavior := recordLayoutItemBehavior(field, behavior)
	label := labelOrFallback(field.Label, fieldName)
	itemLabel := label
	if alias := standardLayoutItemLabels[objectName][fieldName]; alias != "" {
		itemLabel = alias
	}
	return map[string]any{
		"fieldApiName":      fieldName,
		"label":             itemLabel,
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
	if field.Type != storage.FieldPicklist && field.Type != storage.FieldMultiPicklist {
		return nil, &lwcbrowser.WireError{Code: "INVALID_FIELD", Message: fmt.Sprintf("Field %s is not a picklist.", canonical)}
	}
	defaultValue := defaultPicklistValue(field, req.RecordTypeID)
	defaultPicklistStatusAttributes(org, objectName, canonical, defaultValue)
	return map[string]any{
		"controllerValues": map[string]any{},
		"defaultValue":     defaultValue,
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
		if len(field.PicklistValues) == 0 {
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
	parentObjectName, parentRecord, ok := findOrgRecord(org, req.ParentRecordID)
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("parent record not found: %s", req.ParentRecordID)}
	}
	_, ok = org.Objects[parentObjectName]
	if !ok {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("parent object not found: %s", parentObjectName)}
	}
	relationshipName := strings.TrimSpace(req.RelatedListID)
	childObjectName, childObject, relation, ok := relatedListChild(org, parentObjectName, relationshipName)
	if !ok || relation.ChildRelationship == "" || relation.Field == "" {
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("related list not found: %s", relationshipName)}
	}
	records := make([]map[string]any, 0)
	for id, record := range childObject.Records {
		if record.System.IsDeleted {
			continue
		}
		value, ok := record.Fields[relation.Field]
		if !ok || !storage.IDsEqual(storage.ID(fmt.Sprint(storageValueJSON(value))), parentRecord.ID) {
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
	objectName, object, ok := findOrgObject(org, req.APIName)
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
		value, wireErr := recordInputStorageValue(object.Definition, org.Namespace, fieldName, raw, "create")
		if wireErr != nil {
			return nil, wireErr
		}
		record.Fields[fieldName] = value
	}
	engine := dml.NewEngine(org)
	results := engine.Insert([]storage.Record{record})
	if len(results) != 1 || !results[0].Success {
		return nil, recordDMLWireError(object.Definition, firstDMLResult(results, "create failed"), "create", req.Fields)
	}
	stored := org.Objects[objectName].Records[results[0].ID]
	stored.ID = results[0].ID
	return recordWireMutationPayload(org, objectName, stored), nil
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
	if wireErr := recordCollisionWireError(org, record, req.IfUnmodifiedSince); wireErr != nil {
		return nil, wireErr
	}
	updates := storage.Record{
		ID:            record.ID,
		Object:        objectName,
		Fields:        map[string]storage.Value{},
		ExplicitNulls: map[string]bool{},
	}
	definition := org.Objects[objectName].Definition
	for fieldName, raw := range req.Fields {
		if strings.EqualFold(fieldName, "Id") {
			continue
		}
		// LDS ignores readonly update inputs; other DML transports still
		// enforce their own writability rules.
		if canonical, ok := storage.ResolveFieldName(definition, org.Namespace, fieldName); ok && !fieldUpdateable(definition.Fields[canonical]) {
			continue
		}
		value, wireErr := recordInputStorageValue(definition, org.Namespace, fieldName, raw, "update")
		if wireErr != nil {
			return nil, wireErr
		}
		if value.Kind == storage.ValueNull {
			updates.ExplicitNulls[fieldName] = true
			continue
		}
		updates.Fields[fieldName] = value
	}
	engine := dml.NewEngine(org)
	results := engine.Update([]storage.Record{updates})
	if len(results) != 1 || !results[0].Success {
		return nil, recordDMLWireError(definition, firstDMLResult(results, "update failed"), "update", req.Fields)
	}
	stored := org.Objects[objectName].Records[record.ID]
	stored.ID = record.ID
	return recordWireMutationPayload(org, objectName, stored), nil
}

func deleteRecordWireData(org *storage.OrgState, recordID string) (map[string]any, *lwcbrowser.WireError) {
	recordID = strings.TrimSpace(recordID)
	objectName, record, ok := findOrgRecord(org, recordID)
	if !ok {
		for _, object := range org.Objects {
			if _, deleted, found := storage.LookupRecordByID(object.Records, storage.ID(recordID)); found && deleted.System.IsDeleted {
				return nil, recordDMLWireError(object.Definition, dml.Result{StatusCode: "ENTITY_IS_DELETED", Error: "entity is deleted"}, "delete")
			}
		}
		return nil, &lwcbrowser.WireError{Message: fmt.Sprintf("record not found: %s", recordID)}
	}
	engine := dml.NewEngine(org)
	results := engine.Delete([]storage.Record{{ID: record.ID, Object: objectName}})
	if len(results) != 1 || !results[0].Success {
		return nil, recordDMLWireError(org.Objects[objectName].Definition, firstDMLResult(results, "delete failed"), "delete")
	}
	return map[string]any{"id": string(record.ID), "apiName": objectName, "deleted": true}, nil
}

func firstDMLResult(results []dml.Result, fallback string) dml.Result {
	if len(results) > 0 {
		return results[0]
	}
	return dml.Result{Error: fallback, StatusCode: "UNKNOWN_EXCEPTION"}
}

func recordWireMutationPayload(org *storage.OrgState, objectName string, record storage.Record) map[string]any {
	fields := map[string]any{}
	for name, value := range record.Fields {
		fields[name] = map[string]any{"value": recordWireValueJSON(value)}
	}
	return map[string]any{
		"id":                 string(record.ID),
		"apiName":            objectName,
		"childRelationships": recordChildRelationships(org, objectName),
		"fields":             fields,
		"lastModifiedById":   recordLastModifiedByID(record),
		"lastModifiedDate":   recordLastModifiedDate(record),
		"recordTypeId":       recordTypeIDForRecord(org, objectName, record),
		"recordTypeInfo":     nil,
		"systemModstamp":     record.System.SystemModstamp,
	}
}

// LDS mutation inputs carry unwrapped JSON values. Coercion is confined to this
// transport; Apex DML and the REST record routes retain their own input rules.
func recordInputStorageValue(definition storage.ObjectDefinition, namespace, fieldName string, raw any, operation string) (storage.Value, *lwcbrowser.WireError) {
	canonical, ok := storage.ResolveFieldName(definition, namespace, fieldName)
	if !ok {
		return storage.Value{}, recordParseWireError(operation, "field", fmt.Sprintf("Field %s does not exist.", fieldName))
	}
	field := definition.Fields[canonical]
	if raw == nil {
		return storage.NullValue(), nil
	}
	if number, ok := raw.(float64); ok {
		if field.Type == storage.FieldString && (field.DisplayType == "" || strings.EqualFold(field.DisplayType, "STRING")) {
			message := fmt.Sprintf("Value for field '%s' in object '%s' with data type 'STRING' should be a String but instead is a BigDecimal.", canonical, definition.APIName)
			return storage.Value{}, recordParseWireError(operation, "field", message)
		}
		if field.Type == storage.FieldInteger {
			return storage.DecimalValue(strconv.FormatFloat(math.Trunc(number), 'f', -1, 64)), nil
		}
	}
	if text, ok := raw.(string); ok {
		if text == "" && field.Type == storage.FieldString {
			return storage.NullValue(), nil
		}
		if field.Type == storage.FieldInteger {
			integer, err := strconv.ParseInt(text, 10, 64)
			if err != nil {
				return storage.Value{}, recordParseWireError(operation, "number", fmt.Sprintf("Unparseable number: %q", text))
			}
			return storage.IntegerValue(integer), nil
		}
		if field.Type == storage.FieldDate {
			if _, err := time.Parse("2006-01-02", text); err != nil {
				return storage.Value{}, recordParseWireError(operation, "field", "Invalid date format: "+text)
			}
		}
		if field.Type == storage.FieldDecimal {
			if number, err := strconv.ParseFloat(text, 64); err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
				return storage.Value{}, recordParseWireError(operation, "number", fmt.Sprintf("Unparseable number: %q", text))
			}
		}
		if field.Type == storage.FieldDecimal && field.Scale == 0 {
			// UI API truncates string inputs at a scale-zero numeric boundary;
			// raw JSON numbers retain their fraction. Use exact rational parsing
			// so large decimal strings do not lose digits through float64.
			if number, err := strconv.ParseFloat(text, 64); err == nil && !math.IsInf(number, 0) && !math.IsNaN(number) {
				if decimal, ok := new(big.Rat).SetString(text); ok {
					integer := new(big.Int).Quo(decimal.Num(), decimal.Denom())
					return storage.DecimalValue(integer.String()), nil
				}
			}
		}
	}
	return storageValueFromAny(raw), nil
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
				"attributes": nil,
				"label":      labelOrFallback(value.Label, value.Value),
				"value":      value.Value,
				"validFor":   []string{},
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
