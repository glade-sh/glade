package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
	"github.com/glade-sh/glade/internal/watch"
)

func runDevVF(ctx context.Context, args []string, w io.Writer) error {
	addr := "127.0.0.1:8080"
	root := "."
	readyFile := ""
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
		case "--ready-file":
			if i+1 >= len(args) {
				return errors.New("--ready-file requires a value")
			}
			readyFile = args[i+1]
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
	if err := applyDevVFProjectDataFixtures(p.Root, &org); err != nil {
		return err
	}
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
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	actualAddr := listener.Addr().String()
	if readyFile != "" {
		if err := writeDevVFReadyFile(readyFile, actualAddr, p); err != nil {
			_ = listener.Close()
			return err
		}
	}
	httpServer := &http.Server{Addr: addr, Handler: handler}
	printDevVFStartupSummary(w, actualAddr, p)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	go runDevVFReload(ctx, root, handler, w)
	return normalizeDevVFServeError(httpServer.Serve(listener))
}

func normalizeDevVFServeError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
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
					strings.Contains(path, "/aura/") || strings.Contains(path, "/lwc/") || strings.Contains(path, "/staticresources/") ||
					strings.HasSuffix(path, ".app") || strings.HasSuffix(path, ".resource") || strings.HasSuffix(path, ".resource-meta.xml") {
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
				machine, runtimeErr := buildDevVFRuntime(index)
				srv.ReloadProjectState(source, index, machine, runtimeErr)
			}
			fmt.Fprintf(w, "Reloaded Visualforce project metadata (%d change(s)).\n", len(changes))
		}
	}
}

func printDevVFStartupSummary(w io.Writer, addr string, p project.Project) {
	fmt.Fprintf(w, "Visualforce dev server: http://%s\n", addr)
	pages := devVFPageRoutes(p)
	if len(pages) > 0 {
		fmt.Fprintln(w, "Pages:")
		for _, page := range pages {
			fmt.Fprintf(w, "  %s\n", page)
		}
	}
	fmt.Fprintf(w, "Watching %s for .page, .component, .cls, aura, lwc, and static resource changes.\n", p.Root)
}

func devVFPageRoutes(p project.Project) []string {
	pages := make([]string, 0, len(p.VisualforcePageFiles))
	seen := make(map[string]bool, len(p.VisualforcePageFiles))
	for _, file := range p.VisualforcePageFiles {
		name := filepath.Base(file)
		name = strings.TrimSuffix(name, ".page-meta.xml")
		name = strings.TrimSuffix(name, ".page")
		if strings.TrimSpace(name) == "" {
			continue
		}
		route := "/apex/" + name
		if !seen[route] {
			pages = append(pages, route)
			seen[route] = true
		}
	}
	sort.Strings(pages)
	return pages
}

type devVFReadyFile struct {
	URL   string   `json:"url"`
	Addr  string   `json:"addr"`
	Pages []string `json:"pages"`
}

func writeDevVFReadyFile(path, addr string, p project.Project) error {
	ready := devVFReadyFile{
		URL:   "http://" + addr,
		Addr:  addr,
		Pages: devVFPageRoutes(p),
	}
	data, err := json.MarshalIndent(ready, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func applyDevVFProjectDataFixtures(root string, org *storage.OrgState) error {
	if org == nil {
		return nil
	}
	matches, err := filepath.Glob(filepath.Join(root, "data", "*.json"))
	if err != nil {
		return err
	}
	sort.Strings(matches)
	applied := false
	for _, path := range matches {
		fixture, ok, err := readDevVFProjectDataFixture(path)
		if err != nil {
			return fmt.Errorf("read dev vf data fixture %s: %w", filepath.ToSlash(path), err)
		}
		if !ok {
			continue
		}
		if err := storage.ApplyFixture(org, fixture); err != nil {
			return fmt.Errorf("apply dev vf data fixture %s: %w", filepath.ToSlash(path), err)
		}
		applied = true
	}
	if applied {
		storage.EnsureDeterministicPlatformData(org)
	}
	return nil
}

func readDevVFProjectDataFixture(path string) (storage.Fixture, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return storage.Fixture{}, false, err
	}
	var header struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return storage.Fixture{}, false, err
	}
	if strings.TrimSpace(header.Version) != storage.FixtureVersion {
		return storage.Fixture{}, false, nil
	}
	fixture, err := storage.ReadFixture(bytes.NewReader(data))
	if err != nil {
		return storage.Fixture{}, false, err
	}
	return fixture, true, nil
}

func printDevVFHelp(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`
Start a local Visualforce development server with hot reload.

Usage:
  glade dev vf [--project <root>] [--port <port>|--addr <host:port>] [--ready-file <path>]

Examples:
  glade dev vf --project .
  glade dev vf --port 8080
  glade dev vf --addr 127.0.0.1:0 --ready-file /tmp/glade-vf-ready.json
`)+"\n")
}

func installDevVFRuntime(srv *server.Server, index typesys.Index) {
	if srv == nil {
		return
	}
	machine, runtimeErr := buildDevVFRuntime(index)
	srv.SetProjectRuntime(index, machine, runtimeErr)
}

func buildDevVFRuntime(index typesys.Index) (*vm.VM, error) {
	machine := vm.New(nil)
	machine.SetCurrentNamespace(index.Project.Namespace)
	runtimeErr := apextest.RegisterProjectRuntimeForRequest(machine, index)
	return machine, runtimeErr
}
