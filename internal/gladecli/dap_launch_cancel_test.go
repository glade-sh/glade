package gladecli

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/dap"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestDAPLaunchCancelEndsWhileProjectLoadIsRunning(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	finished := make(chan struct{})
	previous := dapStartupLoader
	dapStartupLoader = func(string) dapStartup {
		close(started)
		<-release
		close(finished)
		return dapStartup{err: errors.New("released")}
	}
	t.Cleanup(func() {
		dapStartupLoader = previous
		select {
		case <-release:
		default:
			close(release)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		handler := dap.NewHandler(dap.Snapshot{})
		result <- handleDAPLaunch(ctx, handler, dap.LaunchRequest{AnonymousBody: "System.debug(1);"}, t.TempDir(), "", false)
	}()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("launch error = %v, want context canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("launch stayed blocked in the project load after cancel")
	}
	select {
	case <-finished:
		t.Fatal("the load should still be running")
	default:
	}
	close(release)
	<-finished
}

func TestDAPTestLaunchBuildsTheProjectIndexOnce(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg"))
	project := filepath.Join(root, "project")
	writeTestFile(t, filepath.Join(project, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeTestFile(t, filepath.Join(project, "force-app/main/classes/Adder.cls"), "public class Adder { public static Integer add(Integer a, Integer b) { return a + b; } }")
	writeTestFile(t, filepath.Join(project, "force-app/main/classes/AdderTest.cls"), "@IsTest private class AdderTest { @IsTest static void adds() { System.assertEquals(3, Adder.add(1, 2)); } }")

	extraLoads := 0
	previous := dapTestIndexLoader
	dapTestIndexLoader = func(root string) (typesys.Index, error) {
		extraLoads++
		return previous(root)
	}
	t.Cleanup(func() { dapTestIndexLoader = previous })

	launch := func() {
		t.Helper()
		handler := dap.NewHandler(dap.Snapshot{})
		err := handleDAPLaunch(context.Background(), handler, dap.LaunchRequest{ClassName: "AdderTest", MethodName: "adds", AnonymousBody: "System.debug(1);"}, project, "", true)
		if err != nil {
			t.Fatal(err)
		}
	}
	launch() // startup cache miss: the index built for the cache is reused
	if extraLoads != 0 {
		t.Fatalf("cold test launch built the index %d extra times", extraLoads)
	}
	launch() // startup cache hit: no index in hand, so one is built
	if extraLoads != 1 {
		t.Fatalf("warm test launch built the index %d times, want 1", extraLoads)
	}
}
