package config

import (
	"reflect"
	"testing"
)

func TestHierarchyAccessObjectsConfig(t *testing.T) {
	cfg, err := parseYAMLSubset("org:\n  hierarchyAccessObjects: [One__c, pkg__Two__c]\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Org.HierarchyAccessObjects, []string{"One__c", "pkg__Two__c"}) {
		t.Fatalf("objects: %#v", cfg.Org.HierarchyAccessObjects)
	}
	for _, input := range []string{"[Account]", "[__c]", "[One__c, '']"} {
		if _, err := parseYAMLSubset("org:\n  hierarchyAccessObjects: " + input + "\n"); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}
