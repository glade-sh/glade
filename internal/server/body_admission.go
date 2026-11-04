package server

import (
	"io"
	"mime"
	"net/http"
	"os"
	"time"

	"github.com/glade-sh/glade/internal/visualforce"
)

// maxStagedRequestBytes bounds temporary disk usage for routes without a smaller limit.
const maxStagedRequestBytes int64 = 32 << 20

// stageRequestBody reads bodies before taking the org lock, within a disk budget.
func (s *Server) stageRequestBody(w http.ResponseWriter, r *http.Request) (func(), bool) {
	if r.Body == nil || r.Body == http.NoBody || r.Method == http.MethodHead {
		return func() {}, true
	}
	s.bodySlotsOnce.Do(func() { s.bodySlots = make(chan struct{}, 16) })
	select {
	case s.bodySlots <- struct{}{}:
	default:
		http.Error(w, "server request capacity reached", http.StatusServiceUnavailable)
		return nil, false
	}
	releaseSlot := func() { <-s.bodySlots }
	limit, rejectDeclared := requestStagingLimit(r)
	globalLimit := limit == 0
	if globalLimit {
		limit = maxStagedRequestBytes
	}
	if rejectDeclared && r.ContentLength > limit {
		// The handler checks ContentLength before reading. Keep that check and
		// its routing/error precedence, without waiting for an oversized body.
		r.Body = io.NopCloser(&emptyStagedBody{})
		return releaseSlot, true
	}
	controller := http.NewResponseController(w)
	deadlineSet := controller.SetReadDeadline(time.Now().Add(5*time.Second)) == nil
	clearDeadline := func() {
		if deadlineSet {
			_ = controller.SetReadDeadline(time.Time{})
			deadlineSet = false
		}
	}
	body, err := os.CreateTemp("", "glade-request-body-*")
	if err != nil {
		clearDeadline()
		releaseSlot()
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return nil, false
	}
	cleanup := func() { clearDeadline(); _ = body.Close(); _ = os.Remove(body.Name()); releaseSlot() }
	originalBody := r.Body
	var source io.Reader = originalBody
	if limit > 0 {
		// Keep one extra byte so MaxBytesReader in the handler can distinguish
		// a body exactly at its limit from an oversized body.
		source = io.LimitReader(source, limit+1)
	}
	n, err := io.Copy(body, source)
	// net/http owns the original body and closes it after the handler returns.
	// Closing a partially consumed server body here can itself block while it
	// drains the peer, delaying the route's response.
	if n <= limit {
		_ = r.Body.Close()
	}
	if err == nil {
		_, err = body.Seek(0, io.SeekStart)
	}
	if err != nil {
		cleanup()
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return nil, false
	}
	clearDeadline()
	if globalLimit && n > limit {
		// Renew the read budget only when the handler reaches the transport tail.
		// Waiting for the org lock and reading the staged prefix consume no budget.
		tail := &stagedTailReader{reader: originalBody, start: func() {
			deadlineSet = controller.SetReadDeadline(time.Now().Add(5*time.Second)) == nil
		}}
		r.Body = io.NopCloser(io.MultiReader(body, tail))
	} else {
		r.Body = body
	}
	return cleanup, true
}

// stagedTailReader starts the transport deadline on its first read, after the
// caller has obtained the org lock and consumed the staged prefix.
type stagedTailReader struct {
	reader io.Reader
	start  func()
}

func (r *stagedTailReader) Read(p []byte) (int, error) {
	if r.start != nil {
		r.start()
		r.start = nil
	}
	return r.reader.Read(p)
}

// emptyStagedBody keeps Body non-nil for handlers that check it before their
// declared-length rejection.
type emptyStagedBody struct{}

func (*emptyStagedBody) Read([]byte) (int, error) { return 0, io.EOF }

// requestStagingLimit mirrors limits already applied by the receiving handlers.
// Zero selects bounded prefix staging without imposing a request size limit.
func requestStagingLimit(r *http.Request) (limit int64, rejectDeclared bool) {
	parts := splitPath(r.URL.EscapedPath())
	if len(parts) >= 2 && parts[0] == "services" && parts[1] == "apexrest" {
		return maxLocalRequestBodyBytes, true
	}
	if len(parts) >= 2 && parts[0] == "apex" && r.Method == http.MethodPost {
		if _, ok := visualforceRemotingPageParts(parts[1:]); ok {
			return visualforce.MaxVisualforceRemotingRequestBytes, false
		}
		if _, ok := visualforceRemoteObjectsPageParts(parts[1:]); ok {
			return visualforce.MaxVisualforceRemotingRequestBytes, false
		}
		if visualforceIsMultipartForm(r) {
			return visualforce.MaxVisualforceUploadBytes + visualforce.MaxVisualforceHeaderBytes, false
		}
		// ParseForm applies this standard-library limit to URL-encoded posts.
		if contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); contentType == "application/x-www-form-urlencoded" {
			return 10 << 20, false
		}
	}
	if len(parts) >= 3 && parts[0] == "lightning" && r.Method == http.MethodPost {
		if parts[1] == "wire" || (parts[1] == "apex" && len(parts) == 4) {
			return 1 << 20, false
		}
	}
	if len(parts) < 4 || parts[0] != "services" || parts[1] != "data" {
		return 0, false
	}
	rest := parts[3:]
	if rest[0] == "composite" {
		// The generic decoder already rejects declared oversized bodies. Unknown
		// lengths must retain the base behavior and stream without that limit.
		if len(rest) == 1 && r.Method == http.MethodPost && r.ContentLength >= 0 {
			return maxLocalRequestBodyBytes, true
		}
		// Tree and graph use different handlers and have no such limit.
		if len(rest) > 1 && rest[1] != "tree" && rest[1] != "graph" && r.Method != http.MethodGet {
			return maxLocalRequestBodyBytes, true
		}
	}
	if len(rest) == 2 && rest[0] == "tooling" && rest[1] == "executeAnonymous" && r.Method == http.MethodPost && r.URL.Query().Get("anonymousBody") == "" {
		return maxLocalRequestBodyBytes, true
	}
	if len(rest) >= 2 && rest[0] == "glade" {
		if r.Method == http.MethodPost && (rest[1] == "reset" || (rest[1] == "fixture" && len(rest) == 2)) {
			return maxLocalRequestBodyBytes, false
		}
		if len(rest) >= 5 && rest[1] == "db-manager" && rest[2] == "objects" && rest[4] == "records" &&
			((len(rest) == 5 && r.Method == http.MethodPost) || (len(rest) == 6 && r.Method == http.MethodPatch)) {
			return maxLocalRequestBodyBytes, true
		}
	}
	return 0, false
}
