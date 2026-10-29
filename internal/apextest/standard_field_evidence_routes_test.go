package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Synthetic metadata routes for the captured membership contract in
// standard_recordtype_field_policy/org.tsv:16-25 (API62/67). Inline mdapi
// recordTypes must reach the same schema conversion as source-format metadata.
// Contact and Opportunity exercise the shared rule beyond Account. The setup
// layouts omit cumulusci.yml: unpackaged/pre and unpackaged/post still deploy
// before tests, so their record types are project evidence.
func TestStandardFieldEvidenceMetadataRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, layout := range []string{"package", "extra-metadata", "cci-pre", "cci-post", "setup-pre", "setup-post"} {
			t.Run(api+"/"+layout, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer"}`)
				dir := "force-app"
				switch layout {
				case "extra-metadata":
					dir = "setup"
					writeFile(t, filepath.Join(root, "glade.yml"), "project:\n  extraMetadataDirs: [\"setup\"]\n")
				case "cci-pre", "cci-post":
					dir = "unpackaged/" + layout[4:] + "/owned"
					writeFile(t, filepath.Join(root, "cumulusci.yml"), "project:\n  name: OwnedMetadata\n")
				case "setup-pre", "setup-post":
					// The setup stage directory alone declares deployment before tests.
					dir = "unpackaged/" + layout[6:] + "/owned"
				}
				body := ""
				for i, object := range []string{"Account", "Contact", "Opportunity"} {
					writeFile(t, filepath.Join(root, dir, "objects", object+".object"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><recordTypes><fullName>Owned</fullName><label>Owned</label><active>true</active></recordTypes></CustomObject>`)
					field := "RecordTypeId"
					if i == 0 {
						field = "RecordTypeID"
					}
					body += object + " row" + string(rune('a'+i)) + "=new " + object + "(" + field + "=null); System.assertEquals(null,row" + string(rune('a'+i)) + ".RecordTypeId); System.assertEquals(true," + object + ".SObjectType.getDescribe().fields.getMap().containsKey('RecordTypeId'));"
				}
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}

// P10/ORG-RUN.txt and P10/org.tsv capture B2BCommerce + Communities +
// commerceEnabled; P10/license-only retains the rejection twin. The unknown
// shape is a source-only compatibility control, not a new native assertion.
func TestStandardFieldEvidenceCommerceProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"source-only", "cci-feature-list", "cci-feature-string"} {
			t.Run(api+"/"+shape, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				if shape != "source-only" {
					features := `["B2BCommerce","Communities"]`
					if shape == "cci-feature-string" {
						features = `"B2BCommerce, Communities"`
					}
					writeFile(t, filepath.Join(root, "cumulusci.yml"), "orgs:\n  scratch:\n    dev:\n      config_file: orgs/owned.json\n")
					writeFile(t, filepath.Join(root, "orgs/owned.json"), `{"edition":"Developer","features":`+features+`,"settings":{"communitiesSettings":{"enableNetworksEnabled":true},"commerceSettings":{"commerceEnabled":true}}}`)
				}
				runStandardFieldEvidenceRoutes(t, root, api, "WebStore row=new WebStore(Name='Owned store'); System.assertEquals('Owned store',row.Name);")
			})
		}
	}
}

func runStandardFieldEvidenceRoutes(t *testing.T, root, api, body string) {
	t.Helper()
	const class = "OwnedEvidenceTest"
	namedPath := filepath.Join(root, "force-app/main/default/classes/OwnedEvidenceNamed.cls")
	writeFile(t, namedPath, "public class OwnedEvidenceNamed { public static void sample(){"+body+"} }")
	writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	path := filepath.Join(root, "force-app/main/default/classes", class+".cls")
	writeFile(t, path, "@IsTest private class "+class+" { @IsTest static void sample(){"+body+"} }")
	writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	index := loadTestIndex(t, root)
	for _, route := range []string{"anonymous", "is-test"} {
		analysis := sema.Analyze(index)
		if route == "anonymous" {
			analysis = sema.AnalyzeAnonymous(index, body, api)
		}
		if index.HasErrors() || analysis.HasErrors() {
			t.Fatalf("%s compilation: %v %v", route, index.Diagnostics, analysis.Diagnostics)
		}
	}
	org := orgFromIndex(index)
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"cold", "warm"} {
		t.Run(state, func(t *testing.T) {
			if _, err := runner.execute(program); err != nil {
				t.Fatal(err)
			}
			named, err := vm.CompileAnonymousWithOptions("OwnedEvidenceNamed.sample();", vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := runner.execute(named); err != nil {
				t.Fatal(err)
			}
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
				t.Fatalf("project runner: %+v", run.Summary())
			}
		})
	}
}

// Exact LiveAgent-enabled native membership and query rows at API62/67:
// standard_case_source_evidence/org.tsv:2-5,14-17 and ORG-RUN.txt. Replay the
// retained positive, two neighboring negative fields and Subject boundary.
func TestStandardFieldEvidenceLiveAgentProjectRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_case_source_evidence/"
	raw, err := os.ReadFile(dir + "observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Route, Case, Source, Observation string
		SourceSHA                             string `json:"source_sha256"`
		Compiled                              bool
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 12 {
		t.Fatalf("native rows=%d, want 12", len(rows))
	}
	for _, row := range rows {
		digest := sha256.Sum256([]byte(row.Source))
		if hex.EncodeToString(digest[:]) != row.SourceSHA {
			t.Fatal("native source digest changed")
		}
		if row.Route != "anonymous" || row.Case != "membership" {
			continue
		}
		t.Run(row.API, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row.API+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			definition, err := os.ReadFile(dir + "definition.json")
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(definition))
			var membership map[string]bool
			if err := json.Unmarshal([]byte(row.Observation), &membership); err != nil {
				t.Fatal(err)
			}
			body := "Map<String,Schema.SObjectField> fields=Case.SObjectType.getDescribe().fields.getMap();"
			for _, field := range []string{"SourceId", "HasCommentsUnreadByOwner", "IsVisibleInSelfService", "Subject", "OwnedMissing__c"} {
				want := "false"
				if membership[field] {
					want = "true"
				}
				body += "System.assertEquals(" + want + ",fields.containsKey('" + field + "'),'" + field + "');"
			}
			body += "List<Case> cases=[SELECT SourceId,Subject FROM Case LIMIT 1];"
			runStandardFieldEvidenceRoutes(t, root, row.API, body)
		})
	}
}
