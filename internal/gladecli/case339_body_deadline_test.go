package gladecli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestCase339ServerBoundsSlowRequestBody(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serveHTTPWithContext(ctx, addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusBadRequest)
		}))
	}()
	defer func() { cancel(); <-done }()
	var conn net.Conn
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		conn, err = net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(8 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(conn, "POST / HTTP/1.1\r\nHost: localhost\r\nContent-Length: 100\r\n\r\n{"); err != nil {
		t.Fatal(err)
	}
	_, err = bufio.NewReader(conn).ReadString('\n')
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		t.Fatal("server did not bound slow request body")
	}
}
