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

// API67 retained Location Usage L12-13: getter-equivalent properties are readonly.
// Primary SHA256: 51d822efbbc3526388b9716bf421d97a944c87596184db37c9484cb622312109.
// These are semantic analysis controls; no VM, numerical-distance or native claim.
const locationReadonlyAPI = "67.0"
const locationReadonlyNamedPrefix = "public class Probe {\n  public static void run() {\n"
const locationReadonlyNamedSuffix = "\n  }\n}\n"
const locationReadonlyMetadata = "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>"
const locationReadonlyReads = `System.Location p = System.Location.newInstance(28.635308, 77.22496);
System.Location q = System.Location.newInstance(37.7749295, -122.4194155);
Double latitude = p.getLatitude();
Double longitude = p.getLongitude();
Double latitudeProperty = p.latitude;
Double longitudeProperty = p.longitude;
Double otherLatitude = q.getLatitude();
Double otherLongitude = q.getLongitude();
Double otherLatitudeProperty = q.latitude;
Double otherLongitudeProperty = q.longitude;
Double instanceMiles = p.getDistance(q, 'mi');
Double instanceKilometers = p.getDistance(q, 'km');
Double staticMiles = System.Location.getDistance(p, q, 'mi');
Double staticKilometers = System.Location.getDistance(p, q, 'km');
`

type locationReadonlyCase struct {
	name string
	body string
	anonymous bool
	field string
	shadow bool
}

func TestLocationReadOnlyContractAPI67(t *testing.T) {
	for _, tc := range []locationReadonlyCase{
		{name: "anonymousLatitudeWrite", anonymous: true, field: "latitude", body: "System.Location p = System.Location.newInstance(28.635308, 77.22496);\np.latitude = 1.0;\n"},
		{name: "anonymousLongitudeWrite", anonymous: true, field: "longitude", body: "System.Location p = System.Location.newInstance(28.635308, 77.22496);\np.longitude = 1.0;\n"},
		{name: "namedLatitudeWrite", field: "latitude", body: "System.Location p = System.Location.newInstance(28.635308, 77.22496);\np.latitude = 1.0;\n"},
		{name: "namedLongitudeWrite", field: "longitude", body: "System.Location p = System.Location.newInstance(28.635308, 77.22496);\np.longitude = 1.0;\n"},
		{name: "anonymousReadsAndMethods", anonymous: true, body: locationReadonlyReads},
		{name: "namedReadsAndMethods", body: locationReadonlyReads},
		{name: "projectLocationShadowWritable", shadow: true, body: "Location p = new Location();\np.latitude = 1.0;\np.longitude = 2.0;\nSystem.Location builtin = System.Location.newInstance(28.635308, 77.22496);\nDouble latitude = builtin.latitude;\nDouble longitude = builtin.getLongitude();\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result Result
			source := tc.body
			if tc.anonymous {
				index := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: locationReadonlyAPI}}
				result = AnalyzeAnonymous(index, tc.body, locationReadonlyAPI)
			} else {
				source = locationReadonlyNamedPrefix + tc.body + locationReadonlyNamedSuffix
				result = locationReadonlyAnalyzeNamed(t, source, tc.shadow)
			}
			if result.Project.SourceAPIVersion != locationReadonlyAPI {
				t.Fatalf("LOCATION_PROFILE_STOP [%s]: got %q", tc.name, result.Project.SourceAPIVersion)
			}
			diagnostics := append([]diagnostic.Diagnostic{}, result.Diagnostics...)
			for i := range diagnostics {
				diagnostics[i].File = filepath.Base(diagnostics[i].File)
				if result.Diagnostics[i].File == "" { diagnostics[i].File = "" }
			}
			lhs := ""
			start, end := -1, -1
			if tc.field != "" {
				lhs = "p." + tc.field
				start = strings.Index(source, lhs + " =")
				end = start + len(lhs)
				if start < 0 { t.Fatalf("LOCATION_SETUP_STOP [%s]: assignment span absent", tc.name) }
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
			}{tc.name, locationReadonlyAPI, tc.anonymous, hex.EncodeToString(bodySum[:]), lhs, start, end, diagnostics}
			encoded, err := json.Marshal(observation)
			if err != nil { t.Fatalf("LOCATION_SETUP_STOP [%s]: %v", tc.name, err) }
			t.Logf("LOCATION_OBSERVATION %s", encoded)
			if tc.field == "" {
				if len(diagnostics) != 0 { t.Fatalf("LOCATION_CONTROL_STOP [%s]: %s", tc.name, encoded) }
				return
			}
			if len(diagnostics) == 0 {
				t.Fatalf("LOCATION_READONLY_MISSING [%s]: %s", tc.name, encoded)
			}
			for _, item := range diagnostics {
				fileOK := (tc.anonymous && item.File == "") || (!tc.anonymous && item.File == "Probe.cls")
				codeOK := item.Code == "GLADESEMA027" || item.Code == "GLADESEMA028"
				if item.Severity != diagnostic.Error || !codeOK || !fileOK || item.Range == nil || item.Range.Start.Offset != start || item.Range.End.Offset != end || !strings.Contains(item.Message, lhs) {
					t.Fatalf("LOCATION_UNRELATED_STOP [%s]: %s", tc.name, encoded)
				}
			}
		})
	}
}

func locationReadonlyAnalyzeNamed(t *testing.T, source string, shadow bool) Result {
	t.Helper()
	root := t.TempDir()
	probe := filepath.Join(root, "Probe.cls")
	writeSemaFile(t, probe, source)
	writeSemaFile(t, probe + "-meta.xml", locationReadonlyMetadata)
	paths := []string{probe}
	if shadow {
		local := filepath.Join(root, "Location.cls")
		writeSemaFile(t, local, "public class Location { public Double latitude; public Double longitude; }")
		writeSemaFile(t, local + "-meta.xml", locationReadonlyMetadata)
		paths = append(paths, local)
	}
	for _, path := range paths {
		metadata, err := os.ReadFile(path + "-meta.xml")
		if err != nil || string(metadata) != locationReadonlyMetadata { t.Fatalf("LOCATION_METADATA_STOP: %s: %v", filepath.Base(path), err) }
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: locationReadonlyAPI, ApexFiles: paths}, schema.Schema{})
	seen := map[string]bool{}
	for _, typ := range index.Types {
		for _, path := range paths {
			if typ.File == path && (typ.Name == "Probe" || typ.Name == "Location") {
				if typ.EffectiveAPIVersion != locationReadonlyAPI { t.Fatalf("LOCATION_EFFECTIVE_API_STOP: %s: %q", typ.Name, typ.EffectiveAPIVersion) }
				seen[path] = true
			}
		}
	}
	if len(seen) != len(paths) { t.Fatalf("LOCATION_TYPE_SETUP_STOP: got %d classes want %d", len(seen), len(paths)) }
	if len(index.Diagnostics) != 0 { t.Fatalf("LOCATION_INDEX_STOP: %#v", index.Diagnostics) }
	return Analyze(index)
}
