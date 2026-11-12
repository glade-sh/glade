package gladecli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/startupcache"
)

func runtimeHasMethod(runtime apextest.CompiledProjectRuntime, fragment string) bool {
	for key := range runtime.Methods {
		if strings.Contains(strings.ToLower(key), fragment) {
			return true
		}
	}
	return false
}

// An edit between the index build and cache publication must not produce an
// entry that pairs the old runtime with the new file fingerprint.
func TestLoadDAPStartupStateSkipsPublicationWhenInputsDriftDuringBuild(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	source := filepath.Join(root, "force-app", "main", "classes", "MathUtil.cls")

	edited := false
	dapBeforePublishHook = func() {
		if edited {
			return
		}
		edited = true
		writeTestFile(t, source, `
public class MathUtil {
  public static Integer add(Integer a, Integer b) {
    return a + b;
  }
  public static Integer driftmarker(Integer a) {
    return a;
  }
}
`)
	}
	t.Cleanup(func() { dapBeforePublishHook = nil })

	_, first, _, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if runtimeHasMethod(first, "driftmarker") {
		t.Fatal("first runtime unexpectedly saw the mid-build edit")
	}
	if !edited {
		t.Fatal("publish hook did not run")
	}
	if entry, err := startupcache.Read(root, startupcache.SubdirDAP); err != nil {
		t.Fatalf("read cache: %v", err)
	} else if entry != nil {
		t.Fatal("cache entry was published although inputs changed during the build")
	}

	dapBeforePublishHook = nil
	_, second, _, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if !runtimeHasMethod(second, "driftmarker") {
		t.Fatal("second load served a stale runtime without the edited method")
	}
	if entry, err := startupcache.Read(root, startupcache.SubdirDAP); err != nil || entry == nil {
		t.Fatalf("stable second build did not publish an entry: entry=%v err=%v", entry != nil, err)
	}
}

func TestLoadDAPStartupStateSkipsPublicationWhenNonApexInputDrifts(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	projectFile := filepath.Join(root, "sfdx-project.json")
	dapBeforePublishHook = func() {
		writeTestFile(t, projectFile, `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"64.0"}`)
	}
	t.Cleanup(func() { dapBeforePublishHook = nil })

	if _, _, _, err := loadDAPStartupState(root); err != nil {
		t.Fatal(err)
	}
	if entry, _ := startupcache.Read(root, startupcache.SubdirDAP); entry != nil {
		t.Fatal("cache entry was published although project config changed during the build")
	}
	if _, err := os.Stat(filepath.Join(root, ".glade", "dap", "startup.json")); err == nil {
		t.Fatal("startup cache file exists after drift")
	}
}
