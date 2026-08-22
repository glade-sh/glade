package gladecli

import (
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
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/orgimport"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/trace"
	"github.com/glade-sh/glade/internal/tui"
	"github.com/glade-sh/glade/internal/typesys"
)

func runDB(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if hasUIFlag(args) {
		return runTUIView(ctx, args, tui.BoardData, w, progressW)
	}
	if len(args) == 0 {
		return errors.New("usage: glade db ui|seed|reset|export|inspect|query|describe|import [--project <root>] [--env <name>|--db <path>] [--json] [fixture.json]")
	}
	command := args[0]
	if command == "ui" {
		return runDBUI(ctx, args[1:], w, progressW)
	}
	dbPath := ""
	envName := "dev"
	root := "."
	jsonOut := false
	wizard := false
	limit := 0
	limitSet := false
	queryAll := false
	progress := false
	progressJSON := false
	noProgress := false
	targetOrg := ""
	importFields := make([]string, 0)
	importObjects := make([]string, 0)
	importQuery := ""
	importCategory := "all"
	listObjects := false
	positionals := make([]string, 0)
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--db":
			value, err := takeFlagValue(args, &i, "--db requires a path")
			if err != nil {
				return err
			}
			dbPath = value
		case "--env":
			value, err := takeFlagValue(args, &i, "--env requires a name")
			if err != nil {
				return err
			}
			if err := validateDBEnvName(value); err != nil {
				return err
			}
			envName = value
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return err
			}
			root = value
		case "--json":
			jsonOut = true
		case "--wizard":
			wizard = true
		case "--limit":
			value, err := takeFlagValue(args, &i, "--limit requires a value")
			if err != nil {
				return err
			}
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				return fmt.Errorf("--limit requires a positive integer")
			}
			limit = parsed
			limitSet = true
		case "--query-all":
			queryAll = true
		case "--target-org", "-o":
			value, err := takeFlagValue(args, &i, "--target-org requires a value")
			if err != nil {
				return err
			}
			targetOrg = value
		case "--object":
			value, err := takeFlagValue(args, &i, "--object requires a value")
			if err != nil {
				return err
			}
			importObjects = append(importObjects, value)
		case "--fields":
			value, err := takeFlagValue(args, &i, "--fields requires a value")
			if err != nil {
				return err
			}
			importFields = append(importFields, splitCommaList(value)...)
		case "--query":
			value, err := takeFlagValue(args, &i, "--query requires a value")
			if err != nil {
				return err
			}
			importQuery = value
		case "--category":
			value, err := takeFlagValue(args, &i, "--category requires a value")
			if err != nil {
				return err
			}
			importCategory = value
		case "--list-objects":
			listObjects = true
		case "--progress":
			progress = true
		case "--progress-json":
			progressJSON = true
		case "--no-progress", "--quiet":
			noProgress = true
		case "--":
			positionals = append(positionals, args[i+1:]...)
			i = len(args)
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				return fmt.Errorf("unknown flag %q", args[i])
			}
			positionals = append(positionals, args[i])
		}
	}
	if command != "query" && command != "import" && (limitSet || queryAll) {
		return fmt.Errorf("glade db %s does not accept --limit or --query-all", command)
	}
	if command != "import" && (targetOrg != "" || len(importObjects) > 0 || len(importFields) > 0 || importQuery != "" || importCategory != "all" || listObjects) {
		return fmt.Errorf("glade db %s does not accept Salesforce import flags", command)
	}
	if wizard {
		return writeDBWizard(w, command, resolveDBPath(dbPath, root, envName), root, jsonOut, positionals)
	}
	progressMode := progressModeForFlags(jsonOut, progress, progressJSON, noProgress)
	if command == "import" {
		if dbPath == "" && !listObjects {
			dbPath = projectEnvDBPath(root, envName)
		}
		return runDBImport(ctx, w, progressW, progressMode, dbPath, root, jsonOut, dbImportOptions{
			TargetOrg:   targetOrg,
			Objects:     importObjects,
			Fields:      importFields,
			Query:       importQuery,
			Limit:       limit,
			AllRows:     queryAll,
			Category:    importCategory,
			ListObjects: listObjects,
			Positionals: positionals,
		})
	}
	if dbPath == "" {
		dbPath = projectEnvDBPath(root, envName)
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{
		Stderr: progressW,
		Mode:   progressMode,
	})
	store, org, err := openDBStore(dbPath, root)
	if err != nil {
		return err
	}
	defer store.Close()
	schemaVersion, err := store.SchemaVersion()
	if err != nil {
		return err
	}
	switch command {
	case "query":
		if !jsonOut {
			return errors.New("usage: glade db query [--project <root>] [--env <name>|--db <path>] --json [--limit <n>] [--query-all] <soql>")
		}
		if len(positionals) != 1 {
			return errors.New("usage: glade db query [--project <root>] [--env <name>|--db <path>] --json [--limit <n>] [--query-all] <soql>")
		}
		return writeDBQueryJSON(w, org, positionals[0], limit, limitSet, queryAll)
	case "describe":
		if !jsonOut {
			return errors.New("usage: glade db describe [--project <root>] [--env <name>|--db <path>] --json [ObjectName]")
		}
		if len(positionals) > 1 {
			return fmt.Errorf("unexpected argument %q", positionals[1])
		}
		objectName := ""
		if len(positionals) == 1 {
			objectName = positionals[0]
		}
		return writeDBDescribeJSON(w, org, objectName)
	case "seed":
		if len(positionals) != 1 {
			return errors.New("usage: glade db seed [--project <root>] [--env <name>|--db <path>] <fixture.json>")
		}
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "db seed", Label: "Opening fixture"})
		file, err := os.Open(positionals[0])
		if err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "db seed failed"})
			return err
		}
		defer file.Close()
		fixture, err := storage.ReadFixture(file)
		if err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "db seed failed"})
			return err
		}
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "db seed", Label: "Applying fixture", Current: 1, Total: 3})
		if err := storage.ApplyFixture(&org, fixture); err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "db seed failed"})
			return err
		}
		storage.EnsureDeterministicPlatformData(&org)
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "db seed", Label: "Saving database", Current: 2, Total: 3})
		if err := store.Save(org); err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "db seed failed"})
			return err
		}
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseEnd, Phase: "db seed", Label: "Fixture applied", Current: 3, Total: 3})
		renderer.Finish(cliui.Result{OK: true, Label: "db seed complete"})
		return writeDBInspect(w, dbPath, org, jsonOut, schemaVersion, "")
	case "reset":
		if len(positionals) != 0 {
			return fmt.Errorf("unexpected argument %q", positionals[0])
		}
		storage.ResetData(&org)
		if err := store.Save(org); err != nil {
			return err
		}
		return writeDBInspect(w, dbPath, org, jsonOut, schemaVersion, "")
	case "export":
		if len(positionals) != 0 {
			return fmt.Errorf("unexpected argument %q", positionals[0])
		}
		return storage.WriteFixture(w, storage.FixtureFromOrg(org))
	case "inspect":
		if len(positionals) != 0 {
			return fmt.Errorf("unexpected argument %q", positionals[0])
		}
		return writeDBInspect(w, dbPath, org, jsonOut, schemaVersion, "db inspect")
	default:
		return errors.New("usage: glade db ui|seed|reset|export|inspect|query|describe|import [--project <root>] [--env <name>|--db <path>] [--json] [fixture.json]")
	}
}

