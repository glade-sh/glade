package gladecli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc"
	"github.com/glade-sh/glade/internal/lwcshell"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
)

func runDevLWC(ctx context.Context, args []string, w io.Writer) error {
	addr := "127.0.0.1:8080"
	root := "."
	readyFile := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			value, err := takeFlagValue(args, &i, "--port requires a value")
			if err != nil {
				return err
			}
			addr = "127.0.0.1:" + value
		case "--addr":
			value, err := takeFlagValue(args, &i, "--addr requires a value")
			if err != nil {
				return err
			}
			addr = value
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return err
			}
			root = value
		case "--ready-file":
			value, err := takeFlagValue(args, &i, "--ready-file requires a value")
			if err != nil {
				return err
			}
			readyFile = value
		case "help", "-h", "--help":
			printDevLWCHelp(w)
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
	if err := applyDevLWCProjectDataFixtures(p.Root, &org); err != nil {
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
		fmt.Fprintf(w, "warning: run `glade toolchain install` before opening LWC preview routes\n")
	} else {
		fmt.Fprintf(w, "LWC toolchain: %s\n", root)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	actualAddr := listener.Addr().String()
	if readyFile != "" {
		if err := writeDevLWCReadyFile(readyFile, actualAddr, p); err != nil {
			_ = listener.Close()
			return err
		}
	}
	httpServer := &http.Server{Addr: addr, Handler: handler}
	printDevLWCStartupSummary(w, actualAddr, p)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	go runDevLWCReload(ctx, root, handler, w)
	return normalizeDevVFServeError(httpServer.Serve(listener))
}

func applyDevLWCProjectDataFixtures(root string, org *storage.OrgState) error {
	return applyDevVFProjectDataFixtures(root, org)
}

func runDevLWCReload(ctx context.Context, root string, srv *server.Server, w io.Writer) {
	runDevVFReload(ctx, root, srv, w)
}

func printDevLWCStartupSummary(w io.Writer, addr string, p project.Project) {
	fmt.Fprintf(w, "LWC dev shell: http://%s\n", addr)
	routes := devLWCPreviewRoutes(p)
	if len(routes) > 0 {
		fmt.Fprintln(w, "Routes:")
		for _, route := range routes {
			fmt.Fprintf(w, "  %s\n", route)
		}
	}
	fmt.Fprintf(w, "Watching %s for lwc, flexipage, tab, Visualforce, Apex, and static resource changes.\n", p.Root)
}

type devLWCReadyFile struct {
	URL    string   `json:"url"`
	Addr   string   `json:"addr"`
	Routes []string `json:"routes"`
}

func writeDevLWCReadyFile(path, addr string, p project.Project) error {
	ready := devLWCReadyFile{
		URL:    "http://" + addr,
		Addr:   addr,
		Routes: devLWCPreviewRoutes(p),
	}
	data, err := json.MarshalIndent(ready, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func devLWCPreviewRoutes(p project.Project) []string {
	namespace := strings.TrimSpace(p.Namespace)
	if namespace == "" {
		namespace = "c"
	}
	var routes []string
	bundles, _ := lwc.BuildIndex(p)
	for _, name := range bundles.Names() {
		bundle, ok := bundles.Bundle(name)
		if !ok || bundle.MetaFile == "" {
			continue
		}
		meta, err := lwc.ParseComponentMeta(bundle.MetaFile)
		if err != nil || !meta.IsExposed {
			continue
		}
		routes = append(routes, "/lwc/preview/component/"+namespace+"/"+bundle.Name)
	}
	for _, path := range p.FlexiPageFiles {
		page, err := lwcshell.LoadFlexiPage(path)
		if err != nil {
			continue
		}
		switch strings.ToLower(page.Type) {
		case "recordpage":
			objectAPIName := page.ObjectAPIName
			if objectAPIName == "" {
				objectAPIName = "<objectApiName>"
			}
			routes = append(routes, fmt.Sprintf("/lwc/preview/record/%s/<recordId>?page=%s", objectAPIName, page.Name))
		case "apppage":
			routes = append(routes, "/lwc/preview/app/"+page.Name)
		case "homepage":
			routes = append(routes, "/lwc/preview/home/"+page.Name)
		}
	}
	for _, path := range p.TabFiles {
		tab, err := lwcshell.LoadCustomTab(path)
		if err != nil {
			continue
		}
		route := "/lwc/preview/tab/" + tab.Name
		if tab.Type == lwcshell.TabTypeVisualforce {
			route += " -> /apex/" + tab.Target
		}
		routes = append(routes, route)
	}
	sort.Strings(routes)
	return dedupeDevLWCRoutes(routes)
}

func dedupeDevLWCRoutes(routes []string) []string {
	out := routes[:0]
	seen := map[string]bool{}
	for _, route := range routes {
		if strings.TrimSpace(route) == "" || seen[route] {
			continue
		}
		seen[route] = true
		out = append(out, route)
	}
	return out
}

func printDevLWCHelp(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`
Start a local LWC preview development shell with Salesforce-like context.

Usage:
  glade dev lwc [--project <root>] [--port <port>|--addr <host:port>] [--ready-file <path>]

Preview feature:
  Useful local Lightning-style preview routes. Not full hosted Lightning
  Experience, permissions, console API, full UI API, SLDS, or base-component
  parity.

Preview routes:
  /lwc/preview/component/c/contextProbe
  /lwc/preview/record/Account/<recordId>?page=Account_Record_Page
  /lwc/preview/app/App_Page
  /lwc/preview/home/Home_Page
  /lwc/preview/tab/Lwc_Probe
  /lwc/preview/tab/Visualforce_Tab

Visualforce-backed tabs redirect to /apex/<page>, where <apex:includeLightning/>
and Lightning Out render through the same local Lightning runtime.

Examples:
  glade dev lwc --project .
  glade dev lwc --port 8080
  glade dev lwc --addr 127.0.0.1:0 --ready-file /tmp/glade-lwc-ready.json
`)+"\n")
}
