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
	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
	"github.com/glade-sh/glade/internal/watch"
)

func runDevVF(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	addr := "127.0.0.1:8080"
	root := "."
	readyFile := ""
	progress := false
	progressJSON := false
	noProgress := false
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
		case "--progress":
			progress = true
		case "--progress-json":
			progressJSON = true
		case "--no-progress", "--quiet":
			noProgress = true
		case "help", "-h", "--help":
			printDevVFHelp(w)
			return nil
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: progressW, Mode: progressModeForFlags(false, progress, progressJSON, noProgress)})
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "dev vf", Label: "Loading project"})
	p, err := project.Load(root)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "dev vf failed"})
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "dev vf", Label: "Indexing symbols", Current: 1, Total: 4})
	index, err := loadIndex(root)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "dev vf failed"})
		return err
	}
	org := apextest.OrgFromIndex(index)
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "dev vf", Label: "Applying data fixtures", Current: 2, Total: 4})
	if err := applyDevVFProjectDataFixtures(p.Root, &org); err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "dev vf failed"})
		return err
	}
	source, err := server.NewSourceMetadataFromProject(p)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "dev vf failed"})
		return err
	}
	handler := server.NewWithSource(&org, source)
	installDevVFRuntime(handler, index)
	if root, err := gladehome.EnsureRoot(); err != nil {
		renderer.Render(cliui.Event{Kind: cliui.EventWarn, Phase: "dev vf", Label: "LWC toolchain unavailable", Detail: err.Error()})
	} else {
		renderer.Render(cliui.Event{Kind: cliui.EventInfo, Phase: "dev vf", Label: "LWC toolchain ready", Detail: root})
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "dev vf", Label: "Starting server", Current: 3, Total: 4})
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "dev vf failed"})
		return err
	}
	actualAddr := listener.Addr().String()
	if readyFile != "" {
		if err := writeDevVFReadyFile(readyFile, actualAddr, p); err != nil {
			_ = listener.Close()
			renderer.Finish(cliui.Result{OK: false, Label: "dev vf failed"})
			return err
		}
	}
	httpServer := &http.Server{Addr: addr, Handler: handler}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseEnd, Phase: "dev vf", Label: "Server ready", Current: 4, Total: 4})
	renderer.Finish(cliui.Result{OK: true, Label: "dev vf ready"})
	printDevVFStartupSummary(w, actualAddr, p)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	go runDevVFReload(ctx, root, handler, progressW)
	return normalizeDevVFServeError(httpServer.Serve(listener))
}

func normalizeDevVFServeError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func runDevVFReload(ctx context.Context, root string, srv *server.Server, w io.Writer) {
	if w == nil {
		w = io.Discard
	}
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
	printDevStartupList(w, "Pages", "page", pages, "Use --ready-file for the complete list.")
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
	trimmed := trimDevVFJSONBytes(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return readDevVFSFDXTreeDataFixture(trimmed)
	}
	var header struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(trimmed, &header); err != nil {
		return storage.Fixture{}, false, err
	}
	if strings.TrimSpace(header.Version) != storage.FixtureVersion {
		return storage.Fixture{}, false, nil
	}
	fixture, err := storage.ReadFixture(bytes.NewReader(trimmed))
	if err != nil {
		return storage.Fixture{}, false, err
	}
	return fixture, true, nil
}

func trimDevVFJSONBytes(data []byte) []byte {
	trimmed := bytes.TrimSpace(data)
	trimmed = bytes.TrimPrefix(trimmed, []byte{0xef, 0xbb, 0xbf})
	return bytes.TrimSpace(trimmed)
}

func readDevVFSFDXTreeDataFixture(data []byte) (storage.Fixture, bool, error) {
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(data, &rows); err != nil {
		return storage.Fixture{}, false, err
	}
	if len(rows) == 0 {
		return storage.Fixture{}, false, nil
	}
	hasTreeRecord := false
	for _, row := range rows {
		if _, ok := row["attributes"]; ok {
			hasTreeRecord = true
			break
		}
	}
	if !hasTreeRecord {
		return storage.Fixture{}, false, nil
	}
	byObject := map[string][]storage.FixtureRecord{}
	for i, row := range rows {
		attrs, err := readDevVFSFDXTreeAttributes(row["attributes"])
		if err != nil {
			return storage.Fixture{}, false, fmt.Errorf("records[%d].attributes: %w", i, err)
		}
		if attrs.Type == "" {
			return storage.Fixture{}, false, fmt.Errorf("records[%d].attributes.type is required", i)
		}
		record := storage.FixtureRecord{
			Alias:  attrs.ReferenceID,
			Fields: map[string]storage.Value{},
		}
		for name, raw := range row {
			if strings.EqualFold(name, "attributes") {
				continue
			}
			if strings.EqualFold(name, "Id") {
				id, ok, err := readDevVFSFDXTreeID(raw)
				if err != nil {
					return storage.Fixture{}, false, fmt.Errorf("records[%d].Id: %w", i, err)
				}
				if ok {
					record.ID = id
					continue
				}
			}
			value, isNull, err := readDevVFSFDXTreeValue(raw)
			if err != nil {
				return storage.Fixture{}, false, fmt.Errorf("records[%d].%s: %w", i, name, err)
			}
			if isNull {
				record.ExplicitNulls = append(record.ExplicitNulls, name)
				continue
			}
			record.Fields[name] = value
		}
		byObject[attrs.Type] = append(byObject[attrs.Type], record)
	}
	fixture := storage.NewFixture()
	for objectName, records := range byObject {
		fixture.Objects = append(fixture.Objects, storage.FixtureObject{Name: objectName, Records: records})
	}
	sort.Slice(fixture.Objects, func(i, j int) bool {
		return fixture.Objects[i].Name < fixture.Objects[j].Name
	})
	return fixture, true, nil
}

type devVFSFDXTreeAttributes struct {
	Type        string `json:"type"`
	ReferenceID string `json:"referenceId"`
}

func readDevVFSFDXTreeAttributes(raw json.RawMessage) (devVFSFDXTreeAttributes, error) {
	if len(raw) == 0 {
		return devVFSFDXTreeAttributes{}, errors.New("is required")
	}
	var attrs devVFSFDXTreeAttributes
	if err := json.Unmarshal(raw, &attrs); err != nil {
		return devVFSFDXTreeAttributes{}, err
	}
	attrs.Type = strings.TrimSpace(attrs.Type)
	attrs.ReferenceID = strings.TrimSpace(attrs.ReferenceID)
	return attrs, nil
}

func readDevVFSFDXTreeID(raw json.RawMessage) (storage.ID, bool, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false, err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false, nil
	}
	return storage.ID(value), true, nil
}

func readDevVFSFDXTreeValue(raw json.RawMessage) (storage.Value, bool, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return storage.NullValue(), true, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return storage.StringValue(text), false, nil
	}
	var boolean bool
	if err := json.Unmarshal(raw, &boolean); err == nil {
		return storage.BooleanValue(boolean), false, nil
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		return storage.DecimalValue(number.String()), false, nil
	}
	return storage.Value{}, false, errors.New("unsupported SFDX tree value")
}

func printDevVFHelp(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`
Start a local Visualforce preview development server with hot reload.

Usage:
  glade dev vf [--project <root>] [--port <port>|--addr <host:port>] [--ready-file <path>]

Preview feature:
  Useful local /apex rendering for development. Not full hosted Salesforce
  chrome, lifecycle, component-edge, getContent*, or PDF parity.

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
