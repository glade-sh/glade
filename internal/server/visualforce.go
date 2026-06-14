package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

func (s *Server) handleVisualforcePage(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 0 {
		writeSalesforceError(w, errUnknownEndpoint, "missing Visualforce page name")
		return
	}
	if pageParts, ok := visualforceRemoteObjectsPageParts(parts); ok {
		s.handleVisualforceRemoteObjects(w, r, pageParts)
		return
	}
	if pageParts, ok := visualforceRemotingPageParts(parts); ok {
		s.handleVisualforceRemoting(w, r, pageParts)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleVisualforcePageGet(w, r, parts)
	case http.MethodPost:
		s.handleVisualforcePagePost(w, r, parts)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func visualforceRemotingPageParts(parts []string) ([]string, bool) {
	if len(parts) < 2 || !strings.EqualFold(parts[len(parts)-1], "remoting") {
		return nil, false
	}
	return parts[:len(parts)-1], true
}

func visualforceRemoteObjectsPageParts(parts []string) ([]string, bool) {
	if len(parts) < 2 || !strings.EqualFold(parts[len(parts)-1], "remoteObjects") {
		return nil, false
	}
	return parts[:len(parts)-1], true
}

func (s *Server) handleVisualforceRemoteObjects(w http.ResponseWriter, r *http.Request, pageParts []string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(visualforce.MaxVisualforceRemotingRequestBytes)))
	if err != nil {
		writeSalesforceError(w, errRequestLimitExceeded, err.Error())
		return
	}
	if err := visualforce.ValidateRemotingRequest(body); err != nil {
		writeSalesforceError(w, errRequestLimitExceeded, err.Error())
		return
	}
	var request visualforce.RemoteObjectCRUDRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&request); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return
	}
	if err := s.verifyVisualforceRequestViewState(pageParts, request.ViewState, request.CSRF); err != nil {
		writeJSON(w, http.StatusOK, visualforce.RemoteObjectCRUDResult{
			Success: false,
			Errors:  []visualforce.RemoteObjectCRUDError{{Message: err.Error(), StatusCode: "UNSUPPORTED_FEATURE"}},
		})
		return
	}
	descriptor, err := s.visualforceRemoteObjectsDescriptor(pageParts)
	if err != nil {
		writeJSON(w, http.StatusOK, visualforce.RemoteObjectCRUDResult{
			Success: false,
			Errors:  []visualforce.RemoteObjectCRUDError{{Message: err.Error(), StatusCode: "UNSUPPORTED_FEATURE"}},
		})
		return
	}
	writeJSON(w, http.StatusOK, visualforce.DispatchRemoteObjectCRUD(s.Org, descriptor, request))
}

func (s *Server) visualforceRemoteObjectsDescriptor(pageParts []string) (visualforce.RemoteObjectsDescriptor, error) {
	pageName := strings.TrimSpace(strings.Join(pageParts, "/"))
	pageFile, ok, err := lookupPageForRender(s.Source.Project, pageName)
	if err != nil {
		return visualforce.RemoteObjectsDescriptor{}, err
	}
	if !ok {
		return visualforce.RemoteObjectsDescriptor{}, errVisualforceUnknownPage
	}
	source, err := os.ReadFile(pageFile)
	if err != nil {
		return visualforce.RemoteObjectsDescriptor{}, err
	}
	tree, err := visualforce.ParseMarkupTree(string(source))
	if err != nil {
		return visualforce.RemoteObjectsDescriptor{}, err
	}
	schema := visualforce.RemoteObjectSchemaFromOrg(s.Org)
	descriptor, err := visualforce.BuildRemoteObjectsDescriptor(tree, schema)
	if err != nil && schema != nil {
		descriptor, err = visualforce.BuildRemoteObjectsDescriptor(tree, nil)
	}
	return descriptor, err
}

