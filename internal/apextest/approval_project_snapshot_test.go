package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestApprovalProjectMetadataSnapshotAndWarmRefresh(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ApprovalSnapshot.cls"), `public class ApprovalSnapshot { public static void run() {} }`)
	file := filepath.Join(root, "force-app/main/default/approvalProcesses/Account.Snapshot.approvalProcess-meta.xml")
	body := `<ApprovalProcess xmlns="http://soap.sforce.com/2006/04/metadata"><active>true</active><label>Original</label><entryCriteria><formula>TRUE</formula></entryCriteria></ApprovalProcess>`
	writeFile(t, file, body)
	index := loadTestIndex(t, root)
	sources := newSourceCache()
	generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	if err := sources.captureProjectRuntimeMetadata(index, &generation); err != nil {
		t.Fatal(err)
	}
	first, _, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, file, strings.Replace(body, "Original", "Changed", 1))
	captured, err := sources.readCapturedProjectMetadata(file)
	if err != nil || string(captured) != body {
		t.Fatalf("captured metadata changed: %v", err)
	}
	if err := sources.validateCapturedSourceGenerationRaw(); err == nil {
		t.Fatal("edit did not invalidate captured metadata")
	}
	next, _, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	if first == next {
		t.Fatal("metadata edit reused stale runtime")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	last, _, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
	if err != nil {
		t.Fatal(err)
	}
	if last == next {
		t.Fatal("metadata deletion reused stale runtime")
	}
}
