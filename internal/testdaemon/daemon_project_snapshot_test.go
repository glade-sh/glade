package testdaemon

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

func TestDaemonOptionsUsePublishedProjectSnapshot(t *testing.T) {
	published := project.Project{Root: "published", SourceAPIVersion: "67.0"}
	caller := project.Project{Root: "caller"}
	got := optionsForGeneration(apextest.Options{ProjectSnapshot: &caller}, daemonGeneration{project: published})
	if got.ProjectSnapshot == nil || !reflect.DeepEqual(*got.ProjectSnapshot, published) {
		t.Fatalf("runner project = %#v, want published project %#v", got.ProjectSnapshot, published)
	}
}

func TestDaemonProjectSnapshotRoutes(t *testing.T) {
	opts := apextest.Options{NoDiskCache: true, Parallelism: 1}
	routes := []struct {
		name string
		run  func(*Daemon) (testreport.Run, error)
	}{
		{"filter", func(d *Daemon) (testreport.Run, error) { return d.RunFilter("SnapshotTest"), nil }},
		{"options", func(d *Daemon) (testreport.Run, error) { return d.RunOptions(opts), nil }},
		{"selection", func(d *Daemon) (testreport.Run, error) {
			return d.RunSelectionContext(context.Background(), opts, watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"SnapshotTest"}}), nil
		}},
		{"changed_since", func(d *Daemon) (testreport.Run, error) {
			run, _, err := d.RunChangedSince("HEAD")
			return run, err
		}},
		{"changed_since_options", func(d *Daemon) (testreport.Run, error) {
			run, _, err := d.RunChangedSinceOptions("HEAD", opts)
			return run, err
		}},
		{"request", func(d *Daemon) (testreport.Run, error) {
			run, _, _, err := d.RunRequestV1(context.Background(), NewRunRequestV1(opts, "", 0, 0, false), nil, nil)
			return run, err
		}},
		{"request_changed_since", func(d *Daemon) (testreport.Run, error) {
			run, _, _, err := d.RunRequestV1(context.Background(), NewRunRequestV1(opts, "HEAD", 0, 0, false), nil, nil)
			return run, err
		}},
		{"request_shard", func(d *Daemon) (testreport.Run, error) {
			run, _, _, err := d.RunRequestV1(context.Background(), NewRunRequestV1(opts, "", 1, 0, false), nil, nil)
			return run, err
		}},
		{"request_selected_method", func(d *Daemon) (testreport.Run, error) {
			selected := opts
			selected.SelectedClasses = []string{"SnapshotTest"}
			selected.SelectedMethod = "passes"
			run, _, _, err := d.RunRequestV1(context.Background(), NewRunRequestV1(selected, "", 0, 0, false), nil, nil)
			return run, err
		}},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range routes {
			t.Run(api+"/"+route.name, func(t *testing.T) {
				t.Cleanup(apextest.InvalidateRuntimeCaches)
				root := t.TempDir()
				manifest := filepath.Join(root, "sfdx-project.json")
				writeFile(t, manifest, fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				classPath := filepath.Join(root, "force-app/main/default/classes/SnapshotTest.cls")
				source := `@IsTest private class SnapshotTest { @IsTest static void passes() { System.assertEquals(2, 1 + 1); } }`
				writeFile(t, classPath, source)
				writeFile(t, classPath+"-meta.xml", fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
				protocolWiringGit(t, root, "init", "-q")
				protocolWiringGit(t, root, "add", ".")
				protocolWiringGit(t, root, "-c", "user.name=Glade Tests", "-c", "user.email=tests@glade.local", "-c", "commit.gpgsign=false", "commit", "-qm", "baseline")
				writeFile(t, classPath, source+"\n")
				d, err := New(root)
				if err != nil {
					t.Fatal(err)
				}
				// The runner must use the generation published before this disk edit.
				writeFile(t, manifest, "{\n")
				run, err := route.run(d)
				if err != nil {
					t.Fatal(err)
				}
				if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
					t.Fatalf("captured project run summary = %#v; suites=%#v", summary, run.Suites)
				}
			})
		}
	}
}