func (s *Server) handleVisualforceRemoting(w http.ResponseWriter, r *http.Request, pageParts []string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(visualforce.MaxVisualforceRemotingRequestBytes)))
	if err != nil {
		writeSalesforceError(w, errRequestLimitExceeded, err.Error())
		return
	}
	if err := visualforce.ValidateRemotingRequest(body); err != nil {
		writeSalesforceError(w, errRequestLimitExceeded, err.Error())
		return
	}
	requests, err := decodeVisualforceRemotingRequests(body)
	if err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return
	}
	if err := s.verifyVisualforceRemotingViewState(pageParts, requests); err != nil {
		writeJSON(w, http.StatusOK, visualforceRemotingFailures(requests, err.Error()))
		return
	}
	pageName := strings.TrimSpace(strings.Join(pageParts, "/"))
	vfIndex, err := visualforce.LoadProject(s.Source.Project)
	if err != nil {
		writeJSON(w, http.StatusOK, visualforceRemotingFailures(requests, err.Error()))
		return
	}
	page, ok := vfIndex.Page(pageName)
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "unknown Visualforce page")
		return
	}
	if s.Index == nil {
		writeJSON(w, http.StatusOK, visualforceRemotingFailures(requests, "Visualforce remoting requires a local Apex project index; start the server with --project to load local Apex sources"))
		return
	}
	metadata, err := visualforce.BuildRemotingMetadataFromIndex(page, *s.Index)
	if err != nil {
		writeJSON(w, http.StatusOK, visualforceRemotingFailures(requests, err.Error()))
		return
	}
	machine, err := s.visualforceRuntime()
	if err != nil {
		writeJSON(w, http.StatusOK, visualforceRemotingFailures(requests, err.Error()))
		return
	}
	if s.LimitMode != "" {
		machine.SetLimitMode(s.LimitMode)
	}
	if s.LimitCaps != (vm.LimitCaps{}) {
		machine.SetLimitCaps(s.LimitCaps)
	}
	machine.SetCurrentUser(s.currentUser(r, ""))
	machine.SetServerBaseURL(requestBaseURL(r))
	visualforce.SetVMRenderEnvironment(machine, s.Source.Project)
	responses := visualforce.DispatchRemotingRequests(metadata, requests, func(invocation visualforce.RemotingInvocation) (any, error) {
		args, err := visualforceRemotingVMArgs(invocation.Arguments)
		if err != nil {
			return nil, err
		}
		value, err := machine.CallStatic(invocation.Action.Action, args)
		if err != nil {
			return nil, err
		}
		return s.apexRestJSONValue(value), nil
	})
	writeJSON(w, http.StatusOK, responses)
}

func decodeVisualforceRemotingRequests(body []byte) ([]visualforce.RemotingRequest, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, errors.New("empty Visualforce remoting request")
	}
	if trimmed[0] == '[' {
		var requests []visualforce.RemotingRequest
		if err := json.Unmarshal(trimmed, &requests); err != nil {
			return nil, err
		}
		return requests, nil
	}
	var request visualforce.RemotingRequest
	if err := json.Unmarshal(trimmed, &request); err != nil {
		return nil, err
	}
	return []visualforce.RemotingRequest{request}, nil
}

func (s *Server) verifyVisualforceRemotingViewState(pageParts []string, requests []visualforce.RemotingRequest) error {
	if len(requests) == 0 {
		return nil
	}
	for _, request := range requests {
		viewState, _ := remotingCTXString(request.CTX, "viewState")
		csrf, _ := remotingCTXString(request.CTX, "csrf")
		if err := s.verifyVisualforceRequestViewState(pageParts, viewState, csrf); err != nil {
			return err
		}
	}
	return nil
}

