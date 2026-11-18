package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type releaseWorkflowStep struct {
	Name  string            `yaml:"name"`
	Uses  string            `yaml:"uses"`
	Run   string            `yaml:"run"`
	Shell string            `yaml:"shell"`
	Env   map[string]string `yaml:"env"`
	With  map[string]string `yaml:"with"`
}

type releaseWorkflowJob struct {
	Needs       any                   `yaml:"needs"`
	If          string                `yaml:"if"`
	Permissions map[string]string     `yaml:"permissions"`
	Env         map[string]string     `yaml:"env"`
	Steps       []releaseWorkflowStep `yaml:"steps"`
}

type releaseWorkflowDocument struct {
	On struct {
		WorkflowDispatch struct {
			Inputs map[string]struct {
				Required bool   `yaml:"required"`
				Default  any    `yaml:"default"`
				Type     string `yaml:"type"`
			} `yaml:"inputs"`
		} `yaml:"workflow_dispatch"`
	} `yaml:"on"`
	Jobs map[string]releaseWorkflowJob `yaml:"jobs"`
}

func readReleaseWorkflowDocument(t *testing.T) (releaseWorkflowDocument, string) {
	t.Helper()
	text := string(mustReadFile(t, filepath.Join("..", ".github", "workflows", "release.yml")))
	var document releaseWorkflowDocument
	if err := yaml.Unmarshal([]byte(text), &document); err != nil {
		t.Fatalf("parse release.yml: %v", err)
	}
	return document, text
}

func releaseWorkflowStepIndex(t *testing.T, job releaseWorkflowJob, name string) int {
	t.Helper()
	for index, step := range job.Steps {
		if step.Name == name {
			return index
		}
	}
	t.Fatalf("distribute job missing step %q", name)
	return -1
}

