package scripts

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestReleaseWorkflowMatchesCIToolchain(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "release.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	for _, want := range []string{
		"macos-15-intel",
		"actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3",
		"actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0",
		`go-version: "1.26.6"`,
		"actions/setup-node@249970729cb0ef3589644e2896645e5dc5ba9c38 # v6.5.0",
		`node-version: "22"`,
		"shared-payload:",
		"glade-release-shared-payload",
		"retention-days: 1",
		"actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02 # v4.6.2",
		"actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093 # v4.3.0",
		"release-v1-go-",
		"release-v1-npm-",
		"cyclonedx-gomod@v1.10.0",
		"scripts/release-bundle.py sbom",
		"Upload platform release assets",
		"scripts/release-asset-upload.sh",
		"gh release download",
		"glade-release-artifacts-$VERSION.tar.gz",
	} {
		if !strings.Contains(workflowText, want) {
			t.Fatalf("release.yml missing %q", want)
		}
	}
	requiredCIBlock := releaseWorkflowJobBlock(t, workflowText, "required-ci-attestation", "salesforce-authority")
	if got := strings.Count(requiredCIBlock, "actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02 # v4.6.2"); got != 1 {
		t.Fatalf("Required CI attestation upload count = %d, want 1", got)
	}
	sharedBlock := releaseWorkflowJobBlock(t, workflowText, "shared-payload", "build")
	if got := strings.Count(sharedBlock, "actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02 # v4.6.2"); got != 1 {
		t.Fatalf("shared payload upload count = %d, want 1", got)
	}
	if got := strings.Count(workflowText, "actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093 # v4.3.0"); got != 4 {
		t.Fatalf("release artifact download count = %d, want payload, platform and both approvals", got)
	}
	if got := strings.Count(workflowText, "actions/setup-node@249970729cb0ef3589644e2896645e5dc5ba9c38 # v6.5.0"); got != 1 {
		t.Fatalf("setup-node count = %d, want shared job only", got)
	}
	buildBlock := releaseWorkflowJobBlock(t, workflowText, "build", "attest-and-upload")
	for _, forbidden := range []string{
		"actions/setup-node", "npm ci", "npm run package", "contents: write", "id-token: write",
		"attestations: write", "actions/attest@", "gh release", "gh attestation",
	} {
		if strings.Contains(buildBlock, forbidden) {
			t.Fatalf("matrix build job contains %q", forbidden)
		}
	}
	for _, want := range []string{
		"needs: [prepare, shared-payload]", "contents: read", "cache: false", "RELEASE_SHARED_PAYLOAD_ARCHIVE",
		"cyclonedx-gomod bin", "json.load", "Upload platform workflow artifacts",
		"name: glade-release-platform-${{ matrix.artifact }}", "retention-days: 7",
	} {
		if !strings.Contains(buildBlock, want) {
			t.Fatalf("matrix build job missing %q", want)
		}
	}

	attestBlock := releaseWorkflowJobBlock(t, workflowText, "attest-and-upload", "publish")
	for _, want := range []string{
		"needs: [prepare, build]", "if: startsWith(github.ref, 'refs/tags/')", "contents: write",
		"id-token: write", "attestations: write", "Download platform workflow artifacts",
		"name: glade-release-platform-${{ matrix.artifact }}", "Attest platform archive", "Attest platform SBOM",
		"Verify platform attestations", "Upload platform release assets",
	} {
		if !strings.Contains(attestBlock, want) {
			t.Fatalf("tag-only attestation job missing %q", want)
		}
	}
	if got := strings.Count(attestBlock, "actions/attest@f7c74d28b9d84cb8768d0b8ca14a4bac6ef463e6 # v4.2.0"); got != 2 {
		t.Fatalf("tag-only attestation action count = %d, want 2", got)
	}
	checkoutIndex := strings.Index(attestBlock, "actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3")
	downloadIndex := strings.Index(attestBlock, "- name: Download platform workflow artifacts")
	uploadIndex := strings.Index(attestBlock, "scripts/release-asset-upload.sh")
	if checkoutIndex < 0 || downloadIndex < 0 || uploadIndex < 0 || checkoutIndex > downloadIndex || downloadIndex > uploadIndex {
		t.Fatal("attestation job requires a source checkout before downloading and uploading release assets")
	}
	if !strings.Contains(attestBlock, "actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3\n        with:\n          persist-credentials: false") {
		t.Fatal("attestation source checkout must not persist credentials")
	}
	if !strings.Contains(workflowText, "publish:\n    needs: attest-and-upload") {
		t.Fatal("publish job can run before all tag-only attestation jobs complete")
	}
	checkoutPin := "actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10 # v6.0.3"
	if checkouts, hardened := strings.Count(workflowText, checkoutPin), strings.Count(workflowText, "persist-credentials: false"); checkouts != hardened {
		t.Fatalf("release checkout hardening count = %d, want one for each of %d checkouts", hardened, checkouts)
	}
}

func TestReleaseWorkflowRequiresSalesforceAuthorityBeforePrepare(t *testing.T) {
	workflow := string(mustReadFile(t, filepath.Join("..", ".github", "workflows", "release.yml")))
	for _, want := range []string{
		"glade_tools_sha:",
		"required: true",
		"salesforce-authority:",
		`scripts/verify-salesforce-check.sh --tag-tools-sha "$GITHUB_REF_NAME"`,
		`scripts/verify-salesforce-check.sh "$GITHUB_SHA" "$glade_tools_sha" > salesforce-release-authority.json`,
		"name: salesforce-release-authority",
		"path: salesforce-release-authority.json",
		"needs: [required-ci-attestation, salesforce-authority]",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("release workflow missing Salesforce authority contract %q", want)
		}
	}
	gate := releaseWorkflowJobBlock(t, workflow, "salesforce-authority", "prepare")
	if strings.Contains(gate, "contents: write") || strings.Contains(gate, "checks: write") {
		t.Fatal("Salesforce verification gate has write authority")
	}
	if gateIndex, prepareIndex := strings.Index(workflow, "  salesforce-authority:"), strings.Index(workflow, "  prepare:"); gateIndex < 0 || gateIndex >= prepareIndex {
		t.Fatal("Salesforce authority gate must precede prepare")
	}
}

