package playground

import (
	"io"
	"net/http"
	"net/http/httptest"
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

func TestCase339PublicBoundsConcurrentRequests(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RatePerMinute: 100})
	defer s.Close()
	initial := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{initial}, httptest.NewRequest(http.MethodGet, "/playground/api/workspace", nil))
	cookies := initial.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("missing session")
	}
	const limit = 16
	started := make(chan struct{}, limit+1)
	release := make(chan struct{})
	var group sync.WaitGroup
	launch := func() <-chan int {
		result := make(chan int, 1)
		group.Add(1)
		go func() {
			defer group.Done()
			req := httptest.NewRequest(http.MethodPut, "/playground/api/files", nil)
			req.Body = &case339BlockingBody{started: started, release: release}
			req.AddCookie(cookies[0])
			rec := httptest.NewRecorder()
			s.ServeHTTP(case339ResponseWriter{rec}, req)
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
}
