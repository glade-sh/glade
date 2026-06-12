package cliui

import (
	"fmt"
	"io"
	"strings"
)

type DoctorInfo struct {
	Version          string `json:"version"`
	GoVersion        string `json:"goVersion"`
	OSArch           string `json:"osArch"`
	CWD              string `json:"cwd"`
	ConfigPath       string `json:"configPath,omitempty"`
	ConfigMissing    bool   `json:"configMissing"`
	ProjectRoot      string `json:"projectRoot,omitempty"`
	DefaultNamespace string `json:"defaultNamespace,omitempty"`
	ParserStatus     string `json:"parserStatus"`
	ParserOK         bool   `json:"parserOK"`
	ToolchainPath    string `json:"toolchainPath,omitempty"`
	ToolchainStatus  string `json:"toolchainStatus"`
	ToolchainOK      bool   `json:"toolchainOK"`
}

func WriteDoctor(w io.Writer, info DoctorInfo) error {
	t := NewTheme(w)
	header := FormatBox(t, "glade doctor", "glade "+info.Version, defaultBoxWidth)
	if _, err := fmt.Fprintln(w, header); err != nil {
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
		{true, "go", info.GoVersion},
		{info.ParserOK, "parser", info.ParserStatus},
		{info.ToolchainOK, "lwc.toolchain", toolchainDoctorValue(info)},
	}
	if info.ConfigMissing {
		rows = append(rows, struct {
			ok    bool
			label string
			value string
		}{false, "config", "not found"})
	} else {
		rows = append(rows, struct {
			ok    bool
			label string
			value string
		}{true, "config", info.ConfigPath})
		if info.ProjectRoot != "" {
			rows = append(rows, struct {
				ok    bool
				label string
				value string
			}{true, "project.root", info.ProjectRoot})
		}
		if info.DefaultNamespace != "" {
			rows = append(rows, struct {
				ok    bool
				label string
				value string
			}{true, "project.defaultNamespace", info.DefaultNamespace})
		}
	}

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
		if _, err := fmt.Fprintln(w, FormatRow(t, icon, row.label, row.value)); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, FormatSeparator(defaultBoxWidth)); err != nil {
		return err
	}
	finalIcon := t.Green(t.GlyphPass)
	finalMsg := "All checks passed"
	if !allOK {
		finalIcon = t.Red(t.GlyphFail)
		finalMsg = "Some checks failed"
	}
	if !t.Color {
		if allOK {
			finalIcon = t.GlyphPass
		} else {
			finalIcon = t.GlyphFail
		}
	}
	_, err := fmt.Fprintln(w, FormatRow(t, finalIcon, finalMsg, ""))
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