func remotingCTXString(ctx map[string]any, key string) (string, bool) {
	if len(ctx) == 0 {
		return "", false
	}
	value, ok := ctx[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}

func visualforceRemotingFailures(requests []visualforce.RemotingRequest, message string) []visualforce.RemotingResponse {
	responses := make([]visualforce.RemotingResponse, 0, len(requests))
	for _, request := range requests {
		responses = append(responses, visualforce.RemotingResponse{
			Action:  strings.TrimSpace(request.Action),
			Method:  strings.TrimSpace(request.Method),
			Type:    request.Type,
			TID:     request.TID,
			Status:  false,
			Message: message,
			Errors:  []visualforce.RemotingError{{Message: message}},
		})
	}
	return responses
}

func visualforceRemotingVMArgs(rawArgs []json.RawMessage) ([]vm.Value, error) {
	args := make([]vm.Value, 0, len(rawArgs))
	for _, raw := range rawArgs {
		value, err := visualforceRemotingVMValue(raw)
		if err != nil {
			return nil, err
		}
		args = append(args, value)
	}
	return args, nil
}

func visualforceRemotingVMValue(raw json.RawMessage) (vm.Value, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return vm.Null, err
	}
	return visualforceRemotingDecodedVMValue(decoded)
}

func visualforceRemotingDecodedVMValue(decoded any) (vm.Value, error) {
	switch value := decoded.(type) {
	case nil:
		return vm.Null, nil
	case string:
		return vm.String(value), nil
	case bool:
		return vm.Bool(value), nil
	case json.Number:
		text := value.String()
		if !strings.ContainsAny(text, ".eE") {
			if parsed, err := strconv.ParseInt(text, 10, 64); err == nil {
				return vm.Int(parsed), nil
			}
		}
		parsed, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return vm.Null, err
		}
		out := vm.Decimal(parsed)
		out.Text = text
		return out, nil
	case []any:
		items := make([]vm.Value, 0, len(value))
		for _, item := range value {
			converted, err := visualforceRemotingDecodedVMValue(item)
			if err != nil {
				return vm.Null, err
			}
			items = append(items, converted)
		}
		return vm.List(items...), nil
	case map[string]any:
		out := vm.Map()
		out.Type = "Map<String,Object>"
		out.Static = "Map<String,Object>"
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			converted, err := visualforceRemotingDecodedVMValue(value[key])
			if err != nil {
				return vm.Null, err
			}
			mapKey := visualforceRemotingStringMapKey(key)
			out.Map[mapKey] = converted
			out.MapKeys[mapKey] = vm.String(key)
			out.MapOrder = append(out.MapOrder, mapKey)
		}
		return out, nil
	default:
		return vm.Null, fmt.Errorf("unsupported Visualforce remoting JSON value %T", decoded)
	}
}

func visualforceRemotingStringMapKey(key string) string {
	return string(vm.ValueString) + ":" + key
}

func (s *Server) handleVisualforcePageGet(w http.ResponseWriter, r *http.Request, parts []string) {
	s.renderVisualforceResponse(r.Context(), w, r.URL.RequestURI(), parts, nil, "", nil, visualforceRequestWantsPDF(r))
}

func (s *Server) handleVisualforcePagePost(w http.ResponseWriter, r *http.Request, parts []string) {
	formValues, err := s.visualforcePostFormValues(w, r, parts)
	if err != nil {
		writeSalesforceError(w, errMalformedJSON, "failed to parse Visualforce form: "+err.Error())
		return
	}
	encoded := formValues[visualforce.ViewStateFormFieldName()]
	var payload *visualforce.ViewStatePayload
	pageName := strings.TrimSpace(strings.Join(parts, "/"))
	if strings.TrimSpace(encoded) == "" {
		writeSalesforceError(w, errUnsupportedFeature, "missing Visualforce view state")
		return
	}
	decoded, err := visualforce.DecodeViewState(encoded, s.visualforceViewStateSecretBytes())
	if err != nil {
		if errors.Is(err, visualforce.ErrViewStateTampered) || errors.Is(err, visualforce.ErrViewStateInvalid) || errors.Is(err, visualforce.ErrViewStateExpired) {
			writeSalesforceError(w, errUnsupportedFeature, err.Error())
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "failed to decode view state: "+err.Error())
		return
	}
	if !strings.EqualFold(strings.TrimSpace(decoded.PageName), pageName) {
		writeSalesforceError(w, errUnsupportedFeature, "view state page mismatch")
		return
	}
	if err := visualforce.VerifyViewStateCSRF(decoded, formValues["__vf_csrf"]); err != nil {
		writeSalesforceError(w, errUnsupportedFeature, err.Error())
		return
	}
	payload = &decoded
	ajaxPayload := visualforce.ParseAjaxPayload(formValues)
	action := ajaxPayload.Action
	if ajaxPayload.IsAjax {
		s.renderVisualforceAjaxResponse(w, r.URL.RequestURI(), parts, payload, ajaxPayload)
		return
	}
	s.renderVisualforceResponse(r.Context(), w, r.URL.RequestURI(), parts, payload, action, formValues, false)
}

