package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type blockingVendorWriter struct {
	header  http.Header
	started chan struct{}
	resume  chan struct{}
}

func (w *blockingVendorWriter) Header() http.Header { return w.header }
func (w *blockingVendorWriter) WriteHeader(int)     {}
func (w *blockingVendorWriter) Write(p []byte) (int, error) {
	close(w.started)
	<-w.resume
	return len(p), nil
}

func TestBlockedVendorResponseDoesNotBlockOtherReaders(t *testing.T) {
	for _, first := range []string{"engine-dom.js", "synthetic-shadow.js"} {
		t.Run(first, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "glade")
			t.Setenv("XDG_DATA_HOME", base)
			t.Setenv("GLADE_HOME", "")
			t.Setenv("GLADE_ROOT", "")
			for _, rel := range []string{"third_party/lwc/compile.mjs", "third_party/lwc/node_modules/@lwc/compiler/index.js", "lwcruntime/src/shims/wire-adapter.mjs", "lwcruntime/src/shims/lds-cache.mjs", "lwcruntime/src/shell/app.mjs", "lwcruntime/src/slds/slds-loader.mjs", "lwcruntime/src/lightning/button.mjs", "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js", "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js"} {
				p := filepath.Join(root, rel)
				if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(p, []byte("fixture"), 0644); e != nil {
					t.Fatal(e)
				}
			}
			blocked := &blockingVendorWriter{header: make(http.Header), started: make(chan struct{}), resume: make(chan struct{})}
			done := make(chan struct{})
			go func() {
				(&Server{}).handleLightningVendor(blocked, httptest.NewRequest(http.MethodGet, "/", nil), []string{first})
				close(done)
			}()
			select {
			case <-blocked.started:
			case <-time.After(5 * time.Second):
				close(blocked.resume)
				<-done
				t.Fatal("first response never started")
			}
			second := make(chan struct{})
			go func() {
				w := httptest.NewRecorder()
				(&Server{}).handleLightningVendor(w, httptest.NewRequest(http.MethodGet, "/", nil), []string{"synthetic-shadow.js"})
				close(second)
			}()
			select {
			case <-second:
			case <-time.After(time.Second):
				close(blocked.resume)
				<-done
				<-second
				t.Fatal("stalled response blocked another vendor read")
			}
			close(blocked.resume)
			<-done
		})
	}
}