func TestReleaseWorkflowDoesNotOverwritePublishedAssets(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "release.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	text := string(workflow)

	if strings.Contains(text, "gh release upload") {
		t.Fatal("release workflow must route every asset through checksum-aware release-asset-upload.sh")
	}
	if !strings.Contains(text, "scripts/release-asset-upload.sh") {
		t.Fatal("release workflow must use the checksum-aware release asset uploader")
	}
	uploader := string(mustReadFile(t, "release-asset-upload.sh"))
	for _, want := range []string{"--json isDraft", "release download", "cmp -s", "published asset differs", "published release cannot accept missing asset", "release upload"} {
		if !strings.Contains(uploader, want) {
			t.Fatalf("checksum-aware release asset uploader missing %q", want)
		}
	}
	if strings.Contains(uploader, "--clobber") {
		t.Fatal("checksum-aware release asset uploader can replace a published asset")
	}
	uploads := releaseWorkflowCommands(text, "upload")
	for _, command := range uploads {
		if strings.Contains(command, "--clobber") {
			t.Fatalf("release upload can replace a published asset: %q", command)
		}
	}

	prepare := releaseWorkflowJobBlock(t, text, "prepare", "shared-payload")
	if !strings.Contains(prepare, `gh release view "$GITHUB_REF_NAME" >/dev/null 2>&1`) ||
		!strings.Contains(prepare, `echo "Release $GITHUB_REF_NAME already exists; reusing it without mutation"`) {
		t.Fatal("prepare job must explicitly reuse an existing release without changing it")
	}
	if got := strings.Count(prepare, "gh release create"); got != 1 {
		t.Fatalf("prepare release create count = %d, want 1", got)
	}
	for _, want := range []string{
		`expected_target="$(git rev-list -n 1 "$GITHUB_REF_NAME")"`,
		`--target "$expected_target"`,
		"--draft",
		`--title "$GITHUB_REF_NAME"`,
		"--notes-file release-notes.md",
		`--json tagName --jq .tagName`,
		`test "$release_tag" = "$GITHUB_REF_NAME"`,
		`test "$(git rev-list -n 1 "$release_tag")" = "$expected_target"`,
	} {
		if !strings.Contains(prepare, want) {
			t.Fatalf("new release creation missing checked release metadata %q", want)
		}
	}

	attest := releaseWorkflowJobBlock(t, text, "attest-and-upload", "publish")
	publish := releaseWorkflowJobBlockUntilEnd(t, text, "publish")
	publishUploads := releaseWorkflowScriptCommands(publish, "scripts/release-asset-upload.sh")
	if len(publishUploads) != 1 {
		t.Fatalf("final publish upload command count = %d, want 1", len(publishUploads))
	}
	finalAssets := []string{
		"dist/SHA256SUMS.txt",
		"dist/index.json",
		"dist/release-manifest.json",
		"dist/required-ci-attestation.json",
		"dist/salesforce-release-authority.json",
		"dist/glade-release-artifacts-$VERSION.tar.gz",
	}
	requireExactReleaseWorkflowAssets(t, releaseWorkflowUploadAssets(publishUploads[0]), finalAssets)
	downloads := releaseWorkflowCommands(publish, "download")
	if len(downloads) != 2 || !strings.Contains(downloads[0], "--clobber") {
		t.Fatal("local release download command must retain --clobber for a clean workspace")
	}
	if !strings.Contains(publish, "VERSION: ${{ github.ref_name }}") {
		t.Fatal("final publish must bind VERSION before referencing its aggregate tarball")
	}
	for _, want := range []string{
		"dist/glade_${{ github.ref_name }}_${{ matrix.archive }}.tar.gz",
		"dist/glade_${{ github.ref_name }}_${{ matrix.archive }}.tar.gz.sha256",
		"dist/glade_${{ github.ref_name }}_${{ matrix.archive }}.tar.gz.sbom.json",
		"dist/release-manifest-${{ matrix.artifact }}.json",
	} {
		if !strings.Contains(attest, want) {
			t.Fatalf("platform attestation job does not own %q", want)
		}
		if strings.Contains(publish, want) {
			t.Fatalf("final publish must not re-upload platform asset %q", want)
		}
	}
	if strings.Contains(publish, "find dist") {
		t.Fatal("final publish must name its aggregate assets explicitly")
	}
	seen := make(map[string]int)
	for commandIndex, command := range releaseWorkflowScriptCommands(text, "scripts/release-asset-upload.sh") {
		for _, asset := range releaseWorkflowUploadAssets(command) {
			if previous, exists := seen[asset]; exists {
				t.Fatalf("published asset %q is uploaded by commands %d and %d", asset, previous, commandIndex)
			}
			seen[asset] = commandIndex
		}
	}
	for _, want := range []string{
		"dist/glade_${{ github.ref_name }}_${{ matrix.archive }}.tar.gz",
		"dist/glade_${{ github.ref_name }}_${{ matrix.archive }}.tar.gz.sha256",
		"dist/glade_${{ github.ref_name }}_${{ matrix.archive }}.tar.gz.sbom.json",
		"dist/release-manifest-${{ matrix.artifact }}.json",
		"dist/SHA256SUMS.txt",
		"dist/index.json",
		"dist/release-manifest.json",
		"dist/required-ci-attestation.json",
		"dist/salesforce-release-authority.json",
		"dist/glade-release-artifacts-$VERSION.tar.gz",
	} {
		if _, exists := seen[want]; !exists {
			t.Fatalf("release upload commands omit %q", want)
		}
	}
	verifyIndex := strings.Index(publish, "- name: Verify complete release asset set")
	publishIndex := strings.Index(publish, `gh release edit "$GITHUB_REF_NAME" --draft=false`)
	if verifyIndex < 0 || publishIndex < 0 || verifyIndex > publishIndex {
		t.Fatal("release must verify its complete asset set before publishing the draft")
	}
	for _, want := range []string{
		`actual_assets="$(gh release view "$GITHUB_REF_NAME" --json assets --jq '.assets[].name' | LC_ALL=C sort)"`,
		`test "$actual_assets" = "$expected_assets"`,
		"release-manifest-darwin-amd64.json",
		"release-manifest-darwin-arm64.json",
		"release-manifest-linux-amd64.json",
		"release-manifest-linux-arm64.json",
	} {
		if !strings.Contains(publish, want) {
			t.Fatalf("complete asset verification missing %q", want)
		}
	}
	for _, want := range []string{
		`gh release view "$GITHUB_REF_NAME" --json isDraft --jq .isDraft`,
		`if [[ "$release_draft" == true ]]; then`,
		`elif [[ "$release_draft" != false ]]; then`,
		"already published; immutable release requires no mutation",
	} {
		if !strings.Contains(publish, want) {
			t.Fatalf("final publish is not safely draft-aware: missing %q", want)
		}
	}
}

func releaseWorkflowCommands(workflow, operation string) []string {
	return releaseWorkflowCommandsWithPrefix(workflow, "gh release "+operation)
}

func releaseWorkflowScriptCommands(workflow, script string) []string {
	return releaseWorkflowCommandsWithPrefix(workflow, script)
}

func releaseWorkflowCommandsWithPrefix(workflow, prefix string) []string {
	var commands []string
	var command []string
	for _, line := range strings.Split(workflow, "\n") {
		trimmed := strings.TrimSpace(line)
		if command == nil {
			if strings.HasPrefix(trimmed, prefix+" ") {
				command = append(command, trimmed)
			}
			continue
		}
		command = append(command, trimmed)
		if !strings.HasSuffix(strings.TrimRight(trimmed, " \t"), "\\") {
			commands = append(commands, strings.Join(command, "\n"))
			command = nil
		}
	}
	if len(command) > 0 {
		commands = append(commands, strings.Join(command, "\n"))
	}
	return commands
}

func releaseWorkflowUploadAssets(command string) []string {
	var assets []string
	for _, line := range strings.Split(command, "\n") {
		asset := strings.TrimSpace(line)
		asset = strings.TrimSpace(strings.TrimSuffix(asset, "\\"))
		if strings.HasPrefix(asset, "dist/") {
			assets = append(assets, asset)
		}
	}
	return assets
}

func requireExactReleaseWorkflowAssets(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("release upload asset count = %d, want %d; got %#v", len(got), len(want), got)
	}
	wanted := make(map[string]struct{}, len(want))
	for _, asset := range want {
		wanted[asset] = struct{}{}
	}
	for _, asset := range got {
		if _, exists := wanted[asset]; !exists {
			t.Fatalf("release upload includes unexpected asset %q; got %#v", asset, got)
		}
		delete(wanted, asset)
	}
	if len(wanted) > 0 {
		t.Fatalf("release upload omits aggregate assets %#v", wanted)
	}
}

