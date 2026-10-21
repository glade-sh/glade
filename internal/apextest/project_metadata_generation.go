package apextest

import (
	"crypto/sha256"
	"os"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/typesys"
)

func (cache *sourceCache) seedProjectSnapshot(snapshot *project.Project) {
	if cache == nil || snapshot == nil {
		return
	}
	captured := *snapshot
	cache.mu.Lock()
	cache.capturedProject = &captured
	cache.mu.Unlock()
}

func (cache *sourceCache) captureProjectRuntimeMetadata(index typesys.Index, generation *runtimeMetadataGeneration) error {
	bodies := make(map[string]string)
	var capturedProject *project.Project
	if index.Project.Root != "" {
		if loaded, err := cache.capturedProjectSnapshot(index.Project.Root); err == nil {
			capturedProject = &loaded
			paths := append([]string(nil), loaded.ListViewFiles...)
			paths = append(paths, resource.LayoutMetadataFiles(loaded)...)
			paths = append(paths, loaded.FlowFiles...)
			paths = append(paths, loaded.PermissionSetGroupFiles...)
			paths = append(paths, resource.SetupRecordMetadataFiles(loaded)...)
			for _, path := range paths {
				data, err := os.ReadFile(path) // #nosec G304 -- metadata path comes from the loaded project snapshot.
				if err != nil {
					return err
				}
				bodies[path] = string(data)
				generation.inputs[path] = typesys.ApexMetadataInput{Present: true, Digest: sha256.Sum256(data)}
			}
		}
	}
	units, err := captureRuntimeSourceUnits(index, generation, bodies)
	if err != nil {
		return err
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.projectMetadataBodies = bodies
	cache.projectMetadataPrepared = true
	cache.capturedProject = capturedProject
	cache.runtimeSourceUnits = units
	if cache.capturedMetadata == nil {
		cache.capturedMetadata = make(map[string]typesys.ApexMetadataInput)
	}
	for path := range generation.inputs {
		cache.capturedMetadata[path] = generation.inputs[path]
	}
	return nil
}

func (cache *sourceCache) readCapturedProjectMetadata(path string) ([]byte, error) {
	cache.mu.RLock()
	body, exists := cache.projectMetadataBodies[path]
	prepared := cache.projectMetadataPrepared
	cache.mu.RUnlock()
	if exists {
		return []byte(body), nil
	}
	if prepared {
		return nil, incompleteSourceSnapshotError("missing captured project metadata " + path)
	}
	return os.ReadFile(path) // #nosec G304 -- indexed project metadata path for an uncaptured snapshot.
}

func (cache *sourceCache) capturedProjectSnapshot(root string) (project.Project, error) {
	cache.mu.RLock()
	captured := cache.capturedProject
	cache.mu.RUnlock()
	if captured != nil && captured.Root == root {
		return *captured, nil
	}
	return project.Load(root)
}
