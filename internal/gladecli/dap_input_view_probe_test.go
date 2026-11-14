package gladecli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
)

func TestDAPHiddenSharingReasonDrift(t *testing.T) {
	withoutDAPRetryBackoff(t)
	root, _ := writeDriftProject(t)
	hidden := filepath.Join(root, "force-app", "main", "objects", "Widget__c", "sharingReasons", ".Second__c.sharingReason-meta.xml")
	writeDriftFile(t, hidden, driftSharingReason("Second__c"))
	steady := loadDAPWithDrift(t, root, dapLoadSeams{})
	if steady.err != nil || !strings.Contains(strings.Join(objectSharingReasons(steady.index, "Widget__c"), ","), "Second__c") {
		t.Fatalf("steady reasons=%v err=%v", objectSharingReasons(steady.index, "Widget__c"), steady.err)
	}

	root2, _ := writeDriftProject(t)
	hidden2 := filepath.Join(root2, "force-app", "main", "objects", "Widget__c", "sharingReasons", ".Second__c.sharingReason-meta.xml")
	writeDriftFile(t, hidden2, driftSharingReason("Second__c"))
	var snap fileSnapshot
	got := loadDAPWithDrift(t, root2, dapLoadSeams{
		beforeLoad: func(a int) {
			if a == 1 {
				snap = snapshotFile(t, hidden2)
				_ = os.Remove(hidden2)
			}
		},
		beforeRecheck: func(a int) {
			if a == 1 {
				snap.restore(t)
			}
		},
	})
	if got.err != nil || got.attempts != 2 || !strings.Contains(strings.Join(objectSharingReasons(got.index, "Widget__c"), ","), "Second__c") {
		t.Fatalf("swap attempts=%d reasons=%v err=%v", got.attempts, objectSharingReasons(got.index, "Widget__c"), got.err)
	}

	root3, _ := writeDriftProject(t)
	if _, _, _, err := loadDAPStartupState(root3); err != nil {
		t.Fatal(err)
	}
	writeDriftFile(t, filepath.Join(root3, "force-app", "main", "objects", "Widget__c", "sharingReasons", ".Second__c.sharingReason-meta.xml"), driftSharingReason("Second__c"))
	_, _, _, index, err := loadDAPStartupStateWithIndex(context.Background(), root3)
	if err != nil || index == nil {
		t.Fatalf("hidden addition: warm=%v err=%v", index == nil, err)
	}
}

func TestDAPVisualforcePageSwap(t *testing.T) {
	withoutDAPRetryBackoff(t)
	root, _ := writeDriftProject(t)
	page := filepath.Join(root, "force-app", "main", "pages", "Ghost.page")
	meta := page + "-meta.xml"
	attempts := 0
	_, runtime, _, err := loadDAPStartupStateWithSeams(root, dapLoadSeams{
		beforeLoad: func(a int) { attempts = a },
		afterIndex: func(a int) {
			if a == 1 {
				writeDriftFile(t, page, "<apex:page>hi</apex:page>\n")
				writeDriftFile(t, meta, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexPage xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><label>Ghost</label></ApexPage>\n")
			}
		},
		beforeRecheck: func(a int) {
			if a == 1 {
				_ = os.RemoveAll(filepath.Dir(page))
			}
		},
	})
	entry, _ := startupcache.Read(root, startupcache.SubdirDAP)
	var published []string
	if entry != nil {
		published = entry.Runtime.PageNames
	}
	_, _, _, index, werr := loadDAPStartupStateWithIndex(context.Background(), root)
	if err != nil || attempts != 2 || len(runtime.PageNames) != 0 || len(published) != 0 || index != nil || werr != nil {
		t.Fatalf("attempts=%d served=%v published=%v warm=%v err=%v warmErr=%v", attempts, runtime.PageNames, published, index == nil, err, werr)
	}
}

func TestDAPRetriesTransientRuntimeConfigError(t *testing.T) {
	withoutDAPRetryBackoff(t)
	root, _ := writeDriftProject(t)
	path := filepath.Join(root, "glade.yml")
	original := snapshotFile(t, path)
	got := loadDAPWithDrift(t, root, dapLoadSeams{
		afterIndex: func(attempt int) {
			if attempt == 1 {
				writeDriftFile(t, path, "project: [\n")
			}
		},
		beforeRecheck: func(attempt int) {
			if attempt == 1 {
				original.restore(t)
			}
		},
	})
	if got.err != nil || got.attempts != 2 {
		t.Fatalf("attempts=%d err=%v", got.attempts, got.err)
	}
	assertServedWarm(t, root)
}
