package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStandardOrgSeedCloneCost(t *testing.T) {
	seed := standardApexTestOrg()
	// Compare the same schema and row set, after warming the standard seed.
	start := time.Now()
	full := testing.AllocsPerRun(3, func() { _ = seed.Clone() })
	fullSeconds := time.Since(start).Seconds()
	start = time.Now()
	overlay := testing.AllocsPerRun(3, func() { _ = standardApexTestOrg() })
	overlaySeconds := time.Since(start).Seconds()
	t.Logf("deep clone %.0f allocations %.3fs; seed clone %.0f allocations %.3fs (four calls each)", full, fullSeconds, overlay, overlaySeconds)
	if overlay >= full/4 {
		t.Fatalf("seed clone allocates %.0f; want less than 25%% of deep clone %.0f", overlay, full)
	}
}

func TestStandardOrgSeedRowIsolation(t *testing.T) {
	for _, features := range [][]string{nil, {"MultiCurrency"}, {"PersonAccounts"}} {
		t.Run(stringJSON(t, features), func(t *testing.T) {
			first := shapedStandardApexTestOrg(features, "https://example.my.salesforce.com")
			before := stringJSON(t, first)
			pristine := first.Clone()
			definition, ok := storage.EnsureMutableObjectDefinition(&first, "Account")
			if !ok {
				t.Fatal("missing Account")
			}
			definition.Fields["OverlayValue__c"] = storage.Field{APIName: "OverlayValue__c", Type: storage.FieldString}
			first.Objects["Account"].Records["001000000000001"] = storage.Record{Object: "Account", ID: "001000000000001", Fields: map[string]storage.Value{"Name": storage.StringValue("changed")}}
			first.IDSequences["Account"] = 99
			first.Features = append(first.Features, "Changed")
			second := shapedStandardApexTestOrg(features, "https://example.my.salesforce.com")
			if got := stringJSON(t, second); got != before {
				t.Fatal("row N+1 differs from the pristine seed after row N mutation")
			}
			if got := stringJSON(t, pristine); got != before {
				t.Fatal("deep clone oracle changed")
			}
			if reflect.ValueOf(first.Objects["Contact"].Definition.Fields).Pointer() != reflect.ValueOf(second.Objects["Contact"].Definition.Fields).Pointer() {
				t.Error("untouched standard definitions are not shared")
			}
		})
	}
}

func TestStandardOrgSeedProjectOverlaysStayPrivate(t *testing.T) {
	first := standardApexTestOrg()
	before := stringJSON(t, first)
	applyReferencedStandardFieldSet(&first, map[string]map[string]storage.Field{"Account": {"OverlayValue__c": {APIName: "OverlayValue__c", Type: storage.FieldString}}}, nil)
	ensureProjectDataReferencedObjectField(&first, "Contact", "OverlayData__c")
	second := standardApexTestOrg()
	if got := stringJSON(t, second); got != before {
		t.Fatal("project overlay leaked into the next row")
	}
	if _, ok := first.Objects["Account"].Definition.Fields["OverlayValue__c"]; !ok {
		t.Fatal("missing referenced overlay")
	}
	if _, ok := first.Objects["Contact"].Definition.Fields["OverlayData__c"]; !ok {
		t.Fatal("missing data overlay")
	}
}

func TestOrgFromIndexPublicDefinitionsStayPrivate(t *testing.T) {
	first := OrgFromIndex(typesys.Index{})
	first.Objects["Account"].Definition.Fields["OverlayValue__c"] = storage.Field{APIName: "OverlayValue__c", Type: storage.FieldString}
	second := OrgFromIndex(typesys.Index{})
	if _, ok := second.Objects["Account"].Definition.Fields["OverlayValue__c"]; ok {
		t.Fatal("public mutable definition leaked into the next org")
	}
}

func stringJSON(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// This digest covers the entire serialized org, including standard definitions,
// platform records and sequences, for before/after equivalence measurements.
func TestStandardOrgSeedSerializedEquivalence(t *testing.T) {
	for _, namespace := range []string{"", "pkg"} {
		index := typesys.Index{}
		index.Project.Namespace = namespace
		org, err := orgFromIndexWithError(index)
		if err != nil {
			t.Fatal(err)
		}
		serialized := stringJSON(t, org)
		if got := stringJSON(t, org.Clone()); got != serialized {
			t.Fatal("frozen runtime differs from its deep clone")
		}
		rows := 0
		for _, object := range org.Objects {
			rows += len(object.Records)
		}
		t.Logf("namespace=%q objects=%d records=%d sha256=%x", namespace, len(org.Objects), rows, sha256.Sum256([]byte(serialized)))
	}
}
