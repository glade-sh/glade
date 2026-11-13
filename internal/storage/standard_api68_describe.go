package storage

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/project"
)

//go:embed standard_api68_describe.json.gz
var standardAPI68DescribeData []byte

// The new-object name set is generated from the sidecar so the membership
// test below never decodes the whole bundle (2.97 MB of JSON per process).
//
//go:generate python3 ../../scripts/generate-standard-api68-new-objects.py

type standardAPI68Field struct {
	Field      standardDescribeField `json:"field"`
	Anonymous  bool                  `json:"anonymous"`
	Named      bool                  `json:"named"`
	AccessRule string                `json:"access_rule"`
}

type standardAPI68Object struct {
	Status          int                                `json:"status"`
	Object          standardDescribeObject             `json:"object"`
	Fields          []standardDescribeField            `json:"fields"`
	Anonymous       bool                               `json:"anonymous"`
	Named           bool                               `json:"named"`
	FieldVisibility map[string]standardAPI68Visibility `json:"field_visibility"`
}

type standardAPI68Visibility struct {
	Anonymous bool `json:"anonymous"`
	Named     bool `json:"named"`
}

type standardAPI68Profile struct {
	Features []string                                 `json:"features"`
	Fields   map[string]map[string]standardAPI68Field `json:"fields"`
	Objects  map[string]standardAPI68Object           `json:"objects"`
}

type standardAPI68Bundle struct {
	Version  int                             `json:"version"`
	Profiles map[string]standardAPI68Profile `json:"profiles"`
}

var standardAPI68Once sync.Once
var standardAPI68Profiles map[string]standardAPI68Profile

var standardAPI68ProfileSignatures map[string]string

var standardAPI68SignatureCache struct {
	once       sync.Once
	signatures map[string]string
}

// standardAPI68GeneratedSignatures maps each captured profile's canonical
// feature signature to its name from the generated feature lists, so the
// profile lookup never decodes the describe bundle.
func standardAPI68GeneratedSignatures() map[string]string {
	standardAPI68SignatureCache.once.Do(func() {
		signatures := make(map[string]string, len(standardAPI68ProfileFeatures))
		for name, features := range standardAPI68ProfileFeatures {
			signature := canonicalFeatureSignature(features)
			if _, exists := signatures[signature]; exists {
				panic("duplicate API68 profile signature")
			}
			signatures[signature] = name
		}
		standardAPI68SignatureCache.signatures = signatures
	})
	return standardAPI68SignatureCache.signatures
}

// StandardAPI68DescribeProfile selects only an exactly captured shape. A
// missing edition or an unobserved combination has no inferred profile.
// Plain Developer markers describe derived shapes, not additional org features.
func StandardAPI68DescribeProfile(features []string) (string, bool) {
	shape := make([]string, 0, len(features))
	for _, feature := range features {
		if feature != project.PlainDeveloperObjectShape && feature != project.PlainDeveloperDescribeShape {
			shape = append(shape, feature)
		}
	}
	profile, ok := standardAPI68GeneratedSignatures()[canonicalFeatureSignature(shape)]
	return profile, ok
}

func loadStandardAPI68Profiles() map[string]standardAPI68Profile {
	standardAPI68Once.Do(func() {
		stream, err := gzip.NewReader(bytes.NewReader(standardAPI68DescribeData))
		if err != nil {
			panic(err)
		}
		defer stream.Close()
		var bundle standardAPI68Bundle
		if err := json.NewDecoder(stream).Decode(&bundle); err != nil {
			panic(err)
		}
		if bundle.Version != 1 {
			panic("unsupported API68 describe sidecar version")
		}
		standardAPI68Profiles = bundle.Profiles
		standardAPI68ProfileSignatures = make(map[string]string, len(bundle.Profiles))
		for name, profile := range bundle.Profiles {
			signature := canonicalFeatureSignature(profile.Features)
			if _, exists := standardAPI68ProfileSignatures[signature]; exists {
				panic("duplicate API68 profile signature")
			}
			standardAPI68ProfileSignatures[signature] = name
		}
	})
	return standardAPI68Profiles
}

// standardAPI68CapturedNewObject identifies objects for which a profile has a
// captured 68-only describe shape. It is used only to preserve project origin,
// not to infer object visibility in another profile. It reads the generated
// name set; only API 68 projects pay for decoding the bundle itself.
func standardAPI68CapturedNewObject(object string) bool {
	_, ok := standardAPI68NewObjectNames[strings.ToLower(strings.TrimSpace(object))]
	return ok
}

