package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/pluginhost"
)

var pluginListManifestTimeout = 3 * time.Second

func runPlugins(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		_ = cliui.WriteCommandHelp(stdout, []string{"plugins"})
		return nil
	}
	switch args[0] {
	case "list":
		return runPluginsList(ctx, args[1:], stdout)
	case "available":
		return runPluginsAvailable(ctx, args[1:], stdout, stderr)
	case "search":
		return runPluginsSearch(ctx, args[1:], stdout, stderr)
	case "info":
		return runPluginsInfo(ctx, args[1:], stdout, stderr)
	case "link":
		return runPluginsLink(ctx, args[1:], stdout)
	case "install":
		return runPluginsInstall(ctx, args[1:], stdout, stderr)
	case "remove":
		return runPluginsRemove(args[1:], stdout)
	case "which":
		return runPluginsWhich(args[1:], stdout)
	case "doctor":
		return runPluginsDoctor(ctx, args[1:], stdout, stderr)
	case "lock":
		return runPluginsLock(args[1:], stdout)
	case "restore":
		return runPluginsRestore(ctx, args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown plugins command %q", args[0])
	}
}

func runPluginsList(ctx context.Context, args []string, stdout io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	jsonOut, err := parseJSONOnlyFlag("glade plugins list", args)
	if err != nil {
		return err
	}
	plugins, err := pluginhost.NewStore(pluginhost.DefaultRoot()).ListInstalled()
	if err != nil {
		return err
	}
	if jsonOut {
		return writePluginsListJSON(ctx, stdout, plugins)
	}
	if len(plugins) == 0 {
		_, err := fmt.Fprintln(stdout, "No plugins installed.")
		return err
	}
	for _, plugin := range plugins {
		link := ""
		if plugin.Linked {
			link = " linked"
		}
		fmt.Fprintf(stdout, "%s %s%s %s\n", plugin.IdentityName(), plugin.Version, link, strings.Join(plugin.Commands, ","))
	}
	return nil
}

type pluginsListJSON struct {
	Plugins []pluginListEntryJSON `json:"plugins"`
}

type pluginListEntryJSON struct {
	Identity     string                     `json:"identity"`
	Name         string                     `json:"name"`
	Canonical    string                     `json:"canonicalName,omitempty"`
	Version      string                     `json:"version"`
	Linked       bool                       `json:"linked"`
	CommandRoots []string                   `json:"commandRoots"`
	Executable   string                     `json:"executable"`
	ManifestPath string                     `json:"manifestPath"`
	Source       string                     `json:"source,omitempty"`
	Editor       *pluginhost.EditorManifest `json:"editor,omitempty"`
}

func writePluginsListJSON(ctx context.Context, w io.Writer, plugins []pluginhost.InstalledPlugin) error {
	out := pluginsListJSON{Plugins: make([]pluginListEntryJSON, 0, len(plugins))}
	for _, plugin := range plugins {
		entry := pluginListEntryJSON{
			Identity:     plugin.IdentityName(),
			Name:         plugin.Name,
			Canonical:    plugin.CanonicalName,
			Version:      plugin.Version,
			Linked:       plugin.Linked,
			CommandRoots: append([]string(nil), plugin.Commands...),
			Executable:   plugin.Executable,
			ManifestPath: plugin.Manifest,
			Source:       plugin.Source,
		}
		if manifest, ok, err := loadInstalledPluginManifest(ctx, plugin); err != nil {
			return err
		} else if ok {
			entry.Editor = manifest.Editor
		}
		out.Plugins = append(out.Plugins, entry)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func loadInstalledPluginManifest(ctx context.Context, plugin pluginhost.InstalledPlugin) (pluginhost.Manifest, bool, error) {
	if plugin.Manifest != "" {
		manifest, err := pluginhost.LoadManifestFromFile(plugin.Manifest)
		return manifest, true, err
	}
	if plugin.Linked && plugin.Executable != "" {
		manifestCtx, cancel := context.WithTimeout(ctx, pluginListManifestTimeout)
		defer cancel()
		manifest, err := pluginhost.LoadManifestFromExecutable(manifestCtx, plugin.Executable)
		return manifest, true, err
	}
	return pluginhost.Manifest{}, false, nil
}

func parseJSONOnlyFlag(usage string, args []string) (bool, error) {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json", "-j":
			jsonOut = true
		default:
			return false, fmt.Errorf("unknown %s argument %q", usage, arg)
		}
	}
	return jsonOut, nil
}