type dbUIOptions struct {
	addr      string
	dbPath    string
	envName   string
	root      string
	readyFile string
	open      bool
}

type dbUIReadyFile struct {
	URL     string `json:"url"`
	Addr    string `json:"addr"`
	DB      string `json:"db"`
	Project string `json:"project"`
	Env     string `json:"env,omitempty"`
}

func runDBUI(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	return runDBUIWithOpenURL(ctx, args, w, progressW, openURL)
}

func runDBUIWithOpenURL(ctx context.Context, args []string, w io.Writer, progressW io.Writer, opener func(string) error) error {
	return runDBUIWithOpenURLAndListen(ctx, args, w, progressW, opener, net.Listen)
}

func runDBUIWithOpenURLAndListen(ctx context.Context, args []string, w io.Writer, progressW io.Writer, opener func(string) error, listen func(string, string) (net.Listener, error)) error {
	opts, err := parseDBUIOptions(args)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateServerBindAllowed(opts.addr); err != nil {
		return err
	}
	dbPath := resolveDBPath(opts.dbPath, opts.root, opts.envName)
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: progressW, Mode: cliui.ProgressAuto})
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "db ui", Label: "Opening database"})
	store, org, err := openDBStore(dbPath, opts.root)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "db ui failed"})
		return err
	}
	defer store.Close()
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "db ui", Label: "Starting listener", Current: 1, Total: 2})
	listener, err := listen("tcp", opts.addr)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "db ui failed"})
		return err
	}
	actualAddr := listener.Addr().String()
	url := "http://" + actualAddr + "/db"
	if opts.readyFile != "" {
		if err := writeDBUIReadyFile(opts.readyFile, actualAddr, dbPath, opts.root, opts.envName); err != nil {
			_ = listener.Close()
			renderer.Finish(cliui.Result{OK: false, Label: "db ui failed"})
			return err
		}
	}
	if opts.open {
		if err := opener(url); err != nil {
			_ = listener.Close()
			renderer.Finish(cliui.Result{OK: false, Label: "db ui failed"})
			return err
		}
	}
	handler := dbUIRouteScopedHandler{next: server.NewWithStore(&org, store)}
	httpServer := &http.Server{Addr: opts.addr, Handler: handler}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseEnd, Phase: "db ui", Label: "Record manager ready", Current: 2, Total: 2})
	renderer.Finish(cliui.Result{OK: true, Label: "db ui ready"})
	printDBUIStartupSummary(w, url, actualAddr, dbPath, opts.root)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	return normalizeDevVFServeError(httpServer.Serve(listener))
}

