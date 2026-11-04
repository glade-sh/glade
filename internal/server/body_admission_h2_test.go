package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// HTTP/2 hands every request a non-nil Body, even a GET whose HEADERS frame
// ended the stream. Such a request has no body to stage and must not take a
// staging slot: browsers multiplex many module GETs on one HTTP/2 connection.
func TestBodylessHTTP2RequestDoesNotTakeStagingSlot(t *testing.T) {
	org := storage.NewOrgState()
	s := New(&org)
	s.bodySlotsOnce.Do(func() { s.bodySlots = make(chan struct{}, 16) })
	for i := 0; i < cap(s.bodySlots); i++ {
		s.bodySlots <- struct{}{}
	}
	local := httptest.NewUnstartedServer(s)
	local.EnableHTTP2 = true
	local.StartTLS()
	defer local.Close()
	response, err := local.Client().Get(local.URL + "/services/data")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.ProtoMajor != 2 {
		t.Fatalf("protocol = %s, want HTTP/2", response.Proto)
	}
	if response.StatusCode == http.StatusServiceUnavailable {
		t.Fatalf("bodyless HTTP/2 GET was refused a staging slot: HTTP %d", response.StatusCode)
	}
}
