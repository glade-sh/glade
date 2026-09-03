package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseAssetUploadRespectsReleaseState(t *testing.T) {
	for _, test := range []struct {
		name, draft, candidate, published, want string
		wantErr, wantDownload, wantUpload       bool
	}{
		{name: "identical published asset skips", draft: "false", candidate: "same bytes", published: "same bytes", want: "already has identical bytes; skipping", wantDownload: true},
		{name: "different published asset fails", draft: "false", candidate: "candidate bytes", published: "published bytes", want: "published asset differs", wantErr: true, wantDownload: true},
		{name: "missing draft asset uploads", draft: "true", candidate: "new bytes", wantUpload: true},
		{name: "missing published asset fails", draft: "false", candidate: "new bytes", want: "published release cannot accept missing asset", wantErr: true},
		{name: "unexpected draft state fails", draft: "null", candidate: "new bytes", want: "unexpected release draft state: null", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			asset := filepath.Join(root, "plugin.tar.gz")
			if err := os.WriteFile(asset, []byte(test.candidate), 0o644); err != nil {
				t.Fatal(err)
			}
			existing := filepath.Join(root, "existing")
			assets := ""
			if test.published != "" {
				if err := os.Mkdir(existing, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(existing, "plugin.tar.gz"), []byte(test.published), 0o644); err != nil {
					t.Fatal(err)
				}
				assets = "plugin.tar.gz"
			}
			log := filepath.Join(root, "gh.log")
			command := releaseAssetUploadCommand(t, root, asset, existing, assets, test.draft, log)
			output, err := command.CombinedOutput()
			if (err != nil) != test.wantErr {
				t.Fatalf("release asset upload err=%v, wantErr=%t\n%s", err, test.wantErr, output)
			}
			if test.want != "" && !strings.Contains(string(output), test.want) {
				t.Fatalf("release asset upload output %q missing %q", output, test.want)
			}
			contents, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(contents), "release view v9.9.9 --json isDraft --jq .isDraft") {
				t.Fatalf("release state was not queried:\n%s", contents)
			}
			if got := strings.Contains(string(contents), "release download"); got != test.wantDownload {
				t.Fatalf("release download = %t, want %t:\n%s", got, test.wantDownload, contents)
			}
			if got := strings.Contains(string(contents), "release upload"); got != test.wantUpload {
				t.Fatalf("release upload = %t, want %t:\n%s", got, test.wantUpload, contents)
			}
		})
	}
}

func releaseAssetUploadCommand(t *testing.T, root, asset, existing, assets, draft, log string) *exec.Cmd {
	t.Helper()
	fakeGH := filepath.Join(root, "gh")
	fake := `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$MOCK_GH_LOG"
case "$1 $2" in
  "release view")
    if [[ "$*" == *"isDraft"* ]]; then
      printf '%s\n' "$MOCK_GH_DRAFT"
    else
      printf '%s\n' "$MOCK_GH_ASSETS"
    fi
    ;;
  "release download")
    pattern=""
    destination=""
    shift 2
    while (($#)); do
      case "$1" in
        --pattern) pattern="$2"; shift 2 ;;
        --dir) destination="$2"; shift 2 ;;
        *) shift ;;
      esac
    done
    mkdir -p "$destination"
    cp "$MOCK_GH_EXISTING/$pattern" "$destination/$pattern"
    ;;
  "release upload") ;;
  *) echo "unexpected gh command: $*" >&2; exit 2 ;;
esac
`
	if err := os.WriteFile(fakeGH, []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", "release-asset-upload.sh", "v9.9.9", asset)
	command.Dir = "."
	command.Env = append(os.Environ(),
		"GH_BIN="+fakeGH,
		"MOCK_GH_ASSETS="+assets,
		"MOCK_GH_DRAFT="+draft,
		"MOCK_GH_EXISTING="+existing,
		"MOCK_GH_LOG="+log,
	)
	return command
}
