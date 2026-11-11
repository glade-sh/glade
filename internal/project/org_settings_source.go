package project

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

const sourceSettingsSuffix = ".settings-meta.xml"

// sourceSettingsFiles lists deployed org settings (settings/*.settings-meta.xml)
// under root. The scan is bounded like the Person Accounts evidence scan and
// shares its directory walk (orgShapeEvidenceScan).
func sourceSettingsFiles(root string) []string {
	found, _ := orgShapeEvidenceScan(root)
	return found
}

// sourceSettingsFilesIn lists the settings files directly in one settings
// directory.
func sourceSettingsFilesIn(dir string) []string {
	var found []string
	items, _ := os.ReadDir(dir)
	for _, item := range items {
		if !item.IsDir() && strings.HasSuffix(item.Name(), sourceSettingsSuffix) {
			found = append(found, filepath.Join(dir, item.Name()))
		}
	}
	return found
}

// sourceSettingsDefinition folds deployed settings files into the settings map
// a scratch definition would carry, so one trigger table serves both inputs.
// Settings are keyed by file name (Quote.settings-meta.xml is quoteSettings),
// which is how the metadata deploy identifies the type. Only boolean children
// are read; other values never reach the trigger table.
func sourceSettingsDefinition(root string) (scratchOrgDefinition, bool) {
	return sourceSettingsDefinitionFromFiles(sourceSettingsFiles(root))
}

func sourceSettingsDefinitionFromFiles(files []string) (scratchOrgDefinition, bool) {
	settings := map[string]any{}
	for _, path := range files {
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
