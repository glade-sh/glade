package gladecli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/pluginhost"
)

func runPlugins(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		printPluginsHelp(stdout)
		return nil
	}
	switch args[0] {
	case "list":
		return runPluginsList(ctx, stdout)
	case "link":
		return runPluginsLink(ctx, args[1:], stdout)
	case "install":
		return runPluginsInstall(ctx, args[1:], stdout)
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
  link              Link a local plugin executable.
  install           Install a plugin from the registry or archive.
  remove            Remove an installed plugin.
  doctor            Check installed plugins.
  which             Show the plugin that owns a command.
  lock              Write glade.plugins.lock.json.
  restore           Restore plugins from glade.plugins.lock.json.
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

func runPluginsInstall(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: glade plugins install <plugin-name-or-archive>")
	}
	store := pluginhost.NewStore(pluginhost.DefaultRoot())
	var (
		plugin pluginhost.InstalledPlugin
		err    error
	)
	if isArchiveInstallArg(args[0]) {
		plugin, err = store.InstallArchive(ctx, args[0])
	} else {
		plugin, err = store.InstallFromRegistry(ctx, args[0])
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Installed plugin %s %s with commands %v.\n", plugin.Name, plugin.Version, plugin.Commands)
	return nil
}

func isArchiveInstallArg(arg string) bool {
	if strings.HasSuffix(arg, ".tar.gz") {
		return true
	}
	return strings.ContainsAny(arg, `/\`)
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
	for _, plugin := range lock.Plugins {
		if _, err := pluginhost.NewStore(pluginhost.DefaultRoot()).InstallFromRegistryVersion(ctx, plugin.Name, plugin.Version); err != nil {
			return err
		}
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
