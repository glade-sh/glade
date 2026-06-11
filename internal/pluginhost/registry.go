package pluginhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const DefaultRegistryURL = "https://plugins.glade.sh/index.json"

func RegistryURL() string {
	if override := os.Getenv("GLADE_PLUGIN_REGISTRY_URL"); strings.TrimSpace(override) != "" {
		return override
	}
	return DefaultRegistryURL
}

func FetchRegistry(ctx context.Context, url string) (RegistryIndex, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return RegistryIndex{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return RegistryIndex{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return RegistryIndex{}, fmt.Errorf("fetch plugin registry: %s", resp.Status)
	}
	var index RegistryIndex
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return RegistryIndex{}, err
	}
	if index.Version == 0 {
		index.Version = 1
	}
	return index, nil
}

func (idx RegistryIndex) AssetFor(name, goos, goarch string) (RegistryPlugin, RegistryAsset, bool) {
	return idx.AssetForVersion(name, "", goos, goarch)
}

func (idx RegistryIndex) AssetForVersion(name, version, goos, goarch string) (RegistryPlugin, RegistryAsset, bool) {
	for _, plugin := range idx.Plugins {
		if plugin.Name != name {
			continue
		}
		if version != "" && plugin.Version != version {
			continue
		}
		for _, asset := range plugin.Assets {
			if asset.OS == goos && asset.Arch == goarch {
				return plugin, asset, true
			}
		}
	}
	return RegistryPlugin{}, RegistryAsset{}, false
}

func (idx RegistryIndex) NotFoundError(name string) error {
	return fmt.Errorf("plugin %q was not found in registry", name)
}

func (s Store) InstallFromRegistry(ctx context.Context, name string) (InstalledPlugin, error) {
	return s.InstallFromRegistryVersion(ctx, name, "")
}

func (s Store) InstallFromRegistryVersion(ctx context.Context, name, version string) (InstalledPlugin, error) {
	if err := validatePluginPathToken("plugin name", name); err != nil {
		return InstalledPlugin{}, err
	}
	if version != "" {
		if err := validatePluginPathToken("plugin version", version); err != nil {
			return InstalledPlugin{}, err
		}
	}
	index, err := FetchRegistry(ctx, RegistryURL())
	if err != nil {
		return InstalledPlugin{}, err
	}
	registryPlugin, asset, ok := index.AssetForVersion(name, version, runtime.GOOS, runtime.GOARCH)
	if !ok {
		return InstalledPlugin{}, index.NotFoundError(name)
	}
	if err := validatePluginPathToken("registry plugin name", registryPlugin.Name); err != nil {
		return InstalledPlugin{}, err
	}
	if err := validatePluginPathToken("registry plugin version", registryPlugin.Version); err != nil {
		return InstalledPlugin{}, err
	}
	archivePath := filepath.Join(s.root, "plugins", "downloads", fmt.Sprintf("%s-%s-%s-%s.tar.gz", registryPlugin.Name, registryPlugin.Version, runtime.GOOS, runtime.GOARCH))
	if err := downloadRegistryAsset(ctx, asset, archivePath); err != nil {
		return InstalledPlugin{}, err
	}
	plugin, err := s.InstallArchive(ctx, archivePath)
	if err != nil {
		return InstalledPlugin{}, err
	}
	plugin.Source = "registry:" + registryPlugin.Name
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

func downloadRegistryAsset(ctx context.Context, asset RegistryAsset, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download plugin asset: %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), resp.Body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(got, asset.SHA256) {
		return fmt.Errorf("registry asset checksum mismatch for %s", asset.URL)
	}
	return os.Rename(tmpName, path)
}
