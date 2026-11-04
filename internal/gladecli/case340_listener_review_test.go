package gladecli

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestCase340ListenerRejectsUntrustedAuthority(t *testing.T) {
	if address := os.Getenv("CASE340_LISTENER_CHILD"); address != "" {
		if err := runServer(context.Background(), []string{"--addr", address, "--no-progress"}, io.Discard, io.Discard); err != nil {
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
	cmd := exec.Command(os.Args[0], "-test.run=^TestCase340ListenerRejectsUntrustedAuthority$")
	cmd.Env = append(os.Environ(), "CASE340_LISTENER_CHILD="+address)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	transport := &http.Transport{Proxy: nil}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: time.Second}
	deadline := time.Now().Add(15 * time.Second)
	for {
		req, err := http.NewRequest(http.MethodGet, "http://"+address+"/services/data/", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("configured address: status %d", resp.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("listener did not start: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	hosts := []string{"attacker.invalid:" + addressPort(address), "127.0.0.1.evil.invalid:" + addressPort(address)}
	results := make(chan error, len(hosts))
	for _, host := range hosts {
		go func(host string) {
			req, err := http.NewRequest(http.MethodGet, "http://"+address+"/services/data/", nil)
			if err != nil {
				results <- err
				return
			}
			req.Host = host
			req.Header.Set("Sec-Fetch-Site", "same-origin")
			resp, err := client.Do(req)
			if err != nil {
				results <- err
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				results <- fmt.Errorf("Host %q: status %d, want 403", host, resp.StatusCode)
				return
			}
			results <- nil
		}(host)
	}
	for range hosts {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
}

func addressPort(address string) string {
	_, port, _ := net.SplitHostPort(address)
	return port
}
