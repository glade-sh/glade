package capability

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/apexdocs"
)

func TestBuildCatalogClassifiesInventoryEntries(t *testing.T) {
	inv := apexdocs.Inventory{
		SchemaVersion: 1,
		TotalFiles:    3,
		TotalMembers:  3,
		Documents: []apexdocs.Document{{
			SourcePath: "apex_methods_system_string.md",
			Kind:       "class",
			Namespace:  "System",
			Name:       "String",
			Members: []apexdocs.Member{{
				Kind:      "method",
				Name:      "trim",
				Signature: "trim()",
			}},
		}, {
			SourcePath: "apex_methods_system_database.md",
			Kind:       "class",
			Namespace:  "System",
			Name:       "Database",
			Members: []apexdocs.Member{{
				Kind:      "method",
				Name:      "insert",
				Signature: "insert(records)",
			}},
		}, {
			SourcePath: "apex_connectapi_output_FeedElement.md",
			Kind:       "output",
			Namespace:  "ConnectApi",
			Name:       "FeedElement",
			Members: []apexdocs.Member{{
				Kind:      "property",
				Name:      "body",
				Signature: "body",
			}},
		}},
	}

	catalog := BuildCatalog(inv)
	if catalog.SchemaVersion != CatalogSchemaVersion || catalog.SourceDocuments != 3 || catalog.SourceMembers != 3 {
		t.Fatalf("catalog summary = %#v", catalog)
	}

	stringTrim := findCatalogEntry(t, catalog, "String.trim")
	if stringTrim.Area != "Core stdlib" || stringTrim.Target != TargetExecutableParity || stringTrim.Status != StatusSupported {
		t.Fatalf("String.trim entry = %#v", stringTrim)
	}

	databaseInsert := findCatalogEntry(t, catalog, "Database.insert")
	if databaseInsert.Area != "Data platform" || databaseInsert.Target != TargetLocalModel || databaseInsert.Status != StatusSupported {
		t.Fatalf("Database.insert entry = %#v", databaseInsert)
	}

	connectBody := findCatalogEntry(t, catalog, "ConnectApi.FeedElement.body")
	if connectBody.Area != "Product namespaces" || connectBody.Target != TargetTypedStub || connectBody.Status != StatusUnknown {
		t.Fatalf("ConnectApi entry = %#v", connectBody)
	}
}

func TestWriteCatalogJSON(t *testing.T) {
	catalog := Catalog{
		SchemaVersion: CatalogSchemaVersion,
		Entries: []CatalogEntry{{
			ID:         "string/trim#trim-method",
			Area:       "Core stdlib",
			TypeName:   "String",
			MemberName: "trim",
			Symbol:     "String.trim",
			Kind:       "method",
			Signature:  "trim()",
			Target:     TargetExecutableParity,
			Status:     StatusSupported,
			Owner:      "internal/vm",
		}},
	}
	var out bytes.Buffer
	if err := WriteCatalogJSON(&out, catalog); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"target": "executable-parity"`) {
		t.Fatalf("json = %q", out.String())
	}
	var decoded Catalog
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Entries[0].Symbol != "String.trim" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func findCatalogEntry(t *testing.T, catalog Catalog, symbol string) CatalogEntry {
	t.Helper()
	for _, entry := range catalog.Entries {
		if entry.Symbol == symbol {
			return entry
		}
	}
	t.Fatalf("missing catalog entry %s in %#v", symbol, catalog.Entries)
	return CatalogEntry{}
}
