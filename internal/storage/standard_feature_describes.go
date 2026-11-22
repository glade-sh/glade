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

// Generated from the owned feature-profile, messaging campaign presence and
// auth-flow capture packets; regenerate with
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
	QueryVersions      map[string]map[string]map[string]bool `json:"queryVersions"`
	DescribeFacets     map[string][]standardDescribeField    `json:"describeFacets"`
	CustomFieldRoutes  map[string]map[string]map[string]bool `json:"customFieldRoutes"`
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
	// Lowercase object name to canonical FieldVersions key. The sema
	// field-access path resolves versioned objects on every field reference,
	// so the lookup must not scan the FieldVersions map (auth-flow rows).
	versionedNames map[string]string
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
		standardFeatureDescribeCache.versionedNames = make(map[string]string, len(standardFeatureDescribeCache.bundle.FieldVersions))
		for name := range standardFeatureDescribeCache.bundle.FieldVersions {
			standardFeatureDescribeCache.versionedNames[strings.ToLower(name)] = name
		}
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

// standardFeatureVersionedObjectName canonicalizes an object that carries
// captured per-API field versions. Most are feature-profile describes, but the
// version matrix also covers standard catalog objects that no profile owns.
func standardFeatureVersionedObjectName(object string) (string, bool) {
	loadStandardFeatureDescribes()
	names := standardFeatureDescribeCache.versionedNames
	// Field access checks call this per record write; fold short ASCII names
	// on the stack so the lookup does not allocate.
	var buf [64]byte
	if len(object) <= len(buf) {
		ascii := true
		for i := 0; i < len(object); i++ {
			c := object[i]
			if c >= 0x80 {
				ascii = false
				break
			}
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			buf[i] = c
		}
		if ascii {
			name, ok := names[string(buf[:len(object)])]
			return name, ok
		}
	}
	name, ok := names[strings.ToLower(object)]
	return name, ok
}

// StandardFeatureFieldVersion reports captured source-version membership for
// the isolated feature surfaces. The second value is false for unmeasured
// fields or API versions, which retain the ordinary catalog behavior.
func StandardFeatureFieldVersion(object, field, api string) (bool, bool) {
	bundle := loadStandardFeatureDescribes()
	name, ok := standardFeatureVersionedObjectName(object)
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

// StandardFeatureHasVersionedFields reports whether any per-API field
// membership was captured for the object.
func StandardFeatureHasVersionedFields(object string) bool {
	_, ok := standardFeatureVersionedObjectName(object)
	return ok
}

// StandardFeatureQueryVersion reports captured dynamic-query membership for
// versioned feature fields. The second value is false for unmeasured fields
// or API versions, which retain the source-access visibility.
func StandardFeatureQueryVersion(object, field, api string) (bool, bool) {
	bundle := loadStandardFeatureDescribes()
	object = strings.TrimSpace(object)
	for name, fields := range bundle.QueryVersions {
		if !strings.EqualFold(name, object) {
			continue
		}
		for key, versions := range fields {
			if strings.EqualFold(key, field) {
				value, found := versions[api]
				return value, found
			}
		}
		return false, false
	}
	return false, false
}

// StandardFeatureCustomFieldRoute reports the captured compile routes for a
// custom field on a feature-isolated object. anonymous is the captured
// anonymous-route acceptance and named the captured named-source acceptance
// under the capture's own deployment: accepted routes referenced deployed
// fields, rejected ones missing fields. The third value is false when no
// route was captured; such fields keep the ordinary partial-schema behavior.
func StandardFeatureCustomFieldRoute(object, field string) (anonymous, named, captured bool) {
	bundle := loadStandardFeatureDescribes()
	object = strings.TrimSpace(object)
	for name, fields := range bundle.CustomFieldRoutes {
		if !strings.EqualFold(name, object) {
			continue
		}
		for key, routes := range fields {
			if strings.EqualFold(key, field) {
				return routes["anonymous"], routes["named"], true
			}
		}
		return false, false, false
	}
	return false, false, false
}

// standardFeatureDescribeFacets returns the captured describe facets for
// fields whose object describe is not part of the feature packet.
func standardFeatureDescribeFacets(object string) []standardDescribeField {
	bundle := loadStandardFeatureDescribes()
	object = strings.TrimSpace(object)
	for name, facets := range bundle.DescribeFacets {
		if strings.EqualFold(name, object) {
			return facets
		}
	}
	return nil
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
