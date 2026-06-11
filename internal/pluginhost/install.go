package pluginhost

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type InstallOptions struct {
	CanonicalName   string
	ExpectedVersion string
	RegistryURL     string
	Publisher       string
	Trust           string
	AssetSHA256     string
	AssetOS         string
	AssetArch       string
	Source          string
	StorageName     string
}

func (s Store) InstallArchive(ctx context.Context, archivePath string) (InstalledPlugin, error) {
	return s.InstallArchiveWithOptions(ctx, archivePath, InstallOptions{})
}

func (s Store) InstallArchiveWithOptions(ctx context.Context, archivePath string, opts InstallOptions) (InstalledPlugin, error) {
	if err := ctx.Err(); err != nil {
		return InstalledPlugin{}, err
	}
	stagingParent := filepath.Join(s.root, "plugins", ".tmp")
	if err := os.MkdirAll(stagingParent, 0o755); err != nil {
		return InstalledPlugin{}, err
	}
	extracted, err := extractArchive(archivePath, stagingParent)
	if err != nil {
		return InstalledPlugin{}, err
	}
	defer func() {
		if extracted.dir != "" {
			os.RemoveAll(extracted.dir)
		}
	}()
	checksumsPath := filepath.Join(extracted.dir, "checksums.txt")
	checksums, err := readChecksums(checksumsPath)
	if err != nil {
		return InstalledPlugin{}, err
	}
	if err := verifyArchiveChecksums(extracted.dir, extracted.files, checksums); err != nil {
		return InstalledPlugin{}, err
	}
	manifestPath := filepath.Join(extracted.dir, "plugin.json")
	manifest, err := LoadManifestFromFile(manifestPath)
	if err != nil {
		return InstalledPlugin{}, err
	}
	if opts.ExpectedVersion != "" && manifest.Version != opts.ExpectedVersion {
		return InstalledPlugin{}, fmt.Errorf("manifest version %q does not match expected version %q", manifest.Version, opts.ExpectedVersion)
	}
	exeName := "glade-plugin-" + manifest.Name
	relativeExe := filepath.ToSlash(filepath.Join("bin", exeName))
	if _, ok := checksums["plugin.json"]; !ok {
		return InstalledPlugin{}, fmt.Errorf("plugin archive missing checksum for plugin.json")
	}
	if _, ok := checksums[relativeExe]; !ok {
		return InstalledPlugin{}, fmt.Errorf("plugin archive missing checksum for %s", relativeExe)
	}
	extractedExe, ok := extracted.files[relativeExe]
	if !ok {
		return InstalledPlugin{}, fmt.Errorf("plugin archive missing executable %s", relativeExe)
	}
	if _, err := os.Stat(extractedExe.path); err != nil {
		return InstalledPlugin{}, err
	}
	if opts.CanonicalName != "" {
		ref, err := ParsePluginRef(opts.CanonicalName)
		if err != nil {
			return InstalledPlugin{}, err
		}
		if manifest.Name != ref.ManifestName() {
			return InstalledPlugin{}, fmt.Errorf("manifest name %q does not match catalog package %q", manifest.Name, ref.ManifestName())
		}
	}
	storageName := opts.StorageName
	if storageName == "" {
		storageName = manifest.Name
	}
	if err := validatePluginPathToken("plugin storage name", storageName); err != nil {
		return InstalledPlugin{}, err
	}
	targetDir := filepath.Join(s.root, "plugins", storageName, manifest.Version)
	if err := os.RemoveAll(targetDir); err != nil {
		return InstalledPlugin{}, err
	}
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return InstalledPlugin{}, err
	}
	if err := os.Rename(extracted.dir, targetDir); err != nil {
		return InstalledPlugin{}, err
	}
	extracted.dir = ""
	executable := filepath.Join(targetDir, "bin", exeName)
	if err := os.Chmod(executable, extractedExe.mode|0o100); err != nil {
		return InstalledPlugin{}, err
	}
	absArchive, err := filepath.Abs(archivePath)
	if err != nil {
		return InstalledPlugin{}, err
	}
	source := opts.Source
	if source == "" {
		source = "archive:" + absArchive
	}
	plugin := InstalledPlugin{
		Name:          manifest.Name,
		CanonicalName: opts.CanonicalName,
		StorageName:   storageName,
		Version:       manifest.Version,
		Executable:    executable,
		Manifest:      filepath.Join(targetDir, "plugin.json"),
		Source:        source,
		Linked:        false,
		Commands:      manifest.CommandRoots(),
		Registry:      opts.RegistryURL,
		Publisher:     opts.Publisher,
		Trust:         opts.Trust,
		AssetSHA256:   opts.AssetSHA256,
		AssetOS:       opts.AssetOS,
		AssetArch:     opts.AssetArch,
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
