package apextest

import (
	"os"
	"path/filepath"
	"testing"
)

// cache-deployed rows in org.tsv capture the authored partition at both APIs.
func TestB5B9CachePartitionNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/cache-partition/"
	metadata, err := os.ReadFile(dir + "OwnedPartition.cachePartition-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(dir + "capture.apex")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/cachePartitions/OwnedPartition.cachePartition-meta.xml"), string(metadata))
			runStandardFieldEvidenceRoutes(t, root, api, string(body))
		})
	}
}
