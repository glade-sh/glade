package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/storage"
)

func (s *Server) handleLightningShims(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodGet || len(parts) == 0 {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	switch parts[0] {
	case "core":
		s.serveLightningStaticShim(w, r, parts[1:])
	case "apex":
		s.serveApexWireShim(w, parts[1:])
	case "label":
		s.serveLabelShim(w, parts[1:])
	case "schema":
		s.serveSchemaShim(w, parts[1:])
	case "resourceUrl":
		s.serveResourceURLShim(w, parts[1:])
	case "lightning":
		s.serveLightningAPIShim(w, parts[1:])
	default:
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning shim")
	}
}

func (s *Server) serveLightningStaticShim(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) != 1 || parts[0] != "wire-adapter.js" {
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning shim")
		return
	}
	shims, err := gladehome.ShimsDir()
	if err != nil {
		writeSalesforceError(w, errUnsupportedFeature, err.Error())
		return
	}
	path := filepath.Join(shims, "wire-adapter.mjs")
	content, err := os.ReadFile(path)
	if err != nil {
		writeSalesforceError(w, errUnknownEndpoint, "wire adapter shim missing")
		return
	}
	writeJavaScript(w, content)
}

func (s *Server) serveLightningAPIShim(w http.ResponseWriter, parts []string) {
	if len(parts) != 1 || parts[0] != "uiRecordApi.js" {
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning shim")
		return
	}
	writeJavaScript(w, []byte(lwcbrowser.UIRecordAPIModuleJS()))
}

func (s *Server) serveApexWireShim(w http.ResponseWriter, parts []string) {
	token := shimToken(parts)
	if token == "" {
		writeSalesforceError(w, errUnknownEndpoint, "invalid apex shim")
		return
	}
	className, methodName, ok := lwcbrowser.ParseApexWireToken(token)
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "invalid apex shim")
		return
	}
	writeJavaScript(w, []byte(lwcbrowser.ApexWireModuleJS(className, methodName)))
}

func (s *Server) serveLabelShim(w http.ResponseWriter, parts []string) {
	token := shimToken(parts)
	if token == "" {
		writeSalesforceError(w, errUnknownEndpoint, "invalid label shim")
		return
	}
	if s.Org == nil {
		org := storage.NewOrgState()
		s.Org = &org
	}
	value, ok := lwcbrowser.ResolveLabelValue(s.Org, token)
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "unknown label")
		return
	}
	writeJavaScript(w, []byte(lwcbrowser.LabelModuleJS(value)))
}

func (s *Server) serveSchemaShim(w http.ResponseWriter, parts []string) {
	token := shimToken(parts)
	if token == "" {
		writeSalesforceError(w, errUnknownEndpoint, "invalid schema shim")
		return
	}
	objectName, fieldName, ok := lwcbrowser.ParseSchemaFieldToken(token)
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "invalid schema shim")
		return
	}
	writeJavaScript(w, []byte(lwcbrowser.SchemaFieldModuleJS(objectName, fieldName)))
}

func (s *Server) serveResourceURLShim(w http.ResponseWriter, parts []string) {
	token := shimToken(parts)
	if token == "" {
		writeSalesforceError(w, errUnknownEndpoint, "invalid resourceUrl shim")
		return
	}
	writeJavaScript(w, []byte(lwcbrowser.ResourceURLModuleJS("/resource/"+token)))
}

func shimToken(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	name := strings.TrimSpace(strings.Join(parts, "/"))
	name = strings.TrimSuffix(name, ".js")
	if name == "" || strings.Contains(name, "..") {
		return ""
	}
	return name
}

func writeJavaScript(w http.ResponseWriter, content []byte) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_, _ = w.Write(content)
}
