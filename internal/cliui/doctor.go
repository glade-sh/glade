package cliui

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type DoctorInfo struct {
	SchemaVersion    string   `json:"schemaVersion,omitempty"`
	Command          string   `json:"command,omitempty"`
	Status           string   `json:"status,omitempty"`
	ExitCode         int      `json:"exitCode"`
	Version          string   `json:"version"`
	GoVersion        string   `json:"goVersion"`
	OSArch           string   `json:"osArch"`
	CWD              string   `json:"cwd"`
	ConfigPath       string   `json:"configPath,omitempty"`
	ConfigMissing    bool     `json:"configMissing"`
	ProjectRoot      string   `json:"projectRoot,omitempty"`
	DefaultNamespace string   `json:"defaultNamespace,omitempty"`
	ParserStatus     string   `json:"parserStatus"`
	ParserOK         bool     `json:"parserOK"`
	ToolchainPath    string   `json:"toolchainPath,omitempty"`
	ToolchainStatus  string   `json:"toolchainStatus"`
	ToolchainOK      bool     `json:"toolchainOK"`
	Suggestions      []string `json:"suggestions,omitempty"`
}

func WriteDoctor(w io.Writer, info DoctorInfo) error {
	t := NewTheme(w)
	if _, err := fmt.Fprintln(w, "Glade doctor"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	rows := []struct {
		ok    bool
		label string
		value string
	}{
		{true, "Project", doctorProjectValue(info)},
		{info.ParserOK, "Parser", info.ParserStatus},
		{info.ToolchainOK, "Toolchain", toolchainDoctorValue(info)},
	}
	if info.ConfigMissing {
		rows = append(rows, struct {
			ok    bool
			label string
			value string
		}{false, "Config", "no glade.yml found"})
	} else {
		rows = append(rows, struct {
			ok    bool
			label string
			value string
		}{true, "Config", ProjectRelativePath(info.CWD, info.ConfigPath)})
		if info.DefaultNamespace != "" {
			rows = append(rows, struct {
				ok    bool
				label string
				value string
			}{true, "Namespace", info.DefaultNamespace})
		}
	}
	rows = append(rows, struct {
		ok    bool
		label string
		value string
	}{true, "Runtime", "glade " + info.Version + " · " + info.GoVersion + " · " + info.OSArch})

	allOK := info.ParserOK && info.ToolchainOK && !info.ConfigMissing
	for _, row := range rows {
		icon := t.Green(t.GlyphPass)
		if !row.ok {
			icon = t.Red(t.GlyphFail)
			allOK = false
		}
		if !t.Color {
			if row.ok {
				icon = t.GlyphPass
			} else {
				icon = t.GlyphFail
			}
		}
		if _, err := fmt.Fprintf(w, "%-12s %s %s\n", row.label, icon, row.value); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if allOK {
		if _, err := fmt.Fprintln(w, "Ready."); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Next:"); err != nil {
			return err
		}
		for _, step := range []string{"glade check", "glade test changed --since origin/main", "glade playground --examples --open"} {
			if _, err := fmt.Fprintln(w, "  "+step); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := fmt.Fprintln(w, "Setup steps needed."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Fix:"); err != nil {
		return err
	}
	if info.ConfigMissing {
		if _, err := fmt.Fprintln(w, "  glade init --project . --yes"); err != nil {
			return err
		}
	}
	if !info.ToolchainOK {
		if _, err := fmt.Fprintln(w, "  glade toolchain install --from ."); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, "Then run:\n  glade doctor\n  glade check")
	return err
}

func ParserStatusOK(status string) bool {
	return strings.HasPrefix(status, "ok")
}

func toolchainDoctorValue(info DoctorInfo) string {
	if info.ToolchainPath == "" {
		return info.ToolchainStatus
	}
	if info.ToolchainStatus == "" {
		return info.ToolchainPath
	}
	return info.ToolchainPath + " (" + info.ToolchainStatus + ")"
}

func doctorProjectValue(info DoctorInfo) string {
	if info.ProjectRoot != "" && info.ProjectRoot != "." {
		return "SFDX project found at " + filepath.ToSlash(info.ProjectRoot)
	}
	return "SFDX project found"
}
