package apextest

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestHierarchyNameOwnerNativeConformance(t *testing.T) {
	var data struct {
		Fixtures map[string]struct{ Sources map[string]string }
		Cells    []struct {
			ID, API, Route, Stage, Fixture string
			ExpectedNative                 map[string]any
		}
	}
	raw, err := os.ReadFile(filepath.Join(hierarchyNameOwnerFixtureDir, "native-core.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	matched := 0
	for _, cell := range data.Cells {
		if cell.Route != "istest" || cell.Stage != "runtime" {
			continue // Metadata rejections and uncaptured routes supply no runtime oracle.
		}
		matched++
		t.Run(cell.ID+"/"+cell.API+"/"+cell.Route, func(t *testing.T) {
			classes, objects := map[string]string{}, map[string]string{}
			for path, source := range data.Fixtures[cell.Fixture].Sources {
				name := filepath.Base(path)
				if strings.HasSuffix(name, ".cls") {
					classes[strings.TrimSuffix(name, ".cls")] = source
				} else if strings.HasSuffix(name, ".object-meta.xml") {
					objects[strings.TrimSuffix(name, ".object-meta.xml")] = source
				}
			}
			index := nativeBoundaryIndex(t, cell.API, classes, objects)
			if index.HasErrors() || sema.Analyze(index).HasErrors() {
				t.Fatal("native fixture declarations rejected")
			}
			run := Run(index, Options{SelectedClasses: []string{"HierarchyNameOwner" + cell.ID}, NoDiskCache: true, Parallelism: 1})
			var messages []string
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					if c.Problem == nil || c.Problem.Type != "System.AssertException" {
						t.Fatalf("missing observation transport: %+v", c)
					}
					messages = append(messages, c.Problem.Message)
				}
			}
			// Keep the captured setup and all 32 named assertion chunks intact.
			tag := cell.ID + "-" + cell.API + "-T"
			hierarchyNameOwnerCheckChunks(t, tag, messages)
			chunks := make([]string, 32)
			for _, message := range messages {
				_, encoded, _ := strings.Cut(message, "HNAME_RESULT|"+tag+"|")
				var part struct {
					Part  int
					Chunk string
				}
				if err := json.Unmarshal([]byte(encoded), &part); err != nil {
					t.Fatal(err)
				}
				chunks[part.Part] = part.Chunk
			}
			payload, err := base64.StdEncoding.DecodeString(strings.Join(chunks, ""))
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(payload, &got); err != nil {
				t.Fatal(err)
			}
			context := got["context"].(map[string]any)
			identities := map[string]string{
				context["organizationId"].(string): "[ORG_ID]",
				context["userId"].(string):         "[USER_ID]",
				context["profileId"].(string):      "[PROFILE_ID]",
			}
			// The native export projects identities, not the JSON display width
			// of Id values. Stored owner strings can use the 15-character form.
			for id, token := range identities {
				identities[id[:15]] = token
			}
			if id, ok := got["insertedId"].(string); ok {
				identities[id] = "[RECORD_ID18]"
				identities[id[:15]] = "[RECORD_ID15]"
			}
			var project func(any) any
			project = func(value any) any {
				switch v := value.(type) {
				case string:
					if token, ok := identities[v]; ok {
						return token
					}
				case map[string]any:
					for key, child := range v {
						v[key] = project(child)
					}
				case []any:
					for i, child := range v {
						v[i] = project(child)
					}
				}
				return value
			}
			project(got)
			if _, measured := got["insertedId"]; measured {
				got["insertedId"] = "[RECORD_ID18]"
			}
			if !reflect.DeepEqual(got, cell.ExpectedNative) {
				actual, _ := json.Marshal(got)
				want, _ := json.Marshal(cell.ExpectedNative)
				t.Fatalf("native=%s; local=%s", want, actual)
			}
		})
	}
	if matched != 6 {
		t.Fatalf("matched %d named native runtime cells, want 6", matched)
	}
}
