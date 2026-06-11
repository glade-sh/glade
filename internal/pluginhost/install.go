package pluginhost

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func (s Store) InstallArchive(ctx context.Context, archivePath string) (InstalledPlugin, error) {
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
	targetDir := filepath.Join(s.root, "plugins", manifest.Name, manifest.Version)
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
	plugin := InstalledPlugin{
		Name:       manifest.Name,
		Version:    manifest.Version,
		Executable: executable,
		Manifest:   filepath.Join(targetDir, "plugin.json"),
		Source:     "archive:" + absArchive,
		Linked:     false,
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