func TestReleaseManualBuildHasNoPublishingAuthority(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "release.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	text := string(workflow)
	shared := releaseWorkflowJobBlock(t, text, "shared-payload", "build")
	build := releaseWorkflowJobBlock(t, text, "build", "attest-and-upload")
	for name, block := range map[string]string{"shared-payload": shared, "build": build} {
		for _, forbidden := range []string{"contents: write", "id-token: write", "attestations: write", "gh release", "actions/attest@"} {
			if strings.Contains(block, forbidden) {
				t.Errorf("manual %s job retains %q", name, forbidden)
			}
		}
		if !strings.Contains(block, "permissions:\n      contents: read") {
			t.Errorf("manual %s job lacks explicit contents: read permissions", name)
		}
	}
	if !strings.Contains(build, "needs.prepare.result == 'success' || needs.prepare.result == 'skipped'") {
		t.Fatal("manual build no longer accepts the intentionally skipped tag-only prepare job")
	}
	if strings.Contains(build, "startsWith(github.ref, 'refs/tags/')") {
		t.Fatal("manual platform build became tag-only")
	}
}

func TestProductLicenseIsCanonicalApache20(t *testing.T) {
	contents := mustReadFile(t, filepath.Join("..", "LICENSE"))
	actual := fmt.Sprintf("%x", sha256.Sum256(contents))
	const canonicalApache20SHA256 = "cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30"
	if actual != canonicalApache20SHA256 {
		t.Fatalf("LICENSE sha256 = %s, want canonical Apache-2.0 %s", actual, canonicalApache20SHA256)
	}
}

