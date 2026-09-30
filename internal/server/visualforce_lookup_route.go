package server

import (
	"html"
	"io"
	"net/http"

	"github.com/glade-sh/glade/internal/storage"
)

// handleVisualforceLookupRecord serves the bounded local destination for a
// Visualforce User lookup. It is not a Salesforce record-detail route.
func (s *Server) handleVisualforceLookupRecord(w http.ResponseWriter, r *http.Request, parts []string) {
	setDevNoStore(w)
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	if len(parts) != 2 || parts[0] != "User" {
		visualforceLookupNotFound(w)
		return
	}
	id := storage.ID(parts[1])
	if !validVisualforceLookupID(id) {
		visualforceLookupNotFound(w)
		return
	}
	user, err := s.visualforceHTMLExecutionUser()
	if err != nil {
		visualforceLookupNotFound(w)
		return
	}
	machine, err := s.visualforceRuntime()
	if err != nil {
		visualforceLookupNotFound(w)
		return
	}
	machine.SetCurrentUser(user)

	// Prove object and row access with a USER_MODE ID-only projection. Name
	// permission is separate: a denied Name must not hide an otherwise
	// readable destination or cause a raw-record fallback.
	visible, found, err := machine.ReadVisualforceRecordFields("User", id, []string{"Id"})
	if err != nil || !found || !storage.IDsEqual(visible.ID, id) {
		visualforceLookupNotFound(w)
		return
	}
	name := ""
	if named, found, err := machine.ReadVisualforceRecordFields("User", visible.ID, []string{"Name"}); err == nil && found && named.ID == visible.ID {
		if value, ok := named.GetField("Name"); ok && value.Kind == storage.ValueString {
			name = value.String
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "<!doctype html><html><head><meta charset=\"utf-8\"><title>Record</title></head><body><main><h1>Record</h1><p>Id: "+html.EscapeString(string(visible.ID))+"</p>")
	if name != "" {
		_, _ = io.WriteString(w, "<p>Name: "+html.EscapeString(name)+"</p>")
	}
	_, _ = io.WriteString(w, "</main></body></html>")
}

// ValidateID checks shape, but does not verify the case-derived suffix of an
// 18-character Salesforce ID. Check it before IDsEqual's 15-character match.
func validVisualforceLookupID(id storage.ID) bool {
	if storage.ValidateID(id) != nil {
		return false
	}
	if len(id) == 15 {
		return true
	}
	const suffixAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	for group := 0; group < 3; group++ {
		bits := 0
		for offset := 0; offset < 5; offset++ {
			character := id[group*5+offset]
			if character >= 'A' && character <= 'Z' {
				bits |= 1 << offset
			}
		}
		if id[15+group] != suffixAlphabet[bits] {
			return false
		}
	}
	return true
}

func visualforceLookupNotFound(w http.ResponseWriter) {
	http.Error(w, "Record not found", http.StatusNotFound)
}
