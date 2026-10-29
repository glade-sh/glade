package storage

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"io"
	"strings"
	"sync"
)

// The packed describe catalog sorts fields for deterministic hashing. Keep the
// original describe sequence separately so FIELDS(STANDARD) can preserve it
// without decoding the broad catalog or loading unrelated field metadata.
//
//go:generate python3 ../../scripts/generate-standard-field-order.py
//go:embed standard_field_order.tsv.gz
var standardFieldOrderGZIP []byte

var standardFieldOrderOnce sync.Once
var standardFieldOrder map[string]string

// VisitStandardFields preserves the field sequence in the owned describe
// catalog. Query FIELDS(STANDARD) JSON uses that sequence, filtered to fields
// available in the active schema; no additional catalog fields are selected.
func VisitStandardFields(objectName string, visit func(string)) bool {
	canonical, ok := standardDescribeCatalogCanonicalName(objectName)
	if !ok {
		return false
	}
	standardFieldOrderOnce.Do(loadStandardFieldOrder)
	fields, found := standardFieldOrder[canonical]
	if !found {
		return false
	}
	for fields != "" {
		var field string
		field, fields, _ = strings.Cut(fields, ",")
		visit(field)
	}
	return true
}

func loadStandardFieldOrder() {
	reader, err := gzip.NewReader(bytes.NewReader(standardFieldOrderGZIP))
	if err != nil {
		return
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return
	}
	orders := make(map[string]string, len(standardDescribeCatalogObjectNames))
	for remaining := string(data); remaining != ""; {
		var line string
		line, remaining, _ = strings.Cut(remaining, "\n")
		object, fields, found := strings.Cut(line, "\t")
		if !found || object == "" {
			return
		}
		orders[object] = fields
	}
	standardFieldOrder = orders
}
