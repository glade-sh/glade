package project

import (
	"encoding/xml"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const sourceSettingsSuffix = ".settings-meta.xml"

// sourceSettingsSkipDirs extends the metadata-folder skip list with object
// trees, which are large and never hold org settings.
func sourceSettingsSkipDir(name string) bool {
	return personAccountEvidenceSkipDirs[name] || name == "objects"
}

// sourceSettingsFiles lists deployed org settings (settings/*.settings-meta.xml)
// under root. The scan is bounded like the Person Accounts evidence scan.
func sourceSettingsFiles(root string) []string {
	if root == "" {
		return nil
	}
	var found []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return nil
		}
		if path != root {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil || strings.Count(rel, string(os.PathSeparator)) >= personAccountEvidenceMaxDepth {
				return filepath.SkipDir
			}
		}
		if sourceSettingsSkipDir(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.Name() != "settings" {
			return nil
		}
		items, _ := os.ReadDir(path)
		for _, item := range items {
			if !item.IsDir() && strings.HasSuffix(item.Name(), sourceSettingsSuffix) {
				found = append(found, filepath.Join(path, item.Name()))
			}
		}
		return filepath.SkipDir
	})
	sort.Strings(found)
	return found
}

// sourceSettingsDefinition folds deployed settings files into the settings map
// a scratch definition would carry, so one trigger table serves both inputs.
// Settings are keyed by file name (Quote.settings-meta.xml is quoteSettings),
// which is how the metadata deploy identifies the type. Only boolean children
// are read; other values never reach the trigger table.
func sourceSettingsDefinition(root string) (scratchOrgDefinition, bool) {
	settings := map[string]any{}
	for _, path := range sourceSettingsFiles(root) {
		name := strings.TrimSuffix(filepath.Base(path), sourceSettingsSuffix)
		if name == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		values := sourceSettingsBooleans(data)
		if len(values) == 0 {
			continue
		}
		block := strings.ToLower(name[:1]) + name[1:] + "Settings"
		merged, _ := settings[block].(map[string]any)
		if merged == nil {
			merged = map[string]any{}
			settings[block] = merged
		}
		for key, value := range values {
			merged[key] = value
		}
	}
	return scratchOrgDefinition{Settings: settings}, len(settings) > 0
}

func sourceSettingsBooleans(data []byte) map[string]bool {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	values := map[string]bool{}
	depth := 0
	var current string
	var text strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				current = t.Name.Local
				text.Reset()
			}
		case xml.CharData:
			if depth == 2 {
				text.Write(t)
			}
		case xml.EndElement:
			if depth == 2 && current != "" {
				switch strings.TrimSpace(text.String()) {
				case "true":
					values[current] = true
				case "false":
					values[current] = false
				}
				current = ""
			}
			depth--
		}
	}
	return values
}
