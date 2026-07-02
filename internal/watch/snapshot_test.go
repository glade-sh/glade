package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCaptureSnapshotAndDiffByModtimeAndSize(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "InvoiceService.cls")
	lwcPath := filepath.Join(root, "force-app", "main", "default", "lwc", "accountWorkspace", "accountWorkspace.js")
	fieldPath := filepath.Join(root, "objects", "Invoice__c", "fields", "Amount__c.field-meta.xml")
	ignoredPath := filepath.Join(root, "README.md")
	writeWatchFile(t, classPath, "public class InvoiceService {}")
	writeWatchFile(t, lwcPath, "export default class AccountWorkspace {}")
	writeWatchFile(t, fieldPath, "<CustomField><fullName>Amount__c</fullName></CustomField>")
	writeWatchFile(t, ignoredPath, "ignored")

	before, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Files) != 3 {
		t.Fatalf("snapshot files = %#v", before.Files)
	}

	time.Sleep(time.Nanosecond)
	writeWatchFile(t, classPath, "public class InvoiceService { public void run() {} }")
	writeWatchFile(t, lwcPath, "export default class AccountWorkspace { connectedCallback() {} }")
	addedPath := filepath.Join(root, "InvoiceServiceTest.cls")
	writeWatchFile(t, addedPath, "@IsTest private class InvoiceServiceTest {}")
	if err := os.Remove(fieldPath); err != nil {
		t.Fatal(err)
	}

	after, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	changes := DiffSnapshots(before, after)
	if len(changes) != 4 {
		t.Fatalf("changes = %#v", changes)
	}
	assertChange(t, changes, classPath, ChangeModified, FileKindApexClass)
	assertChange(t, changes, lwcPath, ChangeModified, FileKindLightningWebComponent)
	assertChange(t, changes, addedPath, ChangeAdded, FileKindApexClass)
	assertChange(t, changes, fieldPath, ChangeDeleted, FileKindFieldMeta)
}

func TestCapturePathsSkipsMissingAndIgnoredFiles(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "InvoiceService.cls")
	ignoredPath := filepath.Join(root, "README.md")
	writeWatchFile(t, classPath, "public class InvoiceService {}")
	writeWatchFile(t, ignoredPath, "ignored")

	snapshot, err := CapturePaths([]string{classPath, ignoredPath, filepath.Join(root, "Missing.cls")})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Files) != 1 {
		t.Fatalf("snapshot files = %#v", snapshot.Files)
	}
	if _, ok := snapshot.Files[absPath(t, classPath)]; !ok {
		t.Fatalf("missing class state: %#v", snapshot.Files)
	}
}

func assertChange(t *testing.T, changes []Change, path string, op ChangeOp, kind FileKind) {
	t.Helper()
	for _, change := range changes {
		if change.Path == absPath(t, path) {
			if change.Op != op || change.Kind != kind {
				t.Fatalf("change for %s = %#v", path, change)
			}
			return
		}
	}
	t.Fatalf("missing change for %s in %#v", path, changes)
}

func writeWatchFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func absPath(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
