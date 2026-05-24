package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func requireWellFormedJSONBody(w http.ResponseWriter, r *http.Request) bool {
	var body any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return false
	}
	return true
}

type compositeSubrequestEnvelope struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	ReferenceID string            `json:"referenceId"`
	Body        json.RawMessage   `json:"body,omitempty"`
	HTTPHeaders map[string]string `json:"httpHeaders,omitempty"`
}

type compositeRequestEnvelope struct {
	AllOrNone        bool                          `json:"allOrNone"`
	CompositeRequest []compositeSubrequestEnvelope `json:"compositeRequest"`
}

func requireCompositeRequestEnvelope(w http.ResponseWriter, r *http.Request) bool {
	_, ok := decodeCompositeRequestEnvelope(w, r)
	return ok
}

func decodeCompositeRequestEnvelope(w http.ResponseWriter, r *http.Request) (compositeRequestEnvelope, bool) {
	var body struct {
		AllOrNone        bool                           `json:"allOrNone"`
		CompositeRequest *[]compositeSubrequestEnvelope `json:"compositeRequest"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return compositeRequestEnvelope{}, false
	}
	if body.CompositeRequest == nil || len(*body.CompositeRequest) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "compositeRequest is required and must contain at least one subrequest")
		return compositeRequestEnvelope{}, false
	}
	if !validateCompositeSubrequests(w, *body.CompositeRequest, "compositeRequest", true) {
		return compositeRequestEnvelope{}, false
	}
	return compositeRequestEnvelope{AllOrNone: body.AllOrNone, CompositeRequest: *body.CompositeRequest}, true
}

func requireCompositeBatchEnvelope(w http.ResponseWriter, r *http.Request) bool {
	var body struct {
		BatchRequests *[]compositeSubrequestEnvelope `json:"batchRequests"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return false
	}
	if body.BatchRequests == nil || len(*body.BatchRequests) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "batchRequests is required and must contain at least one subrequest")
		return false
	}
	return validateCompositeSubrequests(w, *body.BatchRequests, "batchRequests", false)
}

func requireCompositeTreeEnvelope(w http.ResponseWriter, r *http.Request) bool {
	var body struct {
		Records *[]map[string]json.RawMessage `json:"records"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return false
	}
	if body.Records == nil || len(*body.Records) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "records is required and must contain at least one tree record")
		return false
	}
	for i, record := range *body.Records {
		attrsRaw, ok := record["attributes"]
		if !ok {
			writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("records[%d].attributes.referenceId is required", i))
			return false
		}
		var attrs struct {
			ReferenceID string `json:"referenceId"`
		}
		if err := json.Unmarshal(attrsRaw, &attrs); err != nil {
			writeSalesforceError(w, errMalformedJSON, "attributes must be a JSON object")
			return false
		}
		if strings.TrimSpace(attrs.ReferenceID) == "" {
			writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("records[%d].attributes.referenceId is required", i))
			return false
		}
	}
	return true
}

func requireCompositeGraphEnvelope(w http.ResponseWriter, r *http.Request) bool {
	var body struct {
		Graphs *[]struct {
			GraphID          string                         `json:"graphId"`
			CompositeRequest *[]compositeSubrequestEnvelope `json:"compositeRequest"`
		} `json:"graphs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return false
	}
	if body.Graphs == nil || len(*body.Graphs) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "graphs is required and must contain at least one graph")
		return false
	}
	for i, graph := range *body.Graphs {
		if strings.TrimSpace(graph.GraphID) == "" {
			writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("graphs[%d].graphId is required", i))
			return false
		}
		if graph.CompositeRequest == nil || len(*graph.CompositeRequest) == 0 {
			writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("graphs[%d].compositeRequest is required and must contain at least one subrequest", i))
			return false
		}
		if !validateCompositeSubrequests(w, *graph.CompositeRequest, fmt.Sprintf("graphs[%d].compositeRequest", i), true) {
			return false
		}
	}
	return true
}

