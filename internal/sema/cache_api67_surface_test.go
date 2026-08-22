package sema

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestAPI67CacheRejectedShapes(t *testing.T) {
	tests := map[string]string{
		"Cache.Org.isAvailable does not exist":           `Cache.Org.isAvailable();`,
		"Cache.Org.getAvgValueSize removed":              `Cache.Org.getAvgValueSize();`,
		"Cache.Org.getMaxValueSize removed":              `Cache.Org.getMaxValueSize();`,
		"Cache.Session.getAvgValueSize removed":          `Cache.Session.getAvgValueSize();`,
		"Cache.Session.getMaxValueSize removed":          `Cache.Session.getMaxValueSize();`,
		"partition getAvgValueSize removed":              `Cache.OrgPartition p = Cache.Org.getPartition('local'); p.getAvgValueSize();`,
		"partition getMaxValueSize removed":              `Cache.OrgPartition p = Cache.Org.getPartition('local'); p.getMaxValueSize();`,
		"createFullyQualifiedKey through instance":       `Cache.OrgPartition p = Cache.Org.getPartition('local'); p.createFullyQualifiedKey('a', 'b', 'c');`,
		"createFullyQualifiedPartition through instance": `Cache.SessionPartition p = Cache.Session.getPartition('local'); p.createFullyQualifiedPartition('a', 'b');`,
		"validatePartitionName through instance":         `Cache.Partition p = Cache.Org.getPartition('local'); p.validatePartitionName('a');`,
		"validateKey through instance":                   `Cache.OrgPartition p = Cache.Org.getPartition('local'); p.validateKey(false, 'a');`,
		"validateKeyValue through instance":              `Cache.OrgPartition p = Cache.Org.getPartition('local'); p.validateKeyValue(false, 'a', 'v');`,
		"validateKeys through instance":                  `Cache.OrgPartition p = Cache.Org.getPartition('local'); p.validateKeys(false, new Set<String>{'a'});`,
		"Cache.Partition.validateKeys removed":           `Cache.Partition.validateKeys(false, new Set<String>{'a'});`,
		"Cache.OrgPartition.validateKeys removed":        `Cache.OrgPartition.validateKeys(false, new Set<String>{'a'});`,
		"Cache.SessionPartition.validateKeys removed":    `Cache.SessionPartition.validateKeys(false, new Set<String>{'a'});`,
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, source, "67.0")
			if len(result.Diagnostics) == 0 {
				t.Fatalf("accepted Salesforce-API-67-rejected cache call: %s", source)
			}
		})
	}
}

func TestAPI67CacheAcceptedShapes(t *testing.T) {
	tests := map[string]string{
		"Cache.Session.isAvailable exists":     `Cache.Session.isAvailable();`,
		"partition isAvailable exists":         `Cache.OrgPartition p = Cache.Org.getPartition('local'); p.isAvailable();`,
		"createFullyQualifiedKey static":       `Cache.OrgPartition.createFullyQualifiedKey('local', 'default', 'account');`,
		"createFullyQualifiedPartition static": `Cache.Partition.createFullyQualifiedPartition('local', 'default');`,
		"validatePartitionName static":         `Cache.SessionPartition.validatePartitionName('default');`,
		"validateKey static":                   `Cache.OrgPartition.validateKey(false, 'account');`,
		"validateKeyValue static":              `Cache.OrgPartition.validateKeyValue(false, 'account', 'value');`,
		"validateCacheBuilder static":          `Cache.Partition.validateCacheBuilder(String.class);`,
		"getMissRate kept":                     `Cache.Org.getMissRate();`,
		"getAvgGetSize kept":                   `Cache.Org.getAvgGetSize();`,
		"remove returns Boolean":               `Boolean removed = Cache.Org.remove('k');`,
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, source, "67.0")
			if result.HasErrors() {
				t.Fatalf("rejected allowed Salesforce cache call: %#v", result.Diagnostics)
			}
		})
	}
}

func TestAPI67CachePartitionStaticOnlyDiagnosticMessage(t *testing.T) {
	result := AnalyzeAnonymous(typesys.Index{}, `Cache.OrgPartition p = Cache.Org.getPartition('local'); p.createFullyQualifiedKey('a', 'b', 'c');`, "67.0")
	for _, diagnostic := range result.Diagnostics {
		if strings.Contains(diagnostic.Message, "createFullyQualifiedKey") {
			return
		}
	}
	t.Fatalf("instance static cache call produced no diagnostic: %#v", result.Diagnostics)
}
