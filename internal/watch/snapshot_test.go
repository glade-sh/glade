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

func TestCaptureSnapshotTracksAuthoritativeProjectConfigs(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "sfdx-project.json")
	configPath := filepath.Join(root, "glade.yml")
	writeWatchFile(t, manifestPath, `{"sourceApiVersion":"63.0","packageDirectories":[]}`)
	writeWatchFile(t, configPath, "project:\n  defaultNamespace: localpkg\n")

	before, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := before.Files[manifestPath]; !ok {
		t.Fatalf("snapshot omitted %s: %#v", manifestPath, before.Files)
	}
	if _, ok := before.Files[configPath]; !ok {
		t.Fatalf("snapshot omitted %s: %#v", configPath, before.Files)
	}

	writeWatchFile(t, manifestPath, `{"sourceApiVersion":"64.00","packageDirectories":[]}`)
	writeWatchFile(t, configPath, "project:\n  defaultNamespace: otherpackage\n")
	after, err := CaptureSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	changes := DiffSnapshots(before, after)
	assertChange(t, changes, manifestPath, ChangeModified, FileKindIgnored)
	assertChange(t, changes, configPath, ChangeModified, FileKindIgnored)
}

func TestCaptureScopeTracksDisjointRootsAndOnlyExactConfigs(t *testing.T) {
	workspace := t.TempDir()
	consumer := filepath.Join(workspace, "consumer")
	dependency := filepath.Join(workspace, "dependency")
	unrelated := filepath.Join(workspace, "unrelated")
	consumerClass := filepath.Join(consumer, "force-app", "Consumer.cls")
	dependencyClass := filepath.Join(dependency, "force-app", "Dependency.cls")
	unrelatedClass := filepath.Join(unrelated, "force-app", "Unrelated.cls")
	consumerConfig := filepath.Join(consumer, "sfdx-project.json")
	nestedConfig := filepath.Join(consumer, "nested", "sfdx-project.json")
	unrelatedConfig := filepath.Join(unrelated, "glade.yml")
	for path, content := range map[string]string{
		consumerClass:   "public class Consumer {}",
		dependencyClass: "public class Dependency {}",
		unrelatedClass:  "public class Unrelated {}",
		consumerConfig:  `{"packageDirectories":[]}`,
		nestedConfig:    `{"packageDirectories":[]}`,
		unrelatedConfig: "project: {}\n",
	} {
		writeWatchFile(t, path, content)
	}

	snapshot, err := CaptureScope(Scope{
		Roots: []string{consumer, dependency},
		Files: []string{consumerConfig},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{consumerClass, dependencyClass, consumerConfig} {
		if _, ok := snapshot.Files[absPath(t, path)]; !ok {
			t.Fatalf("CaptureScope() omitted %s: %#v", path, snapshot.Files)
		}
	}
	for _, path := range []string{unrelatedClass, nestedConfig, unrelatedConfig} {
		if _, ok := snapshot.Files[absPath(t, path)]; ok {
			t.Fatalf("CaptureScope() included out-of-scope path %s: %#v", path, snapshot.Files)
		}
	}
}

func TestCaptureScopeAllowsMissingRootsAndExactFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "dependency")
	configPath := filepath.Join(t.TempDir(), "glade.yml")
	scope := Scope{Roots: []string{root}, Files: []string{configPath}}

	initial, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(initial.Files) != 0 {
		t.Fatalf("initial files = %#v, want empty", initial.Files)
	}

	classPath := filepath.Join(root, "force-app", "Dependency.cls")
	writeWatchFile(t, classPath, "public class Dependency {}")
	writeWatchFile(t, configPath, "project: {}\n")
	after, err := CaptureScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{classPath, configPath} {
		if _, ok := after.Files[absPath(t, path)]; !ok {
			t.Fatalf("CaptureScope() omitted newly present %s: %#v", path, after.Files)
		}
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
