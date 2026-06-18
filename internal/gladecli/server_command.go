package gladecli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/playground"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func runServer(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := "127.0.0.1:8080"
	dbPath := ""
	root := "."
	projectProvided := false
	limitMode := vm.LimitMode("")
	limitProfile := ""
	limitCapValues := make(map[string]string)
	progress := false
	progressJSON := false
	noProgress := false
	for i := 0; i < len(args); i++ {
		if name, ok := strings.CutPrefix(args[i], "--"); ok && isLimitCapFlag(name) {
			value, err := takeFlagValue(args, &i, "--"+name+" requires a value")
			if err != nil {
				return err
			}
			limitCapValues[name] = value
			continue
		}
		switch args[i] {
		case "--addr":
			value, err := takeFlagValue(args, &i, "--addr requires a value")
			if err != nil {
				return err
			}
			addr = value
		case "--db":
			value, err := takeFlagValue(args, &i, "--db requires a path")
			if err != nil {
				return err
			}
			dbPath = value
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return err
			}
			root = value
			projectProvided = true
		case "--limit-mode":
			value, err := takeFlagValue(args, &i, "--limit-mode requires a value")
			if err != nil {
				return err
			}
			mode, err := parseLimitMode(value)
			if err != nil {
				return err
			}
			limitMode = mode
		case "--limit-profile":
			value, err := takeFlagValue(args, &i, "--limit-profile requires a value")
			if err != nil {
				return err
			}
			limitProfile = value
		case "--progress":
			progress = true
		case "--progress-json":
			progressJSON = true
		case "--no-progress", "--quiet":
			noProgress = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: progressW, Mode: progressModeForFlags(false, progress, progressJSON, noProgress)})
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "server", Label: "Opening database"})
	limitCaps, limitCapsSet, err := parseLimitCapsFromFlags(limitProfile, func(name string) string {
		return limitCapValues[name]
	})
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "server failed"})
		return err
	}
	var org storage.OrgState
	var handler http.Handler
	if dbPath != "" {
		store, loaded, err := openDBStore(dbPath, root)
		if err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "server failed"})
			return err
		}
		defer store.Close()
		org = loaded
		source, _ := serverSourceMetadata(root)
		handler = server.NewWithStoreAndSource(&org, store, source)
	} else {
		var err error
		org, err = orgForProject(root)
		if err != nil {
			if projectProvided {
				renderer.Finish(cliui.Result{OK: false, Label: "server failed"})
				return err
			}
			org = storageBaseline()
		}
		source, _ := serverSourceMetadata(root)
		handler = server.NewWithSource(&org, source)
	}
	if limitMode != "" {
		if srv, ok := handler.(*server.Server); ok {
			srv.LimitMode = limitMode
		}
	}
	if srv, ok := handler.(*server.Server); ok {
		srv.LimitProfile = limitProfile
		if limitCapsSet {
			srv.LimitCaps = limitCaps
		}
	}
	if srv, ok := handler.(*server.Server); ok {
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "server", Label: "Indexing symbols", Current: 2, Total: 3})
		if index, err := loadIndex(root); err == nil {
			srv.SetProjectIndex(index)
		} else if projectProvided {
			renderer.Finish(cliui.Result{OK: false, Label: "server failed"})
			return err
		}
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseEnd, Phase: "server", Label: "Starting listener", Current: 3, Total: 3})
	renderer.Finish(cliui.Result{OK: true, Label: "server ready"})
	url := server.URL(addr)
	fmt.Fprintln(w, "Glade server")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Local Salesforce-shaped REST API started")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Address   %s\n", url)
	if dbPath != "" {
		fmt.Fprintf(w, "Database  %s\n", filepath.ToSlash(dbPath))
	} else {
		fmt.Fprintln(w, "Database  memory-only")
	}
	fmt.Fprintf(w, "Project   %s\n", filepath.ToSlash(root))
	fmt.Fprintln(w, "Mode      local")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Try:")
	fmt.Fprintf(w, "  curl %q\n", url+"/services/data/v60.0/query?q=SELECT+Name+FROM+Account")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Stop:")
	fmt.Fprintln(w, "  Ctrl-C")
	return http.ListenAndServe(addr, handler)
}

