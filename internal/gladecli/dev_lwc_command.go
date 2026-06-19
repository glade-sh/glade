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
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc"
	"github.com/glade-sh/glade/internal/lwcshell"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
)

func runDevLWC(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	opts, err := parseDevLWCOptions(args)
	if err != nil {
		if errors.Is(err, errDevLWCHelp) {
			printDevLWCHelp(w)
			return nil
		}
		return err
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: progressW, Mode: opts.progressMode})
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "dev lwc", Label: "Loading project"})
	p, err := project.Load(opts.root)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "dev lwc failed"})
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "dev lwc", Label: "Resolving selected context", Current: 1, Total: 6})
	selection, err := opts.selection()
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "dev lwc failed"})
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "dev lwc", Label: "Indexing symbols", Current: 2, Total: 6})
	index, err := loadIndex(opts.root)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "dev lwc failed"})
		return err
	}
	org := apextest.OrgFromIndex(index)
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "dev lwc", Label: "Applying data fixtures", Current: 3, Total: 6})
	if err := applyDevLWCProjectDataFixtures(p.Root, &org); err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "dev lwc failed"})
		return err
	}
	source, err := server.NewSourceMetadataFromProject(p)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "dev lwc failed"})
		return err
	}
	handler := server.NewWithSource(&org, source)
	installDevVFRuntime(handler, index)
	if root, err := gladehome.EnsureRoot(); err != nil {
		renderer.Render(cliui.Event{Kind: cliui.EventWarn, Phase: "dev lwc", Label: "LWC toolchain unavailable", Detail: err.Error()})
	} else {
		renderer.Render(cliui.Event{Kind: cliui.EventInfo, Phase: "dev lwc", Label: "LWC toolchain ready", Detail: root})
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "dev lwc", Label: "Building route metadata", Current: 4, Total: 6})
	_ = devLWCPreviewRoutes(p)
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "dev lwc", Label: "Starting shell", Current: 5, Total: 6})
	listener, err := net.Listen("tcp", opts.addr)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "dev lwc failed"})
		return err
	}
	actualAddr := listener.Addr().String()
	baseURL := "http://" + actualAddr
	selectedURL := devLWCSelectedURL(baseURL, selection.Context)
	if opts.readyFile != "" {
		if err := writeDevLWCReadyFile(opts.readyFile, actualAddr, p, selection); err != nil {
			_ = listener.Close()
			renderer.Finish(cliui.Result{OK: false, Label: "dev lwc failed"})
			return err
		}
	}
	if opts.open {
		openTarget := selectedURL
		if openTarget == "" {
			openTarget = baseURL + "/lwc"
		}
		if err := devLWCOpenURL(openTarget); err != nil {
			_ = listener.Close()
			renderer.Finish(cliui.Result{OK: false, Label: "dev lwc failed"})
			return err
		}
	}
	httpServer := &http.Server{Addr: opts.addr, Handler: handler}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseEnd, Phase: "dev lwc", Label: "Shell ready", Current: 6, Total: 6})
	renderer.Finish(cliui.Result{OK: true, Label: "dev lwc ready"})
	printDevLWCStartupSummary(w, actualAddr, p, selection)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	go runDevLWCReload(ctx, opts.root, handler, progressW)
	return normalizeDevVFServeError(httpServer.Serve(listener))
}

var errDevLWCHelp = errors.New("dev lwc help requested")
var devLWCOpenURL = openURL

type devLWCOptions struct {
	addr         string
	root         string
	readyFile    string
	open         bool
	contextName  string
	contextFile  string
	explicit     devLWCContextPresetFlags
	progressMode cliui.ProgressMode
}

type devLWCContextPresetFlags struct {
	preset        lwcshell.ContextPreset
	targetSet     bool
	componentSet  bool
	objectSet     bool
	recordSet     bool
	pageSet       bool
	tabSet        bool
	actionSet     bool
	appSet        bool
	formFactorSet bool
	stateSet      bool
	flowSet       bool
	flowInputSet  bool
}

type devLWCSelection struct {
	Name    string
	Context lwcshell.PageContext
}

