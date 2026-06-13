package cliui

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

func ProjectRelativePath(root, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) {
		return filepath.ToSlash(cleanPath)
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return filepath.ToSlash(cleanPath)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return filepath.ToSlash(cleanPath)
	}
	rel, err := filepath.Rel(absRoot, cleanPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return filepath.ToSlash(cleanPath)
	}
	return filepath.ToSlash(rel)
}

func FormatCount(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	if plural == "" {
		plural = singular + "s"
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func FprintlnKV(w io.Writer, label, value string, width int) error {
	if width <= 0 {
		width = len(label)
	}
	_, err := fmt.Fprintf(w, "  %-*s  %s\n", width, label, value)
	return err
}

func StatusText(ok bool, okText, failText string) string {
	if ok {
		if okText != "" {
			return okText
		}
		return "passed"
	}
	if failText != "" {
		return failText
	}
	return "failed"
}