func runPlayground(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := "127.0.0.1:1789"
	addrProvided := false
	dbPath := filepath.Join(".glade", "playground", "org.sqlite")
	dataRoot := filepath.Join(".glade", "playground")
	workspaceID := "default"
	projectRoot := ""
	projectRefs := []playground.ProjectReference{}
	showExamples := false
	listExamples := false
	exampleID := ""
	noDB := false
	resetOnStart := false
	limitMode := vm.LimitModePermissive
	openBrowser := false
	openBrowserSet := false
	once := false
	wizard := false
	public := false
	runTimeout := time.Duration(0)
	ratePerMinute := 0
	progress := false
	progressJSON := false
	noProgress := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--addr":
			value, err := takeFlagValue(args, &i, "--addr requires a value")
			if err != nil {
				return err
			}
			addr = value
			addrProvided = true
		case "--db":
			value, err := takeFlagValue(args, &i, "--db requires a path")
			if err != nil {
				return err
			}
			dbPath = value
		case "--no-db":
			dbPath = ""
			noDB = true
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return err
			}
			projectRoot = value
		case "--project-ref":
			value, err := takeFlagValue(args, &i, "--project-ref requires a value")
			if err != nil {
				return err
			}
			ref, err := parsePlaygroundProjectRef(value)
			if err != nil {
				return err
			}
			projectRefs = append(projectRefs, ref)
		case "--examples":
			showExamples = true
		case "--list-examples":
			listExamples = true
		case "--example":
			value, err := takeFlagValue(args, &i, "--example requires a value")
			if err != nil {
				return err
			}
			exampleID = strings.TrimSpace(value)
			showExamples = true
		case "--reset-on-start":
			resetOnStart = true
		case "--public":
			public = true
		case "--run-timeout":
			value, err := takeFlagValue(args, &i, "--run-timeout requires a value")
			if err != nil {
				return err
			}
			d, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			runTimeout = d
		case "--rate-per-minute":
			value, err := takeFlagValue(args, &i, "--rate-per-minute requires a value")
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			ratePerMinute = n
		case "--data-root":
			value, err := takeFlagValue(args, &i, "--data-root requires a value")
			if err != nil {
				return err
			}
			dataRoot = value
		case "--workspace":
			value, err := takeFlagValue(args, &i, "--workspace requires a value")
			if err != nil {
				return err
			}
			workspaceID = value
		case "--limit-mode":
			value, err := takeFlagValue(args, &i, "--limit-mode requires a value")
			if err != nil {
				return err
			}
			mode, err := parseLimitMode(value)
			if err != nil {
				return err
			}
			limitMode = mode
		case "--open":
			openBrowser = true
			openBrowserSet = true
		case "--no-open":
			openBrowser = false
			openBrowserSet = true
		case "--once":
			once = true
		case "--wizard":
			wizard = true
		case "--progress":
			progress = true
		case "--progress-json":
			progressJSON = true
		case "--no-progress", "--quiet":
			noProgress = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: progressW, Mode: progressModeForFlags(false, progress, progressJSON, noProgress)})
	if exampleID != "" && !playgroundExampleExists(exampleID) {
		return fmt.Errorf("unknown playground example %q", exampleID)
	}
	if exampleID != "" && projectRoot != "" {
		return errors.New("--example requires the managed scratch workspace; remove --project")
	}
	if exampleID != "" && len(projectRefs) > 0 {
		return errors.New("--example cannot be combined with --project-ref")
	}
	if listExamples {
		return writePlaygroundExamplesList(w, projectRefs)
	}
	if resetOnStart && projectRoot != "" {
		return errors.New("--reset-on-start refuses --project because it would delete project source")
	}
	if projectRoot == "" {
		if _, err := playgroundWorkspaceRoot(dataRoot, workspaceID); err != nil {
			return err
		}
	}
	if noDB {
		dbPath = ""
	}
	if public && !addrProvided {
		port := strings.TrimSpace(os.Getenv("PORT"))
		if port == "" {
			port = "8080"
		}
		addr = "0.0.0.0:" + port
	}
	if wizard {
		return writePlaygroundWizard(w, playgroundWizardOptions{
			addr:           addr,
			addrProvided:   addrProvided,
			dbPath:         dbPath,
			dataRoot:       dataRoot,
			workspaceID:    workspaceID,
			projectRoot:    projectRoot,
			projectRefs:    projectRefs,
			showExamples:   showExamples,
			exampleID:      exampleID,
			noDB:           noDB,
			resetOnStart:   resetOnStart,
			limitMode:      limitMode,
			openBrowser:    openBrowser,
			openBrowserSet: openBrowserSet,
			public:         public,
			runTimeout:     runTimeout,
			ratePerMinute:  ratePerMinute,
		})
	}
	if resetOnStart {
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "playground", Label: "Preparing database"})
		if err := resetPlaygroundState(dataRoot, workspaceID, dbPath); err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "playground failed"})
			return err
		}
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "playground", Label: "Opening workspace"})
	ws, err := playground.OpenWorkspace(playground.WorkspaceOptions{
		DataRoot:    dataRoot,
		ID:          workspaceID,
		ProjectRoot: projectRoot,
	})
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "playground failed"})
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "playground", Label: "Starting workbench", Current: 2, Total: 2})
	handler := playground.NewServer(ws, playground.ServerOptions{
		Version:           Version,
		DBPath:            dbPath,
		DefaultLimitMode:  limitMode,
		ProjectReferences: projectRefs,
		ShowExamples:      showExamples,
		Public:            public,
		RunTimeout:        runTimeout,
		RatePerMinute:     ratePerMinute,
	})
	url := playgroundURL(addr, exampleID)
	fmt.Fprintln(w, "Glade playground")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Started local browser workbench")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "URL       %s\n", url)
	if showExamples {
		fmt.Fprintln(w, "Examples  enabled")
	} else {
		fmt.Fprintln(w, "Examples  disabled")
	}
	if noDB {
		fmt.Fprintln(w, "Database  memory-only")
	} else {
		fmt.Fprintf(w, "Database  %s\n", filepath.ToSlash(dbPath))
	}
	if resetOnStart {
		fmt.Fprintln(w, "Reset     completed")
	}
	renderer.Finish(cliui.Result{OK: true, Label: "playground ready"})
	if once {
		_ = handler
		return nil
	}
	if openBrowser {
		_ = openURL(url)
	}
	return http.ListenAndServe(addr, handler)
}

