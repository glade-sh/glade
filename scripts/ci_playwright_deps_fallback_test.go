package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type playwrightDepsFixture struct {
	root  string
	bin   string
	calls string
}

// newPlaywrightDepsFixture builds a fake playwright that records its
// arguments and fails every install that carries --with-deps. FAIL_PLAIN
// fails the install without it, FAIL_PROBE fails the screenshot probe.
func newPlaywrightDepsFixture(t *testing.T) *playwrightDepsFixture {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	f := &playwrightDepsFixture{root: root, bin: bin, calls: filepath.Join(root, "calls")}
	writeApexFixtureExecutable(t, filepath.Join(bin, "playwright"), `#!/bin/bash
printf '%s\n' "$*" >>"$FIXTURE_ROOT/calls"
case "$1" in
install)
	if [[ " $* " == *" --with-deps "* ]]; then exit "${DEPS_STATUS:-17}"; fi
	exit "${PLAIN_STATUS:-0}"
	;;
screenshot) exit "${PROBE_STATUS:-0}" ;;
esac
`)
	writeApexFixtureExecutable(t, filepath.Join(bin, "sleep"), "#!/bin/sh\nexit 0\n")
	return f
}

// withTimeout adds a fake timeout that records its leading arguments, then
// either reports a timeout (TIMEOUT_STATUS) or runs the command.
func (f *playwrightDepsFixture) withTimeout(t *testing.T) {
	t.Helper()
	writeApexFixtureExecutable(t, filepath.Join(f.bin, "timeout"), `#!/bin/bash
printf '%s\n' "$1 $2" >>"$FIXTURE_ROOT/timeouts"
if [[ -n "${TIMEOUT_STATUS:-}" ]]; then exit "$TIMEOUT_STATUS"; fi
shift 2
exec "$@"
`)
}

func (f *playwrightDepsFixture) run(t *testing.T, path string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"ci-install-playwright.sh"}, args...)...)
	cmd.Env = append(os.Environ(), "PATH="+path, "FIXTURE_ROOT="+f.root, "PLAYWRIGHT_DOWNLOAD_HOST=")
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	exit, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatal(err)
	}
	return string(out), exit.ExitCode()
}

func (f *playwrightDepsFixture) read(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.root, name))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

func (f *playwrightDepsFixture) path() string { return f.bin + ":" + os.Getenv("PATH") }

// pathWithoutTimeout holds symlinks to only the tools the helper needs, so
// no timeout program is visible whatever the host provides.
func (f *playwrightDepsFixture) pathWithoutTimeout(t *testing.T) string {
	t.Helper()
	tools := filepath.Join(f.root, "tools")
	if err := os.MkdirAll(tools, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bash", "mktemp", "rm", "dirname", "cat"} {
		target, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s not found", name)
		}
		if err := os.Symlink(target, filepath.Join(tools, name)); err != nil {
			t.Fatal(err)
		}
	}
	return f.bin + ":" + tools
}

var probeCall = regexp.MustCompile(`^screenshot --browser chromium about:blank \S.*/probe\.png$`)

