package server

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type case339BlockingBody struct {
	started chan<- struct{}
	release <-chan struct{}
	once    sync.Once
}

func (body *case339BlockingBody) Read([]byte) (int, error) {
	body.once.Do(func() { body.started <- struct{}{} })
	<-body.release
	return 0, io.EOF
}
func (body *case339BlockingBody) Close() error { return nil }

type case339DeadlineTransport struct {
	*httptest.ResponseRecorder
	mu       sync.Mutex
	deadline time.Time
}

func (w *case339DeadlineTransport) SetReadDeadline(deadline time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.deadline = deadline
	return nil
}
func (w *case339DeadlineTransport) currentDeadline() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.deadline
}

type case339DeadlineBody struct {
	r                 io.Reader
	w                 *case339DeadlineTransport
	read              int64
	freshTailDeadline bool
	staged            chan struct{}
	firstRead         chan struct{}
	firstOnce         sync.Once
	once              sync.Once
}

func (b *case339DeadlineBody) Read(p []byte) (int, error) {
	if b.firstRead != nil {
		b.firstOnce.Do(func() { close(b.firstRead) })
	}
	d := b.w.currentDeadline()
	if b.read >= (32<<20)+1 {
		b.freshTailDeadline = !d.IsZero() && time.Now().Before(d)
	}
	if !d.IsZero() && time.Now().After(d) {
		return 0, os.ErrDeadlineExceeded
	}
	n, err := b.r.Read(p)
	b.read += int64(n)
	if b.read >= (32<<20)+1 {
		b.once.Do(func() { close(b.staged) })
	}
	return n, err
}
func (*case339DeadlineBody) Close() error { return nil }

func TestCase339ServerBoundsConcurrentBodies(t *testing.T) {
	org := testOrg()
	s := New(&org)
	const limit = 16
	started := make(chan struct{}, limit+1)
	release := make(chan struct{})
	var group sync.WaitGroup
	launch := func() <-chan int {
		result := make(chan int, 1)
		group.Add(1)
		go func() {
			defer group.Done()
			req := httptest.NewRequest(http.MethodPost, serverTestDataPath+"/glade/fixture", nil)
			req.Body = &case339BlockingBody{started: started, release: release}
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)
			result <- rec.Code
		}()
		return result
	}
	for i := 0; i < limit; i++ {
		launch()
	}
	for i := 0; i < limit; i++ {
		select {
		case <-started:
		case <-time.After(10 * time.Second):
			close(release)
			group.Wait()
			t.Fatal("requests did not reach body reads")
		}
	}
	extra := launch()
	var status int
	select {
	case status = <-extra:
	case <-time.After(2 * time.Second):
	}
	close(release)
	group.Wait()
	if status != http.StatusServiceUnavailable {
		t.Fatalf("extra request status = %d, want admission rejection", status)
	}
	for _, known := range []bool{false, true} {
		t.Run(fmt.Sprintf("apex_rest_get_body_lock_wait/known=%t", known), func(t *testing.T) {
			org := testOrg()
			server := New(&org)
			server.SetProjectIndex(writeApexRestProject(t, map[string]string{
				"SyntheticRest.cls": `@RestResource(urlMapping='/synthetic/*')
				global class SyntheticRest {
				  @HttpGet global static String getIt() { return RestContext.request.requestBody.toString(); }
				}`,
			}))
			server.mu.Lock()
			w := &case339DeadlineTransport{ResponseRecorder: httptest.NewRecorder(), deadline: time.Now().Add(5 * time.Second)}
			body := &case339DeadlineBody{r: strings.NewReader("synthetic payload"), w: w, staged: make(chan struct{}), firstRead: make(chan struct{})}
			req := httptest.NewRequest(http.MethodGet, "/services/apexrest/synthetic/42", nil)
			req.Body = body
			if known {
				req.ContentLength = int64(len("synthetic payload"))
			} else {
				req.ContentLength = -1
			}
			done := make(chan struct{})
			go func() { server.ServeHTTP(w, req); close(done) }()
			select {
			case <-body.firstRead:
			case <-time.After(2 * time.Second):
			}
			time.Sleep(6 * time.Second)
			server.mu.Unlock()
			select {
			case <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("GET request did not finish")
			}
			if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `"synthetic payload"` {
				t.Fatalf("GET body lost during lock wait: %d %s", w.Code, w.Body.String())
			}
		})
	}
	t.Run("large_body_waits_for_org_lock", func(t *testing.T) {
		org := testOrg()
		server := New(&org)
		server.mu.Lock()
		w := &case339DeadlineTransport{ResponseRecorder: httptest.NewRecorder()}
		payload := `{"ignored":"` + strings.Repeat("x", 32<<20) + `","compositeRequest":[]}`
		body := &case339DeadlineBody{r: strings.NewReader(payload), w: w, staged: make(chan struct{})}
		req := httptest.NewRequest(http.MethodPost, serverTestDataPath+"/composite", nil)
		req.Body = body
		req.ContentLength = -1
		done := make(chan struct{})
		go func() { server.ServeHTTP(w, req); close(done) }()
		select {
		case <-body.staged:
		case <-time.After(30 * time.Second):
			server.mu.Unlock()
			<-done
			t.Fatal("prefix was not staged")
		}
		time.Sleep(6 * time.Second)
		server.mu.Unlock()
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Fatal("request did not finish")
		}
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "compositeRequest is required") {
			t.Fatalf("ready upload expired while waiting for org lock: %d %s", w.Code, w.Body.String())
		}
		if !body.freshTailDeadline {
			t.Fatal("tail read did not receive a fresh deadline")
		}
		if d := w.currentDeadline(); !d.IsZero() {
			t.Fatalf("deadline retained after response: %v", d)
		}
	})

}
