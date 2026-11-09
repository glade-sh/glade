package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func scratchOrgConfigFiles(root string) []string {
	paths := []string{
		filepath.Join(root, "config", "project-scratch-def.json"),
		filepath.Join(root, "config", "hc-project-scratch-def.json"),
	}
	paths = append(paths, packageDefinitionFiles(root)...)
	return dedupeStrings(append(paths, cumulusCIOrgConfigFiles(root)...))
}

// packageDefinitionFiles returns the scratch definitions that
// sfdx-project.json packageDirectories name with definitionFile. Paths resolve
// against the project root and must stay inside it.
func packageDefinitionFiles(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, "sfdx-project.json"))
	if err != nil {
		return nil
	}
	var manifest struct {
		PackageDirectories []struct {
			DefinitionFile string `json:"definitionFile"`
		} `json:"packageDirectories"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return nil
	}
	cleanRoot := filepath.Clean(root)
	var paths []string
	for _, dir := range manifest.PackageDirectories {
		name := strings.TrimSpace(dir.DefinitionFile)
		if name == "" || filepath.IsAbs(name) {
			continue
		}
		full := filepath.Clean(filepath.Join(root, filepath.FromSlash(name)))
		if !strings.HasPrefix(full, cleanRoot+string(os.PathSeparator)) || !PathInsideRoot(root, full) {
			continue
		}
		paths = append(paths, full)
	}
	return paths
}

// ScratchDefinitionWarnings reports features and settings the local shape loader does not
// read. It preserves unknown settings and never includes their values.
func ScratchDefinitionWarnings(root string) []string {
	readKeys := make(map[string]bool)
	for _, trigger := range scratchSettingTriggers {
		readKeys[trigger.block+"."+trigger.key] = true
		if trigger.foldCase {
			readKeys[strings.ToLower(trigger.block+"."+trigger.key)] = true
		}
	}
	var warnings []string
	for _, path := range scratchOrgConfigFiles(root) {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			relative = path
		}
		source := filepath.ToSlash(relative)
		var def scratchOrgDefinition
		if err != nil || json.Unmarshal(data, &def) != nil {
			warnings = append(warnings, fmt.Sprintf("%s: cannot read scratch definition; org features and settings from this file are ignored", source))
			continue
		}
		var unread []string
		seenFeatures := make(map[string]bool)
		for _, raw := range def.Features {
			feature := strings.TrimSpace(raw)
			lowerKey := strings.ToLower(feature)
			if feature == "" || isMappedScratchFeature(feature) || seenFeatures[lowerKey] {
				continue
			}
			seenFeatures[lowerKey] = true
			unread = append(unread, "features."+feature)
		}
		for block, value := range def.Settings {
			settings, ok := value.(map[string]any)
			if !ok || len(settings) == 0 {
				unread = append(unread, "settings."+block)
				continue
			}
			for key := range settings {
				if !readKeys[block+"."+key] && !readKeys[strings.ToLower(block+"."+key)] {
					unread = append(unread, "settings."+block+"."+key)
				}
			}
		}
		sort.Strings(unread)
		for _, key := range unread {
			warnings = append(warnings, fmt.Sprintf("%s: %s is not read by Glade", source, key))
		}
	}
	return warnings
}
