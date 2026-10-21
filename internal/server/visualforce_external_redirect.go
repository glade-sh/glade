package server

import (
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Native URL controls open this same-origin hop in a new tab, carrying the
// exact destination in url. API59/67 observed HTTP 200 and no Location header;
// the hosted confirmation body and terminal navigation were not observed.
// This local confirmation is an explicit hosted boundary, not a replica of
// that unobserved page. Never fetch a destination on behalf of the server.
func (s *Server) handleVisualforceExternalRedirect(w http.ResponseWriter, r *http.Request) {
	setDevNoStore(w)
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	target := r.URL.Query().Get("url")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; form-action 'none'")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "<!doctype html><html><head><meta charset=\"utf-8\"><title>External destination</title></head><body><main><h1>External destination</h1><p>"+html.EscapeString(target)+"</p>")
	// Keep malformed or active-looking targets inert. The captured rows prove
	// their target text and hop, not that Salesforce navigates to such a URL.
	parsed, err := url.Parse(target)
	if err == nil && (strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")) && parsed.Hostname() != "" && parsed.User == nil && !strings.ContainsAny(target, "\r\n\t") {
		_, _ = io.WriteString(w, `<a href="`+html.EscapeString(target)+`" rel="noreferrer noopener">Continue</a>`)
	}
	_, _ = io.WriteString(w, "</main></body></html>")
}