func parseDevLWCOptions(args []string) (devLWCOptions, error) {
	opts := devLWCOptions{addr: "127.0.0.1:8080", root: ".", progressMode: cliui.ProgressAuto}
	progress := false
	progressJSON := false
	noProgress := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			value, err := takeFlagValue(args, &i, "--port requires a value")
			if err != nil {
				return opts, err
			}
			opts.addr = "127.0.0.1:" + value
		case "--addr":
			value, err := takeFlagValue(args, &i, "--addr requires a value")
			if err != nil {
				return opts, err
			}
			opts.addr = value
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return opts, err
			}
			opts.root = value
		case "--ready-file":
			value, err := takeFlagValue(args, &i, "--ready-file requires a value")
			if err != nil {
				return opts, err
			}
			opts.readyFile = value
		case "--open":
			opts.open = true
		case "--context":
			value, err := takeFlagValue(args, &i, "--context requires a value")
			if err != nil {
				return opts, err
			}
			opts.contextName = value
		case "--context-file":
			value, err := takeFlagValue(args, &i, "--context-file requires a value")
			if err != nil {
				return opts, err
			}
			opts.contextFile = value
		case "--target":
			value, err := takeFlagValue(args, &i, "--target requires a value")
			if err != nil {
				return opts, err
			}
			opts.explicit.preset.Target = value
			opts.explicit.targetSet = true
		case "--component":
			value, err := takeFlagValue(args, &i, "--component requires a value")
			if err != nil {
				return opts, err
			}
			opts.explicit.preset.Component = value
			opts.explicit.componentSet = true
		case "--object":
			value, err := takeFlagValue(args, &i, "--object requires a value")
			if err != nil {
				return opts, err
			}
			opts.explicit.preset.ObjectAPIName = value
			opts.explicit.objectSet = true
		case "--record":
			value, err := takeFlagValue(args, &i, "--record requires a value")
			if err != nil {
				return opts, err
			}
			opts.explicit.preset.RecordID = value
			opts.explicit.recordSet = true
		case "--page":
			value, err := takeFlagValue(args, &i, "--page requires a value")
			if err != nil {
				return opts, err
			}
			opts.explicit.preset.Page = value
			opts.explicit.pageSet = true
		case "--tab":
			value, err := takeFlagValue(args, &i, "--tab requires a value")
			if err != nil {
				return opts, err
			}
			opts.explicit.preset.Tab = value
			opts.explicit.tabSet = true
		case "--action":
			value, err := takeFlagValue(args, &i, "--action requires a value")
			if err != nil {
				return opts, err
			}
			opts.explicit.preset.Action = value
			opts.explicit.actionSet = true
		case "--app":
			value, err := takeFlagValue(args, &i, "--app requires a value")
			if err != nil {
				return opts, err
			}
			opts.explicit.preset.App = value
			opts.explicit.appSet = true
		case "--form-factor":
			value, err := takeFlagValue(args, &i, "--form-factor requires a value")
			if err != nil {
				return opts, err
			}
			opts.explicit.preset.FormFactor = value
			opts.explicit.formFactorSet = true
		case "--flow":
			value, err := takeFlagValue(args, &i, "--flow requires a value")
			if err != nil {
				return opts, err
			}
			opts.explicit.preset.Flow.APIName = value
			opts.explicit.flowSet = true
		case "--flow-input":
			value, err := takeFlagValue(args, &i, "--flow-input requires key=value")
			if err != nil {
				return opts, err
			}
			key, inputValue, ok := strings.Cut(value, "=")
			if !ok || strings.TrimSpace(key) == "" {
				return opts, errors.New("--flow-input requires key=value")
			}
			if opts.explicit.preset.Flow.InputVariables == nil {
				opts.explicit.preset.Flow.InputVariables = map[string]any{}
			}
			opts.explicit.preset.Flow.InputVariables[strings.TrimSpace(key)] = inputValue
			opts.explicit.flowInputSet = true
		case "--state":
			value, err := takeFlagValue(args, &i, "--state requires key=value")
			if err != nil {
				return opts, err
			}
			key, stateValue, ok := strings.Cut(value, "=")
			if !ok || strings.TrimSpace(key) == "" {
				return opts, errors.New("--state requires key=value")
			}
			if opts.explicit.preset.State == nil {
				opts.explicit.preset.State = map[string]string{}
			}
			opts.explicit.preset.State[strings.TrimSpace(key)] = stateValue
			opts.explicit.stateSet = true
		case "--progress":
			progress = true
		case "--progress-json":
			progressJSON = true
		case "--no-progress", "--quiet":
			noProgress = true
		case "help", "-h", "--help":
			return opts, errDevLWCHelp
		default:
			return opts, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	opts.progressMode = progressModeForFlags(false, progress, progressJSON, noProgress)
	return opts, nil
}