func splitCalls(s string) []string {
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func TestCIPlaywrightDepsFallbackLaunchesWithImageLibraries(t *testing.T) {
	f := newPlaywrightDepsFixture(t)
	out, code := f.run(t, f.pathWithoutTimeout(t), nil, filepath.Join(f.bin, "playwright"), "install", "--with-deps", "chromium")
	if code != 0 {
		t.Fatalf("fallback with a working probe must succeed: %d\n%s", code, out)
	}
	if !strings.Contains(out, "::warning::System packages could not be installed") {
		t.Fatalf("missing fallback warning: %s", out)
	}
	calls := splitCalls(f.read(t, "calls"))
	if len(calls) != 5 {
		t.Fatalf("calls = %q", calls)
	}
	for i := 0; i < 3; i++ {
		if calls[i] != "install --with-deps chromium" {
			t.Fatalf("call %d = %q", i, calls[i])
		}
	}
	if calls[3] != "install chromium" || !probeCall.MatchString(calls[4]) {
		t.Fatalf("fallback calls = %q", calls[3:])
	}
	probePath := strings.TrimPrefix(calls[4], "screenshot --browser chromium about:blank ")
	if _, err := os.Stat(filepath.Dir(probePath)); !os.IsNotExist(err) {
		t.Fatalf("probe directory not removed: %v", err)
	}
}

func TestCIPlaywrightDepsTimeoutEndsDepsAttemptsAndFallsBack(t *testing.T) {
	f := newPlaywrightDepsFixture(t)
	f.withTimeout(t)
	out, code := f.run(t, f.path(), []string{"TIMEOUT_STATUS=124"}, filepath.Join(f.bin, "playwright"), "install", "--with-deps", "chromium")
	if code != 0 {
		t.Fatalf("timeout then working fallback must succeed: %d\n%s", code, out)
	}
	if !strings.Contains(out, "::warning::") || !strings.Contains(out, "timed out") {
		t.Fatalf("missing timeout warning: %s", out)
	}
	if got := len(splitCalls(f.read(t, "timeouts"))); got != 1 {
		t.Fatalf("timeout invoked %d times, want 1 (no second --with-deps attempt)", got)
	}
	calls := splitCalls(f.read(t, "calls"))
	if len(calls) != 2 || calls[0] != "install chromium" || !probeCall.MatchString(calls[1]) {
		t.Fatalf("calls after timeout = %q", calls)
	}
}

func TestCIPlaywrightDepsTimeoutKilledStatusEndsAttempts(t *testing.T) {
	f := newPlaywrightDepsFixture(t)
	f.withTimeout(t)
	out, code := f.run(t, f.path(), []string{"TIMEOUT_STATUS=137", "PLAIN_STATUS=3"}, filepath.Join(f.bin, "playwright"), "install", "--with-deps", "chromium")
	if code != 137 {
		t.Fatalf("failed fallback must keep the original status 137: %d\n%s", code, out)
	}
	if got := len(splitCalls(f.read(t, "timeouts"))); got != 1 {
		t.Fatalf("timeout invoked %d times, want 1", got)
	}
}

func TestCIPlaywrightDepsTimeoutArguments(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  []string
		want string
	}{
		{"default", nil, "--kill-after=15 240"},
		{"override", []string{"CI_PLAYWRIGHT_DEPS_TIMEOUT=45"}, "--kill-after=15 45"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlaywrightDepsFixture(t)
			f.withTimeout(t)
			f.run(t, f.path(), tc.env, filepath.Join(f.bin, "playwright"), "install", "--with-deps", "chromium")
			timeouts := splitCalls(f.read(t, "timeouts"))
			if len(timeouts) != 3 {
				t.Fatalf("each --with-deps attempt must be bounded: %q", timeouts)
			}
			for _, got := range timeouts {
				if got != tc.want {
					t.Fatalf("timeout arguments = %q, want %q", got, tc.want)
				}
			}
		})
	}
}

func TestCIPlaywrightDepsProbeFailureKeepsOriginalStatus(t *testing.T) {
	f := newPlaywrightDepsFixture(t)
	out, code := f.run(t, f.pathWithoutTimeout(t), []string{"PROBE_STATUS=9"}, filepath.Join(f.bin, "playwright"), "install", "--with-deps", "chromium")
	if code != 17 {
		t.Fatalf("probe failure must keep status 17: %d\n%s", code, out)
	}
	if strings.Contains(out, "preinstalled libraries") {
		t.Fatalf("probe failure must not claim success: %s", out)
	}
	if len(splitCalls(f.read(t, "calls"))) != 5 {
		t.Fatalf("calls = %q", f.read(t, "calls"))
	}
}

func TestCIPlaywrightDepsFallbackInstallFailureKeepsOriginalStatus(t *testing.T) {
	f := newPlaywrightDepsFixture(t)
	out, code := f.run(t, f.pathWithoutTimeout(t), []string{"PLAIN_STATUS=5"}, filepath.Join(f.bin, "playwright"), "install", "--with-deps", "chromium")
	if code != 17 {
		t.Fatalf("fallback install failure must keep status 17: %d\n%s", code, out)
	}
	calls := splitCalls(f.read(t, "calls"))
	if len(calls) != 6 {
		t.Fatalf("3 deps + 3 plain attempts expected, got %q", calls)
	}
	for _, c := range calls {
		if strings.HasPrefix(c, "screenshot") {
			t.Fatalf("probe must not run after a failed install: %q", calls)
		}
	}
}

func TestCIPlaywrightDepsFallbackCanBeDisabled(t *testing.T) {
	f := newPlaywrightDepsFixture(t)
	f.withTimeout(t)
	out, code := f.run(t, f.path(), []string{"CI_PLAYWRIGHT_DEPS_FALLBACK=0"}, filepath.Join(f.bin, "playwright"), "install", "--with-deps", "chromium")
	if code != 17 {
		t.Fatalf("disabled fallback must keep status 17: %d\n%s", code, out)
	}
	calls := splitCalls(f.read(t, "calls"))
	if len(calls) != 3 {
		t.Fatalf("calls = %q, want 3 --with-deps attempts only", calls)
	}
	for _, c := range calls {
		if c != "install --with-deps chromium" {
			t.Fatalf("unexpected call %q", c)
		}
	}
}

