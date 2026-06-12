package compile

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/project"
)

type Options struct {
	OutDir    string
	Namespace string
	RepoRoot  string
}

type ModuleEntry struct {
	ModuleKey string `json:"moduleKey"`
	Tag       string `json:"tag"`
	File      string `json:"file"`
}

type Manifest struct {
	Modules map[string]ModuleEntry `json:"modules"`
	OutDir  string
}

type compileConfig struct {
	ProjectRoot  string   `json:"projectRoot"`
	OutDir       string   `json:"outDir"`
	Namespace    string   `json:"namespace"`
	LWCFiles     []string `json:"lwcFiles"`
	LWCHTMLFiles []string `json:"lwcHtmlFiles"`
}

type compileResult struct {
	Modules      map[string]ModuleEntry `json:"modules"`
	ManifestPath string                 `json:"manifestPath"`
}

func Compile(p project.Project, opts Options) (Manifest, error) {
	if strings.TrimSpace(opts.OutDir) == "" {
		return Manifest{}, fmt.Errorf("compile: OutDir is required")
	}
	repoRoot := strings.TrimSpace(opts.RepoRoot)
	if repoRoot == "" {
		var err error
		repoRoot, err = compileRepoRoot()
		if err != nil {
			return Manifest{}, err
		}
	}
	namespace := strings.TrimSpace(opts.Namespace)
	if namespace == "" {
		namespace = strings.TrimSpace(p.Namespace)
	}
	if namespace == "" {
		namespace = "c"
	}
	outDir, err := filepath.Abs(opts.OutDir)
	if err != nil {
		return Manifest{}, err
	}
	projectRoot, err := filepath.Abs(p.Root)
	if err != nil {
		return Manifest{}, err
	}
	cfg := compileConfig{
		ProjectRoot:  projectRoot,
		OutDir:       outDir,
		Namespace:    namespace,
		LWCFiles:     relativize(projectRoot, p.LWCFiles),
		LWCHTMLFiles: relativize(projectRoot, p.LWCHTMLFiles),
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		return Manifest{}, err
	}
	script := filepath.Join(repoRoot, "third_party", "lwc", "compile.mjs")
	cmd := exec.Command("node", script)
	cmd.Dir = filepath.Join(repoRoot, "third_party", "lwc")
	cmd.Stdin = strings.NewReader(string(payload))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Manifest{}, fmt.Errorf("lwc compile: %w\n%s", err, string(out))
	}
	var result compileResult
	if err := json.Unmarshal(out, &result); err != nil {
		return Manifest{}, fmt.Errorf("decode compile result: %w\n%s", err, string(out))
	}
	if result.Modules == nil {
		result.Modules = map[string]ModuleEntry{}
	}
	return Manifest{Modules: result.Modules, OutDir: outDir}, nil
}

// FindRepoRoot returns the glade source checkout (for testdata and development files).
func FindRepoRoot() (string, error) {
	return gladehome.RepoRoot()
}

func compileRepoRoot() (string, error) {
	if root, err := gladehome.RepoRoot(); err == nil {
		if _, err := os.Stat(filepath.Join(root, "third_party", "lwc", "compile.mjs")); err == nil {
			return root, nil
		}
	}
	return gladehome.EnsureRoot()
}

func relativize(root string, paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		out = append(out, filepath.ToSlash(rel))
	}
	return out
}
