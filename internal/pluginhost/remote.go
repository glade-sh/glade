package pluginhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s Store) InstallRemoteArchive(ctx context.Context, archiveURL, wantSHA256 string, opts InstallOptions) (InstalledPlugin, error) {
	want := strings.ToLower(strings.TrimSpace(wantSHA256))
	if want == "" {
		return InstalledPlugin{}, fmt.Errorf("remote plugin archive installs require --sha256")
	}
	if len(want) != sha256.Size*2 {
		return InstalledPlugin{}, fmt.Errorf("remote plugin archive sha256 must be %d hex characters", sha256.Size*2)
	}
	if _, err := hex.DecodeString(want); err != nil {
		return InstalledPlugin{}, fmt.Errorf("remote plugin archive sha256 must be hex")
	}
	downloadDir := filepath.Join(s.root, "plugins", "downloads")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return InstalledPlugin{}, err
	}
	archivePath := filepath.Join(downloadDir, "remote-"+want[:16]+".tar.gz")
	if err := downloadURLWithSHA256(ctx, archiveURL, want, archivePath); err != nil {
		return InstalledPlugin{}, err
	}
	if opts.Source == "" {
		opts.Source = "url:" + archiveURL
	}
	if opts.Trust == "" {
		opts.Trust = "unlisted"
	}
	opts.AssetSHA256 = want
	return s.InstallArchiveWithOptions(ctx, archivePath, opts)
}

func downloadURLWithSHA256(ctx context.Context, url, want, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download plugin archive: %s", resp.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), resp.Body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if got != want {
		return fmt.Errorf("remote plugin archive checksum mismatch for %s", url)
	}
	return os.Rename(tmpName, path)
}
