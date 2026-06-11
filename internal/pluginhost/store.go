package pluginhost

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Store struct {
	root string
}

func DefaultRoot() string {
	if override := os.Getenv("GLADE_HOME"); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".glade"
	}
	return filepath.Join(home, ".glade")
}

func NewStore(root string) Store {
	return Store{root: root}
}

func (s Store) InstalledPath() string {
	return filepath.Join(s.root, "plugins", "installed.json")
}

func (s Store) PluginDir(name string) string {
	return filepath.Join(s.root, "plugins", name)
}

func (p InstalledPlugin) IdentityName() string {
	if p.CanonicalName != "" {
		return p.CanonicalName
	}
	return p.Name
}

func (p InstalledPlugin) StorageKey() string {
	if p.StorageName != "" {
		return p.StorageName
	}
	return p.Name
}

func (s Store) ReadInstalled() (InstalledState, error) {
	path := s.InstalledPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return InstalledState{Version: 1}, nil
		}
		return InstalledState{}, err
	}
	var state InstalledState
	if err := json.Unmarshal(data, &state); err != nil {
		return InstalledState{}, err
	}
	if state.Version == 0 {
		state.Version = 1
	}
	return state, nil
}

func (s Store) WriteInstalled(state InstalledState) error {
	if state.Version == 0 {
		state.Version = 1
	}
	path := s.InstalledPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicWriteFile(path, data, 0o644)
}

func (s Store) ListInstalled() ([]InstalledPlugin, error) {
	state, err := s.ReadInstalled()
	if err != nil {
		return nil, err
	}
	return state.Plugins, nil
}

func (s Store) Remove(name string) error {
	if _, err := ParsePluginRef(name); err != nil {
		if err := validatePluginPathToken("plugin name", name); err != nil {
			return err
		}
	}
	state, err := s.ReadInstalled()
	if err != nil {
		return err
	}
	var removed *InstalledPlugin
	next := state.Plugins[:0]
	for i := range state.Plugins {
		plugin := state.Plugins[i]
		if installedPluginMatchesName(plugin, name) {
			removed = &plugin
			continue
		}
		next = append(next, plugin)
	}
	state.Plugins = next
	if err := s.WriteInstalled(state); err != nil {
		return err
	}
	if removed != nil && !removed.Linked {
		if err := os.RemoveAll(s.PluginDir(removed.StorageKey())); err != nil {
			return err
		}
	}
	return nil
}

func installedPluginMatchesName(plugin InstalledPlugin, name string) bool {
	if plugin.Name == name || plugin.CanonicalName == name {
		return true
	}
	if canonical, ok := firstPartyAliases[name]; ok && plugin.CanonicalName == canonical {
		return true
	}
	return false
}

func replaceInstalled(plugins []InstalledPlugin, plugin InstalledPlugin) []InstalledPlugin {
	out := plugins[:0]
	for _, existing := range plugins {
		if installedPluginsConflict(existing, plugin) {
			continue
		}
		out = append(out, existing)
	}
	return append(out, plugin)
}

func installedPluginsConflict(existing, plugin InstalledPlugin) bool {
	if existing.IdentityName() == plugin.IdentityName() {
		return true
	}
	if existing.Name != "" && existing.Name == plugin.Name {
		return true
	}
	if existing.StorageKey() == plugin.StorageKey() {
		return true
	}
	if existing.CanonicalName != "" && firstPartyAliases[plugin.Name] == existing.CanonicalName {
		return true
	}
	if plugin.CanonicalName != "" && firstPartyAliases[existing.Name] == plugin.CanonicalName {
		return true
	}
	return false
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