func (s *Server) verifyVisualforceRequestViewState(pageParts []string, encoded string, csrf string) error {
	pageName := strings.TrimSpace(strings.Join(pageParts, "/"))
	if strings.TrimSpace(encoded) == "" {
		return errors.New("missing Visualforce view state")
	}
	decoded, err := visualforce.DecodeViewState(encoded, s.visualforceViewStateSecretBytes())
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(decoded.PageName), pageName) {
		return errors.New("view state page mismatch")
	}
	return visualforce.VerifyViewStateCSRF(decoded, csrf)
}

func (s *Server) visualforcePostFormValues(w http.ResponseWriter, r *http.Request, parts []string) (map[string]string, error) {
	if visualforceIsMultipartForm(r) {
		return s.visualforceMultipartPostFormValues(w, r, parts)
	}
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	return latestFormValues(r.PostForm), nil
}

func visualforceIsMultipartForm(r *http.Request) bool {
	if r == nil {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "multipart/form-data")
}

func (s *Server) visualforceMultipartPostFormValues(w http.ResponseWriter, r *http.Request, parts []string) (map[string]string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(visualforce.MaxVisualforceUploadBytes+visualforce.MaxVisualforceHeaderBytes))
	if err := r.ParseMultipartForm(int64(visualforce.MaxVisualforceHeaderBytes)); err != nil {
		return nil, err
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	formValues := latestFormValues(r.PostForm)
	bindings, err := s.visualforceUploadBindings(parts)
	if err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return formValues, nil
	}
	assignments, err := visualforce.BindInputFileUploadAssignments(r, bindings)
	if err != nil {
		return nil, err
	}
	for key, value := range assignments {
		if strings.TrimSpace(key) == "" {
			continue
		}
		formValues[key] = visualforceUploadAssignmentString(value)
	}
	return formValues, nil
}

func latestFormValues(values map[string][]string) map[string]string {
	formValues := make(map[string]string, len(values))
	for key, items := range values {
		value, ok := latestVisualforceFormValue(key, items)
		if ok {
			formValues[key] = value
		}
	}
	return formValues
}

func latestVisualforceFormValue(key string, items []string) (string, bool) {
	if len(items) == 0 {
		return "", false
	}
	if visualforceFormKeyUsesLastValue(key, items) {
		return items[len(items)-1], true
	}
	return strings.Join(items, ";"), true
}

func visualforceFormKeyUsesLastValue(key string, items []string) bool {
	switch key {
	case visualforce.ViewStateActionFieldName(), visualforce.ViewStateFormFieldName(), "__vf_csrf", "__vf_ajax", "__vf_rerender":
		return true
	}
	return len(items) == 2 && strings.EqualFold(strings.TrimSpace(items[0]), "false") && strings.EqualFold(strings.TrimSpace(items[1]), "true")
}

func (s *Server) visualforceUploadBindings(parts []string) ([]visualforce.InputFileUploadBinding, error) {
	pageName := strings.TrimSpace(strings.Join(parts, "/"))
	pageFile, ok, err := lookupPageForRender(s.Source.Project, pageName)
	if err != nil || !ok {
		return nil, err
	}
	source, err := os.ReadFile(pageFile)
	if err != nil {
		return nil, err
	}
	return visualforce.InputFileUploadBindingsFromMarkup(string(source))
}

