package server

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/visualforce"
)

func TestVisualforceViewStateKeyEntropyFailureFailsClosed(t *testing.T) {
	key, err := newVisualforceViewStateSecret(bytes.NewReader(nil))
	if key != nil || !errors.Is(err, errVisualforceViewStateKey) {
		t.Fatalf("entropy failure produced key=%x error=%v", key, err)
	}
	recorder := httptest.NewRecorder()
	writeVisualforceRenderError(recorder, err, visualforce.PageRenderResult{})
	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), `"SERVER_ERROR"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "EOF") {
		t.Fatal("internal entropy diagnostic leaked into response")
	}
}

func TestVisualforceViewStateKeyIsStableAndServerScoped(t *testing.T) {
	first, second := &Server{}, &Server{}
	key, err := first.visualforceViewStateSecretBytes()
	if err != nil || len(key) != 32 {
		t.Fatalf("key length=%d error=%v", len(key), err)
	}
	again, err := first.visualforceViewStateSecretBytes()
	if err != nil || !bytes.Equal(key, again) {
		t.Fatal("server key changed between requests")
	}
	other, err := second.visualforceViewStateSecretBytes()
	if err != nil || bytes.Equal(key, other) {
		t.Fatal("independent servers share a key")
	}
}

func TestVisualforceViewStateCodecEntropyFailureReturnsServerError(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := fmt.Errorf("%w: injected diagnostic", visualforce.ErrViewStateEntropy)
	writeVisualforceRenderError(recorder, err, visualforce.PageRenderResult{})
	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), `"SERVER_ERROR"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "injected diagnostic") {
		t.Fatal("internal codec entropy diagnostic leaked into response")
	}
}

func TestVisualforceViewStateKeyConcurrentInitialization(t *testing.T) {
	server := &Server{}
	var workers sync.WaitGroup
	keys := make(chan []byte, 16)
	for i := 0; i < cap(keys); i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			key, err := server.visualforceViewStateSecretBytes()
			if err != nil {
				t.Error(err)
			}
			keys <- key
		}()
	}
	workers.Wait()
	close(keys)
	var first []byte
	for key := range keys {
		if len(key) != 32 {
			t.Fatalf("invalid concurrent key length=%d", len(key))
		}
		if first == nil {
			first = key
		} else if !bytes.Equal(first, key) {
			t.Fatal("concurrent callers received different keys")
		}
	}
}