type dbUIRouteScopedHandler struct {
	next http.Handler
}

func (h dbUIRouteScopedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !dbUIRouteAllowed(r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	h.next.ServeHTTP(w, r)
}

func dbUIRouteAllowed(path string) bool {
	trimmed := strings.Trim(path, "/")
	if trimmed == "db" || strings.HasPrefix(trimmed, "db/") {
		return true
	}
	parts := strings.Split(trimmed, "/")
	return len(parts) >= 5 &&
		parts[0] == "services" &&
		parts[1] == "data" &&
		parts[3] == "glade" &&
		parts[4] == "db-manager"
}

func parseDBUIOptions(args []string) (dbUIOptions, error) {
	opts := dbUIOptions{addr: "127.0.0.1:0", envName: "dev", root: ".", open: true}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--db":
			value, err := takeFlagValue(args, &i, "--db requires a path")
			if err != nil {
				return opts, err
			}
			opts.dbPath = value
		case "--env":
			value, err := takeFlagValue(args, &i, "--env requires a name")
			if err != nil {
				return opts, err
			}
			if err := validateDBEnvName(value); err != nil {
				return opts, err
			}
			opts.envName = value
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return opts, err
			}
			opts.root = value
		case "--addr":
			value, err := takeFlagValue(args, &i, "--addr requires a value")
			if err != nil {
				return opts, err
			}
			opts.addr = value
		case "--port":
			value, err := takeFlagValue(args, &i, "--port requires a value")
			if err != nil {
				return opts, err
			}
			port, err := strconv.Atoi(value)
			if err != nil || port < 0 || port > 65535 {
				return opts, errors.New("--port requires a port number between 0 and 65535")
			}
			opts.addr = "127.0.0.1:" + value
		case "--ready-file":
			value, err := takeFlagValue(args, &i, "--ready-file requires a value")
			if err != nil {
				return opts, err
			}
			opts.readyFile = value
		case "--open":
			opts.open = true
		case "--no-open":
			opts.open = false
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				return opts, fmt.Errorf("unknown flag %q", args[i])
			}
			return opts, fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	return opts, nil
}

func writeDBUIReadyFile(path, addr, dbPath, root, envName string) error {
	ready := dbUIReadyFile{
		URL:     "http://" + addr + "/db",
		Addr:    addr,
		DB:      filepath.ToSlash(dbPath),
		Project: root,
		Env:     envName,
	}
	data, err := json.MarshalIndent(ready, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeReadyFileAtomically(path, data, 0o644, os.Rename)
}

func printDBUIStartupSummary(w io.Writer, url, addr, dbPath, root string) {
	fmt.Fprintln(w, "Glade db ui")
	fmt.Fprintf(w, "URL: %s\n", url)
	fmt.Fprintf(w, "addr: %s\n", addr)
	fmt.Fprintf(w, "project: %s\n", root)
	fmt.Fprintf(w, "db: %s\n", dbPath)
}

func resolveDBPath(dbPath, root, envName string) string {
	if dbPath != "" {
		return dbPath
	}
	return projectEnvDBPath(root, envName)
}

func projectEnvDBPath(root, envName string) string {
	if envName == "" {
		envName = "dev"
	}
	return filepath.Join(root, ".glade", "envs", envName+".sqlite")
}

func validateDBEnvName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("--env requires a name")
	}
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return fmt.Errorf("invalid db environment name %q", name)
	}
	return nil
}

