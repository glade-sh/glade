package pluginhost

import (
	"context"
	"fmt"
	"os"
)

type DoctorResult struct {
	Plugin  InstalledPlugin
	OK      bool
	Message string
}

func (s Store) Doctor(ctx context.Context) ([]DoctorResult, error) {
	state, err := s.ReadInstalled()
	if err != nil {
		return nil, err
	}
	results := make([]DoctorResult, 0, len(state.Plugins))
	for _, plugin := range state.Plugins {
		result := DoctorResult{Plugin: plugin}
		if _, err := os.Stat(plugin.Executable); err != nil {
			result.Message = fmt.Sprintf("missing executable: %s", plugin.Executable)
			results = append(results, result)
			continue
		}
		manifest, err := LoadManifestFromExecutable(ctx, plugin.Executable)
		if err != nil {
			result.Message = err.Error()
			results = append(results, result)
			continue
		}
		if !commandsStillPresent(plugin.Commands, manifest.CommandRoots()) {
			result.Message = "installed commands no longer appear in manifest"
			results = append(results, result)
			continue
		}
		result.OK = true
		result.Message = "ok"
		results = append(results, result)
	}
	return results, nil
}

func commandsStillPresent(installed, manifest []string) bool {
	if len(installed) == 0 {
		return false
	}
	roots := map[string]struct{}{}
	for _, root := range manifest {
		roots[root] = struct{}{}
	}
	for _, root := range installed {
		if _, ok := roots[root]; ok {
			return true
		}
	}
	return false
}
