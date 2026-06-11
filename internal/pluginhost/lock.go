package pluginhost

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

const LockFileName = "glade.plugins.lock.json"

func ReadLockFile(path string) (PluginLock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PluginLock{}, err
	}
	var lock PluginLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return PluginLock{}, err
	}
	if lock.Version == 0 {
		lock.Version = 1
	}
	return lock, nil
}

func WriteLockFile(path string, state InstalledState, includeLinked bool) error {
	lock := PluginLock{Version: 1}
	for _, plugin := range state.Plugins {
		if plugin.Linked && !includeLinked {
			continue
		}
		lock.Plugins = append(lock.Plugins, LockedPlugin{
			Name:     plugin.Name,
			Version:  plugin.Version,
			Source:   plugin.Source,
			Commands: append([]string(nil), plugin.Commands...),
		})
	}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicWriteFile(path, data, 0o644)
}

func DefaultLockPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, LockFileName), nil
}

type LockInstaller func(ctx context.Context, name, version string) (InstalledPlugin, error)

func (s Store) RestoreLock(ctx context.Context, lock PluginLock, install LockInstaller) error {
	if install == nil {
		install = s.InstallFromRegistryVersion
	}
	for _, plugin := range lock.Plugins {
		if _, err := install(ctx, plugin.Name, plugin.Version); err != nil {
			return err
		}
	}
	return nil
}