func validateCompositeSubrequests(w http.ResponseWriter, requests []compositeSubrequestEnvelope, field string, requireReferenceID bool) bool {
	for i, request := range requests {
		prefix := fmt.Sprintf("%s[%d]", field, i)
		if strings.TrimSpace(request.Method) == "" {
			writeSalesforceError(w, errRequiredFieldMissing, prefix+".method is required")
			return false
		}
		if strings.TrimSpace(request.URL) == "" {
			writeSalesforceError(w, errRequiredFieldMissing, prefix+".url is required")
			return false
		}
		if requireReferenceID && strings.TrimSpace(request.ReferenceID) == "" {
			writeSalesforceError(w, errRequiredFieldMissing, prefix+".referenceId is required")
			return false
		}
	}
	return true
}

func methodAllowed(r *http.Request, allowed ...string) bool {
	for _, method := range allowed {
		if r.Method == method {
			return true
		}
	}
	return false
}

func hasServerModifier(modifiers []string, expected string) bool {
	for _, modifier := range modifiers {
		normalized := strings.TrimPrefix(strings.TrimSpace(modifier), "@")
		if idx := strings.IndexByte(normalized, '('); idx >= 0 {
			normalized = normalized[:idx]
		}
		if strings.EqualFold(normalized, expected) {
			return true
		}
	}
	return false
}

func (s *Server) handleExecuteAnonymous(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
		return
	}
	source := r.URL.Query().Get("anonymousBody")
	if source == "" && r.Method == http.MethodPost {
		var err error
		source, err = executeAnonymousBodySource(r)
		if err != nil {
			writeJSON(w, http.StatusOK, executeAnonymousFailure(false, err.Error(), nil))
			return
		}
	}
	if source == "" {
		writeJSON(w, http.StatusOK, executeAnonymousFailure(false, "anonymousBody is required", nil))
		return
	}
	program, err := vm.CompileAnonymous(source)
	if err != nil {
		writeJSON(w, http.StatusOK, executeAnonymousFailure(false, err.Error(), nil))
		return
	}
	next := s.Org.Clone()
	machine := vm.New(nil)
	machine.SetOrg(&next)
	machine.SetCurrentUser(s.currentUser(r, ""))
	if s.LimitMode != "" {
		machine.SetLimitMode(s.LimitMode)
	}
	if s.LimitCaps != (vm.LimitCaps{}) {
		machine.SetLimitCaps(s.LimitCaps)
	}
	result, err := machine.Execute(program)
	if err != nil {
		writeJSON(w, http.StatusOK, executeAnonymousFailure(true, err.Error(), result.Debug))
		return
	}
	if err := machine.DrainAsync(&result); err != nil {
		writeJSON(w, http.StatusOK, executeAnonymousFailure(true, err.Error(), result.Debug))
		return
	}
	if err := s.commitOrg(next); err != nil {
		writeSalesforceError(w, errStoreFailure, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"compiled":            true,
		"success":             true,
		"compileProblem":      nil,
		"exceptionMessage":    nil,
		"exceptionStackTrace": nil,
		"line":                -1,
		"column":              -1,
		"logs":                strings.Join(result.Debug, "\n"),
	})
}

func executeAnonymousBodySource(r *http.Request) (string, error) {
	contentType := requestContentType(r)
	switch contentType {
	case "application/json":
		return executeAnonymousJSONSource(r)
	case "application/x-www-form-urlencoded":
		return executeAnonymousFormSource(r)
	default:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return "", err
		}
		if len(strings.TrimSpace(string(body))) == 0 {
			return "", nil
		}
		source, formErr := executeAnonymousFormEncodedSource(string(body))
		if source != "" {
			return source, nil
		}
		source, err = executeAnonymousJSONBytesSource(body)
		if err != nil {
			if formErr != nil {
				return "", formErr
			}
			return "", err
		}
		return source, nil
	}
}