// standardAPI68BundleProfileFeatures decodes the sidecar and returns each
// profile's feature list, for the test that keeps the generated copy in step.
func standardAPI68BundleProfileFeatures() map[string][]string {
	out := map[string][]string{}
	for name, profile := range loadStandardAPI68Profiles() {
		out[name] = append([]string(nil), profile.Features...)
	}
	return out
}

// standardAPI68BundleNewObjects decodes the sidecar and returns the same set
// the generator derives, for the test that keeps the two in step.
func standardAPI68BundleNewObjects() map[string]struct{} {
	out := map[string]struct{}{}
	for _, profile := range loadStandardAPI68Profiles() {
		for object, captured := range profile.Objects {
			if captured.Status == 200 {
				out[strings.ToLower(object)] = struct{}{}
			}
		}
	}
	return out
}

// StandardAPI68ConfiguredField reports whether a pre-overlay snapshot proves
// a field was configured. A missing snapshot leaves the field's origin unknown.
func StandardAPI68ConfiguredField(definition ObjectDefinition, field string) (configured, recorded bool) {
	_, recorded = definition.Metadata[standardConfiguredPolicyFields]
	if !recorded {
		return false, false
	}
	return standardFieldPolicyPreservesConfiguredField(definition, field), true
}

// StandardAPI68DescribeObject returns an owned object shape observed for the
// exact profile. A missing profile or object is not evidence of absence.
func StandardAPI68DescribeObject(profile, object string) (ObjectDefinition, bool) {
	return StandardAPI68DescribeObjectForRoute(profile, object, false)
}

// StandardAPI68DescribeObjectForRoute projects only fields observed in the
// captured source route; named-only members remain separate from REST.
func StandardAPI68DescribeObjectForRoute(profile, object string, named bool) (ObjectDefinition, bool) {
	captured, ok := loadStandardAPI68Profiles()[profile]
	if !ok {
		return ObjectDefinition{}, false
	}
	for name, value := range captured.Objects {
		if !strings.EqualFold(name, object) || value.Status != 200 || (named && !value.Named) || (!named && !value.Anonymous) {
			continue
		}
		def := ObjectDefinition{APIName: value.Object.Name, Label: value.Object.Label, PluralLabel: value.Object.LabelPlural, Fields: describeFieldMap(value.Fields)}
		for field := range def.Fields {
			visible := value.FieldVisibility[field]
			if (named && !visible.Named) || (!named && !visible.Anonymous) {
				delete(def.Fields, field)
			}
		}
		for field, extra := range StandardAPI68DescribeFields(profile, name, named) {
			if _, exists := def.Fields[field]; !exists {
				def.Fields[field] = extra
			}
		}
		if value.Object.KeyPrefix != nil {
			def.KeyPrefix = *value.Object.KeyPrefix
		}
		return def, true
	}
	return ObjectDefinition{}, false
}

// StandardAPI68DescribeObjectNames returns only objects observed on the
// selected route of the exact captured profile.
func StandardAPI68DescribeObjectNames(profile string, named bool) []string {
	captured, ok := loadStandardAPI68Profiles()[profile]
	if !ok {
		return nil
	}
	var names []string
	for name, value := range captured.Objects {
		if value.Status == 200 && ((named && value.Named) || (!named && value.Anonymous)) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// StandardAPI68DescribeFields returns owned field definitions observed for the
// exact profile and source route. It does not infer visibility in other shapes.
func StandardAPI68DescribeFields(profile, object string, named bool) map[string]Field {
	captured, ok := loadStandardAPI68Profiles()[profile]
	if !ok {
		return nil
	}
	out := map[string]Field{}
	for name, fields := range captured.Fields {
		if !strings.EqualFold(name, object) {
			continue
		}
		for fieldName, value := range fields {
			if (named && value.Named) || (!named && value.Anonymous) {
				out[fieldName] = describeField(value.Field)
			}
		}
		break
	}
	return out
}

// StandardAPI68DescribeAccessRule reports an explicitly captured access
// boundary for a field. Empty means this sidecar installs no special rule.
func StandardAPI68DescribeAccessRule(profile, object, field string) string {
	captured, ok := loadStandardAPI68Profiles()[profile]
	if !ok {
		return ""
	}
	for name, fields := range captured.Fields {
		if !strings.EqualFold(name, object) {
			continue
		}
		for fieldName, value := range fields {
			if strings.EqualFold(fieldName, field) {
				return value.AccessRule
			}
		}
	}
	return ""
}
