package pluginhost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func LoadManifestFromExecutable(ctx context.Context, executable string) (Manifest, error) {
	// Linux can briefly reject exec while another process holds the executable
	// open for writing, including a descriptor inherited during a concurrent fork.
	const attempts = 5
	for attempt := 0; ; attempt++ {
		manifest, err := loadManifestFromExecutableOnce(ctx, executable)
		if !errors.Is(err, syscall.ETXTBSY) || attempt == attempts-1 {
			return manifest, err
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Manifest{}, fmt.Errorf("read plugin manifest: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func loadManifestFromExecutableOnce(ctx context.Context, executable string) (Manifest, error) {
	cmd := exec.CommandContext(ctx, executable, "manifest", "--json")
	configureManifestCommand(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Manifest{}, fmt.Errorf("read plugin manifest: %w: %s", ctxErr, stderr.String())
		}
		if errors.Is(err, exec.ErrWaitDelay) {
			if cancelErr := cmd.Cancel(); cancelErr != nil && !errors.Is(cancelErr, os.ErrProcessDone) {
				return Manifest{}, fmt.Errorf("read plugin manifest: clean up delayed plugin process: %w", cancelErr)
			}
			if manifest, decodeErr := decodeManifest(stdout.Bytes()); decodeErr == nil {
				return manifest, nil
			}
		}
		return Manifest{}, fmt.Errorf("read plugin manifest: %w: %s", err, stderr.String())
	}
	return decodeManifest(stdout.Bytes())
}

func LoadManifestFromFile(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	return decodeManifest(data)
}

func decodeManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
