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
	ref, err := ParsePluginRef(name)
	if err != nil {
		return RegistryPlugin{}, RegistryAsset{}, false
	}
	ref.Version = version
	return idx.AssetForRef(ref, goos, goarch)
}

func (idx RegistryIndex) AssetForRef(ref PluginRef, goos, goarch string) (RegistryPlugin, RegistryAsset, bool) {
	for _, plugin := range idx.Plugins {
		if !registryPluginMatchesRef(plugin, ref) {
			continue
		}
		if ref.Version != "" && plugin.Version != ref.Version {
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

func (idx RegistryIndex) Search(query string) []RegistryPlugin {
	q := strings.ToLower(strings.TrimSpace(query))
	var out []RegistryPlugin
	for _, plugin := range idx.Plugins {
		fields := []string{
			plugin.Name, plugin.Version, plugin.Publisher, plugin.Trust, plugin.Summary, plugin.DocsURL, plugin.SourceURL,
		}
		fields = append(fields, plugin.Aliases...)
		fields = append(fields, plugin.Commands...)
		haystack := strings.ToLower(strings.Join(fields, " "))
		if q == "" || strings.Contains(haystack, q) {
			out = append(out, plugin)
		}
	}
	return out
}

func registryPluginMatchesRef(plugin RegistryPlugin, ref PluginRef) bool {
	if plugin.Name == ref.Name || plugin.Name == ref.ManifestName() {
		return true
	}
	for _, alias := range plugin.Aliases {
		if alias == ref.Name || alias == ref.ManifestName() || firstPartyAliases[alias] == ref.Name {
			return true
		}
	}
	return false
}

func (idx RegistryIndex) NotFoundError(name string) error {
	ref, err := ParsePluginRef(name)
	if err != nil {
		return fmt.Errorf("plugin %q was not found in registry", name)
	}
	return idx.NotFoundErrorForRef(ref, runtime.GOOS, runtime.GOARCH)
}

func (idx RegistryIndex) NotFoundErrorForRef(ref PluginRef, goos, goarch string) error {
	var platforms []string
	for _, plugin := range idx.Plugins {
		if !registryPluginMatchesRef(plugin, ref) {
			continue
		}
		for _, asset := range plugin.Assets {
			platforms = append(platforms, asset.OS+"/"+asset.Arch)
		}
	}
	if len(platforms) > 0 {
		return fmt.Errorf("plugin %q has no asset for %s/%s; available platforms: %s", ref.Name, goos, goarch, strings.Join(platforms, ", "))
	}
	return fmt.Errorf("plugin %q was not found in registry; run `glade plugins search %s`", ref.Name, ref.ManifestName())
}

func (s Store) InstallFromRegistry(ctx context.Context, name string) (InstalledPlugin, error) {
	return s.InstallFromRegistryVersion(ctx, name, "")
}

func (s Store) InstallFromRegistryVersion(ctx context.Context, name, version string) (InstalledPlugin, error) {
	ref, err := ParsePluginRef(name)
	if err != nil {
		return InstalledPlugin{}, err
	}
	if version != "" {
		if err := validatePluginPathToken("plugin version", version); err != nil {
			return InstalledPlugin{}, err
		}
		ref.Version = version
	}
	return s.InstallFromRegistryURL(ctx, RegistryURL(), ref)
}

func (s Store) InstallFromRegistryURL(ctx context.Context, registryURL string, ref PluginRef) (InstalledPlugin, error) {
	return s.installFromRegistryURL(ctx, registryURL, ref, "")
}

func (s Store) installFromRegistryURL(ctx context.Context, registryURL string, ref PluginRef, expectedSHA256 string) (InstalledPlugin, error) {
	index, err := FetchRegistry(ctx, registryURL)
	if err != nil {
		return InstalledPlugin{}, err
	}
	registryPlugin, asset, ok := index.AssetForRef(ref, runtime.GOOS, runtime.GOARCH)
	if !ok {
		return InstalledPlugin{}, index.NotFoundErrorForRef(ref, runtime.GOOS, runtime.GOARCH)
	}
	catalogRef, err := ParsePluginRef(registryPlugin.Name)
	if err != nil {
		return InstalledPlugin{}, err
	}
	if registryPlugin.Version == "" {
		return InstalledPlugin{}, fmt.Errorf("registry plugin %q is missing version", registryPlugin.Name)
	}
	if err := validatePluginPathToken("registry plugin version", registryPlugin.Version); err != nil {
		return InstalledPlugin{}, err
	}
	expectedSHA256 = strings.ToLower(strings.TrimSpace(expectedSHA256))
	if expectedSHA256 != "" {
		if len(expectedSHA256) != sha256.Size*2 {
			return InstalledPlugin{}, fmt.Errorf("locked checksum for %s %s must be %d hex characters", registryPlugin.Name, registryPlugin.Version, sha256.Size*2)
		}
		if _, err := hex.DecodeString(expectedSHA256); err != nil {
			return InstalledPlugin{}, fmt.Errorf("locked checksum for %s %s must be hex", registryPlugin.Name, registryPlugin.Version)
		}
		if !strings.EqualFold(asset.SHA256, expectedSHA256) {
			return InstalledPlugin{}, fmt.Errorf("registry asset checksum for %s %s does not match lock", registryPlugin.Name, registryPlugin.Version)
		}
	}
	archivePath := filepath.Join(s.root, "plugins", "downloads", fmt.Sprintf("%s-%s-%s-%s.tar.gz", catalogRef.StorageName(), registryPlugin.Version, runtime.GOOS, runtime.GOARCH))
	if err := downloadRegistryAsset(ctx, asset, archivePath); err != nil {
		return InstalledPlugin{}, err
	}
	plugin, err := s.InstallArchiveWithOptions(ctx, archivePath, InstallOptions{
		CanonicalName:   registryPlugin.Name,
		ExpectedVersion: registryPlugin.Version,
		RegistryURL:     registryURL,
		Publisher:       registryPlugin.Publisher,
		Trust:           registryPlugin.Trust,
		AssetSHA256:     strings.ToLower(asset.SHA256),
		AssetOS:         asset.OS,
		AssetArch:       asset.Arch,
		Source:          "registry:" + registryPlugin.Name,
		StorageName:     catalogRef.StorageName(),
	})
	if err != nil {
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
