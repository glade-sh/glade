package apextest

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Exercise compiled static-field templates and per-worker runtime cloning.
// Apex clone/member semantics are pinned by primitive_set_receiver/org.tsv;
// independent runtime ownership is an internal isolation invariant.
func TestPrimitiveSetEmptyCloneProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, frozen := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/frozen-%v", api, frozen), func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				classes := filepath.Join(root, "force-app", "main", "default", "classes")
				writeFile(t, filepath.Join(classes, "CollectionCloneProbe.cls"), `public class CollectionCloneProbe {
    public static Set<String> words = new Set<String>();
    public static Set<Id> ids = new Set<Id>();
    public static Integer size() { return words.size() + ids.size(); }
    public static List<Object> copies() { return new List<Object>{words.clone(), ids.clone()}; }
}`)
				writeFile(t, filepath.Join(classes, "CollectionCloneProbe.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				index := loadTestIndex(t, root)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
				template := runner.newMachine()
				if value, err := template.CallStatic("CollectionCloneProbe.size", nil); err != nil || value.Int != 0 {
					t.Fatalf("initialize: %v, %v", value, err)
				}
				template.FreezeClassLookup()
				clones := []*vm.VM{template.CloneRuntime(nil), template.CloneRuntime(nil)}
				if frozen {
					clones = []*vm.VM{template.CloneRuntimeFrozenShared(nil), template.CloneRuntimeFrozenShared(nil)}
				}
				// Both Apex clone calls use the same empty source in the project runtime.
				// Bind the returned Sets directly as anonymous inputs, so argument type
				// transport cannot rebuild an index and mask shared clone ownership.
				for _, clone := range clones {
					value, err := template.CallStatic("CollectionCloneProbe.copies", nil)
					if err != nil || len(value.List) != 2 {
						t.Fatalf("project clone: %v, %v", value, err)
					}
					clone.Globals["words"], clone.Globals["ids"] = value.List[0], value.List[1]
				}
				var group sync.WaitGroup
				start := make(chan struct{})
				for i, clone := range clones {
					group.Add(1)
					go func(i int, clone *vm.VM) {
						defer group.Done()
						<-start
						program, err := vm.CompileAnonymousWithOptions(fmt.Sprintf("words.add('%d'); ids.add((Id)'001%012d'); System.assertEquals(1, words.size()); System.assertEquals(1, ids.size());", i, i+1), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Error(err)
							return
						}
						if _, err := clone.Execute(program); err != nil {
							t.Errorf("worker %d append: %v", i, err)
						}
					}(i, clone)
				}
				close(start)
				group.Wait()
				if value, err := template.CallStatic("CollectionCloneProbe.size", nil); err != nil || value.Int != 0 {
					t.Fatalf("template changed: %v, %v", value, err)
				}
			})
		}
	}
}
