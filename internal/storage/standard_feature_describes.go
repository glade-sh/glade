package storage

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

// Generated from the owned feature-profile capture packet; regenerate with
// scripts/generate-standard-feature-describes.py.
//
//go:embed standard_feature_describes.json.gz
var standardFeatureDescribeData []byte

type standardFeatureDescribeBundle struct {
	ReadOnly           map[string][]string                   `json:"readOnly"`
	Objects            map[string]standardDescribeObject     `json:"objects"`
	FieldVersions      map[string]map[string]map[string]bool `json:"fieldVersions"`
	NamedOnly          map[string][]string                   `json:"namedOnly"`
	NamedQueryVersions map[string]map[string]map[string]bool `json:"namedQueryVersions"`
}

var standardFeatureDescribeCache struct {
	sync.Once
	bundle  standardFeatureDescribeBundle
	names   map[string]string
	entries map[string]standardObjectCatalogEntry
	// Canonical names in a fixed order. Every catalog name resolution asks
	// standardFeatureDescribeName first and almost always misses, so the miss
	// path must not iterate a map or allocate (v0.3.5 b5b9features perf).
	canonical []string
}

func loadStandardFeatureDescribes() *standardFeatureDescribeBundle {
	standardFeatureDescribeCache.Do(func() {
		stream, err := gzip.NewReader(bytes.NewReader(standardFeatureDescribeData))
		if err != nil {
			panic(err)
		}
		defer stream.Close()
		if err := json.NewDecoder(stream).Decode(&standardFeatureDescribeCache.bundle); err != nil {
			panic(err)
		}
		standardFeatureDescribeCache.names = map[string]string{}
		standardFeatureDescribeCache.entries = map[string]standardObjectCatalogEntry{}
		for name, desc := range standardFeatureDescribeCache.bundle.Objects {
			key := strings.ToLower(name)
			standardFeatureDescribeCache.names[key] = name
			standardFeatureDescribeCache.entries[key] = standardObjectCatalogEntry{Definition: ObjectDefinition{
				APIName: name, Label: desc.Label, KeyPrefix: desc.keyPrefixValue(),
				Fields: describeFieldMap(desc.Fields), Relations: describeRelationships(name, desc.Fields, nil),
			}}
			standardFeatureDescribeCache.canonical = append(standardFeatureDescribeCache.canonical, name)
		}
		sort.Strings(standardFeatureDescribeCache.canonical)
	})
	return &standardFeatureDescribeCache.bundle
}

func standardFeatureDescribeName(name string) (string, bool) {
	loadStandardFeatureDescribes()
	name = strings.TrimSpace(name)
	for _, canonical := range standardFeatureDescribeCache.canonical {
		if strings.EqualFold(name, canonical) {
			return canonical, true
		}
	}
	return "", false
}

func standardFeatureDescribeEntry(name string) (standardObjectCatalogEntry, bool) {
	loadStandardFeatureDescribes()
	entry, ok := standardFeatureDescribeCache.entries[strings.ToLower(strings.TrimSpace(name))]
	return entry, ok
}

func standardFeatureDescribeField(object, field string) (standardDescribeField, bool) {
	bundle := loadStandardFeatureDescribes()
	name, ok := standardFeatureDescribeName(object)
	if !ok {
		return standardDescribeField{}, false
	}
	for _, candidate := range bundle.Objects[name].Fields {
		if strings.EqualFold(candidate.Name, field) {
			return candidate, true
		}
	}
	return standardDescribeField{}, false
}

// StandardFeatureCapturedReference recognizes only reference edges observed in
// the feature describe packet; it does not make the target a visible type.
func StandardFeatureCapturedReference(object, field, target string) bool {
	desc, ok := standardFeatureDescribeField(object, field)
	if !ok {
		return false
	}
	for _, name := range desc.ReferenceTo {
		if strings.EqualFold(name, target) {
			return true
		}
	}
	return false
}

// StandardFeatureFieldVersion reports captured source-version membership for
// the isolated feature surfaces. The second value is false for unmeasured
// fields or API versions, which retain the ordinary catalog behavior.
func StandardFeatureFieldVersion(object, field, api string) (bool, bool) {
	bundle := loadStandardFeatureDescribes()
	name, ok := standardFeatureDescribeName(object)
	if !ok {
		return false, false
	}
	for key, versions := range bundle.FieldVersions[name] {
		if strings.EqualFold(key, field) {
			value, found := versions[api]
			return value, found
		}
	}
	return false, false
}

func StandardFeatureHasVersionedFields(object string) bool {
	bundle := loadStandardFeatureDescribes()
	name, ok := standardFeatureDescribeName(object)
	return ok && len(bundle.FieldVersions[name]) > 0
}

// StandardFeatureNamedOnlyField identifies fields observed in named Apex but
// absent in anonymous describes in the captured DocGen profile.
func StandardFeatureNamedOnlyField(object, field string) bool {
	bundle := loadStandardFeatureDescribes()
	name, ok := standardFeatureDescribeName(object)
	if !ok {
		return false
	}
	for _, candidate := range bundle.NamedOnly[name] {
		if strings.EqualFold(candidate, field) {
			return true
		}
	}
	return false
}

func StandardFeatureHasNamedOnlyFields(object string) bool {
	bundle := loadStandardFeatureDescribes()
	name, ok := standardFeatureDescribeName(object)
	return ok && len(bundle.NamedOnly[name]) > 0
}

// StandardFeatureNamedQueryVersion returns only observed named-query results.
func StandardFeatureNamedQueryVersion(object, field, api string) (bool, bool) {
	bundle := loadStandardFeatureDescribes()
	name, ok := standardFeatureDescribeName(object)
	if !ok {
		return false, false
	}
	for key, versions := range bundle.NamedQueryVersions[name] {
		if strings.EqualFold(key, field) {
			value, found := versions[api]
			return value, found
		}
	}
	return false, false
}

// StandardFeatureReadOnlyField reports only write rejections captured for both
// static and dynamic access, not general describe create/update permissions.
func StandardFeatureReadOnlyField(object, field string) (string, string, bool) {
	bundle := loadStandardFeatureDescribes()
	name, ok := standardFeatureDescribeName(object)
	if !ok {
		return "", "", false
	}
	for _, candidate := range bundle.ReadOnly[name] {
		if strings.EqualFold(candidate, field) {
			return name, candidate, true
		}
	}
	return "", "", false
}