func requestContentType(r *http.Request) string {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return ""
	}
	return strings.ToLower(contentType)
}

func executeAnonymousJSONSource(r *http.Request) (string, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	return executeAnonymousJSONBytesSource(body)
}

func executeAnonymousJSONBytesSource(body []byte) (string, error) {
	var payload struct {
		AnonymousBody string `json:"anonymousBody"`
		Source        string `json:"source"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.AnonymousBody != "" {
		return payload.AnonymousBody, nil
	}
	return payload.Source, nil
}

func executeAnonymousFormSource(r *http.Request) (string, error) {
	if err := r.ParseForm(); err != nil {
		return "", err
	}
	if source := r.PostForm.Get("anonymousBody"); source != "" {
		return source, nil
	}
	return r.PostForm.Get("source"), nil
}

func executeAnonymousFormEncodedSource(body string) (string, error) {
	form, err := url.ParseQuery(body)
	if err != nil {
		return "", err
	}
	if source := form.Get("anonymousBody"); source != "" {
		return source, nil
	}
	return form.Get("source"), nil
}

func executeAnonymousFailure(compiled bool, message string, logs []string) map[string]any {
	payload := map[string]any{
		"compiled":            compiled,
		"success":             false,
		"line":                1,
		"column":              1,
		"logs":                strings.Join(logs, "\n"),
		"exceptionStackTrace": nil,
	}
	if compiled {
		payload["compileProblem"] = nil
		payload["exceptionMessage"] = message
	} else {
		payload["compileProblem"] = message
		payload["exceptionMessage"] = nil
	}
	return payload
}

func (s *Server) handleComposite(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{
				"resources": map[string]string{
					"composite": "/services/data/" + version + "/composite",
					"sobjects":  "/services/data/" + version + "/composite/sobjects",
				},
				"unsupported": []string{"batch", "graph", "tree"},
			})
		case http.MethodPost:
			body, ok := decodeCompositeRequestEnvelope(w, r)
			if !ok {
				return
			}
			s.handleGenericComposite(w, r, version, body)
		default:
			writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
		return
	}
	if len(parts) >= 1 && parts[0] == "sobjects" {
		s.handleCompositeSObjects(w, r, version, parts)
		return
	}
	if len(parts) >= 1 && parts[0] == "batch" {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		if !requireCompositeBatchEnvelope(w, r) {
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Composite batch is not implemented in the local server")
		return
	}
	if len(parts) >= 1 && parts[0] == "tree" {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			writeSalesforceError(w, errRequiredFieldMissing, "object name is required for Composite tree")
			return
		}
		if !requireCompositeTreeEnvelope(w, r) {
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Composite tree is not implemented in the local server")
		return
	}
	if len(parts) >= 1 && parts[0] == "graph" {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		if !requireCompositeGraphEnvelope(w, r) {
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Composite graph is not implemented in the local server")
		return
	}
	writeSalesforceError(w, errUnknownComposite)
}

type compositeSubresponse struct {
	Body           any               `json:"body"`
	HTTPHeaders    map[string]string `json:"httpHeaders,omitempty"`
	HTTPStatusCode int               `json:"httpStatusCode"`
	ReferenceID    string            `json:"referenceId"`
}

func (s *Server) handleGenericComposite(w http.ResponseWriter, r *http.Request, version string, body compositeRequestEnvelope) {
	next := s.Org.Clone()
	child := *s
	child.Org = &next
	child.Store = nil
	references := make(map[string]any, len(body.CompositeRequest))
	responses := make([]compositeSubresponse, 0, len(body.CompositeRequest))
	hasFailure := false
	hasMutation := false
	for _, subrequest := range body.CompositeRequest {
		resolved, err := resolveCompositeSubrequest(subrequest, references)
		if err != nil {
			hasFailure = true
			responses = append(responses, compositeReferenceErrorResponse(subrequest.ReferenceID, err))
			continue
		}
		if isCompositeMutationMethod(resolved.Method) {
			hasMutation = true
		}
		response := child.executeCompositeSubrequest(r, version, resolved)
		if response.HTTPStatusCode >= 400 {
			hasFailure = true
		}
		references[resolved.ReferenceID] = response.Body
		responses = append(responses, response)
	}
	if !body.AllOrNone || !hasFailure {
		s.queryLocators = child.queryLocators
		s.queryOrder = child.queryOrder
		s.nextQueryID = child.nextQueryID
	}
	if hasMutation && (!body.AllOrNone || !hasFailure) {
		if err := s.commitOrg(next); err != nil {
			writeSalesforceError(w, errStoreFailure, err.Error())
			return
		}
	}
	status := http.StatusOK
	if body.AllOrNone && hasFailure {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]any{"compositeResponse": responses})
}

func (s *Server) executeCompositeSubrequest(parent *http.Request, version string, subrequest compositeSubrequestEnvelope) compositeSubresponse {
	var body io.Reader
	if len(subrequest.Body) > 0 {
		body = bytes.NewReader(subrequest.Body)
	}
	req, err := newCompositeHTTPRequest(parent, strings.ToUpper(subrequest.Method), subrequest.URL, body)
	if err != nil {
		return compositeReferenceErrorResponse(subrequest.ReferenceID, err)
	}
	req.Header.Set("Content-Type", "application/json")
	for name, value := range parent.Header {
		for _, item := range value {
			req.Header.Add(name, item)
		}
	}
	for name, value := range subrequest.HTTPHeaders {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	if !compositeSubrequestRouteSupported(req.URL, version) {
		writeSalesforceError(rec, errUnsupportedFeature, "Composite subrequest route is not supported by the local generic Composite orchestrator")
	} else {
		s.serveHTTPLocked(rec, req)
	}
	result := rec.Result()
	defer result.Body.Close()
	rawBody, _ := io.ReadAll(result.Body)
	decodedBody := decodeCompositeSubresponseBody(rawBody)
	return compositeSubresponse{
		Body:           decodedBody,
		HTTPHeaders:    compositeResponseHeaders(result.Header),
		HTTPStatusCode: result.StatusCode,
		ReferenceID:    subrequest.ReferenceID,
	}
}

func newCompositeHTTPRequest(parent *http.Request, method, target string, body io.Reader) (*http.Request, error) {
	if strings.TrimSpace(method) == "" {
		return nil, fmt.Errorf("Composite subrequest method is required")
	}
	if strings.TrimSpace(target) == "" {
		return nil, fmt.Errorf("Composite subrequest url is required")
	}
	requestURL := target
	if strings.HasPrefix(target, "/") {
		requestURL = "http://local.glade.test" + target
	}
	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, err
	}
	req.RequestURI = ""
	req.Host = parent.Host
	req.RemoteAddr = parent.RemoteAddr
	return req, nil
}

func compositeSubrequestRouteSupported(u *url.URL, version string) bool {
	parts := splitPath(u.EscapedPath())
	if len(parts) < 4 || parts[0] != "services" || parts[1] != "data" || parts[2] != version {
		return false
	}
	rest := parts[3:]
	switch {
	case len(rest) >= 1 && rest[0] == "sobjects":
		return true
	case len(rest) >= 1 && (rest[0] == "query" || rest[0] == "queryAll"):
		return true
	case len(rest) == 1 && rest[0] == "limits":
		return true
	case len(rest) >= 2 && rest[0] == "composite" && rest[1] == "sobjects":
		return true
	default:
		return false
	}
}

func resolveCompositeSubrequest(request compositeSubrequestEnvelope, references map[string]any) (compositeSubrequestEnvelope, error) {
	url, err := substituteCompositeReferences(request.URL, references)
	if err != nil {
		return compositeSubrequestEnvelope{}, err
	}
	request.URL = url
	if len(request.Body) > 0 {
		body, err := substituteCompositeReferences(string(request.Body), references)
		if err != nil {
			return compositeSubrequestEnvelope{}, err
		}
		request.Body = json.RawMessage(body)
	}
	return request, nil
}

func substituteCompositeReferences(input string, references map[string]any) (string, error) {
	var out strings.Builder
	for {
		start := strings.Index(input, "@{")
		if start < 0 {
			out.WriteString(input)
			return out.String(), nil
		}
		end := strings.Index(input[start+2:], "}")
		if end < 0 {
			return "", fmt.Errorf("unclosed Composite reference starting with %q", input[start:])
		}
		end += start + 2
		out.WriteString(input[:start])
		expr := input[start+2 : end]
		ref, field, ok := strings.Cut(expr, ".")
		if !ok || strings.TrimSpace(ref) == "" || strings.TrimSpace(field) == "" {
			return "", fmt.Errorf("invalid Composite reference %q", expr)
		}
		value, ok := compositeReferenceValue(references[strings.TrimSpace(ref)], strings.Split(strings.TrimSpace(field), "."))
		if !ok {
			return "", fmt.Errorf("Composite reference %s could not be resolved", expr)
		}
		out.WriteString(fmt.Sprint(value))
		input = input[end+1:]
	}
}

func compositeReferenceValue(body any, fields []string) (any, bool) {
	current := body
	for _, field := range fields {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := object[field]
		if !ok {
			for key, value := range object {
				if strings.EqualFold(key, field) {
					next = value
					ok = true
					break
				}
			}
		}
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func decodeCompositeSubresponseBody(raw []byte) any {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var body any
	if err := json.Unmarshal(raw, &body); err == nil {
		return body
	}
	return string(raw)
}

func compositeResponseHeaders(headers http.Header) map[string]string {
	out := make(map[string]string)
	for name, values := range headers {
		if strings.EqualFold(name, "Content-Type") || len(values) == 0 {
			continue
		}
		out[name] = strings.Join(values, ",")
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func compositeReferenceErrorResponse(referenceID string, err error) compositeSubresponse {
	return compositeSubresponse{
		Body: []salesforceError{{
			ErrorCode: salesforceErrorCode(errInvalidField),
			Message:   err.Error(),
		}},
		HTTPStatusCode: http.StatusBadRequest,
		ReferenceID:    referenceID,
	}
}

func isCompositeMutationMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func (s *Server) handleCompositeSObjects(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodPost:
			s.handleCompositeSObjectInsert(w, r)
			return
		case http.MethodPatch:
			s.handleCompositeSObjectUpdate(w, r)
			return
		case http.MethodDelete:
			s.handleCompositeSObjectDelete(w, r)
			return
		default:
			writeMethodNotAllowed(w, http.MethodPost, http.MethodPatch, http.MethodDelete)
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		if len(parts) == 2 {
			s.handleCompositeSObjectTypedRetrieve(w, r, version, parts[1])
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Composite sObject typed retrieve routes beyond object collections are not implemented in the local server")
		return
	case http.MethodPost:
		writeSalesforceError(w, errUnsupportedFeature, "Composite sObject typed collection routes are not implemented in the local server")
		return
	case http.MethodPatch:
		if len(parts) == 3 {
			s.handleCompositeSObjectTypedUpsert(w, r, parts[1], parts[2])
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Composite sObject collection upsert routes are not implemented in the local server")
		return
	case http.MethodDelete:
		writeSalesforceError(w, errUnsupportedFeature, "Composite sObject collection delete routes are not implemented in the local server")
		return
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete)
		return
	}
}

func (s *Server) handleCompositeSObjectInsert(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeCompositeSObjectBody(w, r, false)
	if !ok {
		return
	}
	next := s.Org.Clone()
	engine := s.newDMLEngine(r, &next)
	results := engine.Insert(body.Records)
	s.writeCompositeMutationResults(w, next, body.AllOrNone, results, body.ReferenceIDs)
}

func (s *Server) handleCompositeSObjectUpdate(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeCompositeSObjectBody(w, r, true)
	if !ok {
		return
	}
	next := s.Org.Clone()
	engine := s.newDMLEngine(r, &next)
	results := engine.Update(body.Records)
	s.writeCompositeMutationResults(w, next, body.AllOrNone, results, body.ReferenceIDs)
}

func (s *Server) handleCompositeSObjectDelete(w http.ResponseWriter, r *http.Request) {
	idsParam := strings.TrimSpace(r.URL.Query().Get("ids"))
	if idsParam == "" {
		writeSalesforceError(w, errRequiredFieldMissing, "ids query parameter is required for local Composite sObject collection delete")
		return
	}
	allOrNone := strings.EqualFold(r.URL.Query().Get("allOrNone"), "true")
	ids := strings.Split(idsParam, ",")
	records := make([]storage.Record, 0, len(ids))
	for _, rawID := range ids {
		id := storage.ID(strings.TrimSpace(rawID))
		if id == "" {
			writeSalesforceError(w, errMalformedID, "ids query parameter contains an empty record id")
			return
		}
		objectName := s.objectNameForRecordID(id)
		records = append(records, storage.Record{Object: objectName, ID: id})
	}
	next := s.Org.Clone()
	engine := s.newDMLEngine(r, &next)
	results := engine.Delete(records)
	s.writeCompositeMutationResults(w, next, allOrNone, results, nil)
}

func (s *Server) writeCompositeMutationResults(w http.ResponseWriter, next storage.OrgState, allOrNone bool, results []dml.Result, referenceIDs []string) {
	hasFailure := false
	hasSuccess := false
	for _, result := range results {
		if !result.Success {
			hasFailure = true
		} else {
			hasSuccess = true
		}
	}
	if allOrNone && hasFailure {
		writeJSON(w, http.StatusBadRequest, compositeResults(compositeAllOrNoneRollbackResults(results), referenceIDs))
		return
	}
	if hasSuccess {
		if err := s.commitOrg(next); err != nil {
			writeSalesforceError(w, errStoreFailure, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, compositeResults(results, referenceIDs))
}

func (s *Server) handleCompositeSObjectTypedRetrieve(w http.ResponseWriter, r *http.Request, version string, objectName string) {
	resolvedObjectName, ok := storage.ResolveObjectName(*s.Org, objectName)
	if !ok {
		writeSalesforceError(w, errUnknownObject, "unknown object "+objectName)
		return
	}
	object := s.Org.Objects[resolvedObjectName]
	idsParam := strings.TrimSpace(r.URL.Query().Get("ids"))
	if idsParam == "" {
		writeSalesforceError(w, errRequiredFieldMissing, "ids query parameter is required for local Composite sObject collection retrieve")
		return
	}
	ids := strings.Split(idsParam, ",")
	records := make([]map[string]any, 0, len(ids))
	fields, hasProjection, ok := compositeRetrieveFields(w, object.Definition, s.Org.Namespace, r.URL.Query().Get("fields"))
	if !ok {
		return
	}
	for _, rawID := range ids {
		id := storage.ID(strings.TrimSpace(rawID))
		if id == "" {
			writeSalesforceError(w, errMalformedID, "ids query parameter contains an empty record id")
			return
		}
		record, ok := object.Records[id]
		if !ok || record.System.IsDeleted {
			writeSalesforceError(w, errUnknownRecord, "record not found: "+string(id))
			return
		}
		if hasProjection {
			records = append(records, projectedRecordPayload(record, version, resolvedObjectName, id, fields))
			continue
		}
		records = append(records, recordPayload(record, version, resolvedObjectName, id))
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": records})
}

func (s *Server) handleCompositeSObjectTypedUpsert(w http.ResponseWriter, r *http.Request, objectName string, externalIDField string) {
	objectName, ok := storage.ResolveObjectName(*s.Org, objectName)
	if !ok {
		writeSalesforceError(w, errUnknownObject, "unknown object "+objectName)
		return
	}
	object := s.Org.Objects[objectName]
	fieldName := externalIDField
	if canonical, ok := storage.ResolveFieldName(object.Definition, s.Org.Namespace, externalIDField); ok {
		fieldName = canonical
	}
	field, ok := object.Definition.Fields[fieldName]
	if !ok {
		writeSalesforceError(w, errInvalidField, "unknown external id field "+objectName+"."+externalIDField)
		return
	}
	if !field.ExternalID {
		writeSalesforceError(w, errInvalidField, "field "+objectName+"."+fieldName+" is not an external id")
		return
	}

	var body struct {
		AllOrNone bool                          `json:"allOrNone"`
		Records   *[]map[string]json.RawMessage `json:"records"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return
	}
	if body.Records == nil || len(*body.Records) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "records is required and must contain at least one row")
		return
	}
	records := make([]storage.Record, 0, len(*body.Records))
	referenceIDs := make([]string, 0, len(*body.Records))
	for i, raw := range *body.Records {
		referenceID := ""
		if attrsRaw, ok := raw["attributes"]; ok {
			attrs, ok := decodeCompositeSObjectAttributes(w, attrsRaw, i, false)
			if !ok {
				return
			}
			referenceID = attrs.ReferenceID
			delete(raw, "attributes")
		}
		if topLevelReferenceID, ok := decodeCompositeTopLevelReferenceID(w, raw, i); ok {
			if topLevelReferenceID != "" {
				referenceID = topLevelReferenceID
			}
			delete(raw, "referenceId")
		} else {
			return
		}
		record, err := recordFromRawFields(objectName, "", raw)
		if err != nil {
			writeSalesforceError(w, errMalformedJSON, err.Error())
			return
		}
		records = append(records, record)
		referenceIDs = append(referenceIDs, referenceID)
	}

	next := s.Org.Clone()
	engine := s.newDMLEngine(r, &next)
	results := engine.UpsertWithExternalID(records, fieldName)
	hasFailure := false
	hasSuccess := false
	for _, result := range results {
		if result.Success {
			hasSuccess = true
		} else {
			hasFailure = true
		}
	}
	if body.AllOrNone && hasFailure {
		writeJSON(w, http.StatusBadRequest, compositeUpsertResults(compositeAllOrNoneRollbackResults(results), referenceIDs))
		return
	}
	if hasSuccess {
		if err := s.commitOrg(next); err != nil {
			writeSalesforceError(w, errStoreFailure, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, compositeUpsertResults(results, referenceIDs))
}

type compositeSObjectBody struct {
	AllOrNone    bool
	Records      []storage.Record
	ReferenceIDs []string
}

func decodeCompositeSObjectBody(w http.ResponseWriter, r *http.Request, requireID bool) (compositeSObjectBody, bool) {
	var rawBody struct {
		AllOrNone bool                          `json:"allOrNone"`
		Records   *[]map[string]json.RawMessage `json:"records"`
	}
	if err := json.NewDecoder(r.Body).Decode(&rawBody); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return compositeSObjectBody{}, false
	}
	if rawBody.Records == nil || len(*rawBody.Records) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "records is required and must contain at least one row")
		return compositeSObjectBody{}, false
	}
	body := compositeSObjectBody{AllOrNone: rawBody.AllOrNone, Records: make([]storage.Record, 0, len(*rawBody.Records)), ReferenceIDs: make([]string, 0, len(*rawBody.Records))}
	for i, raw := range *rawBody.Records {
		objectName := ""
		referenceID := ""
		if attrsRaw, ok := raw["attributes"]; ok {
			attrs, ok := decodeCompositeSObjectAttributes(w, attrsRaw, i, true)
			if !ok {
				return compositeSObjectBody{}, false
			}
			objectName = attrs.Type
			referenceID = attrs.ReferenceID
			delete(raw, "attributes")
		}
		if topLevelReferenceID, ok := decodeCompositeTopLevelReferenceID(w, raw, i); ok {
			if topLevelReferenceID != "" {
				referenceID = topLevelReferenceID
			}
			delete(raw, "referenceId")
		} else {
			return compositeSObjectBody{}, false
		}
		if objectName == "" {
			writeSalesforceError(w, errRequiredFieldMissing, "attributes.type is required")
			return compositeSObjectBody{}, false
		}
		var id storage.ID
		if requireID {
			var err error
			id, err = idFromRawRecord(raw)
			if err != nil {
				writeSalesforceError(w, errMalformedJSON, err.Error())
				return compositeSObjectBody{}, false
			}
			if id == "" {
				writeSalesforceError(w, errRequiredFieldMissing, "Id is required")
				return compositeSObjectBody{}, false
			}
		}
		record, err := recordFromRawFields(objectName, id, raw)
		if err != nil {
			writeSalesforceError(w, errMalformedJSON, err.Error())
			return compositeSObjectBody{}, false
		}
		body.Records = append(body.Records, record)
		body.ReferenceIDs = append(body.ReferenceIDs, referenceID)
	}
	return body, true
}