func TestReleaseBuildSharedPlatformAndDefaultModes(t *testing.T) {
	root, npmLog := makeReleaseBuildFixture(t)
	script := filepath.Join(root, "scripts", "release-build.sh")

	sharedDist := filepath.Join(root, "shared-dist")
	runReleaseBuildFixture(t, root, script, npmLog, sharedDist, "shared-payload", false, nil)
	payload := filepath.Join(sharedDist, "glade-shared-payload.tar.gz")
	payloadSHA := payload + ".sha256"
	for _, path := range []string{payload, payloadSHA} {
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("shared payload artifact %s: info=%v err=%v", path, info, err)
		}
	}
	payloadBytes := mustReadFile(t, payload)
	if len(payloadBytes) < 10 {
		t.Fatalf("shared payload gzip header is too short: %d bytes", len(payloadBytes))
	}
	if payloadBytes[8] != 0 {
		t.Fatalf("shared payload gzip extra flags = %d, want 0 for the deterministic release compression level", payloadBytes[8])
	}
	secondSharedDist := filepath.Join(root, "shared-dist-second")
	runReleaseBuildFixture(t, root, script, npmLog, secondSharedDist, "shared-payload", false, nil)
	secondPayloadSHA := filepath.Join(secondSharedDist, "glade-shared-payload.tar.gz.sha256")
	firstHash := strings.Fields(string(mustReadFile(t, payloadSHA)))[0]
	secondHash := strings.Fields(string(mustReadFile(t, secondPayloadSHA)))[0]
	firstVSIX := releaseArchiveFiles(t, payload)["share/glade/editor/vscode-glade.vsix"]
	secondVSIX := releaseArchiveFiles(t, filepath.Join(secondSharedDist, "glade-shared-payload.tar.gz"))["share/glade/editor/vscode-glade.vsix"]
	assertCanonicalReleaseVSIX(t, firstVSIX)
	assertCanonicalReleaseVSIX(t, secondVSIX)
	if !bytes.Equal(firstVSIX, secondVSIX) {
		t.Fatalf("packaged VSIX is not deterministic: first=%x second=%x", sha256.Sum256(firstVSIX), sha256.Sum256(secondVSIX))
	}
	if firstHash != secondHash {
		t.Fatalf("shared payload is not deterministic: first=%s second=%s", firstHash, secondHash)
	}
	sharedListing := runCommandOutput(t, root, "tar", "-tzf", payload)
	for _, want := range []string{
		"share/glade/editor/vscode-glade.vsix",
		"share/glade/third_party/lwc/package.json",
		"share/glade/lwcruntime/src/lightning/probe.js",
		"share/glade/PAYLOAD-SHA256SUMS.txt",
	} {
		if !strings.Contains(sharedListing, want) {
			t.Fatalf("shared payload missing %q\n%s", want, sharedListing)
		}
	}
	if strings.Contains(sharedListing, "node_modules/.bin") {
		t.Fatalf("shared payload retained non-runtime npm .bin entries:\n%s", sharedListing)
	}
	assertReleasePayloadOnlyRegularFilesAndDirectories(t, payload)
	assertReleaseArchiveShareMatchesPayloadManifest(t, payload)
	npmBefore, err := os.ReadFile(npmLog)
	if err != nil {
		t.Fatal(err)
	}

	platformDist := filepath.Join(root, "platform-dist")
	runReleaseBuildFixture(t, root, script, npmLog, platformDist, "platform", true, map[string]string{
		"RELEASE_SHARED_PAYLOAD_ARCHIVE": payload,
		"RELEASE_SHARED_PAYLOAD_SHA256":  payloadSHA,
	})
	npmAfter, err := os.ReadFile(npmLog)
	if err != nil {
		t.Fatal(err)
	}
	if string(npmAfter) != string(npmBefore) {
		t.Fatalf("platform mode invoked npm:\nbefore=%s\nafter=%s", npmBefore, npmAfter)
	}
	archive := filepath.Join(platformDist, "glade_vtest_linux_amd64.tar.gz")
	archiveListing := runCommandOutput(t, root, "tar", "-tzf", archive)
	assertReleaseArchiveShareMatchesPayloadManifest(t, archive)
	assertReleaseArchiveGoNotices(t, archive, "glade")
	archiveFiles := releaseArchiveFiles(t, archive)
	if notice, ok := archiveFiles["NOTICE"]; !ok || string(notice) != "Glade\nCopyright 2026 Matt Simonis\n" {
		t.Fatalf("platform archive NOTICE = %q, present=%v", notice, ok)
	}
	for _, want := range []string{
		"glade", "LICENSE", "NOTICE", "THIRD_PARTY_NOTICES/NOTICE-MANIFEST.json", "THIRD_PARTY_NOTICES/go/LICENSE",
		"THIRD_PARTY_NOTICES/modules/github.com/glade-sh/apex-parser/@v0.1.0/NOTICE.md",
		"share/glade/editor/vscode-glade.vsix", "share/glade/third_party/lwc/package.json",
	} {
		if !strings.Contains(archiveListing, want) {
			t.Fatalf("platform archive missing %q\n%s", want, archiveListing)
		}
	}
	manifestBytes, err := os.ReadFile(filepath.Join(platformDist, "release-manifest-linux-amd64.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Verification struct {
			SharedPayloadSHA256 string `json:"sharedPayloadSHA256"`
		} `json:"verification"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("parse platform manifest: %v\n%s", err, manifestBytes)
	}
	wantPayloadSHA := strings.Fields(string(mustReadFile(t, payloadSHA)))[0]
	if manifest.Verification.SharedPayloadSHA256 != wantPayloadSHA {
		t.Fatalf("shared payload hash = %q, want %q", manifest.Verification.SharedPayloadSHA256, wantPayloadSHA)
	}

	defaultDist := filepath.Join(root, "default-dist")
	runReleaseBuildFixture(t, root, script, npmLog, defaultDist, "", false, nil)
	defaultArchive := filepath.Join(defaultDist, "glade_vtest_linux_amd64.tar.gz")
	if info, err := os.Stat(defaultArchive); err != nil || info.Size() == 0 {
		t.Fatalf("default mode archive: info=%v err=%v", info, err)
	}
	assertReleaseArchiveShareMatchesPayloadManifest(t, defaultArchive)
	assertReleaseArchiveGoNotices(t, defaultArchive, "glade")
	tarLog := string(mustReadFile(t, filepath.Join(root, "tar.log")))
	creationCount := 0
	for _, line := range strings.Split(strings.TrimSpace(tarLog), "\n") {
		if !strings.Contains(line, " -czf ") {
			continue
		}
		creationCount++
		if !strings.HasPrefix(line, "1 ") {
			t.Fatalf("platform tar creation did not suppress macOS copyfile metadata: %q", line)
		}
	}
	if creationCount != 2 {
		t.Fatalf("platform tar creation count = %d, want 2\n%s", creationCount, tarLog)
	}
}

func TestReleaseBuildPlatformRejectsUnsafePayloadArchives(t *testing.T) {
	root, npmLog := makeReleaseBuildFixture(t)
	script := filepath.Join(root, "scripts", "release-build.sh")
	tests := []struct {
		name    string
		entries []releasePayloadTarEntry
	}{
		{name: "traversal", entries: []releasePayloadTarEntry{{name: "../escape", body: "escape"}}},
		{name: "absolute", entries: []releasePayloadTarEntry{{name: "/tmp/glade-release-absolute", body: "escape"}}},
		{name: "symlink", entries: []releasePayloadTarEntry{{name: "share/glade/link", typeflag: tar.TypeSymlink, linkname: "../../outside"}}},
		{name: "hardlink", entries: []releasePayloadTarEntry{{name: "share/glade/link", typeflag: tar.TypeLink, linkname: "share/glade/target"}}},
		{name: "unexpected-root", entries: []releasePayloadTarEntry{{name: "LICENSE", body: "not shared"}}},
		{name: "special", entries: []releasePayloadTarEntry{{name: "share/glade/fifo", typeflag: tar.TypeFifo}}},
		{name: "manifest-omission", entries: releasePayloadWithManifestOmission(t)},
		{name: "manifest-extra", entries: releasePayloadWithManifestExtra(t)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			archive, checksum := writeReleasePayloadArchive(t, root, tc.name, tc.entries)
			dist := filepath.Join(root, "hostile-dist-"+tc.name)
			out := runReleaseBuildFixtureError(t, root, script, npmLog, dist, map[string]string{
				"RELEASE_SHARED_PAYLOAD_ARCHIVE": archive,
				"RELEASE_SHARED_PAYLOAD_SHA256":  checksum,
			})
			if !strings.Contains(out, "ERROR: unsafe shared payload") && !strings.Contains(out, "ERROR: shared payload manifest") {
				t.Fatalf("platform rejection lacked safe-extraction diagnostic:\n%s", out)
			}
		})
	}
	if _, err := os.Stat(filepath.Join(root, "escape")); !os.IsNotExist(err) {
		t.Fatalf("traversal payload wrote outside extraction root: %v", err)
	}
}

func TestReleaseBuildRejectsMissingLinkedModuleNotice(t *testing.T) {
	root, npmLog := makeReleaseBuildFixture(t)
	script := filepath.Join(root, "scripts", "release-build.sh")
	if err := os.Remove(filepath.Join(root, "go-mod-cache", "example.com", "linked@v1.2.3", "LICENSE")); err != nil {
		t.Fatal(err)
	}
	payloadDist := filepath.Join(root, "shared-dist")
	runReleaseBuildFixture(t, root, script, npmLog, payloadDist, "shared-payload", false, nil)
	out := runReleaseBuildFixtureError(t, root, script, npmLog, filepath.Join(root, "platform-dist"), map[string]string{
		"RELEASE_SHARED_PAYLOAD_ARCHIVE": filepath.Join(payloadDist, "glade-shared-payload.tar.gz"),
		"RELEASE_SHARED_PAYLOAD_SHA256":  filepath.Join(payloadDist, "glade-shared-payload.tar.gz.sha256"),
	})
	if !strings.Contains(out, "linked module source lacks notice evidence") {
		t.Fatalf("missing linked notice rejection diagnostic:\n%s", out)
	}
}

type releasePayloadTarEntry struct {
	name     string
	typeflag byte
	linkname string
	body     string
}

func writeReleasePayloadArchive(t *testing.T, root, name string, entries []releasePayloadTarEntry) (string, string) {
	t.Helper()
	archive := filepath.Join(root, "hostile-"+name+".tar.gz")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		header := &tar.Header{
			Name:     entry.name,
			Mode:     0o644,
			Size:     int64(len(entry.body)),
			Typeflag: typeflag,
			Linkname: entry.linkname,
		}
		if typeflag == tar.TypeDir {
			header.Mode = 0o755
			header.Size = 0
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Size > 0 {
			if _, err := tarWriter.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	contents := mustReadFile(t, archive)
	checksum := filepath.Join(root, "hostile-"+name+".tar.gz.sha256")
	digest := sha256.Sum256(contents)
	writeReleaseFixtureFile(t, checksum, fmt.Sprintf("%x  %s\n", digest, filepath.Base(archive)))
	return archive, checksum
}

func releasePayloadWithManifestOmission(t *testing.T) []releasePayloadTarEntry {
	t.Helper()
	directories := []string{
		"share", "share/glade", "share/glade/editor", "share/glade/third_party", "share/glade/third_party/lwc",
		"share/glade/lwcruntime", "share/glade/lwcruntime/src", "share/glade/lwcruntime/src/experience",
		"share/glade/lwcruntime/src/lightning", "share/glade/lwcruntime/src/shell", "share/glade/lwcruntime/src/shims",
		"share/glade/lwcruntime/src/slds",
	}
	entries := make([]releasePayloadTarEntry, 0, len(directories)+8)
	for _, directory := range directories {
		entries = append(entries, releasePayloadTarEntry{name: directory, typeflag: tar.TypeDir})
	}
	files := map[string]string{
		"share/glade/VSCODE-EXTENSION-STATUS.txt":           "not present\n",
		"share/glade/third_party/lwc/package.json":          "{}\n",
		"share/glade/lwcruntime/src/experience/probe.js":    "experience\n",
		"share/glade/lwcruntime/src/lightning/probe.js":     "lightning\n",
		"share/glade/lwcruntime/src/shell/probe.js":         "shell\n",
		"share/glade/lwcruntime/src/shims/probe.js":         "shims\n",
		"share/glade/lwcruntime/src/slds/unlisted-probe.js": "omitted\n",
	}
	manifest := strings.Builder{}
	for path, body := range files {
		entries = append(entries, releasePayloadTarEntry{name: path, body: body})
		if strings.Contains(path, "unlisted-probe") {
			continue
		}
		digest := sha256.Sum256([]byte(body))
		fmt.Fprintf(&manifest, "%x  %s\n", digest, path)
	}
	entries = append(entries, releasePayloadTarEntry{name: "share/glade/PAYLOAD-SHA256SUMS.txt", body: manifest.String()})
	return entries
}

func releasePayloadWithManifestExtra(t *testing.T) []releasePayloadTarEntry {
	t.Helper()
	entries := releasePayloadWithManifestOmission(t)
	omittedDigest := sha256.Sum256([]byte("omitted\n"))
	extraDigest := sha256.Sum256([]byte("missing\n"))
	for i := range entries {
		if entries[i].name == "share/glade/PAYLOAD-SHA256SUMS.txt" {
			entries[i].body += fmt.Sprintf("%x  share/glade/lwcruntime/src/slds/unlisted-probe.js\n", omittedDigest)
			entries[i].body += fmt.Sprintf("%x  share/glade/missing.txt\n", extraDigest)
			return entries
		}
	}
	t.Fatal("manifest entry missing from fixture")
	return nil
}

func runReleaseBuildFixtureError(t *testing.T, root, script, npmLog, dist string, extra map[string]string) string {
	t.Helper()
	cmd := exec.Command("bash", script, "platform")
	cmd.Dir = root
	env := append(os.Environ(),
		"PATH="+filepath.Join(root, "fake-bin")+":"+os.Getenv("PATH"),
		"FAKE_NPM_LOG="+npmLog,
		"FAKE_TAR_LOG="+filepath.Join(root, "tar.log"),
		"FAKE_GLADE_BINARY="+filepath.Join(root, "fake-glade"),
		"FAKE_GO_MOD_CACHE="+filepath.Join(root, "go-mod-cache"),
		"FAKE_GO_ROOT="+filepath.Join(root, "go-install", "libexec"),
		"FAIL_NPM=1",
		"VERSION=vtest",
		"DIST_DIR="+dist,
	)
	for key, value := range extra {
		env = append(env, key+"="+value)
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("hostile platform payload unexpectedly succeeded:\n%s", out)
	}
	return string(out)
}

func releaseWorkflowJobBlock(t *testing.T, workflow, start, end string) string {
	t.Helper()
	startMarker := "  " + start + ":"
	startAt := strings.Index(workflow, startMarker)
	if startAt < 0 {
		t.Fatalf("workflow missing job %q", start)
	}
	endAt := strings.Index(workflow[startAt+len(startMarker):], "\n  "+end+":")
	if endAt < 0 {
		t.Fatalf("workflow missing job %q after %q", end, start)
	}
	return workflow[startAt : startAt+len(startMarker)+endAt]
}

func releaseWorkflowJobBlockUntilEnd(t *testing.T, workflow, start string) string {
	t.Helper()
	startMarker := "  " + start + ":"
	startAt := strings.Index(workflow, startMarker)
	if startAt < 0 {
		t.Fatalf("workflow missing job %q", start)
	}
	return workflow[startAt:]
}

func makeReleaseBuildFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	releaseScript := mustReadFile(t, filepath.Join("release-build.sh"))
	writeExecutable(t, filepath.Join(root, "scripts", "release-build.sh"), string(releaseScript))
	writeReleaseFixtureFile(t, filepath.Join(root, "scripts", "release-go-notices.py"), string(mustReadFile(t, filepath.Join("release-go-notices.py"))))
	writeReleaseFixtureFile(t, filepath.Join(root, "LICENSE"), "fixture license\n")
	writeReleaseFixtureFile(t, filepath.Join(root, "NOTICE"), "Glade\nCopyright 2026 Matt Simonis\n")
	writeReleaseFixtureFile(t, filepath.Join(root, "third_party", "glade-apex-parser", "LICENSE"), "parser license\n")
	writeReleaseFixtureFile(t, filepath.Join(root, "third_party", "glade-apex-parser", "NOTICE.md"), "parser notice\n")
	writeReleaseFixtureFile(t, filepath.Join(root, "go-mod-cache", "example.com", "linked@v1.2.3", "LICENSE"), "linked license\n")
	if err := os.MkdirAll(filepath.Join(root, "go-install", "libexec"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeReleaseFixtureFile(t, filepath.Join(root, "go-install", "LICENSE"), "Go distribution license\n")
	writeReleaseFixtureFile(t, filepath.Join(root, "third_party", "lwc", "package.json"), "{}\n")
	writeReleaseFixtureFile(t, filepath.Join(root, "contrib", "vscode-glade", "package.json"), "{}\n")
	for _, dir := range []string{"experience", "lightning", "shell", "shims", "slds"} {
		writeReleaseFixtureFile(t, filepath.Join(root, "lwcruntime", "src", dir, "probe.js"), "export default 1;\n")
	}

	binDir := filepath.Join(root, "fake-bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	npmLog := filepath.Join(root, "npm.log")
	writeExecutable(t, filepath.Join(binDir, "npm"), `#!/usr/bin/env bash
set -euo pipefail
echo "$PWD $*" >> "${FAKE_NPM_LOG}"
if [[ "${FAIL_NPM:-0}" == "1" ]]; then exit 97; fi
if [[ "${1:-}" == "ci" ]]; then
  mkdir -p node_modules/.bin node_modules/fixture-tool node_modules/@locker/fixture-plugin/node_modules/.bin node_modules/@locker/fixture-plugin/node_modules/jsesc/bin
  printf 'console.log("fixture");\n' > node_modules/fixture-tool/cli.js
  printf 'console.log("nested fixture");\n' > node_modules/@locker/fixture-plugin/node_modules/jsesc/bin/jsesc
  ln -s ../fixture-tool/cli.js node_modules/.bin/fixture-tool
  ln -s ../jsesc/bin/jsesc node_modules/@locker/fixture-plugin/node_modules/.bin/jsesc
fi
if [[ "${1:-} ${2:-}" == "run package" ]]; then
  mkdir -p dist
  sequence="$(grep -c 'run package' "${FAKE_NPM_LOG}")"
  FAKE_VSIX_SEQUENCE="${sequence}" python3 - <<'PY'
import os
from zipfile import ZipFile, ZipInfo, ZIP_DEFLATED

sequence = int(os.environ["FAKE_VSIX_SEQUENCE"])
entries = [("extension/z.txt", b"z\n"), ("extension/a.txt", b"a\n")]
if sequence % 2 == 0:
    entries.reverse()
with ZipFile("dist/vscode-glade-fixture.vsix", "w") as archive:
    for name, body in entries:
        info = ZipInfo(name, (2026, 9, 4, 13, 0, sequence * 2))
        info.compress_type = ZIP_DEFLATED
        archive.writestr(info, body)
PY
fi
`)
	fakeGlade := filepath.Join(root, "fake-glade")
	writeExecutable(t, fakeGlade, `#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  doctor) printf '{"parserOK": true}\n' ;;
  version) printf 'glade vtest\n' ;;
  parse) printf '{"name": "ParserSmoke"}\n' ;;
  *) exit 2 ;;