type playgroundWizardOptions struct {
	addr           string
	addrProvided   bool
	dbPath         string
	dataRoot       string
	workspaceID    string
	projectRoot    string
	projectRefs    []playground.ProjectReference
	showExamples   bool
	exampleID      string
	noDB           bool
	resetOnStart   bool
	limitMode      vm.LimitMode
	openBrowser    bool
	openBrowserSet bool
	public         bool
	runTimeout     time.Duration
	ratePerMinute  int
}

func writePlaygroundWizard(w io.Writer, opts playgroundWizardOptions) error {
	args := []string{"glade", "playground"}
	if opts.public {
		args = append(args, "--public")
	} else if opts.addrProvided {
		args = append(args, "--addr", opts.addr)
	}
	if opts.projectRoot != "" {
		args = append(args, "--project", opts.projectRoot)
	}
	if opts.dbPath != "" && opts.dbPath != filepath.Join(".glade", "playground", "org.sqlite") {
		args = append(args, "--db", opts.dbPath)
	}
	if opts.dataRoot != "" {
		args = append(args, "--data-root", opts.dataRoot)
	}
	if opts.workspaceID != "" && opts.workspaceID != "default" {
		args = append(args, "--workspace", opts.workspaceID)
	}
	for _, ref := range opts.projectRefs {
		args = append(args, "--project-ref", ref.Name+"="+ref.Path)
	}
	if opts.showExamples {
		args = append(args, "--examples")
	}
	if opts.exampleID != "" {
		args = append(args, "--example", opts.exampleID)
	}
	if opts.noDB {
		args = append(args, "--no-db")
	}
	if opts.resetOnStart {
		args = append(args, "--reset-on-start")
	}
	if opts.limitMode != "" && opts.limitMode != vm.LimitModePermissive {
		args = append(args, "--limit-mode", string(opts.limitMode))
	}
	if opts.runTimeout > 0 {
		args = append(args, "--run-timeout", opts.runTimeout.String())
	}
	if opts.ratePerMinute > 0 {
		args = append(args, "--rate-per-minute", strconv.Itoa(opts.ratePerMinute))
	}
	if opts.openBrowserSet {
		if opts.openBrowser {
			args = append(args, "--open")
		} else {
			args = append(args, "--no-open")
		}
	} else {
		args = append(args, "--open")
	}
	fmt.Fprintln(w, "Playground wizard")
	fmt.Fprintf(w, "  %s\n", shellCommand(args...))
	fmt.Fprintln(w, "  Open: "+playgroundURL(opts.addr, opts.exampleID))
	return nil
}

