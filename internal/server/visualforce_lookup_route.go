package server

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"strings"
	"unicode"

	"github.com/glade-sh/glade/internal/storage"
)

// handleVisualforceLookupRecord serves the bounded local destination for a
// Visualforce Account or User lookup. It is not a full record-detail route.
func (s *Server) handleVisualforceLookupRecord(w http.ResponseWriter, r *http.Request, parts []string) {
	setDevNoStore(w)
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	objectName := "User"
	var id storage.ID
	switch {
	case len(parts) == 2 && parts[0] == "User":
		id = storage.ID(parts[1])
	case len(parts) == 1:
		id = storage.ID(parts[0])
	default:
		visualforceLookupNotFound(w)
		return
	}
	if !validVisualforceLookupID(id) {
		visualforceLookupNotFound(w)
		return
	}
	if len(parts) == 1 {
		switch string(id[:3]) {
		case storage.StandardKeyPrefix("Account"):
			objectName = "Account"
		case storage.StandardKeyPrefix("User"):
			objectName = "User"
		default:
			visualforceLookupNotFound(w)
			return
		}
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
	visible, found, err := machine.ReadVisualforceRecordFields(objectName, id, []string{"Id"})
	if err != nil || !found || !storage.IDsEqual(visible.ID, id) {
		visualforceLookupNotFound(w)
		return
	}
	name := ""
	if named, found, err := machine.ReadVisualforceRecordFields(objectName, visible.ID, []string{"Name"}); err == nil && found && named.ID == visible.ID {
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

// handleVisualforceLookupSearch serves the field widget's scoped local popup.
// Only the principal's bounded USER_MODE Id/Name projection reaches the page.
func (s *Server) handleVisualforceLookupSearch(w http.ResponseWriter, r *http.Request) {
	setDevNoStore(w)
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	query := r.URL.Query()
	formID, fieldID, prefix := query.Get("lkfm"), query.Get("lknm"), query.Get("lktp")
	validComponent := func(value string) bool {
		if value == "" || len(value) > 512 {
			return false
		}
		for _, c := range value {
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' && c != ':' && c != '-' {
				return false
			}
		}
		return true
	}
	if !validComponent(formID) || !validComponent(fieldID) || !strings.HasPrefix(fieldID, formID+":") || len(prefix) != 3 {
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
	objectName := ""
	for name, object := range machine.Org.Objects {
		if object.Definition.KeyPrefix == prefix {
			if objectName != "" {
				visualforceLookupNotFound(w)
				return
			}
			objectName = name
		}
	}
	if objectName == "" {
		visualforceLookupNotFound(w)
		return
	}
	records, err := machine.ReadVisualforceRecords(objectName)
	if err != nil {
		visualforceLookupNotFound(w)
		return
	}
	search := strings.ToLower(query.Get("lksrch"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, `<!doctype html><html><head><meta charset="utf-8"><title>Lookup</title></head><body><main><h1>Lookup</h1><ul>`)
	for _, record := range records {
		value, ok := record.GetField("Name")
		if !ok || value.Kind != storage.ValueString || !strings.HasPrefix(strings.ToLower(value.String), search) || !validVisualforceLookupID(record.ID) {
			continue
		}
		arguments, _ := json.Marshal([]string{formID, fieldID, string(record.ID), value.String})
		code := "if(window.opener&&window.opener.gladeVisualforceSelectLookup.apply(window.opener," + string(arguments) + ")){window.close();}return false;"
		_, _ = io.WriteString(w, `<li><a href="/`+html.EscapeString(string(record.ID))+`" onclick="`+html.EscapeString(code)+`">`+html.EscapeString(value.String)+`</a></li>`)
	}
	_, _ = io.WriteString(w, `</ul></main></body></html>`)
}
