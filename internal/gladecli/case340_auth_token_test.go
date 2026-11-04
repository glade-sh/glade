package gladecli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCase340OrgAuthRequestsIssuedToken(t *testing.T) {
	t.Setenv("GLADE_SERVER_CAPABILITY", "test-capability")
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	for _, tc := range []struct {
		status    int
		body      string
		wantError bool
	}{
		{http.StatusOK, `{"access_token":"issued-example"}`, false},
		{http.StatusForbidden, `{"error":"forbidden"}`, true},
		{http.StatusOK, `{}`, true},
	} {
		http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != "POST" || r.URL.Path != "/services/oauth2/token" || r.Header.Get("X-Glade-Client") != "1" || r.Header.Get("X-Glade-Capability") != "test-capability" || r.Header.Get("X-GLADE-User-Id") != "005000000000123" {
				t.Errorf("unexpected local token request: %s %s %v", r.Method, r.URL.Path, r.Header)
			}
			return &http.Response{StatusCode: tc.status, Status: http.StatusText(tc.status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.body)), Request: r}, nil
		})
		token, err := requestOrgAccessToken(context.Background(), orgConfig{InstanceURL: "http://127.0.0.1:1789", UserID: "005000000000123"})
		if (err != nil) != tc.wantError {
			t.Fatalf("status %d: error %v", tc.status, err)
		}
		if !tc.wantError && token != "issued-example" {
			t.Fatalf("token = %q", token)
		}
	}
}

func TestCase340OrgAuthPassesIssuedToken(t *testing.T) {
	root := t.TempDir()
	cfg := orgConfig{Alias: "demo", Project: root, InstanceURL: "http://127.0.0.1:1789", Addr: "127.0.0.1:1789", UserID: "005000000000123"}
	if err := writeOrgConfig(root, cfg); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "sf"), []byte("#!/bin/sh\nprintf '%s' \"$SF_ACCESS_TOKEN\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{}`
		if r.URL.Path == "/services/oauth2/token" {
			body = `{"access_token":"issued-example"}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	out, err := os.CreateTemp(root, "sf-output-")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err := runOrgAuth(context.Background(), []string{"demo", "--project", root}, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "issued-example" {
		t.Fatal("sf did not receive the issued token")
	}
}

func TestCase340OrgAuthPrintUsesIssuedToken(t *testing.T) {
	root := t.TempDir()
	if err := writeOrgConfig(root, orgConfig{Alias: "demo", Project: root, InstanceURL: "http://127.0.0.1:1789"}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runOrgAuth(context.Background(), []string{"demo", "--project", root, "--print"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `SF_ACCESS_TOKEN="$(glade org token `) {
		t.Fatalf("printed command does not obtain an issued token: %s", out.String())
	}
}

func TestCase340OrgTokenReturnsIssuedToken(t *testing.T) {
	root := t.TempDir()
	if err := writeOrgConfig(root, orgConfig{Alias: "demo", Project: root, InstanceURL: "http://127.0.0.1:1789", UserID: "005000000000123"}); err != nil {
		t.Fatal(err)
	}
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"access_token":"issued-example"}`)), Request: r}, nil
	})
	var out bytes.Buffer
	if err := runOrg(context.Background(), []string{"token", "demo", "--project", root}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if out.String() != "issued-example\n" {
		t.Fatal("token command did not return the issued credential")
	}
}
