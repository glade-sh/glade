package cliui

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type DoctorInfo struct {
	SchemaVersion    string           `json:"schemaVersion,omitempty"`
	Command          string           `json:"command,omitempty"`
	Status           string           `json:"status,omitempty"`
	ExitCode         int              `json:"exitCode"`
	Version          string           `json:"version"`
	GoVersion        string           `json:"goVersion"`
	OSArch           string           `json:"osArch"`
	CWD              string           `json:"cwd"`
	ConfigPath       string           `json:"configPath,omitempty"`
	ConfigMissing    bool             `json:"configMissing"`
	ProjectRoot      string           `json:"projectRoot,omitempty"`
	DefaultNamespace string           `json:"defaultNamespace,omitempty"`
	ParserStatus     string           `json:"parserStatus"`
	ParserOK         bool             `json:"parserOK"`
	ToolchainPath    string           `json:"toolchainPath,omitempty"`
	ToolchainStatus  string           `json:"toolchainStatus"`
	ToolchainOK      bool             `json:"toolchainOK"`
	LocalData        *DoctorLocalData `json:"localData,omitempty"`
	Suggestions      []string         `json:"suggestions,omitempty"`
}

type DoctorLocalData struct {
	Env           string `json:"env"`
	Path          string `json:"path"`
	Status        string `json:"status"`
	OK            bool   `json:"ok"`
	Exists        bool   `json:"exists"`
	SchemaVersion int    `json:"schemaVersion,omitempty"`
	Objects       int    `json:"objects,omitempty"`
	Records       int    `json:"records,omitempty"`
	Detail        string `json:"detail,omitempty"`
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
	if info.LocalData != nil {
		rows = append(rows, struct {
			ok    bool
			label string
			value string
		}{info.LocalData.OK, "Local data", doctorLocalDataValue(info)})
	}
	rows = append(rows, struct {
		ok    bool
		label string
		value string
	}{true, "Runtime", "glade " + info.Version + " · " + info.GoVersion + " · " + info.OSArch})

	allOK := info.ParserOK && info.ToolchainOK && !info.ConfigMissing && (info.LocalData == nil || info.LocalData.OK)
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

func doctorLocalDataValue(info DoctorInfo) string {
	if info.LocalData == nil {
		return ""
	}
	data := info.LocalData
	path := ProjectRelativePath(info.CWD, data.Path)
	switch data.Status {
	case "missing":
		return data.Env + " " + filepath.ToSlash(path) + " (not created)"
	case "ready":
		return fmt.Sprintf("%s %s (%d objects, %d records)", data.Env, filepath.ToSlash(path), data.Objects, data.Records)
	case "mismatch":
		if data.Detail != "" {
			return data.Env + " " + filepath.ToSlash(path) + " (" + data.Detail + ")"
		}
		return data.Env + " " + filepath.ToSlash(path) + " (schema mismatch)"
	default:
		if data.Detail != "" {
			return data.Env + " " + filepath.ToSlash(path) + " (" + data.Detail + ")"
		}
		return data.Env + " " + filepath.ToSlash(path)
	}
}