esac
`)
	writeExecutable(t, filepath.Join(binDir, "go"), `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "env" ]]; then
  case "${2:-}" in GOOS) echo linux ;; GOARCH) echo amd64 ;; GOMODCACHE) echo "${FAKE_GO_MOD_CACHE}" ;; GOROOT) echo "${FAKE_GO_ROOT}" ;; *) exit 2 ;; esac
  exit 0
fi
if [[ "${1:-}" == "version" && "${2:-}" == "-m" ]]; then
  cat <<'EOF'
path fixture/glade
dep example.com/linked v1.2.3
dep github.com/glade-sh/apex-parser v0.1.0
=> ./third_party/glade-apex-parser (devel)
EOF
  exit 0
fi
if [[ "${1:-}" == "build" ]]; then
  out=""
  shift
  while (($#)); do
    if [[ "$1" == "-o" ]]; then out="$2"; shift 2; continue; fi
    shift
  done
  cp "${FAKE_GLADE_BINARY}" "$out"
  chmod +x "$out"
  exit 0
fi
exit 2
`)
	writeExecutable(t, filepath.Join(binDir, "tar"), `#!/usr/bin/env bash
set -euo pipefail
echo "${COPYFILE_DISABLE:-unset} $*" >> "${FAKE_TAR_LOG}"
exec /usr/bin/tar "$@"
`)
	return root, npmLog
}

func runReleaseBuildFixture(t *testing.T, root, script, npmLog, dist, mode string, failNPM bool, extra map[string]string) {
	t.Helper()
	args := []string{script}
	if mode != "" {
		args = append(args, mode)
	}
	cmd := exec.Command("bash", args...)
	cmd.Dir = root
	env := append(os.Environ(),
		"PATH="+filepath.Join(root, "fake-bin")+":"+os.Getenv("PATH"),
		"FAKE_NPM_LOG="+npmLog,
		"FAKE_TAR_LOG="+filepath.Join(root, "tar.log"),
		"FAKE_GLADE_BINARY="+filepath.Join(root, "fake-glade"),
		"FAKE_GO_MOD_CACHE="+filepath.Join(root, "go-mod-cache"),
		"FAKE_GO_ROOT="+filepath.Join(root, "go-install", "libexec"),
		"VERSION=vtest",
		"DIST_DIR="+dist,
	)
	if failNPM {
		env = append(env, "FAIL_NPM=1")
	}
	for key, value := range extra {
		env = append(env, key+"="+value)
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release-build mode %q failed: %v\n%s", mode, err, out)
	}
}

func assertReleaseArchiveGoNotices(t *testing.T, archive, binary string) {
	t.Helper()
	files := releaseArchiveFiles(t, archive)
	manifestBytes, ok := files["THIRD_PARTY_NOTICES/NOTICE-MANIFEST.json"]
	if !ok {
		t.Fatal("archive Go notice manifest is missing")
	}
	var manifest struct {
		BinarySHA256 string `json:"binarySHA256"`
		GoLicense    string `json:"goLicense"`
		Components   []struct {
			Module      string   `json:"module"`
			NoticeFiles []string `json:"noticeFiles"`
		} `json:"components"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("parse archive Go notice manifest: %v", err)
	}
	binaryBytes, ok := files[binary]
	if !ok {
		t.Fatalf("archive binary %q is missing", binary)
	}
	actualHash := sha256.Sum256(binaryBytes)
	if manifest.BinarySHA256 != fmt.Sprintf("%x", actualHash) {
		t.Fatalf("Go notice binary hash = %q, want %x", manifest.BinarySHA256, actualHash)
	}
	if manifest.GoLicense != "go/LICENSE" || len(files["THIRD_PARTY_NOTICES/go/LICENSE"]) == 0 {
		t.Fatal("archive Go distribution notice is missing")
	}
	parserFound := false
	for _, component := range manifest.Components {
		if component.Module == "github.com/glade-sh/apex-parser" {
			parserFound = true
		}
		for _, notice := range component.NoticeFiles {
			if len(files["THIRD_PARTY_NOTICES/"+notice]) == 0 {
				t.Fatalf("archive is missing nonempty component notice %q", notice)
			}
		}
	}
	if !parserFound {
		t.Fatal("archive Go notice manifest lacks vendored parser component")
	}
}

