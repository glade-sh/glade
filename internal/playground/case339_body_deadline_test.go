package playground

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestCase339PublicSlowBodyDeadline(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewServer(ws, ServerOptions{Public: true})
	t.Cleanup(handler.Close)
	s := &http.Server{Handler: handler}
	done := make(chan struct{})
	go func() {
		_ = s.Serve(listener)
		close(done)
	}()
	defer func() { _ = s.Close(); <-done }()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(8 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprint(conn, "POST /playground/api/run HTTP/1.1\r\nHost: localhost\r\nContent-Length: 100\r\n\r\n{"); err != nil {
		t.Fatal(err)
	}
	_, err = bufio.NewReader(conn).ReadString('\n')
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		t.Fatal("public playground left the body read unbounded")
	}

	// Exercise the public handler over HTTP/2 while work preceding response
	// generation lasts longer than the write budget.
	{
		started := make(chan struct{})
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(&case339NotifyDeadlineWriter{ResponseWriter: w, started: started}, r)
		}))
		server.EnableHTTP2 = true
		server.StartTLS()
		defer server.Close()
		handler.sessions.mu.Lock()
		released := make(chan struct{})
		cancel := make(chan struct{})
		go func() {
			select {
			case <-started:
				time.Sleep(6 * time.Second)
			case <-cancel:
			}
			handler.sessions.mu.Unlock()
			close(released)
		}()
		defer func() { close(cancel); <-released }()
		response, err := server.Client().Get(server.URL + "/playground/api/workspace")
		if err != nil {
			t.Fatalf("HTTP/2 long processing lost response: %v", err)
		}
		defer response.Body.Close()
		if response.ProtoMajor != 2 || response.StatusCode != http.StatusOK {
			t.Fatalf("protocol=%s status=%d", response.Proto, response.StatusCode)
		}
		if _, err := io.ReadAll(response.Body); err != nil {
			t.Fatal(err)
		}
	}

	// A net.Pipe has no send buffer: the final HTTP/1 flush blocks until the
	// peer reads, so this observes the deadline after the handler returns.
	{
		client, serverConn := net.Pipe()
		listener := &case339PipeListener{conn: serverConn, done: make(chan struct{})}
		finished := make(chan struct{})
		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
			close(finished)
		})}
		done := make(chan struct{})
		go func() { _ = server.Serve(listener); close(done) }()
		defer func() { _ = client.Close(); _ = server.Close(); <-done }()
		if _, err := fmt.Fprint(client, "GET /playground/api/runs/latest HTTP/1.1\r\nHost: localhost\r\n\r\n"); err != nil {
			t.Fatal(err)
		}
		select {
		case <-finished:
		case <-time.After(15 * time.Second):
			t.Fatal("handler did not complete")
		}
		if err := client.SetReadDeadline(time.Now().Add(8 * time.Second)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(6 * time.Second)
		buf := make([]byte, 1)
		if n, err := client.Read(buf); n != 0 || !errors.Is(err, io.EOF) {
			t.Fatalf("buffered response escaped write deadline: n=%d err=%v", n, err)
		}
	}
}

// Signal after the real stream deadline operation, so the slow-processing
// interval begins after the deadline would have started on the old handler.
type case339NotifyDeadlineWriter struct {
	http.ResponseWriter
	started chan struct{}
	once    sync.Once
}

func (w *case339NotifyDeadlineWriter) SetWriteDeadline(deadline time.Time) error {
	err := http.NewResponseController(w.ResponseWriter).SetWriteDeadline(deadline)
	w.once.Do(func() { close(w.started) })
	return err
}
func (w *case339NotifyDeadlineWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

type case339PipeListener struct {
	conn     net.Conn
	done     chan struct{}
	once     sync.Once
	accepted bool
}

func (l *case339PipeListener) Accept() (net.Conn, error) {
	if !l.accepted {
		l.accepted = true
		return l.conn, nil
	}
	<-l.done
	return nil, net.ErrClosed
}
func (l *case339PipeListener) Close() error   { l.once.Do(func() { close(l.done) }); return nil }
func (l *case339PipeListener) Addr() net.Addr { return l.conn.LocalAddr() }

type case339UnreadBody struct{ read bool }

func (body *case339UnreadBody) Read([]byte) (int, error) {
	body.read = true
	return 0, io.EOF
}
func (body *case339UnreadBody) Close() error { return nil }

func TestCase339PublicRejectsDeclaredOversizeBeforeRead(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewServer(ws, ServerOptions{Public: true})
	defer handler.Close()
	body := &case339UnreadBody{}
	req := httptest.NewRequest(http.MethodPost, "/playground/api/run", nil)
	req.Body = body
	req.ContentLength = (1 << 20) + 1
	rec := httptest.NewRecorder()
	handler.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code != http.StatusRequestEntityTooLarge || body.read {
		t.Fatalf("status=%d bodyRead=%t, want 413 before reading", rec.Code, body.read)
	}
}
