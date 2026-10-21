package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const aggregateSumNumberFixture = "testdata/conformance/sum_number"

type aggregateSumNumberNative struct {
	Outcome           string          `json:"outcome"`
	Value             json.RawMessage `json:"value,omitempty"`
	ExceptionType     string          `json:"exceptionType,omitempty"`
	ExceptionMessage  string          `json:"exceptionMessage,omitempty"`
	RawSHA256Verified bool            `json:"raw_sha256_verified,omitempty"`
}

type aggregateSumNumberCell struct {
	API, Route string
	Native     aggregateSumNumberNative
}

type aggregateSumNumberSource struct {
	Source, SourceSHA256, ClassName, WireID, OriginalApex, OriginalApexSHA256 string
}

type aggregateSumNumberRow struct {
	ID     string
	Routes map[string]aggregateSumNumberSource
	Cells  []aggregateSumNumberCell
}

type aggregateSumNumberData struct {
	APIVersions, Routes                              []string
	ShapeCount, NativeObservationCount, NoAnswerRows int
	RowErrorsFilePresent                             bool
	Files                                            []struct{ Path, SHA256 string }
	Rows                                             []aggregateSumNumberRow
}

func TestAggregateSumNumberOrgConformance(t *testing.T) {
	data := loadAggregateSumNumberConformance(t)
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "AggregateSumNumber", api)
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			object, err := os.ReadFile(filepath.Join(aggregateSumNumberFixture, "setup/SumProbe__c.object"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/SumProbe__c/SumProbe__c.object-meta.xml"), string(object))
			fixtureProject, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := gladeschema.LoadProject(fixtureProject)
			if err != nil {
				t.Fatal(err)
			}
			buildIndex := func(source aggregateSumNumberSource) typesys.Index {
				p := fixtureProject
				p.ApexFiles = append([]string{}, fixtureProject.ApexFiles...)
				if source.ClassName != "" {
					path := filepath.Join(root, "force-app/main/default/classes", source.ClassName+".cls")
					writeFile(t, path, source.Source)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					p.ApexFiles = append(p.ApexFiles, path)
				}
				return typesys.Build(p, metadata)
			}
			index := buildIndex(aggregateSumNumberSource{})
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native setup accepted; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for ordinal, row := range data.Rows {
				t.Run(fmt.Sprintf("case_%02d", ordinal), func(t *testing.T) {
					for _, route := range data.Routes {
						t.Run(route, func(t *testing.T) {
							var cell aggregateSumNumberCell
							for _, candidate := range row.Cells {
								if candidate.API == api && candidate.Route == route {
									cell = candidate
								}
							}
							countRoute := route
							if route == "istest" {
								countRoute = "isTest"
							}
							counts.TrackKind(t, countRoute, "exact", 1)
							source := row.Routes[route]
							if route == "anonymous" {
								analysis := sema.AnalyzeAnonymous(index, source.Source, api)
								program, err := vm.CompileAnonymousWithOptions(source.Source, vm.CompileOptions{APIVersion: api})
								if analysis.HasErrors() || err != nil {
									t.Fatalf("native compilation accepted; Glade rejected: %v %v", analysis.Diagnostics, err)
								}
								_, execErr := base.execute(program)
								assertAggregateSumNumberObservation(t, row.ID, source.WireID, cell.Native, execErr)
								return
							}
							namedIndex := buildIndex(source)
							if analysis := sema.Analyze(namedIndex); namedIndex.HasErrors() || analysis.HasErrors() {
								t.Fatalf("native compilation accepted; Glade rejected: %v %v", namedIndex.Diagnostics, analysis.Diagnostics)
							}
							cases := Discover(namedIndex, Options{SelectedClasses: []string{source.ClassName}, SelectedMethod: "observe"})
							if len(cases) != 1 {
								t.Fatalf("one captured test method required, got %d", len(cases))
							}
							methods, methodErrors := compileTestMethods(cases)
							programs, programErrors := compileTestInvokePrograms(cases)
							key := testCaseKey(cases[0])
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
							}
							named := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
							machine := named.newMachine()
							machine.EnableTestContext()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
							assertAggregateSumNumberObservation(t, row.ID, source.WireID, cell.Native, execErr)
						})
					}
				})
			}
		})
	}
}

