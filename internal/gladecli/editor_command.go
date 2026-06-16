package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/gladehome"
)

var editorCommandLookPath = exec.LookPath

var editorCommandRun = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func runEditor(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) < 2 {
		return errors.New("usage: glade editor <install|doctor> vscode [--vsix <path>] [--editor <code|cursor|windsurf>] [--force] [--json]")
	}
	command := args[0]
	target := args[1]
	if target != "vscode" {
		return fmt.Errorf("unsupported editor target %q", target)
	}
	switch command {
	case "install":
		return runEditorInstall(ctx, args[2:], w)
	case "doctor":
		return runEditorDoctor(args[2:], w)
	default:
		return fmt.Errorf("unknown editor command %q", command)
	}
}

func runEditorInstall(ctx context.Context, args []string, w io.Writer) error {
	editor := "code"
	vsix := ""
	force := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--editor":
			if i+1 >= len(args) {
				return errors.New("--editor requires a value")
			}
			editor = args[i+1]
			i++
		case "--vsix":
			if i+1 >= len(args) {
				return errors.New("--vsix requires a value")
			}
			vsix = args[i+1]
			i++
		case "--force":
			force = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if vsix == "" {
		resolved, err := resolveBundledVSIX()
		if err != nil {
			return fmt.Errorf("--vsix is required when no bundled VS Code extension is available: %w", err)
		}
		vsix = resolved
	} else {
		resolved, err := existingVSIX(vsix)
		if err != nil {
			return err
		}
		vsix = resolved
	}
	editorPath, err := resolveEditorCommand(editor)
	if err != nil {
		return err
	}
	installArgs := []string{"--install-extension", vsix}
	if force {
		installArgs = append(installArgs, "--force")
	}
	out, err := editorCommandRun(ctx, editorPath, installArgs...)
	if err != nil {
		if len(out) > 0 {
			fmt.Fprint(w, string(out))
		}
		return fmt.Errorf("%s --install-extension failed: %w", editorPath, err)
	}
	fmt.Fprintf(w, "installed vscode extension: %s\n", vsix)
	return nil
}

func runEditorDoctor(args []string, w io.Writer) error {
	editor := "code"
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--editor":
			if i+1 >= len(args) {
				return errors.New("--editor requires a value")
			}
			editor = args[i+1]
			i++
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	report := editorDoctorReport{Target: "vscode"}
	report.Editor.Command = editorCommandName(editor)
	editorPath, err := resolveEditorCommand(editor)
	if err != nil {
		report.Editor.Error = err.Error()
	} else {
		report.Editor.Path = editorPath
		report.Editor.OK = true
	}
	report.Glade.Command = "glade"
	if gladePath, err := editorCommandLookPath("glade"); err != nil {
		report.Glade.Error = err.Error()
	} else {
		report.Glade.Path = gladePath
		report.Glade.OK = true
	}
	if vsix, err := resolveBundledVSIX(); err == nil {
		report.BundledVSIX.Path = vsix
		report.BundledVSIX.Exists = true
	}
	report.OK = report.Editor.OK && report.Glade.OK
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	if report.Editor.OK {
		fmt.Fprintf(w, "editor: %s (%s)\n", editorCommandName(editor), report.Editor.Path)
	} else {
		fmt.Fprintf(w, "editor: %s (missing: %s)\n", editor, report.Editor.Error)
	}
	if report.Glade.OK {
		fmt.Fprintf(w, "glade: %s\n", report.Glade.Path)
	} else {
		fmt.Fprintf(w, "glade: missing (%s)\n", report.Glade.Error)
	}
	return nil
}

type editorDoctorReport struct {
	Target      string              `json:"target"`
	Editor      editorDoctorCommand `json:"editor"`
	Glade       editorDoctorCommand `json:"glade"`
	BundledVSIX editorBundledVSIX   `json:"bundledVsix"`
	OK          bool                `json:"ok"`
}

type editorDoctorCommand struct {
	Command string `json:"command,omitempty"`
	Path    string `json:"path,omitempty"`
	Error   string `json:"error,omitempty"`
	OK      bool   `json:"ok"`
}

type editorBundledVSIX struct {
	Path   string `json:"path,omitempty"`
	Exists bool   `json:"exists"`
}

func resolveEditorCommand(editor string) (string, error) {
	name := editorCommandName(editor)
	if name == "" {
		return "", fmt.Errorf("unsupported editor %q", editor)
	}
	path, err := editorCommandLookPath(name)
	if err != nil {
		return "", fmt.Errorf("editor command %q not found on PATH: %w", name, err)
	}
	return path, nil
}

func editorCommandName(editor string) string {
	switch editor {
	case "", "code", "vscode":
		return "code"
	case "cursor":
		return "cursor"
	case "windsurf":
		return "windsurf"
	default:
		return ""
	}
}

func resolveBundledVSIX() (string, error) {
	if fromEnv := strings.TrimSpace(os.Getenv("GLADE_VSCODE_VSIX")); fromEnv != "" {
		return existingVSIX(fromEnv)
	}
	for _, candidate := range bundledVSIXCandidates() {
		if vsix, err := existingVSIX(candidate); err == nil {
			return vsix, nil
		}
	}
	return "", fmt.Errorf("vscode-glade.vsix not found; from a source checkout run `npm --prefix contrib/vscode-glade install && npm --prefix contrib/vscode-glade run package`, then retry")
}

func bundledVSIXCandidates() []string {
	candidates := []string{}
	if home := strings.TrimSpace(os.Getenv("GLADE_HOME")); home != "" {
		candidates = append(candidates, filepath.Join(home, "editor", "vscode-glade.vsix"))
	}
	candidates = append(candidates, filepath.Join(gladehome.UserShareDir(), "editor", "vscode-glade.vsix"))
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "share", "glade", "editor", "vscode-glade.vsix"),
			filepath.Join(exeDir, "..", "share", "glade", "editor", "vscode-glade.vsix"),
		)
	}
	candidates = append(candidates, sourceCheckoutVSIXCandidates()...)
	return candidates
}

func sourceCheckoutVSIXCandidates() []string {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	var candidates []string
	seen := make(map[string]bool)
	for _, root := range editorSourceCheckoutRoots(cwd) {
		if seen[root] {
			continue
		}
		seen[root] = true
		pattern := filepath.Join(root, "contrib", "vscode-glade", "dist", "vscode-glade-*.vsix")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		sort.Sort(sort.Reverse(sort.StringSlice(matches)))
		candidates = append(candidates, matches...)
	}
	return candidates
}

func editorSourceCheckoutRoots(start string) []string {
	var roots []string
	dir := filepath.Clean(start)
	for {
		if isEditorSourceCheckout(dir) {
			roots = append(roots, dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return roots
}

func isEditorSourceCheckout(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(root, "contrib", "vscode-glade", "package.json")); err != nil {
		return false
	}
	return true
}

func existingVSIX(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("vsix %q: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("vsix %q is a directory", path)
	}
	return path, nil
}
