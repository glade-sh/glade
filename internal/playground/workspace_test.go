package playground

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceCreatesScratchProject(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{
		DataRoot: t.TempDir(),
		ID:       "default",
	})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}

	for _, rel := range []string{
		"sfdx-project.json",
		"force-app/main/default/classes/AccountPlayground.cls",
		"anonymous.apex",
		"seed.json",
	} {
		if _, err := os.Stat(filepath.Join(ws.Root, rel)); err != nil {
			t.Fatalf("default workspace missing %s: %v", rel, err)
		}
	}

	meta, err := ws.Metadata()
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}
	if meta.ID != "default" || len(meta.Files) < 3 {
		t.Fatalf("metadata = %#v", meta)
	}
}

func TestWorkspaceIgnoresUnsupportedFilesWhenCreatingScratchProject(t *testing.T) {
	dataRoot := t.TempDir()
	root := filepath.Join(dataRoot, "workspaces", "default")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws.Root, "force-app/main/default/classes/AccountPlayground.cls")); err != nil {
		t.Fatalf("default class missing: %v", err)
	}
}

func TestWorkspaceReopensLoadedExampleWithoutDefaultFiles(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	if _, err := ws.LoadExample("trigger-contact-task"); err != nil {
		t.Fatalf("LoadExample() error = %v", err)
	}

	reopened, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("reopen OpenWorkspace() error = %v", err)
	}
	meta, err := reopened.Metadata()
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}
	if meta.ExampleID != "trigger-contact-task" {
		t.Fatalf("example id = %q", meta.ExampleID)
	}
	for _, file := range meta.Files {
		if file.Path == "force-app/main/default/classes/AccountPlayground.cls" {
			t.Fatalf("default file was added to loaded example: %#v", meta.Files)
		}
	}
}

func TestWorkspaceRejectsUnsafePaths(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}

	for _, rel := range []string{
		"/tmp/Account.cls",
		"../Account.cls",
		"force-app/main/default/classes/Bad.exe",
		"force-app/main/default/classes",
		"",
	} {
		if _, err := ws.SafePath(rel); err == nil {
			t.Fatalf("SafePath(%q) succeeded, want error", rel)
		}
	}
}

func TestOpenWorkspaceRejectsUnsafeIDs(t *testing.T) {
	dataRoot := t.TempDir()
	for _, id := range []string{"../outside", "a/b", `a\\b`, "/absolute", ".", ".."} {
		if _, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: id}); err == nil {
			t.Fatalf("OpenWorkspace(%q) succeeded, want error", id)
		}
	}
	for _, id := range []string{"", "   ", "normal"} {
		ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: id})
		if err != nil {
			t.Fatalf("OpenWorkspace(%q) error = %v", id, err)
		}
		if id == "normal" && ws.ID != id {
			t.Fatalf("workspace ID = %q, want %q", ws.ID, id)
		}
	}
}

func TestOpenWorkspaceDirectProjectRootPreservesLegacyID(t *testing.T) {
	projectRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ProjectRoot: projectRoot, ID: "a/b"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	if ws.Managed {
		t.Fatal("workspace unexpectedly managed")
	}
	if ws.Root != projectRoot || ws.ProjectRoot != projectRoot || ws.ID != "a/b" {
		t.Fatalf("workspace = %#v", ws)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "sfdx-project.json")); err != nil {
		t.Fatalf("direct project root default file missing: %v", err)
	}
}

func TestWorkspaceSymlinkFilesDoNotEscape(t *testing.T) {
	dataRoot := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.cls")
	if err := os.WriteFile(outside, []byte("outside sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(ws.Root, "force-app/main/default/classes/Outside.cls")
	if err := os.Symlink(outside, leaf); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.SaveFile(FileSaveRequest{Path: "force-app/main/default/classes/Outside.cls", Content: "changed", Version: 0}); err == nil {
		t.Fatal("SaveFile through leaf symlink succeeded")
	}
	if err := ws.DeleteFile("force-app/main/default/classes/Outside.cls"); err == nil {
		t.Fatal("DeleteFile through leaf symlink succeeded")
	}
	if meta, err := ws.Metadata(); err != nil || strings.Contains(meta.AnonymousBody, "outside sentinel") {
		t.Fatalf("Metadata() = %#v, %v", meta, err)
	}
	if _, err := ws.Hash(); err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	data, err := os.ReadFile(outside)
	if err != nil || string(data) != "outside sentinel" {
		t.Fatalf("outside = %q, %v", data, err)
	}
	parentOutside := filepath.Join(t.TempDir(), "parent")
	if err := os.MkdirAll(parentOutside, 0o755); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(ws.Root, "force-app/main/default/classes")
	if err := os.RemoveAll(parent); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(parentOutside, parent); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.SaveFile(FileSaveRequest{Path: "force-app/main/default/classes/Parent.cls", Content: "changed", Version: 0}); err == nil {
		t.Fatal("SaveFile through parent symlink succeeded")
	}
	if _, err := os.Stat(filepath.Join(parentOutside, "Parent.cls")); !os.IsNotExist(err) {
		t.Fatalf("parent symlink wrote outside: %v", err)
	}
}

func TestManagedWorkspaceRejectsReplacedRootPaths(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "sentinel.cls")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	idPath := filepath.Join(dataRoot, "workspaces", "default")
	if err := os.RemoveAll(idPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, idPath); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.SaveFile(FileSaveRequest{Path: "force-app/main/default/classes/Escape.cls", Content: "public class Escape {}"}); err == nil {
		t.Fatal("SaveFile() through replaced managed root succeeded")
	}
	if data, _ := os.ReadFile(sentinel); string(data) != "keep" {
		t.Fatalf("outside sentinel = %q", data)
	}

	if err := os.Remove(idPath); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dataRoot, "workspaces")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dataRoot, "workspaces")); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.LoadExample("trigger-contact-task"); err == nil {
		t.Fatal("LoadExample() through replaced workspaces root succeeded")
	}
	if data, _ := os.ReadFile(sentinel); string(data) != "keep" {
		t.Fatalf("outside sentinel = %q", data)
	}
}

