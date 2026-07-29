package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestTestOneShotCacheStatusExplainsParallelMethodBypass(t *testing.T) {
	restoreDiskCache := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restoreDiskCache)
	got := testOneShotCacheStatus(apextest.Options{
		ParallelMethods: true,
		Parallelism:     2,
	})
	for _, want := range []string{
		"bypassed for parallel methods with more than one worker",
		"glade test serve",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status missing %q: %s", want, got)
		}
	}
}

func TestTestOneShotCacheStatusExplainsNoCache(t *testing.T) {
	got := testOneShotCacheStatus(apextest.Options{NoDiskCache: true})
	for _, want := range []string{
		"disabled by --no-cache",
		"will not be read or written",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("status missing %q: %s", want, got)
		}
	}
}

func TestTestWizardReportsFreshCacheAndOneShotParallelBypass(t *testing.T) {
	restoreDiskCache := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restoreDiskCache)
	previousGOMAXPROCS := runtime.GOMAXPROCS(2)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousGOMAXPROCS) })

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatalf("project.Load() error = %v", err)
	}
	entry := startupcache.NewEntry(root, p, typesys.Build(p, gladeschema.Schema{}), storage.NewOrgState(), startupcache.CompiledRuntime{})
	if err := startupcache.Write(&entry, startupcache.SubdirTest); err != nil {
		t.Fatalf("startupcache.Write() error = %v", err)
	}

	var output bytes.Buffer
	if err := writeTestWizard(context.Background(), root, &output); err != nil {
		t.Fatalf("writeTestWizard() error = %v", err)
	}
	for _, want := range []string{
		"cache: fresh\n",
		"one-shot cache: bypassed for parallel methods with more than one worker",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("wizard output missing %q:\n%s", want, output.String())
		}
	}
}
