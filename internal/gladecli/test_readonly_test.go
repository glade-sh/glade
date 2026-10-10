package gladecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestRunTestReadOnlyPreservesProject(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restoreDiskCache := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restoreDiskCache)

	root := t.TempDir()
	writeReadOnlyProbeProject(t, root)

	before := snapshotReadOnlyProject(t, root)
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"test", "--project", root, "--read-only", "--json", "--no-progress", "--parallelism", "1", "--no-parallel-methods"}, &stdout, &stderr)
	if exit != 1 {
		t.Fatalf("read-only test exit = %d, want 1 for the owned failing assertion; stderr=%q stdout=%q", exit, stderr.String(), stdout.String())
	}
	var envelope struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
		Data     testreport.Run     `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode read-only test JSON: %v; stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if envelope.Status != "failed" || envelope.ExitCode != 1 {
		t.Fatalf("read-only terminal = %q/%d, want failed/1", envelope.Status, envelope.ExitCode)
	}
	wantSummary := testreport.Summary{Total: 2, Passed: 1, Failed: 1}
	if got := envelope.Data.Summary(); got.Total != wantSummary.Total || got.Passed != wantSummary.Passed || got.Failed != wantSummary.Failed || got.Errors != 0 {
		t.Fatalf("read-only summary = %#v, want 2 total, 1 pass, 1 fail, 0 errors", got)
	}
	if got := envelope.Summary; got.Total != wantSummary.Total || got.Passed != wantSummary.Passed || got.Failed != wantSummary.Failed || got.Errors != 0 {
		t.Fatalf("read-only JSON summary = %#v, want 2 total, 1 pass, 1 fail, 0 errors", got)
	}
	gotCases := make(map[string]testreport.Status)
	for _, suite := range envelope.Data.Suites {
		for _, testCase := range suite.Cases {
			gotCases[testCase.ClassName+"."+testCase.MethodName] = testCase.Status
		}
	}
	wantCases := map[string]testreport.Status{
		"ReadOnlyProbeTest.passes": testreport.StatusPass,
		"ReadOnlyProbeTest.fails":  testreport.StatusFail,
	}
	if !reflect.DeepEqual(gotCases, wantCases) {
		t.Fatalf("read-only executed cases = %#v, want %#v", gotCases, wantCases)
	}
	if after := snapshotReadOnlyProject(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("read-only test changed project inventory:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestRunTestReadOnlyRedirectedRoot(t *testing.T) {
	base := t.TempDir()
	wrapper := filepath.Join(base, "wrapper")
	actual := filepath.Join(base, "actual-project")
	writeTestFile(t, filepath.Join(wrapper, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(wrapper, "glade.yml"), "project:\n  root: ../actual-project\n")
	writeReadOnlyProbeProject(t, actual)
	reportPath := filepath.Join(actual, "reports", "run.xml")
	beforeWrapper := snapshotReadOnlyProject(t, wrapper)
	beforeActual := snapshotReadOnlyProject(t, actual)

	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"test", "--project", wrapper, "--read-only", "--json", "--no-progress", "--junit", reportPath, "--parallelism", "1", "--no-parallel-methods"}, &stdout, &stderr)
	if exit == 0 || !strings.Contains(stderr.String(), "--junit output must be outside the project") {
		t.Fatalf("redirected read-only exit=%d stderr=%q stdout=%q; want project-local report rejection", exit, stderr.String(), stdout.String())
	}
	if _, err := os.Lstat(reportPath); !os.IsNotExist(err) {
		t.Fatalf("redirected project report exists or could not be checked: %v", err)
	}
	if after := snapshotReadOnlyProject(t, wrapper); !reflect.DeepEqual(after, beforeWrapper) {
		t.Fatalf("read-only test changed wrapper inventory:\nbefore=%#v\nafter=%#v", beforeWrapper, after)
	}
	if after := snapshotReadOnlyProject(t, actual); !reflect.DeepEqual(after, beforeActual) {
		t.Fatalf("read-only test changed redirected project inventory:\nbefore=%#v\nafter=%#v", beforeActual, after)
	}
}

func TestRunTestReadOnlyDanglingOutputSymlink(t *testing.T) {
	root := t.TempDir()
	writeReadOnlyProbeProject(t, root)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "reports", "new.xml")
	link := filepath.Join(outside, "report.xml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	before := snapshotReadOnlyProject(t, root)

	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"test", "--project", root, "--read-only", "--json", "--no-progress", "--junit", link, "--parallelism", "1", "--no-parallel-methods"}, &stdout, &stderr)
	if exit == 0 || !strings.Contains(stderr.String(), "--junit output must be outside the project") {
		t.Fatalf("dangling-symlink read-only exit=%d stderr=%q stdout=%q; want project target rejection", exit, stderr.String(), stdout.String())
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("dangling symlink created its project target or could not be checked: %v", err)
	}
	if after := snapshotReadOnlyProject(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("read-only test changed project inventory:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestRunTestReadOnlySymlinkParentTraversal(t *testing.T) {
	root := t.TempDir()
	writeReadOnlyProbeProject(t, root)
	projectSubdir := filepath.Join(root, "subdir")
	if err := os.MkdirAll(projectSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(outside, "link")
	if err := os.Symlink(projectSubdir, link); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "new.xml")
	outputPath := link + string(filepath.Separator) + ".." + string(filepath.Separator) + "new.xml"
	before := snapshotReadOnlyProject(t, root)

	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"test", "--project", root, "--read-only", "--json", "--no-progress", "--junit", outputPath, "--parallelism", "1", "--no-parallel-methods"}, &stdout, &stderr)
	if exit == 0 || !strings.Contains(stderr.String(), "ambiguous '..' traversal") {
		t.Fatalf("symlink-parent read-only exit=%d stderr=%q stdout=%q; want ambiguous parent traversal rejection", exit, stderr.String(), stdout.String())
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("symlink-parent output created its project target or could not be checked: %v", err)
	}
	if after := snapshotReadOnlyProject(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("read-only test changed project inventory:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestRunTestReadOnlyDanglingTargetTraversal(t *testing.T) {
	root := t.TempDir()
	writeReadOnlyProbeProject(t, root)
	projectSubdir := filepath.Join(root, "subdir")
	if err := os.MkdirAll(projectSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(outside, "link")
	if err := os.Symlink(projectSubdir, link); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "new.xml")
	linkTarget := link + string(filepath.Separator) + ".." + string(filepath.Separator) + "new.xml"
	reportLink := filepath.Join(outside, "report.xml")
	if err := os.Symlink(linkTarget, reportLink); err != nil {
		t.Fatal(err)
	}
	before := snapshotReadOnlyProject(t, root)

	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"test", "--project", root, "--read-only", "--json", "--no-progress", "--junit", reportLink, "--parallelism", "1", "--no-parallel-methods"}, &stdout, &stderr)
	if exit == 0 || !strings.Contains(stderr.String(), "ambiguous '..' traversal") {
		t.Fatalf("dangling-target read-only exit=%d stderr=%q stdout=%q; want nested target traversal rejection", exit, stderr.String(), stdout.String())
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("dangling target created its project file or could not be checked: %v", err)
	}
	if after := snapshotReadOnlyProject(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("read-only test changed project inventory:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestRunTestReadOnlyProtectsRedirectingConfig(t *testing.T) {
	base := t.TempDir()
	wrapper := filepath.Join(base, "wrapper")
	actual := filepath.Join(base, "actual-project")
	writeTestFile(t, filepath.Join(wrapper, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	configPath := filepath.Join(wrapper, "glade.yml")
	writeTestFile(t, configPath, "project:\n  root: ../actual-project\n")
	writeReadOnlyProbeProject(t, actual)
	configBefore, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeWrapper := snapshotReadOnlyProject(t, wrapper)
	beforeActual := snapshotReadOnlyProject(t, actual)

	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"test", "--project", wrapper, "--read-only", "--json", "--no-progress", "--junit", configPath, "--parallelism", "1", "--no-parallel-methods"}, &stdout, &stderr)
	if exit == 0 || !strings.Contains(stderr.String(), "--junit output must be outside the project") {
		t.Fatalf("redirect-config read-only exit=%d stderr=%q stdout=%q; want wrapper config protection", exit, stderr.String(), stdout.String())
	}
	configAfter, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(configAfter, configBefore) {
		t.Fatalf("read-only report changed redirecting config:\nbefore=%q\nafter=%q", configBefore, configAfter)
	}
	if after := snapshotReadOnlyProject(t, wrapper); !reflect.DeepEqual(after, beforeWrapper) {
		t.Fatalf("read-only test changed wrapper inventory:\nbefore=%#v\nafter=%#v", beforeWrapper, after)
	}
	if after := snapshotReadOnlyProject(t, actual); !reflect.DeepEqual(after, beforeActual) {
		t.Fatalf("read-only test changed redirected project inventory:\nbefore=%#v\nafter=%#v", beforeActual, after)
	}
}

func TestRunTestReadOnlyShardOutputSymlink(t *testing.T) {
	root := t.TempDir()
	sourcePath := writeReadOnlyProbeProject(t, root)
	beforeSource, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "shards")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sourcePath, filepath.Join(outside, "shard-000.txt")); err != nil {
		t.Fatal(err)
	}
	before := snapshotReadOnlyProject(t, root)

	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"test", "--project", root, "--read-only", "--write-class-shards", outside, "--shard-count", "1"}, &stdout, &stderr)
	if exit == 0 || !strings.Contains(stderr.String(), "--read-only cannot be combined with --write-class-shards") {
		t.Fatalf("shard-symlink read-only exit=%d stderr=%q stdout=%q; want artifact-only mode rejection", exit, stderr.String(), stdout.String())
	}
	afterSource, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterSource, beforeSource) {
		t.Fatalf("read-only shard output changed source through symlink:\nbefore=%q\nafter=%q", beforeSource, afterSource)
	}
	if after := snapshotReadOnlyProject(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("read-only shard output changed project inventory:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestRunTestReadOnlyRejectsIncompatibleModes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "wizard", args: []string{"--wizard"}, want: "--read-only cannot be combined with --wizard"},
		{name: "debug", args: []string{"--debug"}, want: "--read-only cannot be combined with --debug"},
		{name: "watch once", args: []string{"--watch-once"}, want: "--read-only cannot be combined with --watch or --watch-once"},
		{name: "daemon", args: []string{"--daemon"}, want: "--read-only cannot be combined with --daemon"},
		{name: "connect", args: []string{"--connect"}, want: "--read-only cannot be combined with --connect"},
		{name: "ui", args: []string{"--ui"}, want: "--read-only cannot be combined with --ui"},
		{name: "class shards", args: []string{"--write-class-shards", "external", "--shard-count", "1"}, want: "--read-only cannot be combined with --write-class-shards"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeReadOnlyProbeProject(t, root)
			args := append([]string{"test", "--project", root, "--read-only"}, tt.args...)
			var stdout, stderr bytes.Buffer
			exit := Run(context.Background(), args, &stdout, &stderr)
			if exit == 0 || !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("exit=%d stderr=%q stdout=%q; want rejection containing %q", exit, stderr.String(), stdout.String(), tt.want)
			}
		})
	}
}

func TestRunTestReadOnlyAllowsExternalArtifacts(t *testing.T) {
	root := t.TempDir()
	writeReadOnlyProbeProject(t, root)
	before := snapshotReadOnlyProject(t, root)
	outputs := filepath.Join(t.TempDir(), "outputs")
	if err := os.MkdirAll(outputs, 0o755); err != nil {
		t.Fatal(err)
	}
	junitPath := filepath.Join(outputs, "run.xml")
	tracePath := filepath.Join(outputs, "trace.json")

	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"test", "--project", root, "--read-only", "--json", "--no-progress", "--junit", junitPath, "--trace", tracePath, "--parallelism", "1", "--no-parallel-methods"}, &stdout, &stderr)
	if exit != 1 {
		t.Fatalf("external-output read-only exit=%d, want 1 for the owned failing assertion; stderr=%q stdout=%q", exit, stderr.String(), stdout.String())
	}
	for _, path := range []string{junitPath, tracePath} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read external output %s: %v", path, err)
		}
		if len(data) == 0 {
			t.Fatalf("external output %s is empty", path)
		}
	}
	if after := snapshotReadOnlyProject(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("external-output read-only run changed project inventory:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func writeReadOnlyProbeProject(t *testing.T, root string) string {
	t.Helper()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	if err := os.MkdirAll(filepath.Join(root, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "force-app/main/default/classes/ReadOnlyProbeTest.cls")
	writeTestFile(t, sourcePath, `@IsTest
private class ReadOnlyProbeTest {
  @IsTest static void passes() {
    System.assertEquals(2, 1 + 1);
  }
  @IsTest static void fails() {
    System.assertEquals(3, 1 + 1);
  }
}`)
	writeTestFile(t, filepath.Join(root, ".glade/test/last-failed.json"), `{"projectRoot":"sentinel","updatedAt":"2026-10-01T00:00:00Z","failures":["PriorTest.prior"]}`+"\n")
	return sourcePath
}

func snapshotReadOnlyProject(t *testing.T, root string) map[string]string {
	t.Helper()
	entries := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			entries[rel] = fmt.Sprintf("symlink:%s:%s:%d", target, info.Mode(), info.ModTime().UnixNano())
			return nil
		}
		if entry.IsDir() {
			entries[rel] = fmt.Sprintf("dir:%s:%d", info.Mode(), info.ModTime().UnixNano())
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries[rel] = fmt.Sprintf("file:%s:%d:%d:%x", info.Mode(), info.Size(), info.ModTime().UnixNano(), sha256.Sum256(data))
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot project inventory: %v", err)
	}
	return entries
}