type dbImportOptions struct {
	TargetOrg   string
	Objects     []string
	Fields      []string
	Query       string
	Limit       int
	AllRows     bool
	Category    string
	ListObjects bool
	Positionals []string
}

func runDBImport(ctx context.Context, w io.Writer, progressW io.Writer, progressMode cliui.ProgressMode, dbPath, root string, jsonOut bool, opts dbImportOptions) error {
	if len(opts.Positionals) != 1 || opts.Positionals[0] != "sf" {
		return errors.New("usage: glade db import sf [--target-org <alias>] [--db <path>] [--object <Object>] [--fields Id,Name] [--limit <n>] [--json]")
	}
	if opts.ListObjects {
		objects, err := orgimport.ListObjects(ctx, orgimport.CommandRunner{}, orgimport.ListObjectsOptions{TargetOrg: opts.TargetOrg, Category: opts.Category})
		if err != nil {
			return err
		}
		if jsonOut {
			payload := struct {
				TargetOrg string   `json:"targetOrg,omitempty"`
				Category  string   `json:"category"`
				Objects   []string `json:"objects"`
			}{TargetOrg: opts.TargetOrg, Category: importCategoryOrDefault(opts.Category), Objects: objects}
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			return enc.Encode(payload)
		}
		for _, object := range objects {
			if _, err := fmt.Fprintln(w, object); err != nil {
				return err
			}
		}
		return nil
	}
	if dbPath == "" {
		return errors.New("glade db import sf requires --db <path>")
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{
		Stderr: progressW,
		Mode:   progressMode,
	})
	store, org, err := openDBStore(dbPath, root)
	if err != nil {
		return err
	}
	defer store.Close()
	schemaVersion, err := store.SchemaVersion()
	if err != nil {
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "db import sf", Label: "Querying Salesforce"})
	result, err := orgimport.Import(ctx, orgimport.CommandRunner{}, orgimport.ImportOptions{
		TargetOrg: opts.TargetOrg,
		Objects:   opts.Objects,
		Fields:    opts.Fields,
		Query:     opts.Query,
		Limit:     opts.Limit,
		AllRows:   opts.AllRows,
	})
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "db import failed"})
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "db import sf", Label: "Applying imported rows", Current: 1, Total: 3})
	if err := storage.ApplyFixture(&org, result.Fixture); err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "db import failed"})
		return err
	}
	storage.EnsureDeterministicPlatformData(&org)
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "db import sf", Label: "Saving database", Current: 2, Total: 3})
	if err := store.Save(org); err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "db import failed"})
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseEnd, Phase: "db import sf", Label: fmt.Sprintf("Imported %d records", result.Records), Current: 3, Total: 3})
	renderer.Finish(cliui.Result{OK: true, Label: "db import complete"})
	return writeDBInspect(w, dbPath, org, jsonOut, schemaVersion, "")
}

func importCategoryOrDefault(category string) string {
	if category == "" {
		return "all"
	}
	return category
}

func writeDBWizard(w io.Writer, command, dbPath, root string, jsonOut bool, positionals []string) error {
	switch command {
	case "seed":
		fixture := "fixture.json"
		if len(positionals) > 0 {
			fixture = positionals[0]
		}
		args := []string{"glade", "db", "seed", "--db", dbPath, "--project", root, "--progress"}
		if jsonOut {
			args = append(args, "--json")
		}
		args = append(args, fixture)
		fmt.Fprintln(w, "DB seed wizard")
		fmt.Fprintf(w, "  %s\n", shellCommand(args...))
		fmt.Fprintf(w, "  %s\n", shellCommand("glade", "db", "inspect", "--db", dbPath, "--project", root))
		return nil
	default:
		return errors.New("usage: glade db seed --wizard [--project <root>] [--env <name>|--db <path>] <fixture.json>")
	}
}