func (opts devLWCOptions) selection() (devLWCSelection, error) {
	var preset lwcshell.ContextPreset
	var name string
	hasPreset := false
	if opts.contextFile != "" {
		file, err := lwcshell.LoadContextPresetFile(opts.contextFile)
		if err != nil {
			return devLWCSelection{}, err
		}
		if opts.contextName != "" || file.DefaultContext != "" {
			loaded, err := file.Preset(opts.contextName)
			if err != nil {
				return devLWCSelection{}, err
			}
			preset = loaded
			name = opts.contextName
			if name == "" {
				name = file.DefaultContext
			}
			hasPreset = true
		}
	} else {
		file, err := lwcshell.LoadContextPresets(opts.root)
		if err != nil {
			return devLWCSelection{}, err
		}
		if opts.contextName != "" || file.DefaultContext != "" {
			loaded, err := file.Preset(opts.contextName)
			if err != nil {
				return devLWCSelection{}, err
			}
			preset = loaded
			name = opts.contextName
			if name == "" {
				name = file.DefaultContext
			}
			hasPreset = true
		}
	}
	if hasPreset {
		applyDevLWCExplicitContext(&preset, opts.explicit)
		ctx, err := preset.ToPageContext()
		if err != nil {
			return devLWCSelection{}, err
		}
		return devLWCSelection{Name: name, Context: ctx}, nil
	}
	if !opts.explicit.any() {
		return devLWCSelection{}, nil
	}
	preset = opts.explicit.preset
	inferDevLWCTarget(&preset)
	ctx, err := preset.ToPageContext()
	if err != nil {
		return devLWCSelection{}, err
	}
	return devLWCSelection{Context: ctx}, nil
}

func (f devLWCContextPresetFlags) any() bool {
	return f.targetSet || f.componentSet || f.objectSet || f.recordSet || f.pageSet || f.tabSet || f.actionSet || f.appSet || f.formFactorSet || f.stateSet || f.flowSet || f.flowInputSet
}

func applyDevLWCExplicitContext(preset *lwcshell.ContextPreset, flags devLWCContextPresetFlags) {
	if flags.targetSet {
		preset.Target = flags.preset.Target
	}
	if flags.componentSet {
		preset.Component = flags.preset.Component
	}
	if flags.objectSet {
		preset.ObjectAPIName = flags.preset.ObjectAPIName
	}
	if flags.recordSet {
		preset.RecordID = flags.preset.RecordID
	}
	if flags.pageSet {
		preset.Page = flags.preset.Page
	}
	if flags.tabSet {
		preset.Tab = flags.preset.Tab
	}
	if flags.actionSet {
		preset.Action = flags.preset.Action
	}
	if flags.appSet {
		preset.App = flags.preset.App
	}
	if flags.formFactorSet {
		preset.FormFactor = flags.preset.FormFactor
	}
	if flags.flowSet {
		preset.Flow.APIName = flags.preset.Flow.APIName
	}
	if flags.flowInputSet {
		if preset.Flow.InputVariables == nil {
			preset.Flow.InputVariables = map[string]any{}
		}
		for key, value := range flags.preset.Flow.InputVariables {
			preset.Flow.InputVariables[key] = value
		}
	}
	if flags.stateSet {
		if preset.State == nil {
			preset.State = map[string]string{}
		}
		for key, value := range flags.preset.State {
			preset.State[key] = value
		}
	}
}

func inferDevLWCTarget(preset *lwcshell.ContextPreset) {
	if strings.TrimSpace(preset.Target) != "" {
		return
	}
	switch {
	case strings.TrimSpace(preset.Flow.APIName) != "" && strings.TrimSpace(preset.Component) != "":
		preset.Target = "flowScreen"
	case strings.TrimSpace(preset.Component) != "":
		preset.Target = "component"
	case strings.TrimSpace(preset.Action) != "":
		if strings.TrimSpace(preset.ObjectAPIName) != "" || strings.TrimSpace(preset.RecordID) != "" {
			preset.Target = "recordAction"
		} else {
			preset.Target = "globalAction"
		}
	case strings.TrimSpace(preset.Tab) != "":
		preset.Target = "tab"
	case strings.TrimSpace(preset.RecordID) != "" || strings.TrimSpace(preset.ObjectAPIName) != "":
		preset.Target = "recordPage"
	case strings.TrimSpace(preset.Page) != "":
		preset.Target = "appPage"
	}
}

func applyDevLWCProjectDataFixtures(root string, org *storage.OrgState) error {
	return applyDevVFProjectDataFixtures(root, org)
}

func runDevLWCReload(ctx context.Context, root string, srv *server.Server, w io.Writer) {
	runDevVFReload(ctx, root, srv, w)
}

func printDevLWCStartupSummary(w io.Writer, addr string, p project.Project, selection devLWCSelection) {
	fmt.Fprintf(w, "LWC dev shell: http://%s\n", addr)
	if selectedURL := devLWCSelectedURL("http://"+addr, selection.Context); selectedURL != "" {
		if selection.Name != "" {
			fmt.Fprintf(w, "Selected context %s: %s\n", selection.Name, selectedURL)
		} else {
			fmt.Fprintf(w, "Selected route: %s\n", selectedURL)
		}
	}
	routes := devLWCPreviewRoutes(p)
	printDevStartupList(w, "Routes", "route", routes, "Open /lwc for home or /lwc/builder for the page builder; use --ready-file or /lightning/local/context.json for the complete list.")
	fmt.Fprintf(w, "Watching %s for lwc, flexipage, tab, Visualforce, Apex, and static resource changes.\n", p.Root)
}

