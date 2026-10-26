package resource

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

type featureParameterXML struct {
	XMLName           xml.Name
	DataflowDirection string `xml:"dataflowDirection"`
	Value             string `xml:"value"`
}

// FeatureParameterMetadataFiles includes declarations from loaded project dependencies.
func FeatureParameterMetadataFiles(p project.Project) []string {
	files := append([]string(nil), p.FeatureParameterFiles...)
	for _, dependency := range p.ManagedPackageDependencies {
		if dependency.Status != "loaded" || dependency.Project == nil {
			continue
		}
		files = append(files, FeatureParameterMetadataFiles(*dependency.Project)...)
	}
	return files
}

// LoadFeatureParameterMetadata loads declared package feature parameters into
// an org using the same captured metadata reader as the rest of the project.
func LoadFeatureParameterMetadata(org *storage.OrgState, paths []string, readFile func(string) ([]byte, error)) error {
	if org == nil || len(paths) == 0 {
		return nil
	}
	parameters, diagnostics := loadFeatureParameterMetadata(paths, readFile)
	org.Metadata.FeatureParameters = parameters
	logFeatureParameterDiagnostics(diagnostics)
	return nil
}

func loadFeatureParameterMetadata(paths []string, readFile func(string) ([]byte, error)) ([]storage.FeatureParameterMetadata, []diagnostic.Diagnostic) {
	if readFile == nil {
		readFile = os.ReadFile
	}
	ordered := append([]string(nil), paths...)
	sort.Strings(ordered)
	parameters := make([]storage.FeatureParameterMetadata, 0, len(ordered))
	diagnostics := make([]diagnostic.Diagnostic, 0)
	seen := make(map[string]struct{}, len(ordered))
	warn := func(path string, err error) {
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Warning,
			Code:     "GLADERESOURCE_FEATURE_PARAMETER",
			Message:  fmt.Sprintf("invalid feature parameter metadata; file skipped: %v", err),
			File:     path,
		})
	}
	for _, path := range ordered {
		kind, name, ok := featureParameterFileIdentity(path)
		if !ok {
			warn(path, fmt.Errorf("invalid feature parameter metadata filename"))
			continue
		}
		data, err := readFile(path)
		if err != nil {
			warn(path, err)
			continue
		}
		var metadata featureParameterXML
		if err := xml.Unmarshal(data, &metadata); err != nil {
			warn(path, fmt.Errorf("invalid feature parameter metadata XML: %w", err))
			continue
		}
		if metadata.XMLName.Local != "FeatureParameter"+kind {
			warn(path, fmt.Errorf("feature parameter metadata type does not match its filename"))
			continue
		}
		direction := strings.TrimSpace(metadata.DataflowDirection)
		if !strings.EqualFold(direction, "SubscriberToLmo") && !strings.EqualFold(direction, "LmoToSubscriber") {
			warn(path, fmt.Errorf("invalid feature parameter dataflow direction"))
			continue
		}
		parameter := storage.FeatureParameterMetadata{
			Name:              name,
			Type:              kind,
			DataflowDirection: direction,
			Value:             strings.TrimSpace(metadata.Value),
		}
		if err := validateFeatureParameterValue(parameter); err != nil {
			warn(path, fmt.Errorf("invalid feature parameter value: %w", err))
			continue
		}
		key := strings.ToLower(kind) + ":" + strings.ToLower(name)
		if _, exists := seen[key]; exists {
			warn(path, fmt.Errorf("duplicate feature parameter metadata"))
			continue
		}
		seen[key] = struct{}{}
		parameters = append(parameters, parameter)
	}
	return parameters, diagnostics
}

func logFeatureParameterDiagnostics(diagnostics []diagnostic.Diagnostic) {
	for _, item := range diagnostics {
		slog.Warn(item.Message, "code", item.Code, "file", item.File)
	}
}

func featureParameterFileIdentity(path string) (kind, name string, ok bool) {
	base := filepath.Base(path)
	lower := strings.ToLower(base)
	switch {
	case strings.HasSuffix(lower, ".featureparameterboolean-meta.xml"):
		kind = "Boolean"
	case strings.HasSuffix(lower, ".featureparameterinteger-meta.xml"):
		kind = "Integer"
	case strings.HasSuffix(lower, ".featureparameterdate-meta.xml"):
		kind = "Date"
	default:
		return "", "", false
	}
	suffix := ".featureparameter" + strings.ToLower(kind) + "-meta.xml"
	name = strings.TrimSpace(base[:len(base)-len(suffix)])
	return kind, name, name != ""
}

func validateFeatureParameterValue(parameter storage.FeatureParameterMetadata) error {
	raw := strings.TrimSpace(parameter.Value)
	switch parameter.Type {
	case "Boolean":
		_, err := strconv.ParseBool(raw)
		return err
	case "Integer":
		_, err := strconv.ParseInt(raw, 10, 64)
		return err
	case "Date":
		_, err := time.Parse("2006-01-02", raw)
		return err
	default:
		return fmt.Errorf("unsupported feature parameter type")
	}
}
