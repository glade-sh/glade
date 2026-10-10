package server

import (
	"net/http"

	"github.com/glade-sh/glade/internal/visualforce"
)

func (s *Server) handleVisualforcePresentationAsset(w http.ResponseWriter, r *http.Request) bool {
	data, contentType, ok := visualforce.VisualforcePresentationAsset(r.URL.Path)
	if !ok {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeMethodNotAllowed(w, http.MethodGet)
		return true
	}
	w.Header().Set("Content-Type", contentType)
	if r.Method == http.MethodGet {
		w.Write(data)
	}
	return true
}
