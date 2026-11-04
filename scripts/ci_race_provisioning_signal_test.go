package scripts

import (
	"os/exec"
	"testing"
)

// Model a runner that times out the whole process group while a download is
// active. This verifies helper behavior under that signal, not GitHub's runner.
func TestCIRaceProvisioningCancellationStopsDownloadAndRetry(t *testing.T) {
	const program = `
import os, pathlib, signal, subprocess, sys, time
root = pathlib.Path(sys.argv[2])
marker = root / 'attempts'
pidfile = root / 'child'
env = os.environ.copy()
env.update(ATTEMPTS=str(marker), CHILD=str(pidfile))
child = 'echo attempt >> "$ATTEMPTS"; echo $$ > "$CHILD"; exec sleep 300'
p = subprocess.Popen(['bash', sys.argv[1], 'bash', '-c', child], env=env, start_new_session=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
try:
    for _ in range(200):
        if pidfile.exists(): break
        if p.poll() is not None: raise RuntimeError('helper exited before download started')
        time.sleep(.025)
    else: raise RuntimeError('download did not start')
    child_pid = int(pidfile.read_text())
    os.killpg(p.pid, signal.SIGTERM)
    if p.wait(timeout=5) == 0: raise RuntimeError('cancelled helper returned success')
    for _ in range(200):
        state = subprocess.run(['ps', '-o', 'stat=', '-p', str(child_pid)], capture_output=True, text=True).stdout.strip()
        if not state or state.startswith('Z'): break
        time.sleep(.025)
    else: raise RuntimeError('download child survived group cancellation')
    if marker.read_text().splitlines() != ['attempt']: raise RuntimeError('cancelled download was retried')
finally:
    try: os.killpg(p.pid, signal.SIGKILL)
    except ProcessLookupError: pass
    p.wait()
`
	cmd := exec.Command("python3", "-c", program, "ci-install-playwright.sh", t.TempDir())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cancelling provisioning must stop its child without retrying: %v\n%s", err, out)
	}
}
