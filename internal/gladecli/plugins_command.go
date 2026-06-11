package gladecli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/glade-sh/glade/internal/pluginhost"
)

func runPlugins(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		printPluginsHelp(stdout)
		return nil
	}
	switch args[0] {
	case "list":
		return runPluginsList(ctx, stdout)
	case "search":
		return runPluginsSearch(ctx, args[1:], stdout)
	case "info":
		return runPluginsInfo(ctx, args[1:], stdout)
	case "link":
		return runPluginsLink(ctx, args[1:], stdout)
	case "install":
		return runPluginsInstall(ctx, args[1:], stdout, stderr)
	case "remove":
		return runPluginsRemove(args[1:], stdout)
	case "which":
		return runPluginsWhich(args[1:], stdout)
	case "doctor":
		return runPluginsDoctor(ctx, stdout)
	case "lock":
		return runPluginsLock(args[1:], stdout)
	case "restore":
		return runPluginsRestore(ctx, stdout)
	default:
		return fmt.Errorf("unknown plugins command %q", args[0])
	}
}

func printPluginsHelp(w io.Writer) {
	fmt.Fprint(w, `Manage Glade plugins.

Usage:
  glade plugins <command> [flags]

Commands:
  list              List installed plugins.
  search            Search the plugin marketplace.
  info              Show marketplace plugin metadata.
  link              Link a local plugin executable.
  install           Install a plugin from the marketplace, registry, URL, or archive.
  remove            Remove an installed plugin.
  doctor            Check installed plugins.
  which             Show the plugin that owns a command.
  lock              Write glade.plugins.lock.json.
  restore           Restore plugins from glade.plugins.lock.json.

Examples:
  glade plugins install @glade/compat
  glade plugins install @glade/performance
  glade plugins search quality
`)
}

func runPluginsList(ctx context.Context, stdout io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	plugins, err := pluginhost.NewStore(pluginhost.DefaultRoot()).ListInstalled()
	if err != nil {
		return err
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
		fmt.Fprintf(stdout, "%s %s%s %s\n", plugin.Name, plugin.Version, link, strings.Join(plugin.Commands, ","))
	}
	return nil
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
	target   string
	registry string
	sha256   string
	yes      bool
}

func parsePluginsInstallArgs(args []string) (pluginsInstallOptions, error) {
	var opts pluginsInstallOptions
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
	return opts, nil
}

func runPluginsInstall(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	opts, err := parsePluginsInstallArgs(args)
	if err != nil {
		return err
	}
	store := pluginhost.NewStore(pluginhost.DefaultRoot())
	var (
		plugin pluginhost.InstalledPlugin
	)
	if isRemoteArchiveInstallArg(opts.target) {
		if err := enforcePluginTrustBeforeInstall(opts.target, "unlisted", opts.yes); err != nil {
			return err
		}
		plugin, err = store.InstallRemoteArchive(ctx, opts.target, opts.sha256, pluginhost.InstallOptions{})
	} else if isArchiveInstallArg(opts.target) {
		plugin, err = store.InstallArchive(ctx, opts.target)
	} else {
		ref, parseErr := pluginhost.ParsePluginRef(opts.target)
		if parseErr != nil {
			return parseErr
		}
		registryURL := pluginhost.RegistryURL()
		if opts.registry != "" {
			registryURL = opts.registry
		}
		if os.Getenv("CI") != "" && !opts.yes {
			index, fetchErr := pluginhost.FetchRegistry(ctx, registryURL)
			if fetchErr != nil {
				return fetchErr
			}
			registryPlugin, _, ok := index.AssetForRef(ref, runtime.GOOS, runtime.GOARCH)
			if !ok {
				return index.NotFoundErrorForRef(ref, runtime.GOOS, runtime.GOARCH)
			}
			if err := enforcePluginTrustBeforeInstall(registryPlugin.Name, registryPlugin.Trust, opts.yes); err != nil {
				return err
			}
		}
		plugin, err = store.InstallFromRegistryURL(ctx, registryURL, ref)
	}
	if err != nil {
		return err
	}
	if plugin.Trust == "community" || plugin.Trust == "unlisted" {
		if os.Getenv("CI") != "" && !opts.yes {
			return fmt.Errorf("plugin %s is %s; rerun with --yes or restore from a lock file in CI", plugin.IdentityName(), plugin.Trust)
		}
		fmt.Fprintf(stderr, "warning: plugin %s is %s; review its source before use\n", plugin.IdentityName(), plugin.Trust)
	}
	fmt.Fprintf(stdout, "Installed plugin %s %s with commands %v.\n", plugin.Name, plugin.Version, plugin.Commands)
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

func runPluginsSearch(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: glade plugins search <query>")
	}
	index, err := pluginhost.FetchRegistry(ctx, pluginhost.RegistryURL())
	if err != nil {
		return err
	}
	results := index.Search(args[0])
	if len(results) == 0 {
		fmt.Fprintf(stdout, "No plugins found for %q.\n", args[0])
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

func runPluginsInfo(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: glade plugins info <name>")
	}
	ref, err := pluginhost.ParsePluginRef(args[0])
	if err != nil {
		return err
	}
	index, err := pluginhost.FetchRegistry(ctx, pluginhost.RegistryURL())
	if err != nil {
		return err
	}
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
	fmt.Fprintf(stdout, "%s is provided by %s %s\n", args[0], plugin.Name, plugin.Version)
	fmt.Fprintf(stdout, "executable: %s\n", plugin.Executable)
	return nil
}

func runPluginsDoctor(ctx context.Context, stdout io.Writer) error {
	results, err := pluginhost.NewStore(pluginhost.DefaultRoot()).Doctor(ctx)
	if err != nil {
		return err
	}
	for _, result := range results {
		if result.OK {
			fmt.Fprintf(stdout, "%s %s ok\n", result.Plugin.Name, result.Plugin.Version)
			continue
		}
		fmt.Fprintf(stdout, "%s %s %s\n", result.Plugin.Name, result.Plugin.Version, result.Message)
	}
	return nil
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

func runPluginsRestore(ctx context.Context, stdout io.Writer) error {
	lock, err := pluginhost.ReadLockFile(filepath.Join(".", "glade.plugins.lock.json"))
	if err != nil {
		return err
	}
	store := pluginhost.NewStore(pluginhost.DefaultRoot())
	if err := store.RestoreLock(ctx, lock, nil); err != nil {
		return err
	}
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
		fmt.Fprintf(stderr, "glade: plugin %s failed: %v\n", plugin.Name, err)
		return 1, true
	}
	return code, true
}
