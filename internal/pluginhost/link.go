package pluginhost

import (
	"context"
	"path/filepath"
)

func (s Store) LinkExecutable(ctx context.Context, executable, source string) (InstalledPlugin, error) {
	abs, err := filepath.Abs(executable)
	if err != nil {
		return InstalledPlugin{}, err
	}
	manifest, err := LoadManifestFromExecutable(ctx, abs)
	if err != nil {
		return InstalledPlugin{}, err
	}
	plugin := InstalledPlugin{
		Name:       manifest.Name,
		Version:    manifest.Version,
		Executable: abs,
		Source:     source,
		Linked:     true,
		Commands:   manifest.CommandRoots(),
	}
	state, err := s.ReadInstalled()
	if err != nil {
		return InstalledPlugin{}, err
	}
	state.Plugins = replaceInstalled(state.Plugins, plugin)
	if err := s.WriteInstalled(state); err != nil {
		return InstalledPlugin{}, err
	}
	return plugin, nil
}