func loadAggregateSumNumberConformance(t *testing.T) aggregateSumNumberData {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(aggregateSumNumberFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var data aggregateSumNumberData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.ShapeCount != 32 || data.NativeObservationCount != 192 || len(data.Rows) != 32 || data.NoAnswerRows != 0 || data.RowErrorsFilePresent || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" {
		t.Fatal("incomplete native matrix")
	}
	native := map[string]map[string]aggregateSumNumberNative{}
	listed := map[string]bool{}
	for _, file := range data.Files {
		if listed[file.Path] || filepath.IsAbs(file.Path) || strings.Contains(file.Path, "..") {
			t.Fatal("invalid fixture file")
		}
		listed[file.Path] = true
		raw, err := os.ReadFile(filepath.Join(aggregateSumNumberFixture, file.Path))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			t.Fatalf("captured setup/evidence changed: %s", file.Path)
		}
		if !strings.HasSuffix(file.Path, "/org.tsv") {
			continue
		}
		lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
		if len(lines) != 33 || lines[0] != "id\tgroup\troute\tstatus\tvalue" {
			t.Fatal("incomplete native TSV")
		}
		cells := map[string]aggregateSumNumberNative{}
		for _, line := range lines[1:] {
			fields := strings.SplitN(line, "\t", 5)
			if len(fields) != 5 || cells[fields[0]].Outcome != "" {
				t.Fatal("invalid/repeated TSV row")
			}
			var value aggregateSumNumberNative
			if err := json.Unmarshal([]byte(fields[4]), &value); err != nil || value.Outcome != fields[3] {
				t.Fatal("invalid TSV value")
			}
			cells[fields[0]] = value
		}
		native[file.Path] = cells
	}
	if len(native) != 6 || len(listed) != 14 {
		t.Fatal("missing native evidence or setup")
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) != 2 || len(row.Cells) != 6 {
			t.Fatal("invalid native row")
		}
		seen[row.ID] = true
		for _, route := range data.Routes {
			source, ok := row.Routes[route]
			sum := sha256.Sum256([]byte(source.Source))
			originalSum := sha256.Sum256([]byte(source.OriginalApex))
			if !ok || source.Source == "" || source.WireID == "" || hex.EncodeToString(sum[:]) != source.SourceSHA256 || hex.EncodeToString(originalSum[:]) != source.OriginalApexSHA256 || (route == "istest") != (source.ClassName != "") {
				t.Fatal("captured source hash/route mismatch")
			}
			for _, api := range data.APIVersions {
				matches := 0
				for _, cell := range row.Cells {
					if cell.API != api || cell.Route != route {
						continue
					}
					matches++
					path := "native/api" + strings.TrimSuffix(api, ".0") + "/" + route + "/org.tsv"
					want, ok := native[path][row.ID]
					if !ok || !scaleZeroNumberJSONEqual(cell.Native, want) {
						t.Fatalf("case detached from native TSV: %s/%s/%s", row.ID, api, route)
					}
					if cell.Native.Outcome == "runtime" {
						if !cell.Native.RawSHA256Verified {
							t.Fatal("unverified native observation transport")
						}
					} else if cell.Native.Outcome != "uncaught_"+map[string]string{"anonymous": "anonymous", "istest": "test_failure"}[route] || cell.Native.ExceptionType != "System.MathException" || cell.Native.ExceptionMessage != "System.MathException: Divide by 0" {
						t.Fatal("artifact outcome cannot be asserted")
					}
				}
				if matches != 1 {
					t.Fatal("missing/repeated native cell")
				}
			}
		}
	}
	return data
}

func assertAggregateSumNumberObservation(t *testing.T, id, wireID string, native aggregateSumNumberNative, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("%s: captured outcome missing: %v", id, execErr)
	}
	if native.Outcome != "runtime" {
		// Uncaught built-in exceptions carry the bare type on RuntimeError;
		// Apex reports them in the System namespace.
		gladeType := runtimeErr.Type
		if !strings.Contains(gladeType, ".") {
			gladeType = "System." + gladeType
		}
		if gladeType != native.ExceptionType || gladeType+": "+runtimeErr.ExceptionMessage() != native.ExceptionMessage {
			t.Fatalf("%s: native %s; Glade %v", id, native.ExceptionMessage, execErr)
		}
		return
	}
	marker := "S|" + wireID + "|"
	message := runtimeErr.ExceptionMessage()
	_, payload, found := strings.Cut(message, marker)
	if runtimeErr.Type != "System.AssertException" || !found || strings.Count(message, marker) != 1 {
		t.Fatalf("%s: captured terminal assertion missing: %v", id, execErr)
	}
	var wire struct {
		N    int
		H, V string
	}
	if err := json.Unmarshal([]byte(payload), &wire); err != nil {
		t.Fatalf("invalid observation framing: %s", payload)
	}
	sum := sha256.Sum256([]byte(wire.V))
	if len(wire.V) != wire.N || hex.EncodeToString(sum[:]) != wire.H {
		t.Fatalf("invalid observation length/hash: %s", payload)
	}
	var expected, observed map[string]any
	if json.Unmarshal(native.Value, &expected) != nil || json.Unmarshal([]byte(wire.V), &observed) != nil {
		t.Fatal("invalid observation JSON")
	}
	// The seed guard checks exact numeric inputs and their sum. Its incidental
	// Decimal string scale is separate from the SUM result formatting oracle.
	if expectedSum, guarded := expected["storedSum"]; guarded {
		wantText, wantOK := expectedSum.(string)
		gotText, gotOK := observed["storedSum"].(string)
		want, wantNumber := new(big.Rat).SetString(wantText)
		got, gotNumber := new(big.Rat).SetString(gotText)
		if !wantOK || !gotOK || !wantNumber || !gotNumber || want.Cmp(got) != 0 || observed["storedInputsExact"] != true || observed["effectStarted"] != true {
			t.Fatalf("%s: seed guard mismatch: native %s; Glade %s", id, native.Value, wire.V)
		}
		delete(expected, "storedSum")
		delete(observed, "storedSum")
	}
	if !scaleZeroNumberJSONEqual(expected, observed) {
		t.Fatalf("%s: native %s; Glade %s", id, native.Value, wire.V)
	}
}
