package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func (s *Server) handleOAuth(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) != 1 {
		writeSalesforceError(w, errUnknownEndpoint)
		return
	}
	switch parts[0] {
	case "userinfo":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, s.userInfoPayload(r))
	case "token":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		writeJSON(w, http.StatusOK, s.localTokenPayload(r))
	case "revoke", "introspect":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, localOAuthUnsupportedMessage)
	case "authorize":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, localOAuthUnsupportedMessage)
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

type apexRestRoute struct {
	ClassName string
	File      string
	Mapping   string
	Method    typesys.MemberSymbol
	PrefixLen int
	Exact     bool
}

func (s *Server) handleApexRest(w http.ResponseWriter, r *http.Request) {
	if s.Index == nil {
		writeSalesforceError(w, errUnsupportedFeature, apexRestUnsupportedMessage+"; start the server with --project to load local Apex sources")
		return
	}
	route, ok := s.apexRestRoute(r)
	if !ok {
		writeSalesforceError(w, errUnsupportedFeature, "No local @RestResource route matched "+r.URL.EscapedPath())
		return
	}
	if !route.MethodStaticNoArgs() {
		writeSalesforceError(w, errUnsupportedFeature, "Apex REST method "+route.ClassName+"."+route.Method.Name+" must be static and take no parameters in the local server")
		return
	}
	if rejectOversizeRequestBody(w, r) {
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxLocalRequestBodyBytes))
	if err != nil {
		if writeRequestBodyLimitError(w, err) {
			return
		}
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return
	}
	if s.runtimeErr != nil {
		writeSalesforceError(w, errUnsupportedFeature, "Apex REST runtime setup failed: "+s.runtimeErr.Error())
		return
	}
	machine := vm.New(nil)
	if s.runtime != nil {
		machine = s.runtime.CloneRuntime(nil)
	}
	machine.SetOrg(s.Org)
	if s.LimitMode != "" {
		machine.SetLimitMode(s.LimitMode)
	}
	if strings.TrimSpace(s.LimitProfile) != "" {
		caps, ok := vm.LimitCapsForProfile(s.LimitProfile)
		if !ok {
			writeSalesforceError(w, errMalformedQuery, "unsupported limit profile "+s.LimitProfile)
			return
		}
		machine.SetLimitCaps(caps)
	}
	if s.LimitCaps != (vm.LimitCaps{}) {
		machine.SetLimitCaps(s.LimitCaps)
	}
	machine.SetCurrentUser(s.currentUser(r, ""))
	machine.SetServerBaseURL(requestBaseURL(r))
	if s.runtime == nil {
		if err := apextest.RegisterProjectRuntimeForRequest(machine, *s.Index); err != nil {
			writeSalesforceError(w, errUnsupportedFeature, "Apex REST runtime setup failed: "+err.Error())
			return
		}
	}
	request := apexRestRequestValue(r, body, route.Mapping)
	response := vm.NewRestResponseValue()
	if err := machine.SetRestContext(request, response); err != nil {
		writeSalesforceError(w, errUnsupportedFeature, err.Error())
		return
	}
	returnValue, err := machine.CallStatic(route.ClassName+"."+route.Method.Name, nil)
	if err != nil {
		writeSalesforceError(w, errUnsupportedFeature, apexRestExecutionError(route, err))
		return
	}
	s.writeApexRestResponse(w, machine.RestResponse(), returnValue, strings.EqualFold(route.Method.Type, "void"))
}

func apexRestExecutionError(route apexRestRoute, err error) string {
	message := "Apex REST execution failed in " + route.ClassName + "." + route.Method.Name
	if loc := methodLocation(route.File, route.Method.Range.Start.Line, route.Method.Range.Start.Column); loc != "" {
		message += " (" + loc + ")"
	}
	message += ": " + err.Error()
	var runtimeErr *vm.RuntimeError
	if errors.As(err, &runtimeErr) && len(runtimeErr.Stack) > 0 {
		frame := runtimeErr.Stack[0]
		frameLabel := frame.Symbol
		if frameLabel == "" {
			frameLabel = route.ClassName + "." + route.Method.Name
		}
		if loc := methodLocation(frame.File, frame.Line, frame.Column); loc != "" {
			message += "; at " + frameLabel + " (" + loc + ")"
		} else if frameLabel != "" {
			message += "; at " + frameLabel
		}
		if stack := apexRestStackSummary(runtimeErr.Stack); stack != "" {
			message += "; VM stack: " + stack
		}
	}
	return message
}

