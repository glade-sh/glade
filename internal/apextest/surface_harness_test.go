package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

type salesforceSurfaceProject struct {
	t    *testing.T
	root string
}

func newSalesforceSurfaceProject(t *testing.T, sfdxProjectJSON string) salesforceSurfaceProject {
	t.Helper()
	root := t.TempDir()
	project := salesforceSurfaceProject{t: t, root: root}
	writeFile(t, filepath.Join(root, "sfdx-project.json"), sfdxProjectJSON)
	return project
}

func (p salesforceSurfaceProject) writeClass(name, source string) {
	p.t.Helper()
	writeFile(p.t, filepath.Join(p.root, "force-app/main/classes/"+name+".cls"), source)
}

func (p salesforceSurfaceProject) writeField(objectName, fieldName, source string) {
	p.t.Helper()
	writeFile(p.t, filepath.Join(p.root, "force-app/main/default/objects", objectName, "fields", fieldName+".field-meta.xml"), source)
}

func (p salesforceSurfaceProject) writeRecordType(objectName, recordTypeName, source string) {
	p.t.Helper()
	writeFile(p.t, filepath.Join(p.root, "force-app/main/default/objects", objectName, "recordTypes", recordTypeName+".recordType-meta.xml"), source)
}

func (p salesforceSurfaceProject) run() testreport.Run {
	p.t.Helper()
	return Run(loadTestIndex(p.t, p.root), Options{})
}

func (p salesforceSurfaceProject) assertSinglePassingRun() {
	p.t.Helper()
	run := p.run()
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		p.t.Fatalf("summary = %#v cases=%#v problem=%#v", got, run.Suites[0].Cases, run.Suites[0].Cases[0].Problem)
	}
}

func TestSalesforceSurfaceProjectHarnessRunsMinimalTest(t *testing.T) {
	project := newSalesforceSurfaceProject(t, `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	project.writeClass("HarnessSmokeTest", `
@isTest
private class HarnessSmokeTest {
  @isTest static void runs() {
    System.assertEquals(2, 1 + 1);
  }
}
`)

	project.assertSinglePassingRun()
}
