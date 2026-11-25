package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseWorkflowVerifiesArchiveBundleBeforeUploading(t *testing.T) {
	workflow, workflowText := readReleaseWorkflowDocument(t)
	job := workflow.Jobs["attest-and-upload"]
	archiveStep := releaseWorkflowStepIndex(t, job, "Attest platform archive")
	sbomStep := releaseWorkflowStepIndex(t, job, "Attest platform SBOM")
	verifyStep := releaseWorkflowStepIndex(t, job, "Verify platform attestations")
	uploadStep := releaseWorkflowStepIndex(t, job, "Upload platform release assets")
	verify := job.Steps[verifyStep]
	if !(archiveStep < sbomStep && sbomStep < verifyStep && verifyStep < uploadStep) {
		t.Fatal("both attestations must be generated and verified before uploading assets")
	}
	if got := verify.Env["ARCHIVE_BUNDLE"]; got != "${{ steps.attest-archive.outputs.bundle-path }}" {
		t.Fatalf("archive bundle must come from the archive attestation output, got %q", got)
	}
	if !strings.Contains(workflowText, "- name: Attest platform archive\n        id: attest-archive\n") {
		t.Fatal("the archive attestation step must own the exported bundle output")
	}

	const archive = "dist/glade_v1.2.3_linux_arm64.tar.gz"
	const bundle = archive + ".sigstore.json"
	const mockGH = `#!/usr/bin/env bash
set -eu
case "$*" in
  'attestation verify dist/glade_v1.2.3_linux_arm64.tar.gz -R example/release') echo build ;;
  'attestation verify dist/glade_v1.2.3_linux_arm64.tar.gz -R example/release --predicate-type https://cyclonedx.org/bom') echo sbom ;;
  'release view v1.2.3 --json assets --jq .assets[].name') printf '%s\n' "$EXISTING_ASSET" ;;
  'release download v1.2.3 --pattern glade_v1.2.3_linux_arm64.tar.gz.sigstore.json --output dist/glade_v1.2.3_linux_arm64.tar.gz.sigstore.json')
    printf '%s' "$EXISTING_BUNDLE" > "$7" ;;
  'attestation verify dist/glade_v1.2.3_linux_arm64.tar.gz --bundle dist/glade_v1.2.3_linux_arm64.tar.gz.sigstore.json --repo example/release --signer-workflow example/release/.github/workflows/release.yml --source-digest test-commit --source-ref refs/tags/v1.2.3')
    test "$(cat "$5")" = "$VERIFIED_BUNDLE"
    echo bundle ;;
  *) echo "unexpected gh invocation: $*" >&2; exit 1 ;;
esac
`
	const mockUpload = `#!/usr/bin/env bash
set -eu
test "$*" = 'v1.2.3 dist/glade_v1.2.3_linux_arm64.tar.gz dist/glade_v1.2.3_linux_arm64.tar.gz.sha256 dist/glade_v1.2.3_linux_arm64.tar.gz.sbom.json dist/glade_v1.2.3_linux_arm64.tar.gz.sigstore.json dist/release-manifest-linux-arm64.json'
test "$(cat "$5")" = "$VERIFIED_BUNDLE"
echo upload
`
	matrix := strings.NewReplacer("${{ matrix.archive }}", "linux_arm64", "${{ matrix.artifact }}", "linux-arm64", "${{ github.ref_name }}", "v1.2.3")
	for _, tc := range []struct {
		name, generated, existing, verified string
	}{
		{name: "fresh", generated: "fresh proof", verified: "fresh proof"},
		{name: "retry", generated: "new signature", existing: "previous proof", verified: "previous proof"},
		{name: "missing action output"},
		{name: "wrong archive", generated: "wrong subject"},
		{name: "invalid prior bundle", generated: "fresh proof", existing: "invalid proof"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, subdir := range []string{"bin", "dist", "scripts"} {
				if err := os.Mkdir(filepath.Join(dir, subdir), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for name, contents := range map[string]string{"bin/gh": mockGH, "scripts/release-asset-upload.sh": mockUpload, archive: "archive bytes"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			generated := filepath.Join(dir, "attestation output.json")
			if tc.generated != "" {
				if err := os.WriteFile(generated, []byte(tc.generated), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var existingAsset string
			if tc.existing != "" {
				existingAsset = filepath.Base(bundle)
			}
			cmd := exec.Command("bash", "-eo", "pipefail", "-c", matrix.Replace(verify.Run+"\n"+job.Steps[uploadStep].Run))
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+filepath.Join(dir, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
				"GITHUB_REPOSITORY=example/release", "GITHUB_REF=refs/tags/v1.2.3", "GITHUB_REF_NAME=v1.2.3",
				"GITHUB_SHA=test-commit", "VERSION=v1.2.3", "ARCHIVE_BUNDLE="+generated,
				"EXISTING_ASSET="+existingAsset, "EXISTING_BUNDLE="+tc.existing, "VERIFIED_BUNDLE="+tc.verified)
			out, err := cmd.CombinedOutput()
			if tc.verified != "" {
				if err != nil || string(out) != "build\nsbom\nbundle\nupload\n" {
					t.Fatalf("expected build, SBOM, and bundle verification followed by upload: %v\n%s", err, out)
				}
			} else if err == nil || !strings.HasPrefix(string(out), "build\nsbom\n") || strings.Contains(string(out), "\nupload\n") {
				t.Fatalf("invalid proof must fail before asset upload: %v\n%s", err, out)
			}
		})
	}
}

func TestReleaseWorkflowAssetGateRequiresEveryArchiveBundle(t *testing.T) {
	workflow, _ := readReleaseWorkflowDocument(t)
	job := workflow.Jobs["publish"]
	gate := job.Steps[releaseWorkflowStepIndex(t, job, "Verify complete release asset set")].Run
	assets := []string{
		"SHA256SUMS.txt", "glade-release-artifacts-v1.2.3.tar.gz", "index.json",
		"required-ci-attestation.json", "salesforce-release-authority.json", "release-manifest.json",
	}
	for _, platform := range []string{"darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64"} {
		archive := "glade_v1.2.3_" + strings.ReplaceAll(platform, "-", "_") + ".tar.gz"
		assets = append(assets, archive, archive+".sha256", archive+".sbom.json", archive+".sigstore.json", "release-manifest-"+platform+".json")
	}
	complete := strings.Join(assets, "\n")
	for _, tc := range []struct {
		name, assets string
		pass         bool
	}{
		{name: "complete", assets: complete, pass: true},
		{name: "missing proof", assets: strings.Replace(complete, "glade_v1.2.3_darwin_amd64.tar.gz.sigstore.json\n", "", 1)},
		{name: "unexpected proof", assets: complete + "\nunexpected.sigstore.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("bash", "-eo", "pipefail", "-c", `gh() { printf '%s\n' "$RELEASE_ASSETS"; }`+"\n"+gate)
			cmd.Env = append(os.Environ(), "GITHUB_REF_NAME=v1.2.3", "RELEASE_ASSETS="+tc.assets)
			if out, err := cmd.CombinedOutput(); (err == nil) != tc.pass || len(out) != 0 {
				t.Fatalf("asset gate success = %t, want %t: %v\n%s", err == nil, tc.pass, err, out)
			}
		})
	}
}
