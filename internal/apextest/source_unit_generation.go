package apextest

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/typesys"
)

// Duplicate declarations belong to the same source units used by semantic
// analysis: the nearest SFDX project and its source tree. Capture every probed
// marker, including absence, so a metadata edit cannot reuse an old binding.
func captureRuntimeSourceUnits(index typesys.Index, generation *runtimeMetadataGeneration, bodies map[string]string) (map[string]string, error) {
	counts := make(map[string]int)
	duplicate := false
	for _, typ := range index.Types {
		if typ.Dependency || !typ.HasSourceSnapshot() {
			continue
		}
		key := strings.ToLower(typ.Namespace + "." + typ.Name)
		counts[key]++
		duplicate = duplicate || counts[key] > 1
	}
	metadataNames := make(map[string]bool)
	for _, record := range index.CustomMetadataRecords {
		key := strings.ToLower(record.ObjectName + "." + record.DeveloperName)
		duplicate = duplicate || metadataNames[key]
		metadataNames[key] = true
	}
	if !duplicate {
		return nil, nil
	}
	units := make(map[string]string)
	files := make(map[string]string)
	for _, record := range index.CustomMetadataRecords {
		if strings.TrimSpace(record.File) != "" {
			files[filepath.Clean(record.File)] = filepath.Clean(index.Project.Root)
		}
	}
	for _, typ := range index.Types {
		if typ.Dependency || !typ.HasSourceSnapshot() || strings.TrimSpace(typ.File) == "" {
			continue
		}
		root := filepath.Clean(typ.SourceRoot)
		if root == "." {
			root = filepath.Clean(index.Project.Root)
		}
		files[filepath.Clean(typ.File)] = root
	}
	for file, root := range files {
		unit := "file:" + file
		for dir := filepath.Dir(file); dir != "" && dir != "."; dir = filepath.Dir(dir) {
			marker := filepath.Join(dir, "sfdx-project.json")
			input, captured := generation.inputs[marker]
			if !captured {
				data, err := os.ReadFile(marker)
				if err != nil && !os.IsNotExist(err) {
					return nil, err
				}
				input = typesys.ApexMetadataInput{Present: err == nil}
				if err == nil {
					input.Digest = sha256.Sum256(data)
					bodies[marker] = string(data)
				}
				generation.inputs[marker] = input
			}
			if input.Present {
				unit = runtimeSourceTreeKey(dir, file)
				break
			}
			if dir == root || filepath.Dir(dir) == dir {
				break
			}
		}
		units[file] = unit
	}
	return units, nil
}

func runtimeSourceTreeKey(root, file string) string {
	key := "sfdx:" + root
	rel, err := filepath.Rel(root, file)
	if err != nil || rel == "." || rel == "" {
		return key
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) == 0 || strings.Contains(parts[0], ".") {
		return key
	}
	tree := parts[0]
	if tree == "packages" && len(parts) > 1 {
		tree += string(filepath.Separator) + parts[1]
	}
	return key + ":" + tree
}

func (cache *sourceCache) runtimeSourceUnit(file string) string {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.runtimeSourceUnits[filepath.Clean(file)]
}
