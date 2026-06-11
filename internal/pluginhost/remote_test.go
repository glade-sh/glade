package pluginhost

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallRemoteArchiveRequiresAndVerifiesSHA256(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive executable mode is unix-specific")
	}
	root := t.TempDir()
	body := makePluginArchive(t, root, "quality", "1.2.0")
	sum := sha256.Sum256(body)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer server.Close()
	store := NewStore(filepath.Join(root, "home"))

	if _, err := store.InstallRemoteArchive(context.Background(), server.URL+"/quality.tar.gz", "", InstallOptions{}); err == nil {
		t.Fatal("expected missing sha256 to fail")
	}
	plugin, err := store.InstallRemoteArchive(context.Background(), server.URL+"/quality.tar.gz", fmt.Sprintf("%x", sum), InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if plugin.Name != "quality" || !strings.HasPrefix(plugin.Source, "url:") || plugin.Trust != "unlisted" {
		t.Fatalf("unexpected plugin: %#v", plugin)
	}
}
