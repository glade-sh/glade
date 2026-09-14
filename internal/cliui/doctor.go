package cliui

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type DoctorInfo struct {
	SchemaVersion            string           `json:"schemaVersion,omitempty"`
	Command                  string           `json:"command,omitempty"`
	Status                   string           `json:"status,omitempty"`
	ExitCode                 int              `json:"exitCode"`
	ReadinessScope           string           `json:"readinessScope,omitempty"`
	ApexReady                bool             `json:"apexReady"`
	Version                  string           `json:"version"`
	GoVersion                string           `json:"goVersion"`
	OSArch                   string           `json:"osArch"`
	CWD                      string           `json:"cwd"`
	ConfigPath               string           `json:"configPath,omitempty"`
	ConfigMissing            bool             `json:"configMissing"`
	ConfigOK                 bool             `json:"configOK"`
	ConfigStatus             string           `json:"configStatus,omitempty"`
	ProjectOK                bool             `json:"projectOK"`
	ProjectStatus            string           `json:"projectStatus,omitempty"`
	ProjectRoot              string           `json:"projectRoot,omitempty"`
	DefaultNamespace         string           `json:"defaultNamespace,omitempty"`
	SourceAPIVersion         string           `json:"sourceApiVersion,omitempty"`
	SourceAPIStatus          string           `json:"sourceApiStatus,omitempty"`
	SourceAPIInCheckedWindow bool             `json:"sourceApiInCheckedWindow"`
	ParserStatus             string           `json:"parserStatus"`
	ParserOK                 bool             `json:"parserOK"`
	ToolchainPath            string           `json:"toolchainPath,omitempty"`
	ToolchainStatus          string           `json:"toolchainStatus"`
	ToolchainOK              bool             `json:"toolchainOK"`
	LocalData                *DoctorLocalData `json:"localData,omitempty"`
	SalesforceBoundary       string           `json:"salesforceBoundary,omitempty"`
	Suggestions              []string         `json:"suggestions,omitempty"`
	Recovery                 []string         `json:"recovery,omitempty"`
	Advisories               []string         `json:"advisories,omitempty"`
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
		ok       bool
		advisory bool
		neutral  bool
		label    string
		value    string
	}{
		{ok: info.ProjectOK, label: "Project", value: doctorProjectValue(info)},
		{ok: info.ParserOK, label: "Parser", value: info.ParserStatus},
		{ok: info.ToolchainOK, advisory: true, label: "LWC tools", value: toolchainDoctorValue(info)},
	}
	if info.SourceAPIVersion != "" || info.SourceAPIStatus != "" {
		rows = append(rows, struct {
			ok       bool
			advisory bool
			neutral  bool
			label    string
			value    string
		}{ok: info.SourceAPIInCheckedWindow, advisory: true, label: "Apex default", value: doctorSourceAPIValue(info)})
	}
	if info.ConfigMissing {
		rows = append(rows, struct {
			ok       bool
			advisory bool
			neutral  bool
			label    string
			value    string
		}{label: "Config", value: "no glade.yml found"})
	} else {
		rows = append(rows, struct {
			ok       bool
			advisory bool
			neutral  bool
			label    string
			value    string
		}{ok: info.ConfigOK, label: "Config", value: doctorConfigValue(info)})
		if info.ConfigOK && info.DefaultNamespace != "" {
			rows = append(rows, struct {
				ok       bool
				advisory bool
				neutral  bool
				label    string
				value    string
			}{ok: true, label: "Namespace", value: info.DefaultNamespace})
		}
	}
	if info.LocalData != nil {
		rows = append(rows, struct {
			ok       bool
			advisory bool
			neutral  bool
			label    string
			value    string
		}{ok: info.LocalData.OK, advisory: true, label: "Local data", value: doctorLocalDataValue(info)})
	}
	rows = append(rows, struct {
		ok       bool
		advisory bool
		neutral  bool
		label    string
		value    string
	}{ok: true, label: "Runtime", value: "glade " + info.Version + " · " + info.GoVersion + " · " + info.OSArch})
	if info.SalesforceBoundary != "" {
		rows = append(rows, struct {
			ok       bool
			advisory bool
			neutral  bool
			label    string
			value    string
		}{neutral: true, label: "Salesforce", value: info.SalesforceBoundary})
	}

	allOK := info.ProjectOK && info.ParserOK && info.ConfigOK
	for _, row := range rows {
		icon := t.Green(t.GlyphPass)
		if row.neutral {
			icon = "-"
		} else if !row.ok && row.advisory {
			icon = t.Yellow(t.GlyphWarn)
		} else if !row.ok {
			icon = t.Red(t.GlyphFail)
			allOK = false
		}
		if !t.Color {
			if row.neutral {
				icon = "-"
			} else if row.ok {
				icon = t.GlyphPass
			} else if row.advisory {
				icon = t.GlyphWarn
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
		if len(info.Advisories) > 0 {
			if _, err := fmt.Fprintln(w, "\nAdvisory:"); err != nil {
				return err
			}
			for _, advisory := range info.Advisories {
				if _, err := fmt.Fprintln(w, "  "+advisory); err != nil {
					return err
				}
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Next:"); err != nil {
			return err
		}
		for _, step := range info.Suggestions {
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
	for _, step := range info.Recovery {
		if _, err := fmt.Fprintln(w, "  "+step); err != nil {
			return err
		}
	}
	return nil
}

func doctorConfigValue(info DoctorInfo) string {
	path := filepath.ToSlash(ProjectRelativePath(info.CWD, info.ConfigPath))
	if info.ConfigOK || info.ConfigStatus == "" {
		return path
	}
	if path == "" {
		return info.ConfigStatus
	}
	return path + " (" + info.ConfigStatus + ")"
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
	if !info.ProjectOK {
		if info.ProjectStatus != "" {
			return info.ProjectStatus
		}
		return "project could not be loaded"
	}
	if info.ProjectRoot != "" {
		return "root " + filepath.ToSlash(ProjectRelativePath(info.CWD, info.ProjectRoot))
	}
	return "project root found"
}

func doctorSourceAPIValue(info DoctorInfo) string {
	if info.SourceAPIStatus != "" {
		return info.SourceAPIStatus
	}
	return info.SourceAPIVersion
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