type devLWCReadyFile struct {
	URL             string   `json:"url"`
	Addr            string   `json:"addr"`
	SelectedURL     string   `json:"selectedUrl,omitempty"`
	SelectedContext string   `json:"selectedContext,omitempty"`
	Routes          []string `json:"routes"`
}

func writeDevLWCReadyFile(path, addr string, p project.Project, selection devLWCSelection) error {
	baseURL := "http://" + addr
	ready := devLWCReadyFile{
		URL:             baseURL,
		Addr:            addr,
		SelectedURL:     devLWCSelectedURL(baseURL, selection.Context),
		SelectedContext: selection.Name,
		Routes:          devLWCPreviewRoutes(p),
	}
	data, err := json.MarshalIndent(ready, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func devLWCSelectedURL(baseURL string, ctx lwcshell.PageContext) string {
	return lwcshell.SelectedURL(baseURL, ctx)
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
		if meta.SupportsTarget("lightning__UrlAddressable") {
			routes = append(routes, "/lwc/preview/cmp/"+namespace+"/"+bundle.Name+"?c__name=value")
		}
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
		case "utilitybar":
			routes = append(routes, "/lwc/preview/utility/"+page.Name)
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
	for _, path := range p.QuickActionFiles {
		action, err := lwcshell.LoadQuickAction(path)
		if err != nil || action.ComponentName == "" {
			continue
		}
		actionName := action.Name
		if objectName := strings.TrimSpace(action.TargetObject); objectName != "" {
			if _, after, ok := strings.Cut(actionName, "."); ok {
				actionName = after
			}
			routes = append(routes, fmt.Sprintf("/lwc/preview/action/%s/<recordId>/%s", objectName, actionName))
			continue
		}
		if strings.Contains(actionName, ".") {
			continue
		}
		routes = append(routes, "/lwc/preview/action/global/"+actionName)
	}
	for _, route := range lwcshell.DiscoverShellRoutes(p) {
		switch route.Kind {
		case lwcshell.RenderTargetCommunityPage, lwcshell.RenderTargetFlowScreen, lwcshell.RenderTargetUtilityBar:
			if strings.TrimSpace(route.URL) == "" {
				continue
			}
			routes = append(routes, route.URL)
		}
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
  glade dev lwc [--project <root>] [--context <name>] [--open]
  glade dev lwc [--project <root>] [--target component|record-page|app-page|home-page|tab|url-addressable|record-action|global-action|utility-bar|flow-screen|flow-action] [flags]

Preview feature:
  Useful local Lightning-style preview routes. Not full hosted Lightning
  Experience, permissions, full console API, full UI API, or exact hosted
  base-component behavior.

Preview routes:
  /lwc/preview/component/c/contextProbe
  /lwc/preview/record/Account/<recordId>?page=Account_Record_Page
  /lwc/preview/app/App_Page
  /lwc/preview/home/Home_Page
  /lwc/preview/tab/Lwc_Probe
  /lwc/preview/utility/Support_Utility
  /lwc/preview/flow/Membership_Flow
  /lwc/preview/cmp/c/actionProbe?c__mode=demo
  /lwc/preview/action/Account/<recordId>/Update_Status
  /lwc/preview/action/global/Global_Status
  /lwc/preview/community/Partner_Portal/Account
  /lwc/preview/community/Partner_Portal/cmp/c/communityProbe

Context flags:
  --context <name>
  --context-file <path>
  --target component|record-page|app-page|home-page|tab|url-addressable|record-action|global-action|utility-bar|flow-screen|flow-action
  --component c:contextProbe
  --object Account
  --record 001000000000001AAA
  --page Account_Record_Page
  --tab Lwc_Probe
  --action Update_Status
  --app Sales
  --flow Membership_Flow
  --flow-input name=value
  --form-factor Large|Medium|Small
  --state key=value
  --open

Community routes open from named contexts in glade.lwc.json. Direct flags cover
component, page, tab, action, utility, and Flow route shapes.

Visualforce-backed tabs redirect to /apex/<page>, where <apex:includeLightning/>
and Lightning Out render through the same local Lightning runtime.

Examples:
  glade dev lwc --project .
  glade dev lwc --project . --context accountRecord --open
  glade dev lwc --project . --target record-page --object Account --record 001000000000001AAA --page Account_Record_Page --open
  glade dev lwc --port 8080
  glade dev lwc --addr 127.0.0.1:0 --ready-file /tmp/glade-lwc-ready.json
`)+"\n")
}
