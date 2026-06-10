package gladecli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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
		return errors.New("usage: glade editor <install|doctor> vscode [--vsix <path>] [--editor <code|cursor|windsurf>] [--force]")
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
		return errors.New("--vsix is required")
	}
	if info, err := os.Stat(vsix); err != nil {
		return fmt.Errorf("vsix %q: %w", vsix, err)
	} else if info.IsDir() {
		return fmt.Errorf("vsix %q is a directory", vsix)
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
	if len(out) > 0 {
		fmt.Fprint(w, string(out))
	}
	if err != nil {
		return fmt.Errorf("%s --install-extension failed: %w", editorPath, err)
	}
	fmt.Fprintf(w, "installed vscode extension: %s\n", vsix)
	return nil
}

func runEditorDoctor(args []string, w io.Writer) error {
	editor := "code"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--editor":
			if i+1 >= len(args) {
				return errors.New("--editor requires a value")
			}
			editor = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	editorPath, err := resolveEditorCommand(editor)
	if err != nil {
		fmt.Fprintf(w, "editor: %s (missing: %v)\n", editor, err)
	} else {
		fmt.Fprintf(w, "editor: %s (%s)\n", editorCommandName(editor), editorPath)
	}
	if gladePath, err := editorCommandLookPath("glade"); err != nil {
		fmt.Fprintf(w, "glade: missing (%v)\n", err)
	} else {
		fmt.Fprintf(w, "glade: %s\n", gladePath)
	}
	return nil
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
