package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The newest completed predecessor run may be unusable (failed before every
// shard uploaded, expired artifacts, or invalid evidence). An older completed
// run for the same SHA with a complete set must still seed the history.
func TestCICase319UnusableNewestPredecessorFallsBackToOlderRun(t *testing.T) {
	for _, newest := range []string{"download-fails", "invalid-evidence"} {
		t.Run(newest, func(t *testing.T) {
			f := newCase319BehaviorFixture(t)
			bad := filepath.Join(f.root, "bad-evidence")
			if err := os.MkdirAll(filepath.Join(bad, "apex-shard-0"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bad, "apex-shard-0", "junk.txt"), []byte("not evidence"), 0600); err != nil {
				t.Fatal(err)
			}
			gh := `#!/bin/bash
set -eu
if [ "$1" = api ]; then
  case "$*" in
    *'--jq'*'[0:5][]'*) printf '901\n902\n';;
    *'--jq'*) printf '901\n';;
    *) exit 2;;
  esac
  exit 0
fi
[ "$1" = run ] && [ "$2" = download ] || exit 2
id=$3
printf '%s\n' "$id" >> "$FAKE_DOWNLOAD_LOG"
shift 3
dest=""
while [ "$#" -gt 0 ]; do case "$1" in --pattern) shift 2;; --dir) dest=$2; shift 2;; *) exit 2;; esac; done
mkdir -p "$dest"
if [ "$id" = 901 ]; then
  [ "$FAKE_NEWEST" = download-fails ] && exit 1
  cp -R "$FAKE_BAD"/apex-shard-* "$dest/"
  exit 0
fi
[ "$id" = 902 ] || exit 2
cp -R "$FAKE_EVIDENCE"/apex-shard-* "$dest/"
`
			if err := os.WriteFile(filepath.Join(f.bin, "gh"), []byte(gh), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("FAKE_NEWEST", newest)
			t.Setenv("FAKE_BAD", bad)
			downloadLog := filepath.Join(f.root, "downloads")
			t.Setenv("FAKE_DOWNLOAD_LOG", downloadLog)
			weights := runCase319BehaviorRoute(t, f, "1")
			attempts, err := os.ReadFile(downloadLog)
			if err != nil || !strings.HasPrefix(string(attempts), "901\n") {
				t.Fatalf("newest predecessor was not attempted: %q (%v)", attempts, err)
			}
			if got := weights["TestBootstrap11"]; got != 12000 {
				t.Fatalf("older usable predecessor was masked: staged duration %d ms, want 12000 ms", got)
			}
			if got := string(attempts); got != "901\n902\n" {
				t.Fatalf("download attempts = %q, want newest then older", got)
			}
		})
	}
}

// Falling back to deterministic scheduling still works when every candidate
// is unusable.
func TestCICase319AllUnusablePredecessorsFallBackToDeterministic(t *testing.T) {
	f := newCase319BehaviorFixture(t)
	gh := "#!/bin/bash\nif [ \"$1\" = api ]; then printf '901\\n902\\n'; exit 0; fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(f.bin, "gh"), []byte(gh), 0700); err != nil {
		t.Fatal(err)
	}
	if weights := runCase319BehaviorRoute(t, f, "1"); len(weights) != 0 {
		t.Fatalf("expected no staged history, got %v", weights)
	}
}
