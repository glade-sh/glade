package apextest

import (
	"os"
	"path/filepath"
	"testing"
)

// The org.tsv and describe exports in b5b9_feature_objects compare the
// Developer, Fundraising-enabled and DocGen-only profiles at APIs 62/67.
func TestB5B9FeatureObjectsNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/"
	profiles := []struct{ name, file, gifts, docs, regulatory string }{
		{"developer", "", "false", "false", "false"},
		{"fundraising", "npc-minimal.json", "true", "false", "false"},
		{"docgen", "docgen-minimal.json", "false", "true", "false"},
		{"publicsector", "publicsector.json", "false", "false", "true"},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, profile := range profiles {
			t.Run(api+"/"+profile.name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				definition := []byte(`{"edition":"Developer"}`)
				if profile.file != "" {
					var err error
					definition, err = os.ReadFile(dir + profile.file)
					if err != nil {
						t.Fatal(err)
					}
				}
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(definition))
				body := `Map<String,Schema.SObjectType> m=Schema.getGlobalDescribe();` +
					`System.assertEquals(` + profile.gifts + `,m.containsKey('GiftTransaction'),'GiftTransaction');` +
					`System.assertEquals(` + profile.gifts + `,m.containsKey('GiftCommitment'),'GiftCommitment');` +
					`System.assertEquals(` + profile.docs + `,m.containsKey('DocumentTemplateContentDoc'),'DocumentTemplateContentDoc');` +
					`System.assertEquals(` + profile.docs + `,m.containsKey('DocumentGenerationProcess'),'DocumentGenerationProcess');` +
					`System.assertEquals(` + profile.regulatory + `,m.containsKey('RegulatoryCode'),'RegulatoryCode');`
				if profile.regulatory == "true" {
					body += `System.assert(m.get('RegulatoryCode').getDescribe().fields.getMap().containsKey('IsActive'));`
				}
				if profile.gifts == "true" {
					body += `System.assert(m.get('GiftTransaction').getDescribe().fields.getMap().containsKey('DonorId'));System.assert(m.get('GiftCommitment').getDescribe().fields.getMap().containsKey('DonorId'));`
				}
				if profile.docs == "true" {
					body += `System.assert(m.get('DocumentTemplateContentDoc').getDescribe().fields.getMap().containsKey('ContentDocumentId'));System.assert(m.get('DocumentGenerationProcess').getDescribe().fields.getMap().containsKey('Status'));`
				}
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}