func playgroundExampleExists(id string) bool {
	for _, example := range playground.ListExampleProjects() {
		if example.ID == id {
			return true
		}
	}
	return false
}

func playgroundURL(addr, exampleID string) string {
	out := "http://" + addr + "/playground/"
	if exampleID != "" {
		out += "?example=" + url.QueryEscape(exampleID)
	}
	return out
}

func writePlaygroundExamplesList(w io.Writer, refs []playground.ProjectReference) error {
	fmt.Fprintln(w, "Playground examples")
	seen := make(map[string]bool)
	for _, example := range playground.ListExampleProjects() {
		if seen[example.ID] {
			continue
		}
		seen[example.ID] = true
		fmt.Fprintf(w, "  %s\t%s\t%d files\t%s\n", example.ID, example.Name, example.FileCount, strings.Join(example.Tags, ", "))
	}
	for _, ref := range normalizePlaygroundCLIProjectReferences(refs) {
		count, err := countPlaygroundProjectRefFiles(ref.Path)
		if err != nil {
			return err
		}
		tags := append([]string{"local"}, ref.Tags...)
		fmt.Fprintf(w, "  %s\t%s\t%d files\t%s\n", ref.ID, ref.Name, count, strings.Join(tags, ", "))
	}
	return nil
}

func runExamples(args []string, w io.Writer) error {
	tag := ""
	if len(args) > 0 && args[0] == "--tag" {
		if len(args) < 2 {
			return errors.New("--tag requires a value")
		}
		tag = strings.ToLower(strings.TrimSpace(args[1]))
		args = args[2:]
	}
	if len(args) == 0 {
		return writeExamplesList(w, tag)
	}
	switch args[0] {
	case "show":
		if len(args) != 2 {
			return errors.New("usage: glade examples show <id>")
		}
		return writeExampleShow(w, args[1])
	case "run":
		if len(args) != 2 {
			return errors.New("usage: glade examples run <id>")
		}
		if !playgroundExampleExists(args[1]) {
			return fmt.Errorf("unknown example %q", args[1])
		}
		fmt.Fprintln(w, "Example run")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Run:\n  glade playground --example %s --open\n", args[1])
		return nil
	default:
		return errors.New("usage: glade examples [--tag <tag>] | glade examples show <id> | glade examples run <id>")
	}
}

func writeExamplesList(w io.Writer, tag string) error {
	fmt.Fprintln(w, "Built-in examples")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "ID                           Name                         Tags")
	for _, example := range playground.ListExampleProjects() {
		if tag != "" && !exampleHasTag(example, tag) {
			continue
		}
		fmt.Fprintf(w, "%-28s %-28s %s\n", example.ID, example.Name, strings.Join(example.Tags, ", "))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Try:")
	fmt.Fprintln(w, "  glade playground --example account-service --open")
	fmt.Fprintln(w, "  glade examples show account-service")
	return nil
}

func writeExampleShow(w io.Writer, id string) error {
	for _, example := range playground.ListExampleProjects() {
		if example.ID != id {
			continue
		}
		fmt.Fprintf(w, "%s — %s\n\n", example.ID, example.Name)
		if strings.TrimSpace(example.Description) != "" {
			fmt.Fprintln(w, example.Description)
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "Files: %d\n", example.FileCount)
		fmt.Fprintf(w, "Tags:  %s\n", strings.Join(example.Tags, ", "))
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Try:")
		fmt.Fprintf(w, "  glade playground --example %s --open\n", example.ID)
		fmt.Fprintf(w, "  glade examples run %s\n", example.ID)
		return nil
	}
	return fmt.Errorf("unknown example %q", id)
}

