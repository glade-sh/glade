package refactorproof

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type APISurfaceOptions struct {
	FailOnBreak bool
}

func CheckAPISurfaceText(before, after string, opts APISurfaceOptions) StageResult {
	beforeSurface := publicSurfaceLines(before)
	afterSurface := publicSurfaceLines(after)

	var missing []string
	for line := range beforeSurface {
		if _, ok := afterSurface[line]; !ok {
			missing = append(missing, line)
		}
	}
	sort.Strings(missing)
	if len(missing) == 0 {
		return StageResult{Name: StageAPISurfaceDelta, Status: StageStatusPass, Message: "no public or global surface delta"}
	}
	status := StageStatusWarn
	if opts.FailOnBreak {
		status = StageStatusFail
	}
	return StageResult{
		Name:    StageAPISurfaceDelta,
		Status:  status,
		Message: "public or global surface changed",
		Details: map[string]any{"removed_or_changed": missing},
	}
}

func CheckAPISurfaceChanges(root, since string, changes []ChangedFile, opts APISurfaceOptions) StageResult {
	var broken []string
	for _, change := range changes {
		if change.Kind != "apex_class" {
			continue
		}
		before := gitFileAt(root, since, change.Path)
		after := ""
		if change.Operation != "deleted" {
			if data, err := os.ReadFile(change.Path); err == nil {
				after = string(data)
			}
		}
		stage := CheckAPISurfaceText(before, after, opts)
		if stage.Status == StageStatusWarn || stage.Status == StageStatusFail {
			if details, ok := stage.Details["removed_or_changed"].([]string); ok {
				for _, item := range details {
					broken = append(broken, change.Symbol+": "+item)
				}
			}
		}
	}
	sort.Strings(broken)
	if len(broken) == 0 {
		return StageResult{Name: StageAPISurfaceDelta, Status: StageStatusPass, Message: "no public or global surface delta"}
	}
	status := StageStatusWarn
	if opts.FailOnBreak {
		status = StageStatusFail
	}
	return StageResult{
		Name:    StageAPISurfaceDelta,
		Status:  status,
		Message: "public or global surface changed",
		Details: map[string]any{"removed_or_changed": broken},
	}
}

func publicSurfaceLines(source string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, " private ") || strings.HasPrefix(lower, "private ") {
			continue
		}
		if containsWord(lower, "public") || containsWord(lower, "global") {
			out[normalizeSurfaceLine(trimmed)] = struct{}{}
		}
	}
	return out
}

func normalizeSurfaceLine(line string) string {
	line = strings.Split(line, "//")[0]
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(line, "{")
	return strings.Join(strings.Fields(line), " ")
}

func containsWord(s, word string) bool {
	for _, field := range strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z')
	}) {
		if field == word {
			return true
		}
	}
	return false
}

func gitFileAt(root, since, path string) string {
	if since == "" {
		return ""
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	cmd := exec.Command("git", "-C", root, "show", since+":"+filepath.ToSlash(rel))
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(bytes.TrimRight(out, "\n"))
}