func openDBStore(path, root string) (*storage.SQLiteStore, storage.OrgState, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, storage.OrgState{}, err
		}
	}
	store, err := storage.OpenSQLite(path)
	if err != nil {
		return nil, storage.OrgState{}, err
	}
	org, err := store.Load()
	if err != nil {
		_ = store.Close()
		return nil, storage.OrgState{}, err
	}
	projectOrg, binding, hasBinding, err := projectOrgAndDBBinding(root)
	if err != nil {
		_ = store.Close()
		return nil, storage.OrgState{}, err
	}
	if len(org.Objects) == 0 {
		if hasBinding {
			org = projectOrg
		} else {
			org, err = orgForProject(root)
			if err != nil {
				_ = store.Close()
				return nil, storage.OrgState{}, err
			}
		}
		storage.EnsureDeterministicPlatformData(&org)
		if err := store.Save(org); err != nil {
			_ = store.Close()
			return nil, storage.OrgState{}, err
		}
		if hasBinding {
			if err := store.SetProjectBinding(binding); err != nil {
				_ = store.Close()
				return nil, storage.OrgState{}, err
			}
		}
	} else if hasBinding {
		org, err = validateDBProjectBinding(store, path, root, binding, org, projectOrg)
		if err != nil {
			_ = store.Close()
			return nil, storage.OrgState{}, err
		}
	}
	return store, org, nil
}

func projectOrgAndDBBinding(root string) (storage.OrgState, storage.ProjectBinding, bool, error) {
	if root == "." && !currentDirIsGladeProjectRoot() {
		return storage.OrgState{}, storage.ProjectBinding{}, false, nil
	}
	p, index, err := loadProjectIndex(root)
	if err != nil {
		if root == "." {
			return storage.OrgState{}, storage.ProjectBinding{}, false, nil
		}
		return storage.OrgState{}, storage.ProjectBinding{}, false, err
	}
	org, err := orgStateFromIndex(root, p, index)
	if err != nil {
		return storage.OrgState{}, storage.ProjectBinding{}, false, err
	}
	fingerprint, err := storage.SchemaFingerprint(org)
	if err != nil {
		return storage.OrgState{}, storage.ProjectBinding{}, false, err
	}
	projectRoot := p.Root
	if projectRoot == "" {
		projectRoot = root
	}
	if abs, err := filepath.Abs(projectRoot); err == nil {
		projectRoot = abs
	}
	return org, storage.ProjectBinding{
		ProjectRoot:       filepath.Clean(projectRoot),
		SchemaFingerprint: fingerprint,
		SourceAPIVersion:  p.SourceAPIVersion,
		Namespace:         org.Namespace,
	}, true, nil
}

func validateDBProjectBinding(store *storage.SQLiteStore, dbPath, root string, expected storage.ProjectBinding, org, projectOrg storage.OrgState) (storage.OrgState, error) {
	stored, ok, err := store.ProjectBinding()
	if err != nil {
		return org, err
	}
	if !ok {
		actualFingerprint, err := storage.SchemaFingerprint(org)
		if err != nil {
			return org, err
		}
		if actualFingerprint != expected.SchemaFingerprint {
			return org, dbProjectMismatchError(dbPath, root, stored)
		}
		return org, store.SetProjectBinding(expected)
	}
	if stored.ProjectRoot != "" && !sameCleanPath(stored.ProjectRoot, expected.ProjectRoot) {
		return org, dbProjectMismatchError(dbPath, root, stored)
	}
	if stored.SchemaFingerprint != "" && stored.SchemaFingerprint != expected.SchemaFingerprint {
		if stored.ProjectRoot == "" {
			return org, dbProjectMismatchError(dbPath, root, stored)
		}
		refreshed, err := refreshDBProjectSchema(store, dbPath, root, org, projectOrg, expected)
		if err != nil {
			return org, err
		}
		return refreshed, nil
	}
	return org, nil
}

func refreshDBProjectSchema(store *storage.SQLiteStore, dbPath, root string, current, projectOrg storage.OrgState, binding storage.ProjectBinding) (storage.OrgState, error) {
	refreshed := projectOrg.Clone()
	refreshed.OrgID = current.OrgID
	refreshed.SystemTimestampBase = current.SystemTimestampBase
	refreshed.SystemTimestampSequence = current.SystemTimestampSequence
	if dropped := droppedRefreshRecordCounts(current, refreshed); len(dropped) > 0 {
		return storage.OrgState{}, destructiveSchemaRefreshError(dbPath, root, dropped)
	}
	for name, existingObject := range current.Objects {
		target, ok := refreshed.Objects[name]
		if !ok {
			continue
		}
		target.Records = make(map[storage.ID]storage.Record, len(existingObject.Records))
		for id, record := range existingObject.Records {
			target.Records[id] = record.Clone()
		}
		refreshed.Objects[name] = target
	}
	if refreshed.IDSequences == nil {
		refreshed.IDSequences = make(map[string]uint64, len(current.IDSequences))
	}
	for objectName, sequence := range current.IDSequences {
		if _, ok := refreshed.Objects[objectName]; ok {
			refreshed.IDSequences[objectName] = sequence
		}
	}
	storage.EnsureDeterministicPlatformData(&refreshed)
	storage.RebuildIndexes(&refreshed)
	if err := store.Save(refreshed); err != nil {
		return storage.OrgState{}, err
	}
	if err := store.SetProjectBinding(binding); err != nil {
		return storage.OrgState{}, err
	}
	return refreshed, nil
}

