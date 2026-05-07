package packageartifact

import (
	"encoding/json"
	"os"
	"time"

	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/typesys"
)

type Artifact struct {
	Namespace             string                        `json:"namespace"`
	PackageName           string                        `json:"packageName,omitempty"`
	Version               string                        `json:"version,omitempty"`
	SourceRoot            string                        `json:"sourceRoot,omitempty"`
	SourceAPIVersion      string                        `json:"sourceApiVersion,omitempty"`
	BuiltAt               time.Time                     `json:"builtAt"`
	ApexTypes             []typesys.TypeSymbol          `json:"apexTypes,omitempty"`
	Objects               []schema.Object               `json:"objects,omitempty"`
	CustomMetadataRecords []schema.CustomMetadataRecord `json:"customMetadataRecords,omitempty"`
	Labels                int                           `json:"labels"`
	StaticResources       int                           `json:"staticResources"`
}

type Summary struct {
	Namespace       string `json:"namespace"`
	SourceRoot      string `json:"sourceRoot"`
	Version         string `json:"version,omitempty"`
	Status          string `json:"status"`
	ApexTypes       int    `json:"apexTypes"`
	Objects         int    `json:"objects"`
	Labels          int    `json:"labels"`
	StaticResources int    `json:"staticResources"`
}

func Build(namespace, version string, p project.Project, s schema.Schema, idx typesys.Index) Artifact {
	return Artifact{
		Namespace:             namespace,
		Version:               version,
		SourceRoot:            p.Root,
		SourceAPIVersion:      p.SourceAPIVersion,
		BuiltAt:               time.Now().UTC(),
		ApexTypes:             nonTestTypes(idx.Types),
		Objects:               s.Objects,
		CustomMetadataRecords: s.CustomMetadataRecords,
		Labels:                len(p.LabelFiles),
		StaticResources:       len(p.StaticResourceFiles) + len(p.StaticResourceMetas),
	}
}

func Summarize(dep project.ManagedPackageDependency, artifact Artifact) Summary {
	return Summary{
		Namespace:       dep.Namespace,
		SourceRoot:      dep.SourceRoot,
		Version:         dep.Version,
		Status:          dep.Status,
		ApexTypes:       len(artifact.ApexTypes),
		Objects:         len(artifact.Objects),
		Labels:          artifact.Labels,
		StaticResources: artifact.StaticResources,
	}
}

func WriteJSON(path string, artifact Artifact) error {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func nonTestTypes(types []typesys.TypeSymbol) []typesys.TypeSymbol {
	out := make([]typesys.TypeSymbol, 0, len(types))
	for _, typ := range types {
		if typ.IsTest {
			continue
		}
		out = append(out, typ)
	}
	return out
}