func visualforceUploadAssignmentString(value any) string {
	switch typed := value.(type) {
	case []byte:
		return string(typed)
	case string:
		return typed
	case int64:
		return strconv.FormatInt(typed, 10)
	case int:
		return strconv.Itoa(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func (s *Server) renderVisualforceResponse(ctx context.Context, w http.ResponseWriter, pageURL string, parts []string, viewState *visualforce.ViewStatePayload, action string, formValues map[string]string, forcePDF bool) {
	w.Header().Del("Content-Type")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	result, err := s.renderVisualforceResult(pageURL, parts, viewState, action, formValues)
	if err != nil {
		writeVisualforceRenderError(w, err, result)
		return
	}
	if result.RedirectURL != "" {
		status := http.StatusFound
		if !result.Redirect {
			status = http.StatusSeeOther
		}
		w.Header().Set("Location", result.RedirectURL)
		w.WriteHeader(status)
		return
	}
	headerOptions, err := s.visualforcePageHeaderOptions(parts)
	if err != nil {
		writeVisualforceRenderError(w, err, result)
		return
	}
	if forcePDF || strings.EqualFold(strings.TrimSpace(result.RenderAs), "pdf") {
		if err := visualforce.CheckVisualforcePDFHTMLResponseSize(len(result.HTML)); err != nil {
			writeVisualforceRenderError(w, err, result)
			return
		}
		pdf, err := visualforce.RenderPDFContent(ctx, result.HTML, "/apex/"+strings.TrimSpace(strings.Join(parts, "/")))
		if err != nil {
			writeVisualforceRenderError(w, err, result)
			return
		}
		if err := visualforce.CheckVisualforcePDFSize(len(pdf)); err != nil {
			writeVisualforceRenderError(w, err, result)
			return
		}
		applyVisualforcePageHeaders(w.Header(), headerOptions, "application/pdf")
		_, _ = w.Write(pdf)
		return
	}
	applyVisualforcePageHeaders(w.Header(), headerOptions, "text/html; charset=utf-8")
	htmlOut := injectVisualforceCSRF(result.HTML, result.ViewState, s.visualforceViewStateSecretBytes())
	if err := visualforce.CheckVisualforceResponseSize(len(htmlOut)); err != nil {
		writeVisualforceRenderError(w, err, result)
		return
	}
	fmt.Fprint(w, htmlOut)
}

func visualforceRequestWantsPDF(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("renderAs")), "pdf")
}

func (s *Server) visualforcePageHeaderOptions(parts []string) (visualforce.VisualforcePageHeaderOptions, error) {
	pageName := strings.TrimSpace(strings.Join(parts, "/"))
	pageFile, ok, err := lookupPageForRender(s.Source.Project, pageName)
	if err != nil || !ok {
		return visualforce.VisualforcePageHeaderOptions{}, err
	}
	source, err := os.ReadFile(pageFile)
	if err != nil {
		return visualforce.VisualforcePageHeaderOptions{}, err
	}
	return visualforce.VisualforcePageHeaderOptionsFromMarkup(string(source))
}

func applyVisualforcePageHeaders(header http.Header, options visualforce.VisualforcePageHeaderOptions, defaultContentType string) {
	header.Del("Content-Type")
	if strings.TrimSpace(defaultContentType) != "" {
		header.Set("Content-Type", defaultContentType)
	}
	options.Apply(header, time.Now())
}

func (s *Server) renderVisualforceAjaxResponse(w http.ResponseWriter, pageURL string, parts []string, viewState *visualforce.ViewStatePayload, payload visualforce.AjaxPayload) {
	w.Header().Del("Content-Type")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	result, err := s.renderVisualforceResult(pageURL, parts, viewState, payload.Action, payload.SubmittedFields)
	if err != nil {
		writeVisualforceRenderError(w, err, result)
		return
	}
	response := visualforce.NewPartialResponse(result.HTML, result.ViewState, payload.RerenderTargets)
	if result.RedirectURL != "" {
		response.Redirect = result.RedirectURL
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(response); err != nil {
		writeSalesforceError(w, errUnsupportedFeature, err.Error())
		return
	}
	if err := visualforce.CheckVisualforceResponseSize(body.Len()); err != nil {
		writeVisualforceRenderError(w, err, result)
		return
	}
	_, _ = w.Write(body.Bytes())
}

var errVisualforceUnknownPage = errors.New("unknown Visualforce page")

func (s *Server) renderVisualforceResult(pageURL string, parts []string, viewState *visualforce.ViewStatePayload, action string, formValues map[string]string) (visualforce.PageRenderResult, error) {
	pageName := strings.TrimSpace(strings.Join(parts, "/"))
	pageFile, ok, err := lookupPageForRender(s.Source.Project, pageName)
	if err != nil {
		return visualforce.PageRenderResult{}, err
	}
	if !ok {
		return visualforce.PageRenderResult{}, errVisualforceUnknownPage
	}
	if diag := visualforceExpressionDiagnostic(pageFile); diag != nil {
		return visualforce.PageRenderResult{HTML: visualforceRenderErrorOverlay(diag), Error: diag}, nil
	}

	machine, setupErr := s.visualforceRuntime()
	if setupErr != nil {
		return visualforce.PageRenderResult{}, setupErr
	}
	vfIndex, err := visualforce.LoadProject(s.Source.Project)
	if err != nil {
		return visualforce.PageRenderResult{}, err
	}
	visualforce.SetVMRenderEnvironment(machine, s.Source.Project)

	req := visualforce.PageRenderRequest{
		Project:         s.Source.Project,
		VFIndex:         vfIndex,
		Org:             s.Org,
		Machine:         machine,
		PageName:        pageName,
		PageURL:         strings.TrimSpace(pageURL),
		ViewState:       viewState,
		FormValues:      formValues,
		Action:          action,
		Debug:           true,
		ViewStateSecret: s.visualforceViewStateSecretBytes(),
	}
	if req.PageURL == "" {
		req.PageURL = "/apex/" + pageName
	}
	lightningUnavailable := false
	if cfg, ok, err := s.lightningBootstrapConfigLocked(); err != nil {
		lightningUnavailable = true
	} else if ok {
		req.LightningBootstrap = cfg
	}
	result, err := visualforce.RenderPage(req)
	if err != nil && result.Error != nil {
		enrichVisualforceRenderError(result.Error, pageFile)
		if result.HTML != "" {
			result.HTML = visualforceRenderErrorOverlay(result.Error)
		}
	}
	if err != nil && result.HTML == "" {
		if result.Error != nil {
			return result, result.Error
		}
		return result, err
	}
	if result.HTML == "" {
		return result, errors.New("empty Visualforce render result")
	}
	if lightningUnavailable && result.Metrics.ComponentCounts["apex:includelightning"] > 0 {
		result.HTML = injectLocalLightningUnavailableNotice(result.HTML)
	}
	return result, nil
}

func injectLocalLightningUnavailableNotice(htmlText string) string {
	if strings.Contains(htmlText, "Lightning Out is not available in local Visualforce preview") {
		return htmlText
	}
	notice := localLightningUnavailableNotice()
	if idx := strings.Index(strings.ToLower(htmlText), "<body>"); idx >= 0 {
		insertAt := idx + len("<body>")
		return htmlText[:insertAt] + notice + htmlText[insertAt:]
	}
	return notice + htmlText
}

func localLightningUnavailableNotice() string {
	return `<div class="glade-vf-lightning-notice" style="margin:1rem;padding:0.75rem 1rem;border:1px solid #c9c9c9;background:#fff8e6;font:14px/1.4 system-ui,sans-serif;">` +
		"Lightning Out is not available in local Visualforce preview. The page markup renders, but $Lightning components will not boot." +
		`</div>`
}

func writeVisualforceRenderError(w http.ResponseWriter, err error, result visualforce.PageRenderResult) {
	if errors.Is(err, errVisualforceUnknownPage) {
		writeSalesforceError(w, errUnknownEndpoint, "unknown Visualforce page")
		return
	}
	if result.Error != nil {
		writeSalesforceError(w, errUnsupportedFeature, result.Error.Error())
		return
	}
	writeSalesforceError(w, errUnsupportedFeature, err.Error())
}

func visualforceExpressionDiagnostic(pageFile string) *visualforce.RenderError {
	source, err := os.ReadFile(pageFile)
	if err != nil {
		return nil
	}
	tree, err := visualforce.ParseMarkupTree(string(source))
	if err != nil {
		return &visualforce.RenderError{Message: "parse Visualforce markup: " + err.Error(), File: pageFile}
	}
	return firstVisualforceExpressionDiagnostic(tree, pageFile)
}

func enrichVisualforceRenderError(renderErr *visualforce.RenderError, pageFile string) {
	if renderErr == nil {
		return
	}
	if strings.TrimSpace(renderErr.File) == "" {
		renderErr.File = pageFile
	}
	if strings.TrimSpace(renderErr.Expr) != "" {
		return
	}
	if diag := visualforceExpressionDiagnostic(pageFile); diag != nil {
		renderErr.Expr = diag.Expr
		if renderErr.Line == 0 {
			renderErr.Line = diag.Line
		}
		if renderErr.Column == 0 {
			renderErr.Column = diag.Column
		}
	}
}

func firstVisualforceExpressionDiagnostic(node *visualforce.MarkupNode, pageFile string) *visualforce.RenderError {
	if node == nil {
		return nil
	}
	for _, raw := range node.Attributes {
		if diag := expressionDiagnosticFromTemplate(raw, pageFile, node.Line, node.Column); diag != nil {
			return diag
		}
	}
	if node.Type == visualforce.MarkupNodeText {
		if diag := expressionDiagnosticFromTemplate(node.Text, pageFile, node.Line, node.Column); diag != nil {
			return diag
		}
	}
	for _, child := range node.Children {
		if diag := firstVisualforceExpressionDiagnostic(child, pageFile); diag != nil {
			return diag
		}
	}
	return nil
}

func expressionDiagnosticFromTemplate(raw string, pageFile string, line int, column int) *visualforce.RenderError {
	for _, expr := range visualforceExpressions(raw) {
		if msg := incompleteVisualforceExpressionMessage(expr); msg != "" {
			return &visualforce.RenderError{Message: msg, File: pageFile, Line: line, Column: column, Expr: expr}
		}
		if _, err := visualforce.EvaluateExpression(expr, &visualforce.ExpressionContext{}); err != nil {
			return &visualforce.RenderError{Message: "malformed Visualforce expression: " + err.Error(), File: pageFile, Line: line, Column: column, Expr: expr}
		}
	}
	return nil
}

func visualforceExpressions(raw string) []string {
	expressions := []string{}
	for offset := 0; offset < len(raw); {
		start := strings.Index(raw[offset:], "{!")
		if start < 0 {
			break
		}
		start += offset
		end := visualforceExpressionEnd(raw, start+2)
		if end < 0 {
			break
		}
		if expr := strings.TrimSpace(raw[start+2 : end]); expr != "" {
			expressions = append(expressions, expr)
		}
		offset = end + 1
	}
	return expressions
}

func visualforceExpressionEnd(raw string, offset int) int {
	var quote byte
	escaped := false
	for i := offset; i < len(raw); i++ {
		ch := raw[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == '}' {
			return i
		}
	}
	return -1
}

func incompleteVisualforceExpressionMessage(expr string) string {
	expr = strings.TrimSpace(expr)
	for _, op := range []string{"&&", "||", "==", "!=", ">=", "<=", "+", "-", "*", "/", ">", "<"} {
		if strings.HasSuffix(expr, op) {
			return "malformed Visualforce expression: missing right operand after " + op
		}
	}
	return ""
}

func visualforceRenderErrorOverlay(err *visualforce.RenderError) string {
	if err == nil {
		return ""
	}
	msg := html.EscapeString(err.Message)
	file := html.EscapeString(err.File)
	expr := html.EscapeString(err.Expr)
	loc := ""
	if err.Line > 0 {
		loc = fmt.Sprintf(":%d", err.Line)
		if err.Column > 0 {
			loc += fmt.Sprintf(":%d", err.Column)
		}
	}
	return `<!DOCTYPE html><html><head><title>Visualforce Error</title><style>body{font-family:system-ui;background:#1e1e1e;color:#eee;margin:0;padding:1rem}.overlay{border:1px solid #c00;background:#2a1212;padding:1rem;border-radius:4px}code{color:#ffb4b4}</style></head><body><div class="overlay"><h1>Visualforce render error</h1><p>` + msg + `</p>` +
		func() string {
			if file == "" {
				return ""
			}
			return `<p><code>` + file + html.EscapeString(loc) + `</code></p>`
		}() +
		func() string {
			if expr == "" {
				return ""
			}
			return `<p>Expression: <code>` + expr + `</code></p>`
		}() +
		`</div></body></html>`
}

func injectVisualforceCSRF(htmlText, encodedViewState string, secret []byte) string {
	if strings.TrimSpace(encodedViewState) == "" || strings.Contains(htmlText, `name="__vf_csrf"`) {
		return htmlText
	}
	payload, err := visualforce.DecodeViewState(encodedViewState, secret)
	if err != nil || strings.TrimSpace(payload.CSRF) == "" {
		return htmlText
	}
	field := `<input type="hidden" name="__vf_csrf" value="` + htmlAttrEscape(payload.CSRF) + `" />`
	if strings.Contains(htmlText, "</form>") {
		return strings.ReplaceAll(htmlText, "</form>", field+"</form>")
	}
	if strings.Contains(htmlText, "</body>") {
		return strings.Replace(htmlText, "</body>", field+"</body>", 1)
	}
	return htmlText + field
}

func (s *Server) visualforceViewStateSecretBytes() []byte {
	if len(s.visualforceViewStateSecret) > 0 {
		return s.visualforceViewStateSecret
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		secret = []byte(fmt.Sprintf("glade-local-vf-server-%p-%d", s, time.Now().UnixNano()))
	}
	s.visualforceViewStateSecret = secret
	return s.visualforceViewStateSecret
}

func htmlAttrEscape(raw string) string {
	raw = strings.ReplaceAll(raw, "&", "&amp;")
	raw = strings.ReplaceAll(raw, `"`, "&quot;")
	return raw
}

func (s *Server) visualforceRuntime() (*vm.VM, error) {
	if s.runtimeErr != nil {
		return nil, fmt.Errorf("Visualforce runtime setup failed: %w", s.runtimeErr)
	}
	machine := vm.New(nil)
	if s.runtime != nil {
		machine = s.runtime.CloneRuntime(nil)
	}
	if s.Org == nil {
		empty := storage.NewOrgState()
		s.Org = &empty
	}
	machine.SetOrg(s.Org)
	namespace := ""
	if s.Source.Project.Namespace != "" {
		namespace = s.Source.Project.Namespace
	} else if s.Index != nil {
		namespace = s.Index.Project.Namespace
	} else if s.Org != nil {
		namespace = s.Org.Namespace
	}
	if strings.TrimSpace(namespace) != "" {
		machine.SetCurrentNamespace(namespace)
	}
	if s.Index != nil && s.runtime == nil {
		if err := apextest.RegisterProjectRuntimeForRequest(machine, *s.Index); err != nil {
			return nil, fmt.Errorf("Visualforce runtime setup failed: %w", err)
		}
	}
	return machine, nil
}

func lookupPageForRender(p project.Project, name string) (string, bool, error) {
	if name == "" {
		return "", false, nil
	}
	idx, err := visualforce.LoadProject(p)
	if err != nil {
		return "", false, err
	}
	if page, ok := idx.Page(name); ok {
		return page.File, true, nil
	}
	return "", false, nil
}
