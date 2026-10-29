package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestConformanceProjectTemplateIdentityAndLifetime(t *testing.T) {
	cache := newConformanceProjectCache(t)
	files := map[string]string{"force-app/main/default/classes/TemplateProbe.cls": "public class TemplateProbe { public static String value(){return 'ready';} }"}
	var first typesys.Index
	t.Run("first row", func(t *testing.T) { first = cache.index(t, "62.0", files) })
	t.Run("next row", func(t *testing.T) {
		again := cache.index(t, "62.0", files)
		if again.Project.Root != first.Project.Root {
			t.Fatal("identical bytes rebuilt the project")
		}
		if _, err := os.Stat(first.Types[0].File); err != nil {
			t.Fatalf("previous row removed the shared source: %v", err)
		}
	})
	for _, change := range []struct{ api, path, source string }{
		{"67.0", "", ""},
		{"62.0", "force-app/main/default/classes/TemplateProbe.cls", "public class TemplateProbe {}"},
		{"62.0", "force-app/main/default/classes/TemplateProbe.cls-meta.xml", "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>"},
		{"62.0", "force-app/main/default/objects/Entry__c/Entry__c.object-meta.xml", "<CustomObject><label>Entry</label><pluralLabel>Entries</pluralLabel></CustomObject>"},
	} {
		changed := make(map[string]string, len(files)+1)
		for path, source := range files {
			changed[path] = source
		}
		if change.path != "" {
			changed[change.path] = change.source
		}
		if cache.index(t, change.api, changed).Project.Root == first.Project.Root {
			t.Fatalf("different project reused the original: %+v", change)
		}
	}
	if newConformanceProjectCache(t).index(t, "62.0", files).Project.Root == first.Project.Root {
		t.Fatal("separate families shared a project")
	}
	if cache.index(t, "62.0", nil).Project.Root != cache.index(t, "62.0", map[string]string{}).Project.Root {
		t.Fatal("nil and empty file sets differ")
	}
}

func TestConformanceProjectTemplateMatchesFreshBuild(t *testing.T) {
	// Native API62/API67 anonymous rows both return ready:
	// testdata/conformance/api_parallel_cases/native/api{62,67}/org.tsv:2.
	cache := newConformanceProjectCache(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			name := "SharedSetupProbe" + strings.TrimSuffix(api, ".0")
			rel := "force-app/main/default/classes/" + name + ".cls"
			raw, err := os.ReadFile(filepath.Join("testdata/conformance/api_parallel_cases/native-project", rel))
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{rel: string(raw), rel + "-meta.xml": "<ApexClass><apiVersion>" + api + "</apiVersion></ApexClass>"}
			cached := cache.index(t, api, files)
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			for path, source := range files {
				writeFile(t, filepath.Join(root, path), source)
			}
			fresh := loadTestIndex(t, root)
			canonical := func(index typesys.Index) string {
				raw, err := json.Marshal(index)
				if err != nil {
					t.Fatal(err)
				}
				// JSON escapes backslashes on Windows; normalize the encoded root.
				encodedRoot, _ := json.Marshal(index.Project.Root)
				return strings.ReplaceAll(string(raw), string(encodedRoot[1:len(encodedRoot)-1]), "<project>")
			}
			if !reflect.DeepEqual(canonical(cached), canonical(fresh)) {
				t.Fatal("cached index differs from fresh build")
			}
			for i, typ := range cached.Types {
				got, ok := cached.SourceDigest(typ.File)
				want, wantOK := fresh.SourceDigest(fresh.Types[i].File)
				if !ok || !wantOK || got != want {
					t.Fatal("source snapshot digest differs")
				}
			}
			program, err := vm.CompileAnonymousWithOptions("System.debug('NATIVE|' + "+name+".value());", vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, cached, conformanceRunnerOptions{LinkProject: true})
			for row := 0; row < 2; row++ {
				result, err := runner.execute(program)
				if err != nil || !strings.Contains(strings.Join(result.Debug, "\n"), "NATIVE|ready") {
					t.Fatalf("native ready: result=%v err=%v", result, err)
				}
			}
		})
	}
}

func TestConformanceProjectTemplateConcurrentRows(t *testing.T) {
	cache := newConformanceProjectCache(t)
	var mu sync.Mutex
	root := ""
	for row := 0; row < 4; row++ {
		runConformanceSubtestParallel(t, string(rune('a'+row)), func(t *testing.T) {
			index := cache.index(t, "62.0", nil)
			mu.Lock()
			defer mu.Unlock()
			if root != "" && root != index.Project.Root {
				t.Error("concurrent identical rows built multiple projects")
			}
			root = index.Project.Root
		})
	}
}

func TestConformanceProjectTemplateBounded(t *testing.T) {
	cache := newConformanceProjectCache(t)
	first := cache.index(t, "62.0", nil)
	for row := 0; row < conformanceProjectCacheLimit; row++ {
		cache.index(t, "62.0", map[string]string{"row.txt": strings.Repeat("x", row+1)})
	}
	if len(cache.entries) > conformanceProjectCacheLimit {
		t.Fatal("unbounded template retention")
	}
	if cache.index(t, "62.0", nil).Project.Root == first.Project.Root {
		t.Fatal("oldest template was not evicted")
	}
	// Eviction releases the cached index; its source lifetime still covers rows.
	if _, err := os.Stat(filepath.Join(first.Project.Root, "sfdx-project.json")); err != nil {
		t.Fatal(err)
	}
}