func TestReleaseWorkflowDistributesStaticDownloadsAfterGitHubPublish(t *testing.T) {
	document, text := readReleaseWorkflowDocument(t)
	distribute, ok := document.Jobs["distribute"]
	if !ok {
		t.Fatal("release workflow lacks the static distribute job")
	}
	if distribute.Needs != "publish" {
		t.Fatalf("distribute needs = %#v, want publish", distribute.Needs)
	}
	for _, want := range []string{
		"always()",
		"github.event_name == 'push' && startsWith(github.ref, 'refs/tags/') && needs.publish.result == 'success'",
		"github.event_name == 'workflow_dispatch' && inputs.distribute_only && github.ref == 'refs/heads/main'",
	} {
		if !strings.Contains(distribute.If, want) {
			t.Fatalf("distribute condition %q lacks %q", distribute.If, want)
		}
	}
	if publishAt, distributeAt := strings.Index(text, "\n  publish:\n"), strings.Index(text, "\n  distribute:\n"); publishAt < 0 || distributeAt < publishAt {
		t.Fatal("static distribution must follow GitHub publication")
	}
	wantPermissions := map[string]string{"actions": "read", "attestations": "read", "checks": "read", "contents": "read"}
	if len(distribute.Permissions) != len(wantPermissions) {
		t.Fatalf("distribute permissions = %#v, want %#v", distribute.Permissions, wantPermissions)
	}
	for scope, level := range wantPermissions {
		if distribute.Permissions[scope] != level {
			t.Fatalf("distribute permissions = %#v, want %#v", distribute.Permissions, wantPermissions)
		}
	}
	if got := distribute.Env["VERSION"]; got != "${{ github.event_name == 'workflow_dispatch' && inputs.version || github.ref_name }}" {
		t.Fatalf("distribute VERSION = %q, want the workflow's dispatch-or-tag version", got)
	}

	order := []string{
		"Require static publisher credentials",
		"Resolve published release",
		"Download and verify release bundle",
		"Install pinned static publisher",
		"Publish static downloads",
		"Verify public downloads",
	}
	previous := -1
	for _, name := range order {
		index := releaseWorkflowStepIndex(t, distribute, name)
		if index <= previous {
			t.Fatalf("distribute step %q is out of order", name)
		}
		previous = index
	}
	steps := map[string]releaseWorkflowStep{}
	for _, step := range distribute.Steps {
		steps[step.Name] = step
		if strings.Contains(step.Run, "gh release ") && !strings.Contains(step.Run, "gh release view") && !strings.Contains(step.Run, "gh release download") {
			t.Fatalf("distribute step %q can change the GitHub Release", step.Name)
		}
		for _, forbidden := range []string{"gh release edit", "gh release upload", "gh release create", "gh release delete", "release-asset-upload.sh", "wrangler r2 object put"} {
			if strings.Contains(step.Run, forbidden) {
				t.Fatalf("distribute step %q contains %q", step.Name, forbidden)
			}
		}
	}
	if releaseWorkflowStepIndex(t, distribute, "Require static publisher credentials") != 0 {
		t.Fatal("missing Cloudflare credentials must stop the job before any download")
	}

	resolve := steps["Resolve published release"].Run
	for _, want := range []string{
		`test "$(git cat-file -t "refs/tags/$VERSION")" = tag`,
		`glade_sha="$(git rev-parse "refs/tags/$VERSION^{commit}")"`,
		`test "$glade_sha" = "$GITHUB_SHA"`,
		`test "$GITHUB_REF" = refs/heads/main`,
		`scripts/verify-salesforce-check.sh --tag-tools-sha "$VERSION"`,
		`gh release view "$VERSION" --json isDraft --jq .isDraft`,
		`echo "GLADE_SHA=$glade_sha" >> "$GITHUB_ENV"`,
	} {
		if !strings.Contains(resolve, want) {
			t.Fatalf("release resolution missing %q", want)
		}
	}

	bundle := steps["Download and verify release bundle"].Run
	for _, want := range []string{
		`bundle="glade-release-artifacts-$VERSION.tar.gz"`,
		`gh release download "$VERSION" --dir "$assets"`,
		`tar --no-same-owner --no-same-permissions -C dist -xzf "$assets/$bundle"`,
		`cmp "$assets/$name" "dist/$VERSION/$name"`,
		`cmp "$assets/index.json" dist/index.json`,
		`cmp "$assets/release-manifest.json" dist/latest/release-manifest.json`,
		"sha256sum --check --strict SHA256SUMS.txt",
		"gh attestation verify",
		"--signer-workflow glade-sh/glade/.github/workflows/release.yml",
		`--source-digest "$GLADE_SHA"`,
		`--source-ref "refs/tags/$VERSION"`,
	} {
		if !strings.Contains(bundle, want) {
			t.Fatalf("bundle verification missing %q", want)
		}
	}
	if strings.Contains(bundle, "--clobber") {
		t.Fatal("bundle download must start from an empty directory, not replace files")
	}

	install := steps["Install pinned static publisher"]
	if install.Run != "npm ci --prefix scripts --ignore-scripts" {
		t.Fatalf("static publisher install = %q", install.Run)
	}
	publish := steps["Publish static downloads"]
	if publish.Run != `node scripts/release-publish.mjs dist "$VERSION" "$GLADE_SHA"` {
		t.Fatalf("static publisher command = %q", publish.Run)
	}
	for name, want := range map[string]string{
		"CLOUDFLARE_ACCOUNT_ID": "${{ vars.CLOUDFLARE_ACCOUNT_ID }}",
		"CLOUDFLARE_API_TOKEN":  "${{ secrets.CLOUDFLARE_API_TOKEN }}",
		"WRANGLER_MODULE":       "${{ github.workspace }}/scripts/node_modules/wrangler",
		"GH_TOKEN":              "${{ github.token }}",
	} {
		if publish.Env[name] != want {
			t.Fatalf("static publisher env %s = %q, want %q", name, publish.Env[name], want)
		}
	}

	verify := steps["Verify public downloads"].Run
	for _, want := range []string{
		"https://downloads.glade.sh/latest/release-manifest.json",
		`test "$(jq -er .version "$public/latest-release-manifest.json")" = "$VERSION"`,
		`"https://downloads.glade.sh/$VERSION/release-manifest.json"`,
		`cmp "dist/$VERSION/release-manifest.json" "$public/release-manifest.json"`,
		"https://downloads.glade.sh/index.json",
		`any(.versions[]; .version == $version)`,
	} {
		if !strings.Contains(verify, want) {
			t.Fatalf("public download verification missing %q", want)
		}
	}
	for _, step := range distribute.Steps {
		if step.Uses != "" && !fullActionRefPattern.MatchString(step.Uses) {
			t.Fatalf("distribute action is not pinned by SHA: %s", step.Uses)
		}
		if strings.HasPrefix(step.Uses, "actions/checkout@") && step.With["persist-credentials"] != "false" {
			t.Fatal("distribute checkout must not persist credentials")
		}
	}
}

