package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/inputview"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestViewBackedRuntimeDoesNotPublishSharedCache(t *testing.T) {
	root := t.TempDir()
	writeViewCacheFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`)
	writeViewCacheFile(t, filepath.Join(root, "src", "classes", "ViewClass.cls"), `public class ViewClass { public static Integer value() { return 1; } }`)
	v := inputview.New()
	p, err := project.LoadWithInputs(root, v)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, s)
	sources := newSourceCache()
	sources.seedProjectSnapshot(&p)
	if err := sources.seedBuildArtifacts(index, &artifacts); err != nil {
		t.Fatal(err)
	}
	generation, err := prepareRuntimeGeneration(index, artifacts.SourceDigests, sources)
	if err != nil {
		t.Fatal(err)
	}
	shapedStandardApexTestOrgCache.mu.Lock()
	priorShape := shapedStandardApexTestOrgCache.key
	shapedStandardApexTestOrgCache.mu.Unlock()
	if _, err := CompileProjectRuntimeForProject(index, p, artifacts); err != nil {
		t.Fatal(err)
	}
	shapedStandardApexTestOrgCache.mu.Lock()
	afterShape := shapedStandardApexTestOrgCache.key
	shapedStandardApexTestOrgCache.mu.Unlock()
	if afterShape != priorShape {
		t.Fatal("view-backed org was published to the shared shape cache")
	}
	runtimeCacheMu.RLock()
	_, found := runtimeCache[generation.key]
	runtimeCacheMu.RUnlock()
	if found {
		t.Fatal("view-backed runtime was published to the shared runtime cache")
	}
	expressionResolverMu.Lock()
	for _, entry := range expressionResolvers {
		if strings.HasPrefix(string(entry.key), string(generation.key)+"\x00") {
			found = true
		}
	}
	expressionResolverMu.Unlock()
	if found {
		t.Fatal("view-backed resolver was published to the shared resolver cache")
	}
	entry, err := startupcache.Read(root, startupcache.SubdirTest)
	if err != nil || entry != nil {
		t.Fatalf("test runtime cache = %v, err=%v", entry != nil, err)
	}
}

func writeViewCacheFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}