func assertCanonicalReleaseVSIX(t *testing.T, contents []byte) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		t.Fatalf("read packaged VSIX: %v", err)
	}
	previous := ""
	for _, file := range reader.File {
		if file.Name < previous {
			t.Errorf("VSIX member order is not canonical: %q before %q", previous, file.Name)
		}
		previous = file.Name
		if file.Modified.Year() != 1980 || file.Modified.Month() != 1 || file.Modified.Day() != 1 ||
			file.Modified.Hour() != 0 || file.Modified.Minute() != 0 || file.Modified.Second() != 0 {
			t.Errorf("VSIX member %q timestamp = %s, want 1980-01-01T00:00:00", file.Name, file.Modified)
		}
	}
}

func releaseArchiveFiles(t *testing.T, archive string) map[string][]byte {
	t.Helper()
	file, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()
	files := make(map[string][]byte)
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return files
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		contents, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatal(err)
		}
		files[header.Name] = contents
	}
}

func writeReleaseFixtureFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func runCommandOutput(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func assertReleasePayloadOnlyRegularFilesAndDirectories(t *testing.T, archive string) {
	t.Helper()
	file, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return
			}
			t.Fatal(err)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA && header.Typeflag != tar.TypeDir {
			t.Fatalf("prepared payload member %q has type %d, want regular file or directory", header.Name, header.Typeflag)
		}
	}
}

func assertReleaseArchiveShareMatchesPayloadManifest(t *testing.T, archive string) {
	t.Helper()
	file, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	actual := make(map[string]bool)
	manifestContents := ""
	const manifestPath = "share/glade/PAYLOAD-SHA256SUMS.txt"
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(filepath.Base(header.Name), "._") {
			t.Fatalf("archive contains AppleDouble member %q", header.Name)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		if !strings.HasPrefix(header.Name, "share/glade/") {
			continue
		}
		if actual[header.Name] {
			t.Fatalf("archive contains duplicate regular member %q", header.Name)
		}
		actual[header.Name] = true
		if header.Name == manifestPath {
			contents, err := io.ReadAll(tarReader)
			if err != nil {
				t.Fatal(err)
			}
			manifestContents = string(contents)
		}
	}
	if manifestContents == "" {
		t.Fatal("archive payload checksum manifest is empty or missing")
	}
	expected := map[string]bool{manifestPath: true}
	for _, line := range strings.Split(strings.TrimSpace(manifestContents), "\n") {
		fields := strings.SplitN(line, "  ", 2)
		if len(fields) != 2 || fields[1] == "" {
			t.Fatalf("invalid payload checksum line %q", line)
		}
		expected[fields[1]] = true
	}
	if len(actual) != len(expected) {
		t.Fatalf("archive share regular-file count = %d, manifest plus itself = %d", len(actual), len(expected))
	}
	for name := range expected {
		if !actual[name] {
			t.Fatalf("archive share regular-file set missing %q", name)
		}
	}
}

func TestInstallScriptSupportsPrivateReleaseToken(t *testing.T) {
	installPath := filepath.Join("..", "site", "install.sh")
	installScript, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatalf("read %s: %v", installPath, err)
	}
	scriptText := string(installScript)
	for _, want := range []string{
		"GLADE_GITHUB_TOKEN",
		"GH_TOKEN",
		"GITHUB_TOKEN",
		"Authorization: Bearer",
		"api.github.com/repos",
		"curl_github",
		"download_asset",
		"Accept: application/octet-stream",
		"private repo",
	} {
		if !strings.Contains(scriptText, want) {
			t.Fatalf("install.sh missing %q", want)
		}
	}
}

func TestReleaseBuildPackagesLWCRuntimeAssets(t *testing.T) {
	releasePath := filepath.Join("..", "scripts", "release-build.sh")
	releaseScript, err := os.ReadFile(releasePath)
	if err != nil {
		t.Fatalf("read %s: %v", releasePath, err)
	}
	scriptText := string(releaseScript)
	for _, want := range []string{
		"lwcruntime/src/experience",
		"lwcruntime/src/lightning",
		"lwcruntime/src/shell",
		"lwcruntime/src/shims",
		"lwcruntime/src/slds",
	} {
		if !strings.Contains(scriptText, want) {
			t.Fatalf("release-build.sh missing %q", want)
		}
	}
}