func droppedRefreshRecordCounts(current, refreshed storage.OrgState) map[string]int {
	dropped := make(map[string]int)
	for name, existingObject := range current.Objects {
		if _, ok := refreshed.Objects[name]; ok || len(existingObject.Records) == 0 {
			continue
		}
		dropped[name] = len(existingObject.Records)
	}
	return dropped
}

func destructiveSchemaRefreshError(dbPath, root string, dropped map[string]int) error {
	names := make([]string, 0, len(dropped))
	for name := range dropped {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	fmt.Fprintf(&b, "schema refresh would drop local records from %s\n", filepath.ToSlash(dbPath))
	fmt.Fprintln(&b, "Dropped records:")
	for _, name := range names {
		fmt.Fprintf(&b, "  %s: %d\n", name, dropped[name])
	}
	fmt.Fprintf(&b, "Export a backup first: glade db export --db %s --project %s > exported-fixture.json", filepath.ToSlash(dbPath), shellPathArg(root))
	return errors.New(strings.TrimSpace(b.String()))
}

func dbProjectMismatchError(dbPath, root string, stored storage.ProjectBinding) error {
	detail := ""
	if stored.ProjectRoot != "" {
		detail = "\nDatabase project: " + filepath.ToSlash(stored.ProjectRoot)
	}
	return fmt.Errorf("database belongs to a different Glade project schema: %s%s\nUse this project's database with: glade db inspect --project %s", filepath.ToSlash(dbPath), detail, shellPathArg(root))
}

func sameCleanPath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr == nil {
		left = leftAbs
	}
	if rightErr == nil {
		right = rightAbs
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func orgForProject(root string) (storage.OrgState, error) {
	p, index, err := loadProjectIndex(root)
	if err != nil {
		if root == "." {
			return storageBaseline(), nil
		}
		return storage.OrgState{}, err
	}
	return orgStateFromIndex(root, p, index)
}

func orgStateFromIndex(root string, p project.Project, index typesys.Index) (storage.OrgState, error) {
	org := storage.NewOrgState()
	org.APIVersion = storage.DefaultRESTAPIVersion
	org.Namespace = index.Project.Namespace
	registry := sobject.BuildDescribeRegistry(gladeschema.Schema{Objects: append([]gladeschema.Object(nil), index.Objects...)})
	for name, describe := range registry.Objects {
		org.Objects[name] = storage.ObjectState{
			Definition: sobject.ToObjectDefinition(describe),
			Records:    make(map[storage.ID]storage.Record),
		}
	}
	if err := storage.ApplyCustomMetadataRecords(&org, index.CustomMetadataRecords); err != nil {
		return storage.OrgState{}, err
	}
	if err := resource.ApplyProject(&org, p); err != nil {
		return storage.OrgState{}, err
	}
	if automationIndex, err := automation.LoadProject(p); err == nil {
		automation.ApplyToOrg(&org, automationIndex)
	}
	storage.EnsureDeterministicPlatformData(&org)
	storage.ApplyOrgShape(&org, project.OrgShapeFeatures(root))
	return org, nil
}

func writeDBInspect(w io.Writer, path string, org storage.OrgState, jsonOut bool, schemaVersion int, command string) error {
	summary := storage.InspectOrg(path, org)
	summary.SchemaVersion = schemaVersion
	if jsonOut {
		if strings.TrimSpace(command) != "" {
			return writeCLIJSONEnvelope(w, cliJSONEnvelope{
				Command:  command,
				Status:   "passed",
				ExitCode: 0,
				Data:     summary,
			})
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}
	fmt.Fprintln(w, "Glade db inspect")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Database: %s\n", filepath.ToSlash(path))
	fmt.Fprintf(w, "db: %s\n", path)
	if schemaVersion > 0 {
		fmt.Fprintf(w, "schemaVersion: %d\n", schemaVersion)
	}
	fmt.Fprintf(w, "objects: %d\n", summary.Objects)
	fmt.Fprintf(w, "records: %d\n", summary.Records)
	fmt.Fprintf(w, "users: %d\n", summary.Users)
	fmt.Fprintf(w, "profiles: %d\n", summary.Profiles)
	fmt.Fprintf(w, "permissions: %d\n", summary.Permissions)
	objects := make([]string, 0, len(summary.ByObject))
	for object := range summary.ByObject {
		objects = append(objects, object)
	}
	sort.Strings(objects)
	if len(objects) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Objects:")
	}
	budget := cliui.OutputBudget{}
	visible := budget.VisibleCount(len(objects))
	for _, object := range objects[:visible] {
		count := summary.ByObject[object]
		fmt.Fprintf(w, "%s: %d\n", object, count)
	}
	if omitted := budget.OmittedCount(len(objects)); omitted > 0 {
		fmt.Fprintf(w, "... %d more objects omitted. Use `glade db inspect --db %s --project . --json` for complete output.\n", omitted, filepath.ToSlash(path))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next:")
	fmt.Fprintf(w, "  glade db export --db %s > exported-fixture.json\n", filepath.ToSlash(path))
	return nil
}

type dbQueryJSON struct {
	Query     string           `json:"query"`
	TotalSize int              `json:"totalSize"`
	Done      bool             `json:"done"`
	Columns   []string         `json:"columns"`
	Records   []map[string]any `json:"records"`
}

func writeDBQueryJSON(w io.Writer, org storage.OrgState, rawQuery string, limit int, limitSet bool, queryAll bool) error {
	query, err := soql.ParseAtWithFiscalYearStartMonth(rawQuery, time.Now().UTC(), soql.FiscalYearStartMonth(org))
	if err != nil {
		return err
	}
	if limitSet {
		query.Limit = limit
		query.HasLimit = true
	}
	if queryAll {
		query.AllRows = true
	}
	result, err := soql.Execute(org, query)
	if err != nil {
		return err
	}
	records := dbQueryRecordsPayload(result.Records)
	payload := dbQueryJSON{
		Query:     rawQuery,
		TotalSize: result.Rows,
		Done:      true,
		Columns:   dbQueryColumns(query, records),
		Records:   records,
	}
	return writeCLIJSONEnvelope(w, cliJSONEnvelope{
		Command:  "db query",
		Status:   "passed",
		ExitCode: 0,
		Data:     payload,
	})
}

func dbQueryColumns(query soql.Query, records []map[string]any) []string {
	recordColumns := dbQueryColumnsFromRecords(records)
	if len(recordColumns) > 0 {
		recordColumnSet := make(map[string]bool, len(recordColumns))
		for _, column := range recordColumns {
			recordColumnSet[column] = true
		}
		ordered := make([]string, 0, len(recordColumns))
		seen := make(map[string]bool, len(recordColumns))
		for _, column := range dbQueryRequestedColumns(query) {
			if recordColumnSet[column] && !seen[column] {
				ordered = append(ordered, column)
				seen[column] = true
			}
		}
		for _, column := range recordColumns {
			if !seen[column] {
				ordered = append(ordered, column)
			}
		}
		return ordered
	}
	return dbQueryRequestedColumns(query)
}

func dbQueryRequestedColumns(query soql.Query) []string {
	columns := append([]string(nil), query.Fields...)
	for _, child := range query.ChildQueries {
		columns = append(columns, child.Relationship)
	}
	for _, spec := range query.Typeofs {
		columns = append(columns, spec.Relationship)
	}
	return columns
}

func dbQueryColumnsFromRecords(records []map[string]any) []string {
	columns := []string{}
	seen := map[string]bool{}
	for _, record := range records {
		names := make([]string, 0, len(record))
		for name := range record {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if seen[name] {
				continue
			}
			seen[name] = true
			columns = append(columns, name)
		}
	}
	return columns
}

func dbQueryRecordsPayload(records []storage.Record) []map[string]any {
	out := make([]map[string]any, 0, len(records))
	for _, record := range records {
		out = append(out, dbQueryRecordPayload(record))
	}
	return out
}

func dbQueryRecordPayload(record storage.Record) map[string]any {
	row := map[string]any{}
	if record.ID != "" {
		row["Id"] = string(record.ID)
	}
	names := make([]string, 0, len(record.Fields)+len(record.ExplicitNulls))
	seen := make(map[string]bool, len(record.Fields)+len(record.ExplicitNulls))
	for name := range record.Fields {
		if name == "Id" {
			continue
		}
		names = append(names, name)
		seen[name] = true
	}
	for name, isNull := range record.ExplicitNulls {
		if !isNull || name == "Id" || seen[name] {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if value, ok := record.Fields[name]; ok {
			row[name] = dbStorageValueJSON(value)
			continue
		}
		row[name] = nil
	}
	for relationship, children := range record.Children {
		row[relationship] = map[string]any{
			"totalSize": len(children),
			"done":      true,
			"records":   dbQueryRecordsPayload(children),
		}
	}
	return row
}

func dbStorageValueJSON(value storage.Value) any {
	switch value.Kind {
	case storage.ValueNull:
		return nil
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return value.String
	case storage.ValueInteger:
		return value.Integer
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueList:
		items := make([]any, 0, len(value.List))
		for _, item := range value.List {
			items = append(items, dbStorageValueJSON(item))
		}
		return items
	default:
		return nil
	}
}

type dbDescribeListJSON struct {
	Objects []dbDescribeObjectSummaryJSON `json:"objects"`
}

type dbDescribeObjectSummaryJSON struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	KeyPrefix string `json:"keyPrefix"`
	Records   int    `json:"records"`
}

type dbDescribeObjectJSON struct {
	Name      string                `json:"name"`
	Label     string                `json:"label"`
	KeyPrefix string                `json:"keyPrefix"`
	Fields    []dbDescribeFieldJSON `json:"fields"`
}

type dbDescribeFieldJSON struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	DisplayType string   `json:"displayType"`
	ReferenceTo []string `json:"referenceTo"`
}

func writeDBDescribeJSON(w io.Writer, org storage.OrgState, objectName string) error {
	if strings.TrimSpace(objectName) == "" {
		return writeCLIJSONEnvelope(w, cliJSONEnvelope{
			Command:  "db describe",
			Status:   "passed",
			ExitCode: 0,
			Data:     dbDescribeListJSON{Objects: dbDescribeObjectSummaries(org)},
		})
	}
	resolved, ok := storage.ResolveObjectName(org, objectName)
	if !ok {
		return fmt.Errorf("unknown object %s", objectName)
	}
	object := org.Objects[resolved]
	return writeCLIJSONEnvelope(w, cliJSONEnvelope{
		Command:  "db describe",
		Status:   "passed",
		ExitCode: 0,
		Data: dbDescribeObjectJSON{
			Name:      firstNonEmpty(object.Definition.APIName, resolved),
			Label:     firstNonEmpty(object.Definition.Label, object.Definition.APIName, resolved),
			KeyPrefix: object.Definition.KeyPrefix,
			Fields:    dbDescribeFields(object.Definition),
		},
	})
}

func dbDescribeObjectSummaries(org storage.OrgState) []dbDescribeObjectSummaryJSON {
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	objects := make([]dbDescribeObjectSummaryJSON, 0, len(names))
	for _, name := range names {
		object := org.Objects[name]
		objects = append(objects, dbDescribeObjectSummaryJSON{
			Name:      firstNonEmpty(object.Definition.APIName, name),
			Label:     firstNonEmpty(object.Definition.Label, object.Definition.APIName, name),
			KeyPrefix: object.Definition.KeyPrefix,
			Records:   len(object.Records),
		})
	}
	return objects
}

func dbDescribeFields(definition storage.ObjectDefinition) []dbDescribeFieldJSON {
	names := make([]string, 0, len(definition.Fields))
	for name := range definition.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	fields := make([]dbDescribeFieldJSON, 0, len(names))
	for _, name := range names {
		field := definition.Fields[name]
		referenceTo := append([]string(nil), field.ReferenceTo...)
		if referenceTo == nil {
			referenceTo = []string{}
		}
		fields = append(fields, dbDescribeFieldJSON{
			Name:        firstNonEmpty(field.APIName, name),
			Label:       firstNonEmpty(field.Label, field.APIName, name),
			Type:        string(field.Type),
			DisplayType: field.DisplayType,
			ReferenceTo: referenceTo,
		})
	}
	return fields
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func writeTraceFile(path string, events []trace.Event) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return trace.WriteJSON(file, trace.NewDocument(events))
}

// writeDebugLog writes a Salesforce-style debug log to path. The sentinel "-"
// writes to the command's stdout writer instead of a file.
func writeDebugLog(path, log string, stdout io.Writer) error {
	if path == "-" {
		_, err := io.WriteString(stdout, log)
		return err
	}
	if dir := filepath.Dir(path); dir != "." && strings.TrimSpace(dir) != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.WriteString(file, log)
	return err
}
