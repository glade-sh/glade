package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// A warm disk runtime built before a record type lands in unpackaged/pre must
// not be reused: the next run sees RecordTypeId, as a fresh org deployed with
// that setup stage does (standard_recordtype_field_policy/org.tsv:2-25,
// API 62/67 baseline rejection versus record-type membership).
func TestSetupStageRecordTypeRefreshesWarmDiskRuntime(t *testing.T) {
	for _, stage := range []string{"pre", "post"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer"}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/OwnedWarm.cls"), "public class OwnedWarm {}")
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/OwnedWarm.cls-meta.xml"), "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
			restore := EnableDiskCacheForTesting()
			t.Cleanup(restore)
			t.Cleanup(InvalidateRuntimeCaches)
			hasRecordTypeID := func() bool {
				InvalidateRuntimeCaches()
				_, built, err := runtimeFromIndexWithSourceDigests(loadTestIndex(t, root), nil, newSourceCache(), true)
				if err != nil {
					t.Fatal(err)
				}
				org := built.restored.CloneOrg()
				for name := range org.Objects["Account"].Definition.Fields {
					if strings.EqualFold(name, "RecordTypeId") {
						return true
					}
				}
				return false
			}
			if hasRecordTypeID() {
				t.Fatal("bare Developer shape exposed Account.RecordTypeId before any record type")
			}
			if hasRecordTypeID() {
				t.Fatal("warm run exposed Account.RecordTypeId before any record type")
			}
			writeFile(t, filepath.Join(root, "unpackaged", stage, "owned/objects/Account.object"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><recordTypes><fullName>Owned</fullName><label>Owned</label><active>true</active></recordTypes></CustomObject>`)
			if !hasRecordTypeID() {
				t.Fatal("warm disk runtime hid Account.RecordTypeId after a setup-stage record type")
			}
		})
	}
}
