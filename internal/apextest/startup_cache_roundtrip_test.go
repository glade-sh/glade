package apextest

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/testreport"
)

// A project run that restores the on-disk startup cache must see the same
// standard schema as the cold run that wrote it. commerce_enabled_surface and
// commerce_extension_shapes capture WebStore.OptionsAutoFacetingEnabled as a
// non-nillable Boolean at APIs 62/67; before the fix the restored field lost
// its false nillable/defaultedOnCreate flags, so warm inserts failed with
// REQUIRED_FIELD_MISSING and describe reported isNillable() true.
func TestStartupDiskCacheColdWarmProjectRunsMatch(t *testing.T) {
	restore := EnableDiskCacheForTesting()
	defer restore()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/OwnedWarmCacheTest.cls"), `@IsTest
private class OwnedWarmCacheTest {
    @IsTest static void booleanDescribe() {
        Schema.DescribeFieldResult field = WebStore.OptionsAutoFacetingEnabled.getDescribe();
        System.assertEquals(false, field.isNillable(), 'Boolean nillability');
        System.assertEquals(true, field.isCreateable());
    }
    @IsTest static void insertStore() {
        WebStore store = new WebStore(Name='Owned warm store');
        insert store;
        System.assertNotEquals(null, store.Id);
    }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/OwnedWarmCacheTest.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	index := loadTestIndex(t, root)
	opts := Options{Parallelism: 1, PerfCounters: true}
	outcomes := func(result testreport.Run) map[string]string {
		got := map[string]string{}
		for _, suite := range result.Suites {
			for _, c := range suite.Cases {
				message := ""
				if c.Problem != nil {
					message = c.Problem.Message
				}
				got[suite.Name+"."+c.MethodName] = string(c.Status) + " " + message
			}
		}
		return got
	}
	InvalidateRuntimeCaches()
	ResetPerfCounters()
	cold := outcomes(Run(index, opts))
	if SnapshotPerfCounters().Phases.DiskCacheHits != 0 {
		t.Fatal("cold run restored a disk cache")
	}
	InvalidateRuntimeCaches()
	ResetPerfCounters()
	warm := outcomes(Run(index, opts))
	if SnapshotPerfCounters().Phases.DiskCacheHits == 0 {
		t.Fatal("warm run did not restore the disk cache")
	}
	if len(cold) != 2 || !reflect.DeepEqual(cold, warm) {
		t.Fatalf("cold/warm outcomes differ:\ncold=%v\nwarm=%v", cold, warm)
	}
	for name, outcome := range cold {
		if !strings.HasPrefix(outcome, "pass") {
			t.Fatalf("%s: %s", name, outcome)
		}
	}
}

// The startup cache must restore the org it persisted. Compare a real
// project org (generated standard schema plus project metadata) after a disk
// round trip. Nil and empty collections are equivalent; everything else,
// including pointer flags that point at zero values, must survive.
func TestStartupDiskCacheOrgRoundTripPreservesSchema(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Owned__c/Owned__c.object-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Owned</label><pluralLabel>Owned</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Owned__c/fields/Flag__c.field-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Flag__c</fullName><label>Flag</label><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`)
	index := loadTestIndex(t, root)
	org := orgFromIndex(index)
	if field := org.Objects["WebStore"].Definition.Fields["OptionsAutoFacetingEnabled"]; field.Nillable == nil || *field.Nillable {
		t.Fatalf("fixture lost its non-nillable Boolean before the round trip: %#v", field)
	}
	proof, err := startupcache.ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := startupcache.NewEntryWithValidatedInput(proof, org, startupcache.CompiledRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	entry.RuntimeABI = "roundtrip-test"
	entry.RuntimeKey = "roundtrip-key"
	if _, err := startupcache.WriteWithStats(&entry, startupcache.SubdirTest); err != nil {
		t.Fatal(err)
	}
	got, err := startupcache.ReadFreshRuntimeWithValidatedInput(root, startupcache.SubdirTest, startupcache.Version, entry.RuntimeABI, entry.RuntimeKey, proof)
	if err != nil || got == nil {
		t.Fatalf("fresh cache=%v err=%v", got, err)
	}
	var diffs []string
	compareCachedValue(reflect.ValueOf(org), reflect.ValueOf(got.Org), "Org", &diffs)
	if len(diffs) != 0 {
		if len(diffs) > 20 {
			diffs = append(diffs[:20], fmt.Sprintf("... %d more", len(diffs)-20))
		}
		t.Fatalf("startup cache round trip changed the org:\n%s", strings.Join(diffs, "\n"))
	}
}

func compareCachedValue(want, got reflect.Value, path string, diffs *[]string) {
	if len(*diffs) > 200 {
		return
	}
	if want.Kind() != got.Kind() {
		*diffs = append(*diffs, path+": kind "+want.Kind().String()+" != "+got.Kind().String())
		return
	}
	switch want.Kind() {
	case reflect.Pointer:
		if want.IsNil() != got.IsNil() {
			*diffs = append(*diffs, fmt.Sprintf("%s: nil=%t restored nil=%t", path, want.IsNil(), got.IsNil()))
			return
		}
		if !want.IsNil() {
			compareCachedValue(want.Elem(), got.Elem(), path, diffs)
		}
	case reflect.Interface:
		if want.IsNil() != got.IsNil() {
			*diffs = append(*diffs, path+": interface nil mismatch")
			return
		}
		if !want.IsNil() {
			compareCachedValue(want.Elem(), got.Elem(), path, diffs)
		}
	case reflect.Struct:
		typ := want.Type()
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if !field.IsExported() {
				continue
			}
			compareCachedValue(want.Field(i), got.Field(i), path+"."+field.Name, diffs)
		}
	case reflect.Slice, reflect.Array:
		if want.Len() != got.Len() {
			*diffs = append(*diffs, fmt.Sprintf("%s: len %d != %d", path, want.Len(), got.Len()))
			return
		}
		for i := 0; i < want.Len(); i++ {
			compareCachedValue(want.Index(i), got.Index(i), fmt.Sprintf("%s[%d]", path, i), diffs)
		}
	case reflect.Map:
		if want.Len() != got.Len() {
			*diffs = append(*diffs, fmt.Sprintf("%s: len %d != %d", path, want.Len(), got.Len()))
			return
		}
		iter := want.MapRange()
		for iter.Next() {
			other := got.MapIndex(iter.Key())
			key := fmt.Sprint(iter.Key().Interface())
			if !other.IsValid() {
				*diffs = append(*diffs, path+"["+key+"]: missing")
				continue
			}
			compareCachedValue(iter.Value(), other, path+"["+key+"]", diffs)
		}
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
	default:
		if !want.Equal(got) {
			*diffs = append(*diffs, fmt.Sprintf("%s: %v != %v", path, want.Interface(), got.Interface()))
		}
	}
}
