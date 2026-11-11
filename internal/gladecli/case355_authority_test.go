package gladecli

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/testutil"
)

func TestCase355PlaygroundListenerAuthority(t *testing.T) {
	if address := os.Getenv("CASE355_LISTENER_CHILD"); address != "" {
		if err := runPlayground(context.Background(), []string{"--addr", address, "--data-root", os.Getenv("CASE355_DATA_ROOT"), "--no-db", "--no-open", "--no-progress"}, io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCase355PlaygroundListenerAuthority$")
	cmd.Env = append(os.Environ(), "CASE355_LISTENER_CHILD="+address, "CASE355_DATA_ROOT="+t.TempDir(), "GLADE_SERVER_ALLOWED_HOSTS=", "GLADE_SERVER_PROXY_ORIGINS=", "GLADE_SERVER_CAPABILITY=")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	transport := &http.Transport{Proxy: nil}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: time.Second}
	request := func(host, origin string) (int, error) {
		req, err := http.NewRequest(http.MethodGet, "http://"+address+"/healthz", nil)
		if err != nil {
			return 0, err
		}
		req.Host = host
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := client.Do(req)
		if err != nil {
			return 0, err
		}
		resp.Body.Close()
		return resp.StatusCode, nil
	}
	deadline := time.Now().Add(testutil.Deadline(20 * time.Second))
	for {
		status, err := request(address, "")
		if err == nil {
			if status != http.StatusOK {
				t.Fatalf("local host: got %d, want 200", status)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("listener did not start: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, host, origin string }{
		{"rebound host", "attacker.invalid:" + port, ""},
		{"lookalike host", "127.0.0.1.evil.invalid:" + port, ""},
		{"hostile origin", address, "http://attacker.invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, err := request(tc.host, tc.origin)
			if err != nil {
				t.Fatal(err)
			}
			if status != http.StatusForbidden {
				t.Errorf("status = %d, want 403", status)
			}
		})
	}
}
