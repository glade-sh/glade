package gladecli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
	"github.com/glade-sh/glade/internal/watch"
)

func runDevVF(ctx context.Context, args []string, w io.Writer) error {
	addr := "127.0.0.1:8080"
	root := "."
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			if i+1 >= len(args) {
				return errors.New("--port requires a value")
			}
			addr = "127.0.0.1:" + args[i+1]
			i++
		case "--addr":
			if i+1 >= len(args) {
				return errors.New("--addr requires a value")
			}
			addr = args[i+1]
			i++
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "help", "-h", "--help":
			printDevVFHelp(w)
			return nil
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	p, err := project.Load(root)
	if err != nil {
		return err
	}
	index, err := loadIndex(root)
	if err != nil {
		return err
	}
	org := apextest.OrgFromIndex(index)
	source, err := server.NewSourceMetadataFromProject(p)
	if err != nil {
		return err
	}
	handler := server.NewWithSource(&org, source)
	installDevVFRuntime(handler, index)
	if root, err := gladehome.EnsureRoot(); err != nil {
		fmt.Fprintf(w, "warning: LWC toolchain unavailable: %v\n", err)
		fmt.Fprintf(w, "warning: Lightning Out pages will show a placeholder until you run `glade toolchain install`\n")
	} else {
		fmt.Fprintf(w, "LWC toolchain: %s\n", root)
	}
	httpServer := &http.Server{Addr: addr, Handler: handler}
	fmt.Fprintf(w, "Visualforce dev server: http://%s\n", addr)
	fmt.Fprintf(w, "Watching %s for .page, .component, and Apex changes.\n", p.Root)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	go runDevVFReload(ctx, root, handler, w)
	return httpServer.ListenAndServe()
}

func runDevVFReload(ctx context.Context, root string, srv *server.Server, w io.Writer) {
	cfg := watch.Config{Root: root}
	previous, err := watch.CaptureSnapshot(root)
	if err != nil {
		fmt.Fprintf(w, "watch setup failed: %v\n", err)
		return
	}
	watcher, _, err := watch.NewBackendWatcher(ctx, cfg, previous)
	if err != nil {
		fmt.Fprintf(w, "watch setup failed: %v\n", err)
		return
	}
	defer watcher.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-watcher.Errors():
			if err != nil {
				fmt.Fprintf(w, "watch error: %v\n", err)
			}
		case changes, ok := <-watcher.Changes():
			if !ok {
				return
			}
			relevant := false
			for _, change := range changes {
				path := strings.ToLower(filepath.ToSlash(change.Path))
				if strings.HasSuffix(path, ".page") || strings.HasSuffix(path, ".component") || strings.HasSuffix(path, ".cls") ||
					strings.Contains(path, "/lwc/") || strings.HasSuffix(path, ".app") {
					relevant = true
					break
				}
			}
			if !relevant {
				continue
			}
			p, err := project.Load(root)
			if err != nil {
				fmt.Fprintf(w, "reload failed: %v\n", err)
				continue
			}
			source, err := server.NewSourceMetadataFromProject(p)
			if err != nil {
				fmt.Fprintf(w, "reload failed: %v\n", err)
				continue
			}
			index, err := loadIndex(root)
			if err != nil {
				fmt.Fprintf(w, "reload failed: %v\n", err)
				continue
			}
			if srv != nil {
				srv.Source = source
				srv.ResetLightningCache()
				installDevVFRuntime(srv, index)
			}
			fmt.Fprintf(w, "Reloaded Visualforce project metadata (%d change(s)).\n", len(changes))
		}
	}
}

func printDevVFHelp(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`
Start a local Visualforce development server with hot reload.

Usage:
  glade dev vf [--project <root>] [--port <port>|--addr <host:port>]

Examples:
  glade dev vf --project .
  glade dev vf --port 8080
`)+"\n")
}

func installDevVFRuntime(srv *server.Server, index typesys.Index) {
	if srv == nil {
		return
	}
	machine := vm.New(nil)
	machine.SetCurrentNamespace(index.Project.Namespace)
	runtimeErr := apextest.RegisterProjectRuntimeForRequest(machine, index)
	srv.SetProjectRuntime(index, machine, runtimeErr)
}
