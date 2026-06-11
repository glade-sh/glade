package pluginhost

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		name := plugin.Name
		if plugin.CanonicalName != "" {
			name = plugin.CanonicalName
		}
		lock.Plugins = append(lock.Plugins, LockedPlugin{
			Name:      name,
			Version:   plugin.Version,
			Registry:  plugin.Registry,
			OS:        plugin.AssetOS,
			Arch:      plugin.AssetArch,
			SHA256:    plugin.AssetSHA256,
			Trust:     plugin.Trust,
			Publisher: plugin.Publisher,
			Source:    plugin.Source,
			Commands:  append([]string(nil), plugin.Commands...),
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
	for _, plugin := range lock.Plugins {
		installer := install
		if installer == nil {
			installer = func(ctx context.Context, name, version string) (InstalledPlugin, error) {
				if strings.HasPrefix(plugin.Source, "url:") {
					return s.InstallRemoteArchive(ctx, strings.TrimPrefix(plugin.Source, "url:"), plugin.SHA256, InstallOptions{
						Trust:           plugin.Trust,
						Source:          plugin.Source,
						ExpectedVersion: plugin.Version,
					})
				}
				ref, err := ParsePluginRef(name)
				if err != nil {
					return InstalledPlugin{}, err
				}
				ref.Version = version
				registryURL := plugin.Registry
				if registryURL == "" {
					registryURL = RegistryURL()
				}
				return s.installFromRegistryURL(ctx, registryURL, ref, plugin.SHA256)
			}
		}
		installed, err := installer(ctx, plugin.Name, plugin.Version)
		if err != nil {
			return err
		}
		if plugin.Version != "" && installed.Version != plugin.Version {
			return fmt.Errorf("restored plugin %s %s version mismatch: got %s", plugin.Name, plugin.Version, installed.Version)
		}
		if plugin.SHA256 != "" && !strings.EqualFold(installed.AssetSHA256, plugin.SHA256) {
			return fmt.Errorf("restored plugin %s %s checksum mismatch", plugin.Name, plugin.Version)
		}
	}
	return nil
}