func TestCIPlaywrightWithoutDepsIsUnchanged(t *testing.T) {
	f := newPlaywrightDepsFixture(t)
	f.withTimeout(t)
	out, code := f.run(t, f.path(), []string{"PLAIN_STATUS=17"}, filepath.Join(f.bin, "playwright"), "install", "chromium")
	if code != 17 {
		t.Fatalf("plain failure must keep status 17: %d\n%s", code, out)
	}
	calls := splitCalls(f.read(t, "calls"))
	if len(calls) != 3 {
		t.Fatalf("calls = %q, want exactly 3", calls)
	}
	for _, c := range calls {
		if c != "install chromium" {
			t.Fatalf("unexpected call %q", c)
		}
	}
	if f.read(t, "timeouts") != "" {
		t.Fatalf("timeout must not run without --with-deps: %q", f.read(t, "timeouts"))
	}
}

func TestCIPlaywrightDepsFallbackCommandPrefixes(t *testing.T) {
	t.Run("space in path", func(t *testing.T) {
		f := newPlaywrightDepsFixture(t)
		spaced := filepath.Join(f.root, "with space")
		if err := os.MkdirAll(spaced, 0o700); err != nil {
			t.Fatal(err)
		}
		writeApexFixtureExecutable(t, filepath.Join(spaced, "playwright"), `#!/bin/bash
printf '%s|' "$0" >>"$FIXTURE_ROOT/vectors"
printf '<%s>' "$@" >>"$FIXTURE_ROOT/vectors"
printf '\n' >>"$FIXTURE_ROOT/vectors"
if [[ " $* " == *" --with-deps "* ]]; then exit 17; fi
`)
		bin := filepath.Join(spaced, "playwright")
		out, code := f.run(t, f.pathWithoutTimeout(t), nil, bin, "install", "--with-deps", "chromium")
		if code != 0 {
			t.Fatalf("spaced prefix must fall back: %d\n%s", code, out)
		}
		lines := splitCalls(f.read(t, "vectors"))
		if len(lines) != 5 {
			t.Fatalf("vectors = %q", lines)
		}
		if lines[3] != bin+"|<install><chromium>" {
			t.Fatalf("fallback vector = %q", lines[3])
		}
		if !regexp.MustCompile(`^` + regexp.QuoteMeta(bin) + `\|<screenshot><--browser><chromium><about:blank><[^>]+/probe\.png>$`).MatchString(lines[4]) {
			t.Fatalf("probe vector = %q", lines[4])
		}
	})
	t.Run("npx prefix", func(t *testing.T) {
		f := newPlaywrightDepsFixture(t)
		writeApexFixtureExecutable(t, filepath.Join(f.bin, "npx"), `#!/bin/bash
printf '%s\n' "$*" >>"$FIXTURE_ROOT/calls"
if [[ " $* " == *" --with-deps "* ]]; then exit 17; fi
`)
		out, code := f.run(t, f.pathWithoutTimeout(t), nil, "npx", "playwright", "install", "--with-deps", "chromium")
		if code != 0 {
			t.Fatalf("npx prefix must fall back: %d\n%s", code, out)
		}
		calls := splitCalls(f.read(t, "calls"))
		if len(calls) != 5 || calls[3] != "playwright install chromium" ||
			!regexp.MustCompile(`^playwright screenshot --browser chromium about:blank \S+/probe\.png$`).MatchString(calls[4]) {
			t.Fatalf("npx calls = %q", calls)
		}
	})
}

func TestCIWorkflowPRBrowserStepsSkipApexShards(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string
				If   string `yaml:"if"`
				Run  string
			}
		}
	}
	if err := yaml.Unmarshal([]byte(readWorkflowFile(t, "ci.yml")), &workflow); err != nil {
		t.Fatal(err)
	}
	steps := workflow.Jobs["pr-tests"].Steps
	if len(steps) == 0 {
		t.Fatal("pr-tests job not found")
	}
	want := "${{ steps.select.outputs.browser == 'true' && matrix.shard >= 12 }}"
	found := map[string]bool{}
	for _, step := range steps {
		switch step.Name {
		case "Install selected LWC test dependencies", "Install selected browser", "Enable selected browser tests":
			found[step.Name] = true
			if step.If != want {
				t.Errorf("%s if = %q, want %q", step.Name, step.If, want)
			}
		}
	}
	if len(found) != 3 {
		t.Fatalf("browser steps found = %v", found)
	}
	// The Apex route must keep occupying shards 0-11 so the guard above
	// stays correct.
	last := steps[len(steps)-1].Run
	m := regexp.MustCompile(`\bapex:apextest:(\d+):(\d+)\b`).FindStringSubmatch(last)
	if m == nil {
		t.Fatalf("apex route spec missing from the last step: %s", last)
	}
	base, _ := strconv.Atoi(m[1])
	count, _ := strconv.Atoi(m[2])
	if base != 0 || base+count != 12 {
		t.Fatalf("apex shards = %d..%d, want 0..11", base, base+count-1)
	}
}
