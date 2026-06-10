package testdaemon

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"
)

func TestServerPingAndRun(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	socket := filepath.Join(root, "serve.sock")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server, err := NewServer(ServerConfig{
		Root:   root,
		Socket: socket,
		Warm:   true,
		Watch:  false,
	})
	if err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe(ctx, io.Discard)
	}()

	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := Ping(ctx, socket)
		if err == nil && resp.Ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not become ready: %v %#v", err, resp)
		}
		time.Sleep(20 * time.Millisecond)
	}

	first, err := Run(ctx, socket, Request{Filter: "WarmOneTest"})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if got := first.Run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("first summary = %#v", got)
	}

	second, err := Run(ctx, socket, Request{Filter: "WarmTwoTest"})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if got := second.Run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("second summary = %#v", got)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve exit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}

func writeTestProject(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmOneTest.cls"), `
@isTest
private class WarmOneTest {
  @isTest static void passes() { System.assertEquals(2, 1 + 1); }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WarmTwoTest.cls"), `
@isTest
private class WarmTwoTest {
  @isTest static void passes() { System.assertEquals(3, 1 + 2); }
}
`)
}