func apexRestStackSummary(stack []vm.StackFrame) string {
	frames := make([]string, 0, len(stack))
	for _, frame := range stack {
		label := frame.Symbol
		if label == "" {
			label = "<statement>"
		}
		if loc := methodLocation(frame.File, frame.Line, frame.Column); loc != "" {
			label += " (" + loc + ")"
		}
		frames = append(frames, label)
	}
	return strings.Join(frames, " <- ")
}

func methodLocation(file string, line, column int) string {
	if file == "" && line <= 0 {
		return ""
	}
	if file == "" {
		return fmt.Sprintf("line %d:%d", line, column)
	}
	if line <= 0 {
		return file
	}
	return fmt.Sprintf("%s:%d:%d", file, line, column)
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		if forwardedScheme := strings.ToLower(strings.TrimSpace(strings.Split(forwarded, ",")[0])); forwardedScheme == "http" || forwardedScheme == "https" {
			scheme = forwardedScheme
		}
	}
	host := r.Host
	if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
		host = strings.TrimSpace(strings.Split(forwardedHost, ",")[0])
	}
	if host == "" && r.URL != nil {
		host = r.URL.Host
	}
	if scheme == "" || host == "" {
		return ""
	}
	return scheme + "://" + host
}

func (r apexRestRoute) MethodStaticNoArgs() bool {
	return hasServerModifier(r.Method.Modifiers, "static") && len(r.Method.Parameters) == 0
}

func (s *Server) apexRestRoute(r *http.Request) (apexRestRoute, bool) {
	resourcePath := apexRestResourcePath(r.URL.EscapedPath())
	verbAnnotation := apexRestVerbAnnotation(r.Method)
	var best apexRestRoute
	for _, typ := range s.Index.Types {
		if typ.Kind != apexast.DeclarationClass {
			continue
		}
		mapping, ok := restResourceURLMapping(typ.Modifiers)
		if !ok {
			continue
		}
		prefixLen, exact, ok := apexRestMappingMatch(mapping, resourcePath)
		if !ok {
			continue
		}
		method, ok := restResourceMethod(typ.Members, verbAnnotation)
		if !ok {
			continue
		}
		candidate := apexRestRoute{ClassName: typ.Name, File: typ.File, Mapping: mapping, Method: method, PrefixLen: prefixLen, Exact: exact}
		if candidate.betterThan(best) {
			best = candidate
		}
	}
	return best, best.ClassName != ""
}

func (r apexRestRoute) betterThan(other apexRestRoute) bool {
	if other.ClassName == "" {
		return true
	}
	if r.Exact != other.Exact {
		return r.Exact
	}
	if r.PrefixLen != other.PrefixLen {
		return r.PrefixLen > other.PrefixLen
	}
	return r.ClassName < other.ClassName
}

func restResourceMethod(members []typesys.MemberSymbol, annotation string) (typesys.MemberSymbol, bool) {
	for _, member := range members {
		if member.Kind == apexast.DeclarationMethod && hasServerModifier(member.Modifiers, annotation) {
			return member, true
		}
	}
	return typesys.MemberSymbol{}, false
}

func restResourceURLMapping(modifiers []string) (string, bool) {
	for _, modifier := range modifiers {
		normalized := strings.TrimSpace(modifier)
		if !strings.HasPrefix(strings.ToLower(normalized), "@restresource") {
			continue
		}
		open := strings.IndexByte(normalized, '(')
		close := strings.LastIndexByte(normalized, ')')
		if open < 0 || close <= open {
			return "", false
		}
		args := normalized[open+1 : close]
		for _, part := range strings.Split(args, ",") {
			name, value, ok := strings.Cut(part, "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(name), "urlMapping") {
				continue
			}
			mapping := strings.TrimSpace(value)
			mapping = strings.Trim(mapping, `"'`)
			if mapping == "" {
				return "", false
			}
			if !strings.HasPrefix(mapping, "/") {
				mapping = "/" + mapping
			}
			return mapping, true
		}
	}
	return "", false
}

func apexRestMappingMatch(mapping, path string) (int, bool, bool) {
	mapping = strings.TrimSpace(mapping)
	if mapping == "" {
		return 0, false, false
	}
	if !strings.HasPrefix(mapping, "/") {
		mapping = "/" + mapping
	}
	if strings.HasSuffix(mapping, "*") {
		prefix := strings.TrimSuffix(mapping, "*")
		return len(prefix), false, strings.HasPrefix(path, prefix)
	}
	return len(mapping), true, path == mapping || strings.TrimRight(path, "/") == strings.TrimRight(mapping, "/")
}

func apexRestVerbAnnotation(method string) string {
	switch method {
	case http.MethodGet:
		return "HttpGet"
	case http.MethodPost:
		return "HttpPost"
	case http.MethodPatch:
		return "HttpPatch"
	case http.MethodPut:
		return "HttpPut"
	case http.MethodDelete:
		return "HttpDelete"
	default:
		return ""
	}
}

