package playground

import (
	"errors"
	"net/http"
	"time"
)

// publicResponseWriter checks deadline support without starting the response
// budget during request processing. The deadline starts with the first response
// write and stays in force while net/http flushes after the handler returns.
func publicResponseWriter(w http.ResponseWriter) (http.ResponseWriter, bool) {
	if _, ok := w.(*deadlineResponseWriter); ok {
		return w, true
	}
	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Time{}); err != nil {
		writeError(w, http.StatusServiceUnavailable, "public response deadlines are unavailable")
		return nil, false
	}
	return &deadlineResponseWriter{ResponseWriter: w}, true
}

type deadlineResponseWriter struct {
	http.ResponseWriter
	started bool
	failed  bool
}

func (w *deadlineResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *deadlineResponseWriter) WriteHeader(status int) {
	if w.started {
		return
	}
	w.started = true
	// Some standard-library error paths clear cache headers before writing.
	w.Header().Set("Cache-Control", "no-store")
	if err := http.NewResponseController(w.ResponseWriter).SetWriteDeadline(time.Now().Add(defaultPublicWriteTimeout)); err != nil {
		w.failed = true
		writeError(w.ResponseWriter, http.StatusServiceUnavailable, "public response deadlines are unavailable")
		return
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *deadlineResponseWriter) Write(p []byte) (int, error) {
	if !w.started {
		w.WriteHeader(http.StatusOK)
	}
	if w.failed {
		return 0, errors.New("public response deadlines are unavailable")
	}
	return w.ResponseWriter.Write(p)
}