func TestReleaseWorkflowDistributeOnlyDispatchSkipsBuildAndPublish(t *testing.T) {
	document, _ := readReleaseWorkflowDocument(t)
	inputs := document.On.WorkflowDispatch.Inputs
	distributeOnly, ok := inputs["distribute_only"]
	if !ok || distributeOnly.Type != "boolean" || distributeOnly.Required || distributeOnly.Default != false {
		t.Fatalf("distribute_only input = %#v, want optional boolean defaulting to false", distributeOnly)
	}
	if _, ok := inputs["version"]; !ok {
		t.Fatal("distribute-only dispatch needs the existing version input")
	}
	if got := document.Jobs["salesforce-authority"].If; got != "${{ !inputs.distribute_only }}" {
		t.Fatalf("salesforce-authority condition = %q; distribute-only must skip the build chain", got)
	}
	// Every job that builds, attests or changes the GitHub Release is tag-only or
	// depends on salesforce-authority, so a distribute-only dispatch cannot reach it.
	for _, name := range []string{"prepare", "attest-and-upload", "publish"} {
		if job := document.Jobs[name]; !strings.Contains(job.If, "startsWith(github.ref, 'refs/tags/')") {
			t.Fatalf("%s is not tag-only: %q", name, job.If)
		}
	}
	for _, name := range []string{"prepare", "shared-payload"} {
		if job := document.Jobs[name]; !strings.Contains(job.If, "needs.salesforce-authority.result == 'success'") {
			t.Fatalf("%s does not require Salesforce authority: %q", name, job.If)
		}
	}
	if job := document.Jobs["build"]; !strings.Contains(job.If, "needs.shared-payload.result == 'success'") {
		t.Fatalf("build does not require the shared payload: %q", job.If)
	}
}

func TestReleaseWorkflowDistributeOnlyRequiresMain(t *testing.T) {
	document, _ := readReleaseWorkflowDocument(t)
	distribute := document.Jobs["distribute"]
	dispatch := distribute.If[strings.Index(distribute.If, "github.event_name == 'workflow_dispatch'"):]
	if !strings.Contains(dispatch, "inputs.distribute_only && github.ref == 'refs/heads/main'") {
		t.Fatalf("distribute-only path is not limited to main: %q", distribute.If)
	}
	guard, ok := document.Jobs["distribute-ref"]
	if !ok {
		t.Fatal("release workflow lacks the distribute-only ref guard")
	}
	if guard.If != "${{ github.event_name == 'workflow_dispatch' && inputs.distribute_only && github.ref != 'refs/heads/main' }}" {
		t.Fatalf("distribute-ref condition = %q", guard.If)
	}
	if len(guard.Permissions) != 0 || len(guard.Steps) != 1 || guard.Steps[0].Uses != "" || len(guard.Steps[0].Env) != 0 {
		t.Fatalf("distribute-ref must be a permissionless, secret-free failing step: %#v", guard)
	}
	command := exec.Command("bash", "-e", "-c", guard.Steps[0].Run)
	command.Env = append(os.Environ(), "GITHUB_REF=refs/heads/feature")
	out, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "::error::distribute_only dispatches must run from refs/heads/main, not refs/heads/feature") {
		t.Fatalf("off-main distribute-only dispatch was not rejected by name: err=%v\n%s", err, out)
	}
}

