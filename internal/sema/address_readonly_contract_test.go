package sema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// API67 retained Address Usage L12-13: getter-equivalent properties are readonly.
// Primary SHA256: da0e81e27d156c081e904e9bec3bfbd64576f86a5cd6f2729f02cf63adb47bfd.
// Semantic analysis only: null receivers are never VM-executed.
// Both read controls must pass; unknown type/property/getter diagnostics are STOP.
const addressReadonlyAPI = "67.0"
const addressReadonlyNamedPrefix = "public class Probe {\n  public static void run() {\n"
const addressReadonlyNamedSuffix = "\n  }\n}\n"
const addressReadonlyMetadata = "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>"
const addressReadonlyReads = `System.Address addr = null;
String propertyCity = addr.city;
String getterCity = addr.getCity();
`

type addressReadonlyCase struct {
	name string
	body string
	anonymous bool
	field string
	shadow bool
	schemaControl bool
}

func TestAddressReadOnlyContractAPI67(t *testing.T) {
	for _, tc := range []addressReadonlyCase{
		{name: "anonymousCityWrite", anonymous: true, field: "city", body: "System.Address addr = null;\naddr.city = 'Paris';\n"},
		{name: "namedCityWrite", field: "city", body: "System.Address addr = null;\naddr.city = 'Paris';\n"},
		{name: "anonymousCityRead", anonymous: true, body: addressReadonlyReads},
		{name: "namedCityRead", body: addressReadonlyReads},
		{name: "schemaAddressCityWritable", schemaControl: true, body: "Schema.Address addr = null;\naddr.City = 'Paris';\n"},
		{name: "projectAddressCityWritable", shadow: true, body: "Address addr = new Address();\naddr.city = 'Paris';\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result Result
			source := tc.body
			if tc.anonymous {
				index := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: addressReadonlyAPI}}
				result = AnalyzeAnonymous(index, tc.body, addressReadonlyAPI)
			} else {
				source = addressReadonlyNamedPrefix + tc.body + addressReadonlyNamedSuffix
				result = addressReadonlyAnalyzeNamed(t, source, tc.shadow, tc.schemaControl)
			}
			if result.Project.SourceAPIVersion != addressReadonlyAPI {
				t.Fatalf("ADDRESS_PROFILE_STOP [%s]: got %q", tc.name, result.Project.SourceAPIVersion)
			}
			diagnostics := append([]diagnostic.Diagnostic{}, result.Diagnostics...)
			for i := range diagnostics {
				diagnostics[i].File = filepath.Base(diagnostics[i].File)
				if result.Diagnostics[i].File == "" { diagnostics[i].File = "" }
			}
			lhs := ""
			start, end := -1, -1
			if tc.field != "" {
				lhs = "addr." + tc.field
				start = strings.Index(source, lhs + " =")
				end = start + len(lhs)
				if start < 0 { t.Fatalf("ADDRESS_SETUP_STOP [%s]: assignment span absent", tc.name) }
			}
			bodySum := sha256.Sum256([]byte(tc.body))
			observation := struct {
				Case string `json:"case"`
				API string `json:"api"`
				Anonymous bool `json:"anonymous"`
				BodySHA256 string `json:"bodySha256"`
				LHS string `json:"lhs"`
				Start int `json:"start"`
				End int `json:"end"`
				Diagnostics []diagnostic.Diagnostic `json:"diagnostics"`
			}{tc.name, addressReadonlyAPI, tc.anonymous, hex.EncodeToString(bodySum[:]), lhs, start, end, diagnostics}
			encoded, err := json.Marshal(observation)
			if err != nil { t.Fatalf("ADDRESS_SETUP_STOP [%s]: %v", tc.name, err) }
			t.Logf("ADDRESS_OBSERVATION %s", encoded)
			if tc.field == "" {
				if len(diagnostics) != 0 { t.Fatalf("ADDRESS_CONTROL_STOP [%s]: %s", tc.name, encoded) }
				return
			}
			if len(diagnostics) == 0 {
				t.Fatalf("ADDRESS_READONLY_MISSING [%s]: %s", tc.name, encoded)
			}
			for _, item := range diagnostics {
				fileOK := (tc.anonymous && item.File == "") || (!tc.anonymous && item.File == "Probe.cls")
				codeOK := item.Code == "GLADESEMA027" || item.Code == "GLADESEMA028"
				if item.Severity != diagnostic.Error || !codeOK || !fileOK || item.Range == nil || item.Range.Start.Offset != start || item.Range.End.Offset != end || !strings.Contains(item.Message, lhs) {
					t.Fatalf("ADDRESS_UNRELATED_STOP [%s]: %s", tc.name, encoded)
				}
			}
		})
	}
}

func addressReadonlyAnalyzeNamed(t *testing.T, source string, shadow, schemaControl bool) Result {
	t.Helper()
	root := t.TempDir()
	probe := filepath.Join(root, "Probe.cls")
	writeSemaFile(t, probe, source)
	writeSemaFile(t, probe + "-meta.xml", addressReadonlyMetadata)
	paths := []string{probe}
	if shadow {
		local := filepath.Join(root, "Address.cls")
		writeSemaFile(t, local, "public class Address { public String city; }")
		writeSemaFile(t, local + "-meta.xml", addressReadonlyMetadata)
		paths = append(paths, local)
	}
	for _, path := range paths {
		metadata, err := os.ReadFile(path + "-meta.xml")
		if err != nil || string(metadata) != addressReadonlyMetadata { t.Fatalf("ADDRESS_METADATA_STOP: %s: %v", filepath.Base(path), err) }
	}
	schemaModel := schema.Schema{}
	if schemaControl {
		schemaModel.Objects = []schema.Object{{Name: "Address", Fields: []schema.Field{{Name: "City", Type: "String"}}}}
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: addressReadonlyAPI, ApexFiles: paths}, schemaModel)
	seen := map[string]bool{}
	for _, typ := range index.Types {
		for _, path := range paths {
			if typ.File == path && (typ.Name == "Probe" || typ.Name == "Address") {
				if typ.EffectiveAPIVersion != addressReadonlyAPI { t.Fatalf("ADDRESS_EFFECTIVE_API_STOP: %s: %q", typ.Name, typ.EffectiveAPIVersion) }
				seen[path] = true
			}
		}
	}
	if len(seen) != len(paths) { t.Fatalf("ADDRESS_TYPE_SETUP_STOP: got %d classes want %d", len(seen), len(paths)) }
	if len(index.Diagnostics) != 0 { t.Fatalf("ADDRESS_INDEX_STOP: %#v", index.Diagnostics) }
	return Analyze(index)
}