func TestReleaseBuildKeepsDoctorJSONWhenDoctorReportsLocalDataFailure(t *testing.T) {
	releasePath := filepath.Join("..", "scripts", "release-build.sh")
	releaseScript, err := os.ReadFile(releasePath)
	if err != nil {
		t.Fatalf("read %s: %v", releasePath, err)
	}
	scriptText := string(releaseScript)
	for _, want := range []string{
		`doctor_out="$("${platform_root}/${binary}" doctor --json 2>&1 || true)"`,
		`doctor_json="$("${verifydir}/${binary}" doctor --json 2>&1 || true)"`,
		`"parserOK": true`,
	} {
		if !strings.Contains(scriptText, want) {
			t.Fatalf("release-build.sh missing %q", want)
		}
	}
}

func TestReleaseWorkflowUsesRepoReleaseNotes(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "release.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	for _, want := range []string{
		"Build release notes",
		`scripts/release-notes.sh "$GITHUB_REF_NAME" > release-notes.md`,
		"--notes-file release-notes.md",
	} {
		if !strings.Contains(workflowText, want) {
			t.Fatalf("release.yml missing %q", want)
		}
	}
	if strings.Contains(workflowText, `--notes "`) {
		t.Fatalf("release.yml should not publish inline release notes")
	}
}

func TestReleaseWorkflowPreservesDownloadIndexHistory(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "release.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	for _, want := range []string{
		"scripts/release-index.py",
		`--version "$VERSION"`,
		"--output index.json",
	} {
		if !strings.Contains(workflowText, want) {
			t.Fatalf("release.yml missing index-history marker %q", want)
		}
	}

	indexPath := filepath.Join("release-index.py")
	indexScript, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read %s: %v", indexPath, err)
	}
	indexText := string(indexScript)
	for _, want := range []string{
		"urllib.request.Request",
		"glade-release-workflow/1.0",
		"versions_by_version",
		"could not read existing download index",
	} {
		if !strings.Contains(indexText, want) {
			t.Fatalf("release-index.py missing index-history marker %q", want)
		}
	}
}

