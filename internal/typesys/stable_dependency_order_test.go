package typesys

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

// These synthetic conflicting dependencies cannot be installed together when
// their component API names collide. They exercise deterministic local index
// ordering, not a claim that Salesforce picks the first package.
func TestStableConflictingDependencyAndGeneratedOrder(t *testing.T) {
	for _, name := range []string{"Cedar", "Maple", "Willow"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			p := project.Project{Root: root}
			var wantClasses, wantFlows []string
			for i := 0; i < 36; i++ {
				dir := filepath.Join(root, fmt.Sprintf("dep%02d", i))
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				className, flowName := name, name
				if i%3 == 0 {
					className = fmt.Sprintf("Other%02d", 36-i)
					flowName = className
				}
				classFile := filepath.Join(dir, className+".cls")
				writeFile(t, classFile, "global class "+className+" {}")
				flowFile := filepath.Join(dir, flowName+".flow-meta.xml")
				dep := &project.Project{Root: dir, ApexFiles: []string{classFile}, FlowFiles: []string{flowFile}}
				p.ManagedPackageDependencies = append(p.ManagedPackageDependencies, project.ManagedPackageDependency{Namespace: "supplier", SourceRoot: dir, Status: "loaded", Project: dep})
				if className == name {
					wantClasses = append(wantClasses, classFile)
					wantFlows = append(wantFlows, flowFile)
				}
			}
			idx := Build(p, schema.Schema{})
			var classes, flows []string
			for _, typ := range idx.Types {
				if typ.Namespace == "supplier" && typ.Name == name {
					classes = append(classes, typ.File)
				}
				if typ.Name == "Flow.Interview.supplier."+name {
					flows = append(flows, typ.File)
				}
			}
			if !reflect.DeepEqual(classes, wantClasses) || !reflect.DeepEqual(flows, wantFlows) {
				t.Fatalf("class sources = %v, want %v; flow sources = %v, want %v; diagnostics = %v", classes, wantClasses, flows, wantFlows, idx.Diagnostics)
			}
		})
	}
}