func decodeCompositeTopLevelReferenceID(w http.ResponseWriter, raw map[string]json.RawMessage, recordIndex int) (string, bool) {
	rawReferenceID, ok := raw["referenceId"]
	if !ok {
		return "", true
	}
	var referenceID string
	if err := json.Unmarshal(rawReferenceID, &referenceID); err != nil {
		writeSalesforceError(w, errMalformedJSON, fmt.Sprintf("records[%d].referenceId must be a string", recordIndex))
		return "", false
	}
	referenceID = strings.TrimSpace(referenceID)
	if referenceID == "" {
		writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("records[%d].referenceId is required when provided", recordIndex))
		return "", false
	}
	return referenceID, true
}

type compositeSObjectAttributes struct {
	Type        string
	ReferenceID string
}

func decodeCompositeSObjectAttributes(w http.ResponseWriter, raw json.RawMessage, recordIndex int, requireType bool) (compositeSObjectAttributes, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		writeSalesforceError(w, errMalformedJSON, fmt.Sprintf("records[%d].attributes must be a JSON object", recordIndex))
		return compositeSObjectAttributes{}, false
	}
	var attrs compositeSObjectAttributes
	if rawType, ok := fields["type"]; ok {
		if err := json.Unmarshal(rawType, &attrs.Type); err != nil {
			writeSalesforceError(w, errMalformedJSON, fmt.Sprintf("records[%d].attributes.type must be a string", recordIndex))
			return compositeSObjectAttributes{}, false
		}
		attrs.Type = strings.TrimSpace(attrs.Type)
	}
	if requireType && attrs.Type == "" {
		writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("records[%d].attributes.type is required", recordIndex))
		return compositeSObjectAttributes{}, false
	}
	if rawReferenceID, ok := fields["referenceId"]; ok {
		if err := json.Unmarshal(rawReferenceID, &attrs.ReferenceID); err != nil {
			writeSalesforceError(w, errMalformedJSON, fmt.Sprintf("records[%d].attributes.referenceId must be a string", recordIndex))
			return compositeSObjectAttributes{}, false
		}
		attrs.ReferenceID = strings.TrimSpace(attrs.ReferenceID)
		if attrs.ReferenceID == "" {
			writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("records[%d].attributes.referenceId is required when provided", recordIndex))
			return compositeSObjectAttributes{}, false
		}
	}
	return attrs, true
}

func idFromRawRecord(raw map[string]json.RawMessage) (storage.ID, error) {
	for _, name := range []string{"Id", "id"} {
		rawID, ok := raw[name]
		if !ok {
			continue
		}
		var id string
		if err := json.Unmarshal(rawID, &id); err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		delete(raw, "Id")
		delete(raw, "id")
		return storage.ID(id), nil
	}
	return "", nil
}
