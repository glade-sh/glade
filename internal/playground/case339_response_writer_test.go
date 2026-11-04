package playground

import (
	"net/http/httptest"
	"time"
)

// Unit handler tests use a recorder; socket tests exercise actual deadlines.
type case339ResponseWriter struct{ *httptest.ResponseRecorder }

func (case339ResponseWriter) SetReadDeadline(time.Time) error  { return nil }
func (case339ResponseWriter) SetWriteDeadline(time.Time) error { return nil }
