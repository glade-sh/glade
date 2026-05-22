package playground

import (
	"os"
	"path/filepath"
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
