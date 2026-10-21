package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// captures omitted-keyword async callbacks under with, without and omitted
// test entries. Only the terminal observation assertion changes for local runs.
// Event principal=other does not identify that user or its permissions, so its
// count alone cannot distinguish sharing behavior from principal visibility.
func TestB44XAsyncSharingOrgConformance(t *testing.T) {
	var fixture struct {
		APIs  []string
		Scope string
		Rows  []struct {
			ID, TestClass, Source, SourceRowSHA256, Variant string
			APIs                                            []string
			Classes, ClassAPIs, Metadata, Expected          map[string]string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/b44x_async_sharing.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if strings.Join(fixture.APIs, ",") != "62.0,67.0" || len(fixture.Rows) != 28 || fixture.Scope == "" {
		t.Fatal("invalid captured B44X matrix")
	}
	var ids []string
	for _, row := range fixture.Rows {
		ids = append(ids, row.ID+row.Variant)
		if strings.Join(row.APIs, ",") != "62.0,67.0" || len(row.Expected) != 2 || len(row.SourceRowSHA256) != 64 || row.Classes[row.TestClass] == "" {
			t.Fatalf("invalid captured row %s", row.ID)
		}
		wantSource := "original"
		if strings.HasSuffix(row.ID, "E") {
			wantSource = "correctedEvent"
			if row.Variant == "principal" {
				wantSource = "principalEvent"
			}
			if row.Variant == "profile" {
				wantSource = "profileEvent"
			}
		}
		if (row.Variant != "" && row.Variant != "principal" && row.Variant != "profile") || row.Source != wantSource {
			t.Fatalf("wrong source for %s", row.ID)
		}
		for _, api := range row.APIs {
			expected := row.Expected[api]
			if !strings.HasPrefix(expected, "observed=") || !strings.HasSuffix(expected, ";with=0;without=2;user=0") {
				t.Fatalf("missing controlled native answer for %s@%s", row.ID, api)
			}
			eventExpected := "observed=callback=2,principal=other;with=0;without=2;user=0"
			if row.Variant == "principal" {
				eventExpected = "observed=callback=2,principal=other,type=AutomatedProcess,userRows=1,typeMatch=true,admin=false,profileSets=0,viewAll=unknown,modifyAll=unknown,with=2,without=2;with=0;without=2;user=0"
			}
			if row.Variant == "profile" {
				eventExpected = "observed=callback=2,principal=other,type=AutomatedProcess,userRows=1,typeMatch=true,admin=false,infoProfileNull=false,userProfileNull=false,profileMatch=true,profileSets=0,viewAll=unknown,modifyAll=unknown,with=2,without=2;with=0;without=2;user=0"
			}
			if strings.HasSuffix(row.ID, "E") && expected != eventExpected {
				t.Fatalf("event principal boundary changed for %s@%s", row.ID, api)
			}
			if row.ID == "B44X05Q" || row.ID == "B44X05F" {
				count := "2"
				if api == "67.0" {
					count = "0"
				}
				if expected != "observed="+count+";with=0;without=2;user=0" {
					t.Fatal("invalid native capture controls")
				}
			}
		}
	}
	if strings.Join(ids, " ") != "B44X01Q B44X01B B44X01F B44X01S B44X01Z B44X01E B44X02Q B44X02B B44X02F B44X02S B44X02Z B44X02E B44X03Q B44X03F B44X04Q B44X04F B44X05Q B44X05F B44X05B B44X05S B44X05Z B44X05E B44X01Eprincipal B44X02Eprincipal B44X05Eprincipal B44X01Eprofile B44X02Eprofile B44X05Eprofile" {
		t.Fatalf("captured row selection changed: %v", ids)
	}
	for _, row := range fixture.Rows {
		for _, api := range row.APIs {
			t.Run(row.ID+row.Variant+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				base := filepath.Join(root, "force-app/main/default")
				for path, source := range row.Metadata {
					writeFile(t, filepath.Join(base, path), source)
				}
				terminal := "System.assert(false,'P|" + row.ID + "|observed='+value+';with='+visibleWith+';without='+visibleWithout+';user='+visibleUser);"
				if strings.Count(row.Classes[row.TestClass], terminal) != 1 {
					t.Fatal("missing exact-source terminal observation")
				}
				observed := "'observed='+value+';with='+visibleWith+';without='+visibleWithout+';user='+visibleUser"
				expected := row.Expected[api]
				assertion := "System.assert(" + conformanceApexString(expected) + ".equals(" + observed + "),'" + row.ID + " actual <'+" + observed + "+'>');"
				var names []string
				for name := range row.Classes {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					source := row.Classes[name]
					if name == row.TestClass {
						source = strings.Replace(source, terminal, assertion, 1)
					}
					version := api
					if fixed := row.ClassAPIs[name]; fixed != "" {
						version = fixed
					}
					path := filepath.Join(base, "classes", name+".cls")
					writeFile(t, path, source)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+version+"</apiVersion></ApexClass>")
				}
				run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
				got := run.Summary()
				t.Logf("%s@%s native <%s> summary=%#v", row.ID, api, expected, got)
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						if result.Status != testreport.StatusPass {
							t.Errorf("%s.%s native <%s>: status %s problem %#v", result.ClassName, result.MethodName, expected, result.Status, result.Problem)
						}
					}
				}
				if got.Total != 1 || got.Passed != 1 {
					t.Fatalf("summary = %#v", got)
				}
			})
		}
	}
}
