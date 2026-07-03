package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRunOrgCreateWritesConfigAndInitializesDB(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	dbPath := filepath.Join(root, ".glade", "orgs", "my-glade-org.sqlite")
	var stdout, stderr bytes.Buffer

	code := Run(context.Background(), []string{"org", "create", "my-glade-org", "--project", root, "--db", dbPath, "--addr", "127.0.0.1:17911", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	configPath := filepath.Join(root, ".glade", "orgs", "my-glade-org", "org.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Alias       string `json:"alias"`
		Project     string `json:"project"`
		DB          string `json:"db"`
		Addr        string `json:"addr"`
		InstanceURL string `json:"instanceUrl"`
		OrgID       string `json:"orgId"`
		UserID      string `json:"userId"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("org config was not JSON: %v\n%s", err, string(data))
	}
	if got.Alias != "my-glade-org" || got.Project != root || got.DB != dbPath || got.Addr != "127.0.0.1:17911" {
		t.Fatalf("org config = %#v", got)
	}
	if got.InstanceURL != "http://127.0.0.1:17911" || got.OrgID != "00D000000000001" || got.UserID != "005000000000001" {
		t.Fatalf("org identity = %#v", got)
	}
	var createOut orgConfig
	env := decodeCLIEnvelopeData(t, stdout.Bytes(), "org create", &createOut)
	if env.Status != "passed" || env.ExitCode != 0 || createOut.Alias != "my-glade-org" || createOut.InstanceURL != "http://127.0.0.1:17911" {
		t.Fatalf("unexpected org create envelope: env=%#v data=%#v\n%s", env, createOut, stdout.String())
	}
	store, err := storage.OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	org, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(org.Objects) == 0 {
		t.Fatalf("created database has no objects")
	}
}

func TestRunOrgCreateRejectsPublicBindWithoutOptIn(t *testing.T) {
	t.Setenv("GLADE_SERVER_PUBLIC", "")
	root := t.TempDir()
	writeTestProject(t, root)
	var stdout, stderr bytes.Buffer

	code := Run(context.Background(), []string{"org", "create", "public-org", "--project", root, "--addr", "0.0.0.0:17911"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "GLADE_SERVER_PUBLIC=1") {
		t.Fatalf("stderr missing public bind guidance: %q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".glade", "orgs", "public-org", "org.json")); !os.IsNotExist(err) {
		t.Fatalf("public org config stat err = %v, want not exist", err)
	}
}

func TestRunOrgListShowsConfiguredOrg(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	dbPath := filepath.Join(root, ".glade", "orgs", "listed.sqlite")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"org", "create", "listed", "--project", root, "--db", dbPath, "--addr", "127.0.0.1:17912"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("create exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	withWorkingDir(t, root, func() {
		stdout.Reset()
		stderr.Reset()
		code = Run(context.Background(), []string{"org", "list", "--json"}, &stdout, &stderr)
	})
	if code != 0 {
		t.Fatalf("list exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var got struct {
		Orgs []struct {
			Alias       string `json:"alias"`
			Project     string `json:"project"`
			DB          string `json:"db"`
			Addr        string `json:"addr"`
			InstanceURL string `json:"instanceUrl"`
		} `json:"orgs"`
	}
	env := decodeCLIEnvelopeData(t, stdout.Bytes(), "org list", &got)
	if env.Status != "passed" || env.ExitCode != 0 {
		t.Fatalf("unexpected org list envelope: %#v\n%s", env, stdout.String())
	}
	if len(got.Orgs) != 1 || got.Orgs[0].Alias != "listed" || got.Orgs[0].InstanceURL != "http://127.0.0.1:17912" {
		t.Fatalf("org list = %#v", got.Orgs)
	}
}

func TestRunOrgCommandsUseProjectFlagForConfigLookup(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"org", "create", "remote", "--project", root, "--addr", "127.0.0.1:17914"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("create exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"org", "list", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("list exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"alias": "remote"`) {
		t.Fatalf("list stdout = %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"org", "status", "remote", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("status exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var statusOut orgStatus
	env := decodeCLIEnvelopeData(t, stdout.Bytes(), "org status", &statusOut)
	if env.Status != "passed" || env.ExitCode != 0 || statusOut.Alias != "remote" || statusOut.Status != "stopped" {
		t.Fatalf("unexpected org status envelope: env=%#v data=%#v\n%s", env, statusOut, stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"org", "auth", "remote", "--project", root, "--print"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "--alias remote") {
		t.Fatalf("auth stdout = %s", stdout.String())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stdout.Reset()
	stderr.Reset()
	code = Run(ctx, []string{"org", "start", "remote", "--project", root}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), context.Canceled.Error()) {
		t.Fatalf("start exit code = %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
}

func TestRunOrgStatusReportsStoppedWhenServerIsNotRunning(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"org", "create", "stopped", "--project", root, "--addr", "127.0.0.1:1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("create exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".glade", "orgs", "stopped.sqlite")); err != nil {
		t.Fatalf("default db was not initialized under project root: %v", err)
	}

	withWorkingDir(t, root, func() {
		stdout.Reset()
		stderr.Reset()
		code = Run(context.Background(), []string{"org", "status", "stopped", "--json"}, &stdout, &stderr)
	})
	if code != 0 {
		t.Fatalf("status exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var got struct {
		Alias  string `json:"alias"`
		Status string `json:"status"`
	}
	env := decodeCLIEnvelopeData(t, stdout.Bytes(), "org status", &got)
	if env.Status != "passed" || env.ExitCode != 0 {
		t.Fatalf("unexpected org status envelope: %#v\n%s", env, stdout.String())
	}
	if got.Alias != "stopped" || got.Status != "stopped" {
		t.Fatalf("status = %#v", got)
	}
}

func TestRunOrgCommandsRejectPathAliases(t *testing.T) {
	for _, args := range [][]string{
		{"org", "status", "../outside"},
		{"org", "start", "../outside"},
		{"org", "auth", "../outside", "--print"},
	} {
		t.Run(strings.Join(args[1:], "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), args, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("exit code = 0, want failure; stdout=%q", stdout.String())
			}
			if !strings.Contains(stderr.String(), `invalid org alias "../outside"`) {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestRunOrgAuthPrintsSfCommand(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"org", "create", "authme", "--project", root, "--addr", "127.0.0.1:17913"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("create exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	withWorkingDir(t, root, func() {
		stdout.Reset()
		stderr.Reset()
		code = Run(context.Background(), []string{"org", "auth", "authme", "--print"}, &stdout, &stderr)
	})
	if code != 0 {
		t.Fatalf("auth exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	got := stdout.String()
	for _, want := range []string{
		"SF_ACCESS_TOKEN='00D000000000001!glade-local-authme'",
		"sf org login access-token",
		"--instance-url http://127.0.0.1:17913",
		"--alias authme",
		"--no-prompt",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("auth print missing %q:\n%s", want, got)
		}
	}
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(original); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}