func apexRestResourcePath(escaped string) string {
	path, err := url.PathUnescape(escaped)
	if err != nil {
		path = escaped
	}
	path = strings.TrimPrefix(path, "/services/apexrest")
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func apexRestRequestValue(r *http.Request, body []byte, routePath string) vm.Value {
	request := vm.NewRestRequestValue()
	resourcePath := apexRestResourcePath(r.URL.EscapedPath())
	requestURI := r.URL.RequestURI()
	if strings.HasSuffix(routePath, "*") {
		resourcePath = "/services/apexrest" + routePath
		requestURI = apexRestResourcePath(r.URL.EscapedPath())
		if r.URL.RawQuery != "" {
			requestURI += "?" + r.URL.RawQuery
		}
	}
	request.Fields["requestURI"] = vm.String(requestURI)
	request.Fields["resourcePath"] = vm.String(resourcePath)
	request.Fields["httpMethod"] = vm.String(r.Method)
	request.Fields["remoteAddress"] = vm.String(r.RemoteAddr)
	request.Fields["headers"] = vm.NewStringMapValue(requestHeaders(r))
	request.Fields["params"] = vm.NewStringMapValue(queryParams(r.URL.Query()))
	request.Fields["requestBody"] = vm.NewBlobValue(string(body))
	return request
}

func requestHeaders(r *http.Request) map[string]string {
	out := make(map[string]string, len(r.Header))
	for name, values := range r.Header {
		out[strings.ToLower(name)] = strings.Join(values, ",")
	}
	return out
}

func queryParams(values url.Values) map[string]string {
	out := make(map[string]string, len(values))
	for name, raw := range values {
		out[name] = strings.Join(raw, ",")
	}
	return out
}

func (s *Server) writeApexRestResponse(w http.ResponseWriter, response, returnValue vm.Value, returnVoid bool) {
	status := http.StatusOK
	if value, ok := response.Fields["statusCode"]; ok && value.Kind == vm.ValueInt && value.Int >= 100 && value.Int <= 599 {
		status = int(value.Int)
	}
	if headers, ok := response.Fields["headers"]; ok {
		for name, value := range vm.StringMapEntries(headers) {
			w.Header().Set(name, value)
		}
	}
	body := response.Fields["responseBody"]
	if body.Kind != "" && body.Kind != vm.ValueNull {
		raw, contentType := apexRestRawBody(body)
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		} else if w.Header().Get("Content-Type") == "" {
			w.Header().Del("Content-Type")
		}
		w.WriteHeader(status)
		_, _ = w.Write(raw)
		return
	}
	if returnVoid || returnValue.Kind == vm.ValueNull || returnValue.Kind == "" {
		w.WriteHeader(status)
		return
	}
	payload := s.apexRestJSONValue(returnValue)
	writeJSON(w, status, payload)
}

func apexRestRawBody(value vm.Value) ([]byte, string) {
	switch {
	case value.Kind == vm.ValueString:
		return []byte(value.Text), "text/plain"
	case value.Kind == vm.ValueObject && value.Type == "Blob":
		if raw, ok := value.Fields["value"]; ok && raw.Kind == vm.ValueString {
			return []byte(raw.Text), ""
		}
	}
	return []byte(value.String()), "text/plain"
}

func (s *Server) apexRestJSONValue(value vm.Value) any {
	switch value.Kind {
	case vm.ValueNull:
		return nil
	case vm.ValueInt:
		return value.Int
	case vm.ValueDecimal:
		return value.Decimal
	case vm.ValueBool:
		return value.Bool
	case vm.ValueString:
		return value.Text
	case vm.ValueList:
		out := make([]any, 0, len(value.List))
		for _, item := range value.List {
			out = append(out, s.apexRestJSONValue(item))
		}
		return out
	case vm.ValueSet:
		out := make([]any, 0, len(value.Set))
		for _, item := range value.Set {
			out = append(out, s.apexRestJSONValue(item))
		}
		return out
	case vm.ValueMap:
		out := map[string]any{}
		for key, item := range vm.StringValueMapEntries(value) {
			out[key] = s.apexRestJSONValue(item)
		}
		return out
	case vm.ValueObject:
		if value.Type == "Blob" {
			if raw, ok := value.Fields["value"]; ok {
				return raw.String()
			}
			return ""
		}
		out := map[string]any{}
		if _, ok := s.Org.Objects[value.Type]; ok {
			out["attributes"] = map[string]any{"type": value.Type}
		}
		for name, field := range value.Fields {
			out[name] = s.apexRestJSONValue(field)
		}
		return out
	default:
		return value.String()
	}
}