func TestManagedWorkspaceRejectsInternalSymlinkAuthority(t *testing.T) {
	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dataRoot, "other")
	writePlaygroundTestFile(t, filepath.Join(other, "sentinel.cls"), "keep")
	idPath := filepath.Join(dataRoot, "workspaces", "default")
	if err := os.RemoveAll(idPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, idPath); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.SaveFile(FileSaveRequest{Path: "force-app/main/default/classes/Escape.cls", Content: "public class Escape {}"}); err == nil {
		t.Fatal("internal ID symlink save succeeded")
	}
	if data, _ := os.ReadFile(filepath.Join(other, "sentinel.cls")); string(data) != "keep" {
		t.Fatalf("sentinel = %q", data)
	}
}

func TestProjectReferenceSymlinkIsNotCopied(t *testing.T) {
	projectRoot := t.TempDir()
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), sfdxProjectJSON)
	outside := filepath.Join(t.TempDir(), "outside.cls")
	if err := os.WriteFile(outside, []byte("outside sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(projectRoot, "force-app/main/default/classes/Outside.cls")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	files, err := collectProjectReferenceFiles(ProjectReference{Path: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files["force-app/main/default/classes/Outside.cls"]; ok {
		t.Fatal("project reference copied symlink target")
	}
}

func TestWorkspaceSaveFileVersionConflict(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}

	first, err := ws.SaveFile(FileSaveRequest{
		Path:    "force-app/main/default/classes/Extra.cls",
		Content: "public class Extra {}",
		Version: 0,
	})
	if err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}
	if first.File.Version != 1 {
		t.Fatalf("version = %d, want 1", first.File.Version)
	}

	if _, err := ws.SaveFile(FileSaveRequest{
		Path:    "force-app/main/default/classes/Extra.cls",
		Content: "public class Extra { public static Integer x(){ return 1; } }",
		Version: 0,
	}); err == nil {
		t.Fatalf("SaveFile() with stale version succeeded")
	}
}

func TestWorkspaceProjectReferenceFilesAreReadOnly(t *testing.T) {
	projectRoot := t.TempDir()
	writePlaygroundTestFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"name":"local-ref","namespace":"","sourceApiVersion":"65.0"}`)
	projectPath := "force-app/main/default/classes/Locked.cls"
	writePlaygroundTestFile(t, filepath.Join(projectRoot, projectPath), `public class Locked {}`)

	dataRoot := t.TempDir()
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	meta, err := ws.LoadProjectReference(ProjectReference{ID: "local", Name: "Local", Path: projectRoot})
	if err != nil {
		t.Fatalf("LoadProjectReference() error = %v", err)
	}
	locked := workspaceFileByPath(meta.Files, projectPath)
	if locked == nil {
		t.Fatalf("%s missing from metadata", projectPath)
	}
	if !locked.ReadOnly {
		t.Fatalf("%s readOnly = false, want true", projectPath)
	}

	if _, err := ws.SaveFile(FileSaveRequest{Path: projectPath, Content: `public class Locked { public static Integer x(){ return 1; } }`, Version: locked.Version}); err == nil {
		t.Fatalf("SaveFile() on read-only project file succeeded")
	}

	scratchPath := "force-app/main/default/classes/ScratchOneOff.cls"
	if _, err := ws.SaveFile(FileSaveRequest{Path: scratchPath, Content: `public class ScratchOneOff {}`, Version: 0}); err != nil {
		t.Fatalf("SaveFile() scratch class error = %v", err)
	}

	reopened, err := OpenWorkspace(WorkspaceOptions{DataRoot: dataRoot, ID: "default"})
	if err != nil {
		t.Fatalf("reopen OpenWorkspace() error = %v", err)
	}
	reopenedMeta, err := reopened.Metadata()
	if err != nil {
		t.Fatalf("reopened Metadata() error = %v", err)
	}
	if file := workspaceFileByPath(reopenedMeta.Files, projectPath); file == nil || !file.ReadOnly {
		t.Fatalf("reopened project file = %#v, want readOnly", file)
	}
	if file := workspaceFileByPath(reopenedMeta.Files, scratchPath); file == nil || file.ReadOnly {
		t.Fatalf("scratch file = %#v, want editable", file)
	}
}

func TestWorkspaceDeletesClassFiles(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir(), ID: "default"})
	if err != nil {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
	path := "force-app/main/default/classes/Extra.cls"
	if _, err := ws.SaveFile(FileSaveRequest{Path: path, Content: "public class Extra {}", Version: 0}); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	if err := ws.DeleteFile(path); err != nil {
		t.Fatalf("DeleteFile() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws.Root, path)); !os.IsNotExist(err) {
		t.Fatalf("deleted file stat error = %v", err)
	}
	if err := ws.DeleteFile("sfdx-project.json"); err == nil {
		t.Fatalf("DeleteFile(sfdx-project.json) succeeded")
	}
}

func workspaceFileByPath(files []WorkspaceFile, path string) *WorkspaceFile {
	for i := range files {
		if files[i].Path == path {
			return &files[i]
		}
	}
	return nil
}
