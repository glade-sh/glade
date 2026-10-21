package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Exact captured clock classes retain millisecond operators, epoch and JSON
// controls alongside native member/assertion equality at signed second boundaries.
func TestDatetimeMethodNativeConformance(t *testing.T) {
	var data struct {
		APIVersions            []string          `json:"apiVersions"`
		Routes                 []string          `json:"routes"`
		NativeObservationCount int               `json:"nativeObservationCount"`
		AssertedFacetCount     int               `json:"assertedFacetCount"`
		OrgIDSHA256            string            `json:"orgIdSha256"`
		Carries                []json.RawMessage `json:"carries"`
		SourcesByAPI           map[string]struct {
			ClassName, ClassSource, ClassMetadata, ClassSHA256, ClassMetadataSHA256 string
			Anonymous, AnonymousSHA256, TestClass, TestSource, TestSourceSHA256     string
		} `json:"sourcesByAPI"`
		Cells []struct {
			APIVersion     string         `json:"apiVersion"`
			Route          string         `json:"route"`
			Observation    map[string]any `json:"observation"`
			EvidenceSHA256 string         `json:"evidenceSha256"`
			Native         struct {
				Transport string `json:"transport"`
			} `json:"native"`
		} `json:"cells"`
		D7Evidence struct {
			OrgTSV          string `json:"observationsTsv"`
			OrgTSVSHA256    string `json:"orgTsvSha256"`
			OrgRunSHA256    string `json:"orgRunSha256"`
			RawOrgRunSHA256 string `json:"rawOrgRunSha256"`
		} `json:"d7Evidence"`
	}
	raw, err := os.ReadFile("testdata/conformance/datetime_method_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" || data.NativeObservationCount != 6 || len(data.Cells) != 6 || len(data.SourcesByAPI) != 3 || len(data.Carries) != 0 || data.AssertedFacetCount != 330 {
		t.Fatal("incomplete native clock matrix")
	}
	checkDigest := func(name, digest string) {
		t.Helper()
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != sha256.Size {
			t.Fatalf("invalid native digest: %s", name)
		}
	}
	checkHash := func(name, text, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(text))
		if text == "" || hex.EncodeToString(sum[:]) != want {
			t.Fatalf("owned evidence changed: %s", name)
		}
	}
	checkDigest("org", data.OrgIDSHA256)
	checkDigest("ORG-RUN.txt", data.D7Evidence.OrgRunSHA256)
	checkDigest("raw ORG-RUN.txt", data.D7Evidence.RawOrgRunSHA256)
	checkHash("org.tsv", data.D7Evidence.OrgTSV, data.D7Evidence.OrgTSVSHA256)
	lines := strings.Split(strings.TrimSuffix(data.D7Evidence.OrgTSV, "\n"), "\n")
	if len(lines) != 7 || lines[0] != "row\tapi\tcontext\tvalue" {
		t.Fatal("incomplete native observation table")
	}
	native := map[string]map[string]any{}
	for _, line := range lines[1:] {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 {
			t.Fatal("invalid native observation row")
		}
		route := fields[2]
		if route == "istest" {
			route = "isTest"
		}
		key := fields[1] + ".0/" + route
		if _, exists := native[key]; exists {
			t.Fatal("duplicate native observation")
		}
		var observed map[string]any
		if err := json.Unmarshal([]byte(fields[3]), &observed); err != nil {
			t.Fatal(err)
		}
		native[key] = observed
	}
	seen := map[string]bool{}
	facets := 0
	for _, cell := range data.Cells {
		key := cell.APIVersion + "/" + cell.Route
		if seen[key] || !reflect.DeepEqual(cell.Observation, native[key]) {
			t.Fatalf("unbound/repeated native observation: %s", key)
		}
		seen[key] = true
		checkDigest(key, cell.EvidenceSHA256)
		want, ok := cell.Observation["clock"].(map[string]any)
		if !ok || len(want) != 8 {
			t.Fatalf("missing native clock facets: %s", key)
		}
		facets += datetimeMethodNativeFacetCount(want)
		t.Run(key, func(t *testing.T) {
			source, ok := data.SourcesByAPI[cell.APIVersion]
			if !ok {
				t.Fatal("missing captured API source")
			}
			checkHash("clock class", source.ClassSource, source.ClassSHA256)
			checkHash("clock metadata", source.ClassMetadata, source.ClassMetadataSHA256)
			checkHash("anonymous transport", source.Anonymous, source.AnonymousSHA256)
			checkHash("named transport", source.TestSource, source.TestSourceSHA256)
			if !strings.Contains(source.ClassMetadata, "<apiVersion>"+cell.APIVersion+"</apiVersion>") {
				t.Fatal("clock API metadata mismatch")
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, cell.APIVersion))
			for name, text := range map[string]string{source.ClassName: source.ClassSource, source.TestClass: source.TestSource} {
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, text)
				writeFile(t, path+"-meta.xml", source.ClassMetadata)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native clock source accepted: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = cell.APIVersion
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var observed string
			if cell.Route == "anonymous" {
				if cell.Native.Transport != "debug" {
					t.Fatal("anonymous native transport changed")
				}
				analysis := sema.AnalyzeAnonymous(index, source.Anonymous, cell.APIVersion)
				program, err := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: cell.APIVersion})
				if err != nil || analysis.HasErrors() {
					t.Fatalf("anonymous clock compile: %v %v", err, analysis.Diagnostics)
				}
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if _, payload, found := strings.Cut(line, "OBS|"); found {
						if observed != "" {
							t.Fatal("duplicate clock observation")
						}
						observed = payload
					}
				}
			} else if cell.Route == "isTest" {
				if cell.Native.Transport != "intentional OBS assertion" {
					t.Fatal("named native transport changed")
				}
				cases := Discover(index, Options{SelectedClasses: []string{source.TestClass}, SelectedMethod: "observe"})
				if len(cases) != 1 {
					t.Fatalf("named clock discovery: %d", len(cases))
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named clock compile: %v %v", methodErrors[key], programErrors[key])
				}
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, err := machine.ExecuteInClass(programs[key], cases[0].ClassName)
				var fault *vm.RuntimeError
				if !errors.As(err, &fault) || fault.Type != "System.AssertException" {
					t.Fatalf("missing named clock observation: %v", err)
				}
				_, payload, found := strings.Cut(fault.ExceptionMessage(), "OBS|")
				if !found {
					t.Fatalf("missing named clock marker: %v", err)
				}
				observed = payload
			} else {
				t.Fatal("unknown native clock route")
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(observed), &got); err != nil {
				t.Fatalf("invalid clock payload %q: %v", observed, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native clock=%v; local clock=%v", want, got)
			}
		})
	}
	if facets != data.AssertedFacetCount {
		t.Fatalf("native clock facet count=%d, want %d", facets, data.AssertedFacetCount)
	}
}

func datetimeMethodNativeFacetCount(value any) int {
	switch value := value.(type) {
	case map[string]any:
		n := 0
		for _, item := range value {
			n += datetimeMethodNativeFacetCount(item)
		}
		return n
	case []any:
		n := 0
		for _, item := range value {
			n += datetimeMethodNativeFacetCount(item)
		}
		return n
	default:
		return 1
	}
}