func parseJSONProgressFlags(usage string, args []string) (bool, cliui.ProgressMode, error) {
	jsonOut := false
	progress := false
	progressJSON := false
	noProgress := false
	for _, arg := range args {
		switch arg {
		case "--json", "-j":
			jsonOut = true
		case "--progress":
			progress = true
		case "--progress-json":
			progressJSON = true
		case "--no-progress", "--quiet":
			noProgress = true
		default:
			return false, cliui.ProgressOff, fmt.Errorf("unknown %s argument %q", usage, arg)
		}
	}
	return jsonOut, progressModeForFlags(jsonOut, progress, progressJSON, noProgress), nil
}

func parsePluginProgressFlags(jsonOut bool, args []string) ([]string, cliui.ProgressMode, error) {
	progress := false
	progressJSON := false
	noProgress := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--progress":
			progress = true
		case "--progress-json":
			progressJSON = true
		case "--no-progress", "--quiet":
			noProgress = true
		default:
			filtered = append(filtered, arg)
		}
	}
	return filtered, progressModeForFlags(jsonOut, progress, progressJSON, noProgress), nil
}

func runPluginsLink(ctx context.Context, args []string, stdout io.Writer) error {
	executable := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--exec":
			if i+1 >= len(args) {
				return errors.New("--exec requires a value")
			}
			executable = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown plugins link argument %q", args[i])
		}
	}
	if executable == "" {
		return errors.New("usage: glade plugins link --exec <plugin-executable>")
	}
	plugin, err := pluginhost.NewStore(pluginhost.DefaultRoot()).LinkExecutable(ctx, executable, "link:"+executable)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Linked plugin %s %s with commands %v.\n", plugin.Name, plugin.Version, plugin.Commands)
	return nil
}

type pluginsInstallOptions struct {
	target       string
	registry     string
	sha256       string
	yes          bool
	progressMode cliui.ProgressMode
}

func parsePluginsInstallArgs(args []string) (pluginsInstallOptions, error) {
	var opts pluginsInstallOptions
	progress := false
	progressJSON := false
	noProgress := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--registry":
			if i+1 >= len(args) {
				return opts, errors.New("--registry requires a value")
			}
			opts.registry = args[i+1]
			i++
		case "--sha256":
			if i+1 >= len(args) {
				return opts, errors.New("--sha256 requires a value")
			}
			opts.sha256 = args[i+1]
			i++
		case "--yes":
			opts.yes = true
		case "--progress":
			progress = true
		case "--progress-json":
			progressJSON = true
		case "--no-progress", "--quiet":
			noProgress = true
		default:
			if opts.target != "" {
				return opts, fmt.Errorf("unexpected plugins install argument %q", args[i])
			}
			opts.target = args[i]
		}
	}
	if opts.target == "" {
		return opts, errors.New("usage: glade plugins install <plugin-name-or-archive> [--registry <url>] [--sha256 <hash>] [--yes]")
	}
	opts.progressMode = progressModeForFlags(false, progress, progressJSON, noProgress)
	return opts, nil
}