func TestReleaseWorkflowNamesMissingCloudflareSecret(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	document, _ := readReleaseWorkflowDocument(t)
	distribute := document.Jobs["distribute"]
	step := distribute.Steps[releaseWorkflowStepIndex(t, distribute, "Require static publisher credentials")]
	if step.Env["CLOUDFLARE_API_TOKEN"] != "${{ secrets.CLOUDFLARE_API_TOKEN }}" || step.Env["CLOUDFLARE_ACCOUNT_ID"] != "${{ vars.CLOUDFLARE_ACCOUNT_ID }}" {
		t.Fatalf("credential check env = %#v", step.Env)
	}
	run := func(account, token string) (string, error) {
		command := exec.Command("bash", "-e", "-c", step.Run)
		command.Env = append(os.Environ(), "VERSION=v1.2.3", "CLOUDFLARE_ACCOUNT_ID="+account, "CLOUDFLARE_API_TOKEN="+token)
		out, err := command.CombinedOutput()
		return string(out), err
	}
	if out, err := run("0123456789abcdef0123456789abcdef", ""); err == nil || !strings.Contains(out, "::error::Repository secret CLOUDFLARE_API_TOKEN is not set") {
		t.Fatalf("missing token accepted or unnamed: err=%v\n%s", err, out)
	}
	if out, err := run("", "token-fixture"); err == nil || !strings.Contains(out, "CLOUDFLARE_ACCOUNT_ID is not set") {
		t.Fatalf("missing account accepted or unnamed: err=%v\n%s", err, out)
	}
	if out, err := run("0123456789abcdef0123456789abcdef", "token-fixture"); err != nil || strings.Contains(out, "token-fixture") {
		t.Fatalf("configured credentials rejected or echoed: err=%v\n%s", err, out)
	}
}

func TestReleaseStaticPublisherIsLockfilePinned(t *testing.T) {
	var manifest struct {
		Private         bool              `json:"private"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Scripts         map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(mustReadFile(t, "package.json"), &manifest); err != nil {
		t.Fatal(err)
	}
	if !manifest.Private || len(manifest.Dependencies) != 1 || len(manifest.DevDependencies) != 0 || len(manifest.Scripts) != 0 {
		t.Fatalf("scripts/package.json must be a private, script-free wrangler pin: %#v", manifest)
	}
	version := manifest.Dependencies["wrangler"]
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(version) {
		t.Fatalf("wrangler dependency %q is not an exact version", version)
	}
	var lock struct {
		LockfileVersion int `json:"lockfileVersion"`
		Packages        map[string]struct {
			Version      string            `json:"version"`
			Integrity    string            `json:"integrity"`
			Dependencies map[string]string `json:"dependencies"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(mustReadFile(t, "package-lock.json"), &lock); err != nil {
		t.Fatal(err)
	}
	if lock.LockfileVersion != 3 || lock.Packages[""].Dependencies["wrangler"] != version {
		t.Fatalf("package-lock.json root does not pin wrangler %s", version)
	}
	wrangler := lock.Packages["node_modules/wrangler"]
	if wrangler.Version != version || !strings.HasPrefix(wrangler.Integrity, "sha512-") {
		t.Fatalf("locked wrangler = %#v, want %s with integrity", wrangler, version)
	}
	// The checked publisher needs Wrangler's remote preview R2 binding (4.37+).
	var major, minor int
	if _, err := fmt.Sscanf(version, "%d.%d.", &major, &minor); err != nil {
		t.Fatalf("parse wrangler version %q: %v", version, err)
	}
	if major < 4 || major == 4 && minor < 37 {
		t.Fatalf("wrangler %s is older than the publisher's 4.37 floor", version)
	}
}