func TestReleaseIndexScriptPreservesHistoryWithReleaseUserAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.UserAgent(); got != "glade-release-workflow/1.0" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "schemaVersion": 1,
  "channel": "stable",
  "latest": "v0.2.3",
  "versions": [
    {"version": "v0.2.3", "manifest": "https://downloads.glade.sh/v0.2.3/release-manifest.json"},
    {"version": "v0.2.2", "manifest": "https://downloads.glade.sh/v0.2.2/release-manifest.json"}
  ]
}`))
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "index.json")
	cmd := exec.Command("python3", "release-index.py",
		"--version", "v0.2.4",
		"--download-base", "https://downloads.glade.sh",
		"--existing-index-url", server.URL,
		"--output", outputPath,
	)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release-index.py failed: %v\n%s", err, out)
	}

	var index struct {
		Latest   string `json:"latest"`
		Versions []struct {
			Version  string `json:"version"`
			Manifest string `json:"manifest"`
		} `json:"versions"`
	}
	indexBytes, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read generated index: %v", err)
	}
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		t.Fatalf("parse generated index: %v\n%s", err, indexBytes)
	}
	if index.Latest != "v0.2.4" {
		t.Fatalf("latest = %q, want v0.2.4", index.Latest)
	}
	gotVersions := make([]string, 0, len(index.Versions))
	for _, row := range index.Versions {
		gotVersions = append(gotVersions, row.Version)
	}
	wantVersions := []string{"v0.2.4", "v0.2.3", "v0.2.2"}
	if strings.Join(gotVersions, ",") != strings.Join(wantVersions, ",") {
		t.Fatalf("versions = %#v, want %#v\n%s", gotVersions, wantVersions, indexBytes)
	}
	if index.Versions[0].Manifest != "https://downloads.glade.sh/v0.2.4/release-manifest.json" {
		t.Fatalf("current manifest = %q", index.Versions[0].Manifest)
	}
}

func TestReleaseWorkflowCopiesVersionManifestIntoVersionDirectory(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "release.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	if !strings.Contains(workflowText, `shutil.copyfile("release-manifest.json", os.path.join(version, "release-manifest.json"))`) {
		t.Fatalf("release.yml should copy the combined manifest into the version directory")
	}
}

func TestReleaseNotesScriptExtractsVersionSectionWithRealLineBreaks(t *testing.T) {
	releaseNotesPath := filepath.Join("..", "docs", "RELEASE_NOTES.md")
	releaseNotes, err := os.ReadFile(releaseNotesPath)
	if err != nil {
		t.Fatalf("read %s: %v", releaseNotesPath, err)
	}
	if !strings.Contains(string(releaseNotes), "## v0.2.9 - 2026-07-25") {
		t.Fatalf("release notes missing v0.2.9 planned release date\n%s", releaseNotes)
	}

	cmd := exec.Command("bash", "release-notes.sh", "v0.2.9")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release-notes.sh v0.2.9 failed: %v\n%s", err, out)
	}
	notes := string(out)
	if strings.TrimSpace(notes) == "" {
		t.Fatal("release notes were empty")
	}
	normalizedNotes := strings.Join(strings.Fields(notes), " ")
	for _, want := range []string{
		"11.45%",
		"11.35%",
		"8.00%",
		"9.00%",
		"4,565",
		"4,561 passes",
		"11,526",
		"same four 60-second cap timeouts",
		"zero failures, unsupported results, load errors, compile errors, and internal errors.",
		"filesystem/root confinement",
		"safe JSON serialization",
		"compatibility",
		"creates an absent release once",
		"metadata, title, and body are reused without editing",
		"duplicate asset name fails instead of replacing published bytes",
	} {
		if !strings.Contains(normalizedNotes, want) {
			t.Fatalf("release notes missing %q\n%s", want, notes)
		}
	}
	for _, notWant := range []string{
		`\n`,
		"## v0.2.9",
		"## v0.2.8",
		"## Unreleased",
	} {
		if strings.Contains(notes, notWant) {
			t.Fatalf("release notes unexpectedly contain %q\n%s", notWant, notes)
		}
	}
	for _, forbidden := range []struct {
		pattern  *regexp.Regexp
		mutation string
	}{
		{regexp.MustCompile(`(?i)\b4,?565\s*(?:/|of)\s*4,?565\b`), "4,565/4,565 passed"},
		{regexp.MustCompile(`(?i)\b(?:all|every)\s+4,?565\s+(?:tests?\s+)?(?:pass(?:ed)?|succeeded)\b`), "All 4,565 tests passed"},
		{regexp.MustCompile(`(?i)\b4,?565\s+(?:tests?\s+)?(?:pass(?:ed)?|succeeded)\b`), "4,565 passed"},
		{regexp.MustCompile(`(?i)\b(?:zero|no)\s+(?:60-second\s+cap\s+)?timeouts?\b`), "No 60-second cap timeouts"},
		{regexp.MustCompile(`(?i)\btimeouts?\s*[:=-]?\s*(?:zero|none|0)\b`), "timeouts: none"},
		{regexp.MustCompile(`(?i)\b(?:zero|no)\s+tests?\s+timed\s+out\b`), "No tests timed out"},
		{regexp.MustCompile(`(?i)\b(?:all\s+salesforce\s+behaviou?r\s+is\s+certified|universal\s+salesforce\s+(?:compatibility|certification))\b`), "All Salesforce behavior is certified"},
	} {
		if !forbidden.pattern.MatchString(forbidden.mutation) {
			t.Fatalf("release note guard does not reject mutation %q", forbidden.mutation)
		}
		if forbidden.pattern.MatchString(normalizedNotes) {
			t.Fatalf("release notes overclaim controlled comparison with %q\n%s", forbidden.pattern, notes)
		}
	}
}

func TestReleaseNotesScriptExtractsV0212TaggedSection(t *testing.T) {
	cmd := exec.Command("bash", "release-notes.sh", "v0.2.12")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release-notes.sh v0.2.12 failed: %v\n%s", err, out)
	}
	notes := string(out)
	normalizedNotes := strings.Join(strings.Fields(notes), " ")
	for _, want := range []string{
		"Tagged v0.2.12 validation:",
		"This final tagged validation is separate from the published v0.2.11 surface snapshot and does not claim blanket Salesforce parity.",
		"moving-correctness",
		"malformed or future Apex project versions and unsupported Execute Anonymous, LWC, or endpoint versions fail rather than silently falling back.",
		"explicit non-parity",
		"source receipt",
		"Local-test progress event rendering is serialized so concurrent workers cannot corrupt terminal or NDJSON output.",
		"macOS and Linux archives for AMD64 and ARM64, `SHA256SUMS.txt`, CycloneDX SBOMs, and provenance attestations.",
		"No migration is required for documented CLI flags, database schemas, or server API behavior.",
		"default local API version remains `v65.0`",
		"https://github.com/glade-sh/glade/blob/v0.2.12/docs/DISTRIBUTION_WORKFLOW.md",
		"https://github.com/glade-sh/glade/blob/v0.2.12/docs/KNOWN_GAPS.md",
	} {
		if !strings.Contains(normalizedNotes, want) {
			t.Fatalf("release notes missing %q\n%s", want, notes)
		}
	}
	for _, notWant := range []string{"## v0.2.12", "## v0.2.11", "## Unreleased"} {
		if strings.Contains(notes, notWant) {
			t.Fatalf("release notes unexpectedly contain %q\n%s", notWant, notes)
		}
	}
}

func TestCIWorkflowDoesNotCheckoutGladeTools(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "ci.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	workflowText := string(workflow)
	for _, forbidden := range []string{
		"Resolve glade-tools ref",
		"scripts/resolve-sibling-ref.sh",
		"steps.glade-tools-ref.outputs.ref",
		"repository: glade-sh/glade-tools",
		"actions/create-github-app-token",
	} {
		if strings.Contains(workflowText, forbidden) {
			t.Fatalf("ci.yml retains unused glade-tools marker %q", forbidden)
		}
	}
	if _, err := os.Stat("resolve-sibling-ref.sh"); err != nil {
		t.Fatalf("release helper must remain available: %v", err)
	}
}

func TestResolveSiblingRefScript(t *testing.T) {
	remoteWithTag := makeGitRemote(t, "v9.9.9")
	if got := runResolveSiblingRef(t, remoteWithTag, "v9.9.9", "main"); got != "v9.9.9" {
		t.Fatalf("tagged remote resolved %q, want v9.9.9", got)
	}

	remoteWithoutTag := makeGitRemote(t, "")
	if got := runResolveSiblingRef(t, remoteWithoutTag, "v9.9.9", "main"); got != "main" {
		t.Fatalf("untagged remote resolved %q, want main", got)
	}
}

func TestInstallScriptStagesToolchainReplacement(t *testing.T) {
	installPath := filepath.Join("..", "site", "install.sh")
	installScript, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatalf("read %s: %v", installPath, err)
	}
	scriptText := string(installScript)
	for _, want := range []string{
		"replace_toolchain_dir",
		".install.",
		".backup.",
		`mv "$share_dir/$name" "$backup"`,
		"restore",
	} {
		if !strings.Contains(scriptText, want) {
			t.Fatalf("install.sh missing staged replacement marker %q", want)
		}
	}
}

func TestInstallDocsPutEnvOnShellSideOfPipe(t *testing.T) {
	badPipe := regexp.MustCompile(`GLADE_[A-Z_]+=.*curl -fsSL https://glade\.sh/install\.sh \| sh`)
	for _, docPath := range []string{
		filepath.Join("..", "docs", "INSTALL.md"),
		filepath.Join("..", "site", "docs-src", "guide", "installation.md"),
	} {
		contents, err := os.ReadFile(docPath)
		if err != nil {
			t.Fatalf("read %s: %v", docPath, err)
		}
		docText := string(contents)
		if badPipe.MatchString(docText) {
			t.Fatalf("%s puts GLADE_* env before curl instead of before sh", docPath)
		}
		for _, want := range []string{
			"curl -fsSL https://glade.sh/install.sh | env GLADE_INSTALL_DIR=/usr/local/bin sh",
			"curl -fsSL https://glade.sh/install.sh | env GLADE_VERSION=vX.Y.Z sh",
		} {
			if !strings.Contains(docText, want) {
				t.Fatalf("%s missing %q", docPath, want)
			}
		}
	}
}

func TestInstallDocsDocumentGuardedUpdate(t *testing.T) {
	for _, docPath := range []string{
		filepath.Join("..", "docs", "INSTALL.md"),
		filepath.Join("..", "site", "docs-src", "guide", "installation.md"),
	} {
		contents, err := os.ReadFile(docPath)
		if err != nil {
			t.Fatalf("read %s: %v", docPath, err)
		}
		docText := string(contents)
		for _, want := range []string{
			"glade update --dry-run",
			"GLADE_UPDATE_ALLOW_SHELL=1 glade update",
			"updates the",
			"directory that contains the current `glade` binary",
		} {
			if !strings.Contains(docText, want) {
				t.Fatalf("%s missing %q", docPath, want)
			}
		}
	}
}

func runResolveSiblingRef(t *testing.T, remote, requested, fallback string) string {
	t.Helper()
	outputPath := filepath.Join(t.TempDir(), "github-output")
	cmd := exec.Command("bash", "resolve-sibling-ref.sh", remote, requested, fallback)
	cmd.Dir = "."
	cmd.Env = append(os.Environ(), "GITHUB_OUTPUT="+outputPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("resolve-sibling-ref.sh failed: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	outputBytes, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read GITHUB_OUTPUT: %v", err)
	}
	if want := "ref=" + got; !strings.Contains(string(outputBytes), want) {
		t.Fatalf("GITHUB_OUTPUT missing %q in %q", want, outputBytes)
	}
	return got
}

func makeGitRemote(t *testing.T, tag string) string {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatalf("mkdir work repo: %v", err)
	}
	runGit(t, work, "init")
	runGit(t, work, "config", "user.email", "glade-test@example.com")
	runGit(t, work, "config", "user.name", "Glade Test")
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, work, "add", "README.md")
	runGit(t, work, "commit", "-m", "initial")
	runGit(t, work, "branch", "-M", "main")
	if tag != "" {
		runGit(t, work, "tag", tag)
	}
	remote := filepath.Join(root, "remote.git")
	runCommand(t, "", "git", "clone", "--bare", work, remote)
	return remote
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	runCommand(t, dir, "git", args...)
}

func runCommand(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}