func runPluginsInstall(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	opts, err := parsePluginsInstallArgs(args)
	if err != nil {
		return err
	}
	store := pluginhost.NewStore(pluginhost.DefaultRoot())
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: stderr, Mode: opts.progressMode})
	failInstall := func(err error) error {
		renderer.Finish(cliui.Result{OK: false, Label: "plugins install failed"})
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "plugins install", Label: "Resolving plugin target"})
	var (
		plugin pluginhost.InstalledPlugin
	)
	if isRemoteArchiveInstallArg(opts.target) {
		if err := enforcePluginTrustBeforeInstall(opts.target, "unlisted", opts.yes); err != nil {
			return failInstall(err)
		}
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "plugins install", Label: "Downloading archive", Current: 1, Total: 3})
		plugin, err = store.InstallRemoteArchive(ctx, opts.target, opts.sha256, pluginhost.InstallOptions{})
	} else if isArchiveInstallArg(opts.target) {
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "plugins install", Label: "Installing archive", Current: 1, Total: 3})
		plugin, err = store.InstallArchive(ctx, opts.target)
	} else {
		ref, parseErr := pluginhost.ParsePluginRef(opts.target)
		if parseErr != nil {
			return failInstall(parseErr)
		}
		registryURL := pluginhost.RegistryURL()
		if opts.registry != "" {
			registryURL = opts.registry
		}
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "plugins install", Label: "Fetching registry", Current: 1, Total: 4})
		index, fetchErr := fetchPluginRegistryForCLI(ctx, registryURL)
		if fetchErr != nil {
			return failInstall(fetchErr)
		}
		if os.Getenv("CI") != "" && !opts.yes {
			registryPlugin, _, ok := index.AssetForRef(ref, runtime.GOOS, runtime.GOARCH)
			if !ok {
				return index.NotFoundErrorForRef(ref, runtime.GOOS, runtime.GOARCH)
			}
			if err := enforcePluginTrustBeforeInstall(registryPlugin.Name, registryPlugin.Trust, opts.yes); err != nil {
				return failInstall(err)
			}
		}
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "plugins install", Label: "Installing plugin", Current: 3, Total: 4})
		plugin, err = store.InstallFromRegistryIndex(ctx, registryURL, index, ref)
	}
	if err != nil {
		return failInstall(err)
	}
	if plugin.Trust == "community" || plugin.Trust == "unlisted" {
		if os.Getenv("CI") != "" && !opts.yes {
			return failInstall(fmt.Errorf("plugin %s is %s; rerun with --yes or restore from a lock file in CI", plugin.IdentityName(), plugin.Trust))
		}
		fmt.Fprintf(stderr, "warning: plugin %s is %s; review its source before use\n", plugin.IdentityName(), plugin.Trust)
	}
	renderer.Finish(cliui.Result{OK: true, Label: "plugins install complete"})
	fmt.Fprintf(stdout, "Installed plugin %s %s with commands %v.\n", plugin.IdentityName(), plugin.Version, plugin.Commands)
	return nil
}

func enforcePluginTrustBeforeInstall(name, trust string, yes bool) error {
	if os.Getenv("CI") == "" || yes {
		return nil
	}
	if trust == "community" || trust == "unlisted" {
		return fmt.Errorf("plugin %s is %s; rerun with --yes or restore from a lock file in CI", name, trust)
	}
	return nil
}

func isRemoteArchiveInstallArg(arg string) bool {
	return strings.HasPrefix(arg, "https://") || strings.HasPrefix(arg, "http://")
}

