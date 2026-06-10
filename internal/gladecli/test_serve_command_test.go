package gladecli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/testdaemon"
)

func TestTryTestServerRunAutoConnect(t *testing.T) {
	root := t.TempDir()
	writeServeTestProject(t, root)
	socket := testdaemon.ServeSocketPath(root)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := testdaemon.NewServer(testdaemon.ServerConfig{
		Root:   root,
		Socket: socket,
		Warm:   true,
		Watch:  false,
	})
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe(ctx, io.Discard) }()

	waitForServer(t, ctx, socket)

	var stdout bytes.Buffer
	result, used, err := tryTestServerRun(ctx, root, false, "WarmOneTest", "", "console", "", false, &stdout)
	if err != nil {
		t.Fatalf("client run: %v", err)
	}
	if !used {
		t.Fatal("expected auto-connect to running server")
	}
	if got := result.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v", got)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected console output")
	}
}

func writeServeTestProject(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "force-app/main/default/classes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"force-app","default":true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "force-app/main/default/classes/WarmOneTest.cls"), []byte(`
@isTest
private class WarmOneTest {
  @isTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func waitForServer(t *testing.T, ctx context.Context, socket string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := testdaemon.Ping(ctx, socket)
		if err == nil && resp.Ready {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("server not ready: %v %#v", err, resp)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