func exampleHasTag(example playground.ExampleProject, tag string) bool {
	for _, candidate := range example.Tags {
		if strings.EqualFold(candidate, tag) {
			return true
		}
	}
	return false
}

func normalizePlaygroundCLIProjectReferences(refs []playground.ProjectReference) []playground.ProjectReference {
	out := make([]playground.ProjectReference, 0, len(refs))
	used := make(map[string]int)
	for _, ref := range refs {
		ref.Path = strings.TrimSpace(ref.Path)
		if ref.Path == "" {
			continue
		}
		if abs, err := filepath.Abs(ref.Path); err == nil {
			ref.Path = abs
		}
		ref.Name = strings.TrimSpace(ref.Name)
		if ref.Name == "" {
			ref.Name = filepath.Base(ref.Path)
		}
		ref.ID = strings.TrimSpace(ref.ID)
		if ref.ID == "" {
			ref.ID = "local-" + playgroundCLISlugID(ref.Name)
		}
		if n := used[ref.ID]; n > 0 {
			used[ref.ID] = n + 1
			ref.ID = fmt.Sprintf("%s-%d", ref.ID, n+1)
		} else {
			used[ref.ID] = 1
		}
		ref.Tags = append([]string(nil), ref.Tags...)
		out = append(out, ref)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func playgroundCLISlugID(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 && b.String()[b.Len()-1] != '-' {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "project"
	}
	return out
}

func countPlaygroundProjectRefFiles(root string) (int, error) {
	info, err := os.Stat(root)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("project reference is not a directory: %s", root)
	}
	count := 0
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if shouldSkipPlaygroundCLIProjectRefDir(entry.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if isAllowedPlaygroundCLIExtension(filepath.Ext(path)) {
			count++
		}
		return nil
	}); err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, fmt.Errorf("project reference has no loadable files: %s", root)
	}
	return count, nil
}

func shouldSkipPlaygroundCLIProjectRefDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case ".git", ".hg", ".svn", ".sf", ".sfdx", ".glade", "node_modules", "dist", "bin":
		return true
	default:
		return false
	}
}

func isAllowedPlaygroundCLIExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".cls", ".trigger", ".apex", ".json", ".xml", ".yml", ".yaml":
		return true
	default:
		return false
	}
}

func resetPlaygroundState(dataRoot, workspaceID, dbPath string) error {
	root, err := playgroundWorkspaceRoot(dataRoot, workspaceID)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(dataRoot, "cache")); err != nil {
		return err
	}
	if dbPath != "" {
		if err := os.Remove(dbPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func playgroundWorkspaceRoot(dataRoot, workspaceID string) (string, error) {
	id := strings.TrimSpace(workspaceID)
	if id == "" {
		id = "default"
	}
	if filepath.IsAbs(id) || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid playground workspace id %q", workspaceID)
	}
	clean := filepath.Clean(id)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean != id {
		return "", fmt.Errorf("invalid playground workspace id %q", workspaceID)
	}
	base := filepath.Join(dataRoot, "workspaces")
	root := filepath.Join(base, id)
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if absRoot != absBase && !strings.HasPrefix(absRoot, absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("workspace path escapes playground data root: %s", workspaceID)
	}
	return root, nil
}

func parsePlaygroundProjectRef(value string) (playground.ProjectReference, error) {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return playground.ProjectReference{}, errors.New("--project-ref must be name=path")
	}
	return playground.ProjectReference{Name: strings.TrimSpace(parts[0]), Path: strings.TrimSpace(parts[1])}, nil
}

func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func serverSourceMetadata(root string) (server.SourceMetadata, error) {
	p, err := project.Load(root)
	if err != nil {
		return server.SourceMetadata{}, err
	}
	return server.NewSourceMetadataFromProject(p)
}