func isArchiveInstallArg(arg string) bool {
	if strings.HasPrefix(arg, "@") {
		return false
	}
	if strings.HasSuffix(arg, ".tar.gz") {
		return true
	}
	return strings.ContainsAny(arg, `/\`)
}

func runPluginsAvailable(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	filtered, progressMode, err := parsePluginProgressFlags(false, args)
	if err != nil {
		return err
	}
	if len(filtered) != 0 {
		return errors.New("usage: glade plugins available")
	}
	return runPluginsSearchWithMode(ctx, nil, stdout, stderr, progressMode)
}

func runPluginsSearch(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	filtered, progressMode, err := parsePluginProgressFlags(false, args)
	if err != nil {
		return err
	}
	return runPluginsSearchWithMode(ctx, filtered, stdout, stderr, progressMode)
}

func runPluginsSearchWithMode(ctx context.Context, args []string, stdout, stderr io.Writer, progressMode cliui.ProgressMode) error {
	if len(args) > 1 {
		return errors.New("usage: glade plugins search [query]")
	}
	query := ""
	if len(args) == 1 {
		query = args[0]
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: stderr, Mode: progressMode})
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "plugins search", Label: "Fetching registry"})
	index, err := fetchPluginRegistryForCLI(ctx, pluginhost.RegistryURL())
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "plugins search failed"})
		return err
	}
	renderer.Finish(cliui.Result{OK: true, Label: "plugins search complete"})
	results := index.Search(query)
	if len(results) == 0 {
		if query == "" {
			fmt.Fprintln(stdout, "No plugins available.")
			return nil
		}
		fmt.Fprintf(stdout, "No plugins found for %q.\n", query)
		return nil
	}
	for _, plugin := range results {
		trust := plugin.Trust
		if trust == "" {
			trust = "community"
		}
		fmt.Fprintf(stdout, "%s %s %s %s\n", plugin.Name, plugin.Version, trust, plugin.Summary)
	}
	return nil
}

func runPluginsInfo(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	args, progressMode, err := parsePluginProgressFlags(false, args)
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return errors.New("usage: glade plugins info <name>")
	}
	ref, err := pluginhost.ParsePluginRef(args[0])
	if err != nil {
		return err
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: stderr, Mode: progressMode})
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "plugins info", Label: "Fetching registry"})
	index, err := fetchPluginRegistryForCLI(ctx, pluginhost.RegistryURL())
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "plugins info failed"})
		return err
	}
	renderer.Finish(cliui.Result{OK: true, Label: "plugins info complete"})
	plugin, _, ok := index.AssetForRef(ref, runtime.GOOS, runtime.GOARCH)
	if !ok {
		return index.NotFoundErrorForRef(ref, runtime.GOOS, runtime.GOARCH)
	}
	fmt.Fprintf(stdout, "%s %s\n", plugin.Name, plugin.Version)
	fmt.Fprintf(stdout, "trust: %s\n", plugin.Trust)
	fmt.Fprintf(stdout, "publisher: %s\n", plugin.Publisher)
	fmt.Fprintf(stdout, "summary: %s\n", plugin.Summary)
	fmt.Fprintf(stdout, "commands: %s\n", strings.Join(plugin.Commands, ", "))
	if plugin.DocsURL != "" {
		fmt.Fprintf(stdout, "docs: %s\n", plugin.DocsURL)
	}
	if plugin.SourceURL != "" {
		fmt.Fprintf(stdout, "source: %s\n", plugin.SourceURL)
	}
	return nil
}

func fetchPluginRegistryForCLI(ctx context.Context, registryURL string) (pluginhost.RegistryIndex, error) {
	index, err := pluginhost.FetchRegistry(ctx, registryURL)
	if err != nil {
		return pluginhost.RegistryIndex{}, formatPluginRegistryFetchError(registryURL, err)
	}
	return index, nil
}

func formatPluginRegistryFetchError(registryURL string, err error) error {
	if registryURL != pluginhost.DefaultRegistryURL {
		return err
	}
	return fmt.Errorf("default plugin registry is in preview and is not reachable at %s; set GLADE_PLUGIN_REGISTRY_URL for a configured registry, install a direct archive, or run `glade plugins link --exec <path>` for a local plugin; detail: %w", registryURL, err)
}

func runPluginsRemove(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: glade plugins remove <plugin-name>")
	}
	if err := pluginhost.NewStore(pluginhost.DefaultRoot()).Remove(args[0]); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Removed plugin %s.\n", args[0])
	return nil
}

func runPluginsWhich(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: glade plugins which <command>")
	}
	state, err := pluginhost.NewStore(pluginhost.DefaultRoot()).ReadInstalled()
	if err != nil {
		return err
	}
	plugin, ok := pluginhost.FindByCommandRoot(state, args[0])
	if !ok {
		return fmt.Errorf("no plugin provides command %q", args[0])
	}
	fmt.Fprintf(stdout, "%s is provided by %s %s\n", args[0], plugin.IdentityName(), plugin.Version)
	fmt.Fprintf(stdout, "executable: %s\n", plugin.Executable)
	return nil
}

func runPluginsDoctor(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	jsonOut, progressMode, err := parseJSONProgressFlags("glade plugins doctor", args)
	if err != nil {
		return err
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: stderr, Mode: progressMode})
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "plugins doctor", Label: "Checking plugins"})
	results, err := pluginhost.NewStore(pluginhost.DefaultRoot()).Doctor(ctx)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "plugins doctor failed"})
		return err
	}
	renderer.Finish(cliui.Result{OK: true, Label: "plugins doctor complete"})
	if jsonOut {
		return writePluginsDoctorJSON(stdout, results)
	}
	for _, result := range results {
		if result.OK {
			fmt.Fprintf(stdout, "%s %s ok\n", result.Plugin.IdentityName(), result.Plugin.Version)
			continue
		}
		fmt.Fprintf(stdout, "%s %s %s\n", result.Plugin.IdentityName(), result.Plugin.Version, result.Message)
	}
	return nil
}

type pluginsDoctorJSON struct {
	OK      bool                    `json:"ok"`
	Plugins []pluginDoctorEntryJSON `json:"plugins"`
}

type pluginDoctorEntryJSON struct {
	Identity     string   `json:"identity"`
	Name         string   `json:"name"`
	Canonical    string   `json:"canonicalName,omitempty"`
	Version      string   `json:"version"`
	Linked       bool     `json:"linked"`
	CommandRoots []string `json:"commandRoots"`
	Executable   string   `json:"executable"`
	ManifestPath string   `json:"manifestPath"`
	Source       string   `json:"source,omitempty"`
	OK           bool     `json:"ok"`
	Message      string   `json:"message"`
}

func writePluginsDoctorJSON(w io.Writer, results []pluginhost.DoctorResult) error {
	out := pluginsDoctorJSON{OK: true, Plugins: make([]pluginDoctorEntryJSON, 0, len(results))}
	for _, result := range results {
		if !result.OK {
			out.OK = false
		}
		plugin := result.Plugin
		out.Plugins = append(out.Plugins, pluginDoctorEntryJSON{
			Identity:     plugin.IdentityName(),
			Name:         plugin.Name,
			Canonical:    plugin.CanonicalName,
			Version:      plugin.Version,
			Linked:       plugin.Linked,
			CommandRoots: append([]string(nil), plugin.Commands...),
			Executable:   plugin.Executable,
			ManifestPath: plugin.Manifest,
			Source:       plugin.Source,
			OK:           result.OK,
			Message:      result.Message,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func runPluginsLock(args []string, stdout io.Writer) error {
	includeLinked := false
	for _, arg := range args {
		switch arg {
		case "--include-linked":
			includeLinked = true
		default:
			return fmt.Errorf("unknown plugins lock argument %q", arg)
		}
	}
	state, err := pluginhost.NewStore(pluginhost.DefaultRoot()).ReadInstalled()
	if err != nil {
		return err
	}
	path := filepath.Join(".", "glade.plugins.lock.json")
	if err := pluginhost.WriteLockFile(path, state, includeLinked); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Wrote %s.\n", path)
	return nil
}

func runPluginsRestore(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	args, progressMode, err := parsePluginProgressFlags(false, args)
	if err != nil {
		return err
	}
	if len(args) != 0 {
		return fmt.Errorf("unknown plugins restore argument %q", args[0])
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: stderr, Mode: progressMode})
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "plugins restore", Label: "Reading lock file"})
	lock, err := pluginhost.ReadLockFile(filepath.Join(".", "glade.plugins.lock.json"))
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "plugins restore failed"})
		return err
	}
	store := pluginhost.NewStore(pluginhost.DefaultRoot())
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "plugins restore", Label: "Installing locked plugins", Current: 1, Total: 2})
	if err := store.RestoreLock(ctx, lock, nil); err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "plugins restore failed"})
		return err
	}
	renderer.Finish(cliui.Result{OK: true, Label: "plugins restore complete"})
	for _, plugin := range lock.Plugins {
		fmt.Fprintf(stdout, "Restored plugin %s %s.\n", plugin.Name, plugin.Version)
	}
	return nil
}

func runInstalledPluginCommand(ctx context.Context, args []string, stdout, stderr io.Writer) (int, bool) {
	if len(args) == 0 {
		return 0, false
	}
	store := pluginhost.NewStore(pluginhost.DefaultRoot())
	state, err := store.ReadInstalled()
	if err != nil {
		fmt.Fprintf(stderr, "glade: read plugin state: %v\n", err)
		return 1, true
	}
	plugin, ok := pluginhost.FindByCommandRoot(state, args[0])
	if !ok {
		return 0, false
	}
	code, err := pluginhost.RunPlugin(ctx, plugin, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "glade: plugin %s failed: %v\n", plugin.IdentityName(), err)
		return 1, true
	}
	return code, true
}
