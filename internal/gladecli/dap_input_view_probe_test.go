package gladecli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/inputview"
	"github.com/glade-sh/glade/internal/project"
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
	entry, readErr := startupcache.Read(root, startupcache.SubdirDAP)
	if readErr != nil || entry == nil {
		t.Fatalf("entry: %v", readErr)
	}
	if !startupcache.Fresh(entry, root, dapCacheVersion) || !dapObservationsHold(root, entry) {
		t.Fatalf("fresh=%v observations=%v raw=%v", startupcache.Fresh(entry, root, dapCacheVersion), dapObservationsHold(root, entry), inputview.Verify(entry.Inputs, nil))
	}
	assertServedWarm(t, root)
}

func TestDAPEventSubscriberMetadataUsesInputView(t *testing.T) {
	root, _ := writeDriftProject(t)
	base := filepath.Join(root, "force-app", "main")
	writeDriftFile(t, filepath.Join(base, "objects", "Alert__e", "Alert__e.object-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Alert</label><pluralLabel>Alerts</pluralLabel><deploymentStatus>Deployed</deploymentStatus><eventType>HighVolume</eventType></CustomObject>`)
	trigger := filepath.Join(base, "triggers", "AlertHandler.trigger")
	writeDriftFile(t, trigger, `trigger AlertHandler on Alert__e (after insert) {}`)
	meta := trigger + "-meta.xml"
	writeDriftFile(t, meta, `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexTrigger>`)
	v := inputview.New()
	p, err := project.LoadWithInputs(root, v)
	if err != nil {
		t.Fatal(err)
	}
	index, _ := buildProjectIndexWithArtifacts(p)
	writeDriftFile(t, meta, `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Inactive</status></ApexTrigger>`)
	_, _ = orgStateFromIndex(root, p, index)
	if v.Err() == nil {
		t.Fatal("event trigger metadata read bypassed the input view")
	}
}

func TestDAPDiscardsCompiledRuntimeAfterPageRemoval(t *testing.T) {
	withoutDAPRetryBackoff(t)
	root, _ := writeDriftProject(t)
	page := filepath.Join(root, "force-app", "main", "pages", "Ghost.page")
	writeDriftFile(t, page, "<apex:page>hi</apex:page>\n")
	writeDriftFile(t, page+"-meta.xml", `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><label>Ghost</label></ApexPage>`)
	attempts := 0
	_, runtime, _, err := loadDAPStartupStateWithSeams(root, dapLoadSeams{
		beforeLoad: func(a int) { attempts = a },
		beforeRecheck: func(a int) {
			if a == 1 {
				if err := os.RemoveAll(filepath.Dir(page)); err != nil {
					t.Fatal(err)
				}
			}
		},
	})
	if err != nil || attempts != 2 || len(runtime.PageNames) != 0 {
		t.Fatalf("attempts=%d pages=%v err=%v", attempts, runtime.PageNames, err)
	}
	assertServedWarm(t, root)
}

func TestDAPRetriesIndexedApexRestoredBeforeCompile(t *testing.T) {
	withoutDAPRetryBackoff(t)
	root := t.TempDir()
	writeTestProject(t, root)
	source := filepath.Join(root, "force-app", "main", "classes", "MathUtil.cls")
	var original fileSnapshot
	attempts := 0
	_, runtime, _, err := loadDAPStartupStateWithSeams(root, dapLoadSeams{
		beforeLoad: func(a int) {
			attempts = a
			if a == 1 {
				original = snapshotFile(t, source)
				writeDriftFile(t, source, "public class MathUtil { public static Integer add(Integer a, Integer b) { return a + b; } public static Integer other() { return 9; } }\n")
			}
		},
		afterIndex: func(a int) {
			if a == 1 {
				original.restore(t)
			}
		},
	})
	if err != nil || attempts != 2 || runtimeHasMethod(runtime, "other") {
		t.Fatalf("attempts=%d wrong method=%v err=%v", attempts, runtimeHasMethod(runtime, "other"), err)
	}
	if sha, ok := publishedManifestSHA(t, root, source); !ok || sha != fileSHA(t, source) {
		t.Fatal("published digest differs from restored source")
	}
	assertServedWarm(t, root)
}

func TestDAPFieldMetadataRestoredBeforeRecheck(t *testing.T) {
	withoutDAPRetryBackoff(t)
	const field = `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Hidden__c</fullName><label>Hidden</label><type>Text</type><length>20</length></CustomField>`
	hasField := func(got dapDriftResult) bool {
		for name := range got.org.Objects["Widget__c"].Definition.Fields {
			if strings.EqualFold(name, "Hidden__c") {
				return true
			}
		}
		return false
	}
	for _, name := range []string{".Hidden__c.field-meta.xml", "Hidden__c.field-meta.xml"} {
		t.Run(name, func(t *testing.T) {
			root, _ := writeDriftProject(t)
			path := filepath.Join(root, "force-app", "main", "objects", "Widget__c", "fields", name)
			writeDriftFile(t, path, field)
			steady := loadDAPWithDrift(t, root, dapLoadSeams{})
			if steady.err != nil {
				t.Fatal(steady.err)
			}
			other, _ := writeDriftProject(t)
			otherPath := filepath.Join(other, "force-app", "main", "objects", "Widget__c", "fields", name)
			writeDriftFile(t, otherPath, field)
			var original fileSnapshot
			restored := loadDAPWithDrift(t, other, dapLoadSeams{
				beforeLoad: func(a int) {
					if a == 1 {
						original = snapshotFile(t, otherPath)
						removeDriftFile(t, otherPath)
					}
				},
				beforeRecheck: func(a int) {
					if a == 1 {
						original.restore(t)
					}
				},
			})
			if restored.err != nil || hasField(restored) != hasField(steady) {
				t.Fatalf("attempts=%d restored=%v steady=%v err=%v", restored.attempts, hasField(restored), hasField(steady), restored.err)
			}
			if hasField(steady) && restored.attempts != 2 {
				t.Fatalf("consumed field restored without retry: attempts=%d", restored.attempts)
			}
			assertServedWarm(t, other)
		})
	}
}

func TestDAPIgnoresFinderChurnDuringBuild(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	attempts := 0
	_, _, _, err := loadDAPStartupStateWithSeams(root, dapLoadSeams{
		beforeLoad: func(attempt int) { attempts = attempt },
		afterIndex: func(attempt int) {
			if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte{byte(attempt)}, 0600); err != nil {
				t.Fatal(err)
			}
		},
	})
	if err != nil || attempts != 1 {
		t.Fatalf("load with Finder churn: attempts=%d, err=%v", attempts, err)
	}
	assertServedWarm(t, root)
}
