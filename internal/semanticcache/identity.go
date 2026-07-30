package semanticcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

type capturedSourceIdentity struct {
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	File           string `json:"file"`
	SourceRoot     string `json:"sourceRoot,omitempty"`
	Namespace      string `json:"namespace,omitempty"`
	Version        string `json:"version,omitempty"`
	Dependency     bool   `json:"dependency,omitempty"`
	NamespaceRemap string `json:"namespaceRemap,omitempty"`
	SourceSHA256   string `json:"sourceSHA256"`
	MetadataExists bool   `json:"metadataExists"`
	MetadataSHA256 string `json:"metadataSHA256"`
}

// IdentityForBuild derives every semantic proof component from one immutable
// index and its matching captured build artifacts. It performs no filesystem
// reads and rejects incomplete generations.
func IdentityForBuild(index typesys.Index, artifacts *typesys.BuildArtifacts, options sema.AnalyzeOptions) (Identity, error) {
	if artifacts == nil || artifacts.Sources == nil || artifacts.SourceDigests == nil || artifacts.ApexMetadataInputs == nil {
		return Identity{}, errors.New("semantic identity requires complete build artifacts")
	}
	sources, err := capturedSourceIdentities(index, artifacts)
	if err != nil {
		return Identity{}, err
	}
	projectPayload := struct {
		Index   typesys.Index            `json:"index"`
		Sources []capturedSourceIdentity `json:"sources"`
	}{
		Index:   index,
		Sources: sources,
	}
	projectDigest, err := jsonSHA256(projectPayload)
	if err != nil {
		return Identity{}, fmt.Errorf("encode semantic project identity: %w", err)
	}
	schemaDigest, err := jsonSHA256(schema.Schema{
		Objects:               index.Objects,
		CustomMetadataRecords: index.CustomMetadataRecords,
	})
	if err != nil {
		return Identity{}, fmt.Errorf("encode semantic schema identity: %w", err)
	}
	privateProjectDigest, ok := typesys.ProjectIdentityDigest(index)
	if !ok {
		return Identity{}, errors.New("semantic identity requires captured project and dependency identity")
	}
	dependencyDigest, err := jsonSHA256(struct {
		ProjectIdentity string                   `json:"projectIdentity"`
		Dependencies    []typesys.DependencyInfo `json:"dependencies"`
	}{
		ProjectIdentity: hex.EncodeToString(privateProjectDigest[:]),
		Dependencies:    index.Dependencies,
	})
	if err != nil {
		return Identity{}, fmt.Errorf("encode semantic dependency identity: %w", err)
	}
	return Identity{
		ProjectContentSHA256: projectDigest,
		SchemaContentSHA256:  schemaDigest,
		DependencySHA256:     dependencyDigest,
		SemanticABI:          sema.SemanticABI,
		PlatformABI:          sema.PlatformABI,
		OptionsFingerprint:   sema.AnalyzeOptionsFingerprint(options),
	}, nil
}

func capturedSourceIdentities(index typesys.Index, artifacts *typesys.BuildArtifacts) ([]capturedSourceIdentity, error) {
	out := make([]capturedSourceIdentity, 0, len(index.Types)+len(index.Triggers))
	for _, typ := range index.Types {
		if !typ.HasSourceSnapshot() {
			continue
		}
		source, ok := artifacts.SourceForType(typ)
		if !ok {
			return nil, fmt.Errorf("semantic identity is missing captured source for %s", typ.File)
		}
		metadata, ok := artifacts.ApexMetadataForType(typ)
		if !ok {
			return nil, fmt.Errorf("semantic identity is missing captured metadata for %s", typ.File)
		}
		out = append(out, sourceIdentity(
			"type", typ.Name, typ.File, typ.SourceRoot, typ.Namespace, typ.Version,
			typ.Dependency, namespaceremap.Fingerprint(typ.SourceNamespaceRemaps),
			source.Digest(), metadata,
		))
	}
	for _, trigger := range index.Triggers {
		if !trigger.HasSourceSnapshot() {
			continue
		}
		source, ok := artifacts.SourceForTrigger(trigger)
		if !ok {
			return nil, fmt.Errorf("semantic identity is missing captured source for %s", trigger.File)
		}
		metadata, ok := artifacts.ApexMetadataForTrigger(trigger)
		if !ok {
			return nil, fmt.Errorf("semantic identity is missing captured metadata for %s", trigger.File)
		}
		out = append(out, sourceIdentity(
			"trigger", trigger.Name, trigger.File, trigger.SourceRoot, trigger.Namespace, trigger.Version,
			trigger.Dependency, namespaceremap.Fingerprint(trigger.SourceNamespaceRemaps),
			source.Digest(), metadata,
		))
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := out[i], out[j]
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.SourceRoot != right.SourceRoot {
			return left.SourceRoot < right.SourceRoot
		}
		if left.Namespace != right.Namespace {
			return left.Namespace < right.Namespace
		}
		return left.NamespaceRemap < right.NamespaceRemap
	})
	return out, nil
}

func sourceIdentity(kind, name, file, root, namespace, version string, dependency bool, remap string, source [sha256.Size]byte, metadata typesys.ApexMetadataInput) capturedSourceIdentity {
	return capturedSourceIdentity{
		Kind:           kind,
		Name:           name,
		File:           file,
		SourceRoot:     root,
		Namespace:      namespace,
		Version:        version,
		Dependency:     dependency,
		NamespaceRemap: remap,
		SourceSHA256:   hex.EncodeToString(source[:]),
		MetadataExists: metadata.Present,
		MetadataSHA256: hex.EncodeToString(metadata.Digest[:]),
	}
}

func jsonSHA256(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
