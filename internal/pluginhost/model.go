package pluginhost

import (
	"errors"
	"fmt"
	"strings"
)

const APIVersion = "glade.plugin.v1"

var coreCommandRoots = map[string]struct{}{
	"version": {}, "completion": {}, "doctor": {}, "config": {}, "init": {},
	"parse": {}, "inspect": {}, "schema": {}, "check": {}, "exec": {},
	"test": {}, "dev": {}, "report": {}, "lsp": {}, "profile": {},
	"debug": {}, "editor": {}, "dap": {}, "package": {}, "server": {},
	"playground": {}, "db": {}, "plugins": {}, "help": {},
}

type Manifest struct {
	APIVersion          string            `json:"apiVersion"`
	Name                string            `json:"name"`
	Version             string            `json:"version"`
	Summary             string            `json:"summary,omitempty"`
	Commands            []CommandManifest `json:"commands"`
	MinimumGladeVersion string            `json:"minimumGladeVersion,omitempty"`
	Source              string            `json:"source,omitempty"`
}

type CommandManifest struct {
	Path    []string `json:"path"`
	Summary string   `json:"summary,omitempty"`
}

type InstalledState struct {
	Version int               `json:"version"`
	Plugins []InstalledPlugin `json:"plugins"`
}

type InstalledPlugin struct {
	Name          string   `json:"name"`
	CanonicalName string   `json:"canonicalName,omitempty"`
	StorageName   string   `json:"storageName,omitempty"`
	Version       string   `json:"version"`
	Executable    string   `json:"executable"`
	Manifest      string   `json:"manifest"`
	Source        string   `json:"source,omitempty"`
	Linked        bool     `json:"linked"`
	Commands      []string `json:"commands"`
	Registry      string   `json:"registry,omitempty"`
	Publisher     string   `json:"publisher,omitempty"`
	Trust         string   `json:"trust,omitempty"`
	AssetSHA256   string   `json:"assetSha256,omitempty"`
	AssetOS       string   `json:"assetOS,omitempty"`
	AssetArch     string   `json:"assetArch,omitempty"`
}

type RegistryIndex struct {
	Version int              `json:"version"`
	Plugins []RegistryPlugin `json:"plugins"`
}

type RegistryPlugin struct {
	Name                string          `json:"name"`
	Aliases             []string        `json:"aliases,omitempty"`
	Version             string          `json:"version"`
	Publisher           string          `json:"publisher,omitempty"`
	Trust               string          `json:"trust,omitempty"`
	Summary             string          `json:"summary,omitempty"`
	Commands            []string        `json:"commands,omitempty"`
	DocsURL             string          `json:"docsURL,omitempty"`
	SourceURL           string          `json:"sourceURL,omitempty"`
	MinimumGladeVersion string          `json:"minimumGladeVersion,omitempty"`
	Assets              []RegistryAsset `json:"assets"`
}

type RegistryAsset struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type PluginLock struct {
	Version int            `json:"version"`
	Plugins []LockedPlugin `json:"plugins"`
}

type LockedPlugin struct {
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Registry  string   `json:"registry,omitempty"`
	OS        string   `json:"os,omitempty"`
	Arch      string   `json:"arch,omitempty"`
	SHA256    string   `json:"sha256,omitempty"`
	Trust     string   `json:"trust,omitempty"`
	Publisher string   `json:"publisher,omitempty"`
	Source    string   `json:"source,omitempty"`
	Commands  []string `json:"commands,omitempty"`
}

func (m Manifest) Validate() error {
	if m.APIVersion != APIVersion {
		return fmt.Errorf("unsupported plugin api version %q", m.APIVersion)
	}
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("plugin name is required")
	}
	if err := validatePluginPathToken("plugin name", m.Name); err != nil {
		return err
	}
	if strings.TrimSpace(m.Version) == "" {
		return errors.New("plugin version is required")
	}
	if err := validatePluginPathToken("plugin version", m.Version); err != nil {
		return err
	}
	if len(m.Commands) == 0 {
		return errors.New("plugin must declare at least one command")
	}
	seen := map[string]struct{}{}
	for _, command := range m.Commands {
		if len(command.Path) == 0 || strings.TrimSpace(command.Path[0]) == "" {
			return errors.New("plugin command root is required")
		}
		for _, part := range command.Path {
			if strings.TrimSpace(part) == "" {
				return errors.New("plugin command path segment is required")
			}
			if err := validatePluginPathToken("plugin command path segment", part); err != nil {
				return err
			}
		}
		root := command.Path[0]
		if _, exists := coreCommandRoots[root]; exists {
			return fmt.Errorf("plugin command %q conflicts with a core glade command", root)
		}
		if _, exists := seen[root]; exists {
			continue
		}
		seen[root] = struct{}{}
	}
	return nil
}

func (m Manifest) CommandRoots() []string {
	seen := map[string]struct{}{}
	var roots []string
	for _, command := range m.Commands {
		if len(command.Path) == 0 {
			continue
		}
		root := strings.TrimSpace(command.Path[0])
		if root == "" {
			continue
		}
		if _, exists := seen[root]; exists {
			continue
		}
		seen[root] = struct{}{}
		roots = append(roots, root)
	}
	return roots
}

func validatePluginPathToken(field, value string) error {
	if strings.TrimSpace(value) != value || value == "" {
		return fmt.Errorf("%s must be a safe path token", field)
	}
	if value == "." || value == ".." || strings.Contains(value, "..") || strings.ContainsAny(value, `/\`) {
		return fmt.Errorf("%s must be a safe path token", field)
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return fmt.Errorf("%s must be a safe path token", field)
		}
	}
	return nil
}
