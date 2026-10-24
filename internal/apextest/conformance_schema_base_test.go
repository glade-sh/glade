package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// A cache belongs to one conformance invocation. Its owner keeps metadata
// projects alive until all API and row subtests have finished.
type conformanceSchemaCache struct {
	owner *testing.T
	mu    sync.Mutex
	bases map[conformanceSchemaKey]*conformanceRunner
}

type conformanceSchemaKey struct {
	api, namespace string
	metadata       [sha256.Size]byte
}

func conformanceSchemaBase(t *testing.T, index typesys.Index, api string) *conformanceRunner {
	t.Helper()
	org, err := orgFromIndexWithError(index)
	if err != nil {
		t.Fatal(err)
	}
	org.APIVersion = api
	return newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
}

func newConformanceSchemaCache(t *testing.T) *conformanceSchemaCache {
	return &conformanceSchemaCache{owner: t, bases: make(map[conformanceSchemaKey]*conformanceRunner)}
}

func (c *conformanceSchemaCache) get(t *testing.T, api, namespace string, metadata map[string]string, build func(*testing.T) *conformanceRunner) *conformanceRunner {
	t.Helper()
	// JSON sorts string map keys, so fixture map iteration cannot change identity.
	if len(metadata) == 0 {
		metadata = map[string]string{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	key := conformanceSchemaKey{api: api, namespace: namespace, metadata: sha256.Sum256(raw)}
	c.mu.Lock()
	defer c.mu.Unlock()
	if base := c.bases[key]; base != nil {
		return base
	}
	base := build(c.owner)
	c.bases[key] = base
	return base
}

func TestConformanceSchemaBaseBuiltOnce(t *testing.T) {
	cache := newConformanceSchemaCache(t)
	builds := 0
	build := func(t *testing.T) *conformanceRunner {
		builds++
		return &conformanceRunner{sources: make(map[string]conformanceSourceGeneration)}
	}
	first := cache.get(t, "62.0", "", map[string]string{"Entry": "one", "Other": "two"}, build)
	again := cache.get(t, "62.0", "", map[string]string{"Other": "two", "Entry": "one"}, build)
	if first != again || builds != 1 {
		t.Errorf("identical schema built %d bases; want one shared base", builds)
	}
	for _, input := range []struct{ api, namespace, source string }{
		{"67.0", "", "one"}, {"62.0", "owned", "one"}, {"62.0", "", "changed"},
	} {
		if got := cache.get(t, input.api, input.namespace, map[string]string{"Entry": input.source, "Other": "two"}, build); got == first {
			t.Error("different API, namespace or metadata reused the original base")
		}
	}
	empty := cache.get(t, "62.0", "", nil, build)
	if cache.get(t, "62.0", "", map[string]string{}, build) != empty {
		t.Error("empty metadata maps must share a base")
	}
	if newConformanceSchemaCache(t).get(t, "62.0", "", nil, build) == empty {
		t.Error("separate conformance invocations shared a base")
	}
}

func TestConformanceSchemaBaseProjectRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			cache := newConformanceSchemaCache(t)
			objects := map[string]string{"SchemaEntry__c": `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Schema Entry</label><pluralLabel>Schema Entries</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`}
			build := func(t *testing.T) *conformanceRunner {
				index := soqlBindNativeIndex(t, api, nil, objects)
				org := orgFromIndex(index)
				org.APIVersion = api
				return newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			}
			for _, value := range []string{"first", "second"} {
				index := soqlBindNativeIndex(t, api, map[string]string{
					"SchemaRow":     "public class SchemaRow { public static String value='" + value + "'; }",
					"SchemaRowTest": "@IsTest private class SchemaRowTest { @IsTest static void observe(){ System.assertEquals('" + value + "',SchemaRow.value); System.assertEquals(0,[SELECT count() FROM SchemaEntry__c]); insert new SchemaEntry__c(Name='row'); System.assertEquals(1,[SELECT count() FROM SchemaEntry__c]); } }",
				}, objects)
				base := cache.get(t, api, "", objects, build)
				reused := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: base.orgSource})
				freshOrg := orgFromIndex(index)
				freshOrg.APIVersion = api
				fresh := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &freshOrg})
				if reused.org != base.org {
					t.Error("row rebuilt the cached schema template")
				}
				conformanceSchemaCompareProjectRows(t, api, value, index, base, reused, fresh)
			}
		})
	}
}
