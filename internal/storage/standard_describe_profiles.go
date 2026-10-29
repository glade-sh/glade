package storage

import "slices"

var standardDescribeProfileCatalog map[string]map[string]StandardDescribeCatalogDelta

// StandardDescribeCatalogDelta records one captured profile relative to its
// named baseline. Field entries are pairs of object and field API names.
type StandardDescribeCatalogDelta struct {
	BaselineProfile string `json:"baseline_profile"`
	Fields          struct {
		Added   [][2]string `json:"added"`
		Removed [][2]string `json:"removed"`
	} `json:"fields"`
	Objects struct {
		Added   []string `json:"added"`
		Removed []string `json:"removed"`
	} `json:"objects"`
}

// StandardDescribeCatalogProfileDelta returns an owned copy of the captured
// profile/API delta. Unknown keys have no result; an observed empty delta does.
// These observations do not select runtime field gates from project features.
func StandardDescribeCatalogProfileDelta(profile, api string) (StandardDescribeCatalogDelta, bool) {
	delta, ok := standardDescribeProfileCatalog[profile][api]
	if !ok {
		return StandardDescribeCatalogDelta{}, false
	}
	delta.Fields.Added = slices.Clone(delta.Fields.Added)
	delta.Fields.Removed = slices.Clone(delta.Fields.Removed)
	delta.Objects.Added = slices.Clone(delta.Objects.Added)
	delta.Objects.Removed = slices.Clone(delta.Objects.Removed)
	return delta, true
}
