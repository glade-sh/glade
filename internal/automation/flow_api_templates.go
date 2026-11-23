package automation

import (
	"regexp"
	"strings"
)

// These shapes are the captured direct endpoint and origin formulas. The
// substitution and appended path are string literals, independent of the
// name of the template or resource.
var capturedDirectAPIFormula = regexp.MustCompile(`(?i)^\s*\$api\.(partner|enterprise)_server_url_560\s*$`)
var capturedAPIOriginFormula = regexp.MustCompile(`(?i)^\s*LEFT\s*\(\s*\$api\.partner_server_url_560\s*,\s*FIND\s*\(\s*["']/services["']\s*,\s*\$api\.partner_server_url_560\s*\)\s*-\s*1\s*\)\s*$`)
var capturedAPISubstitutionFormula = regexp.MustCompile(`(?i)^\s*SUBSTITUTE\s*\(\s*LEFT\s*\(\s*\$api\.partner_server_url_560\s*,\s*FIND\s*\(\s*["']/services["']\s*,\s*\$api\.partner_server_url_560\s*\)\s*-\s*1\s*\)\s*,\s*["'][^"']*["']\s*,\s*["'][^"']*["']\s*\)\s*(\+\s*["'][^"']*["']\s*)?$`)

func capturedTemplateAPIFormula(expression string) bool {
	return capturedDirectAPIFormula.MatchString(expression) || capturedAPIOriginFormula.MatchString(expression) || capturedAPISubstitutionFormula.MatchString(expression)
}

// Preserve only API formula resources used by a captured after-save template.
func flowTemplateAPIFormulas(raw flowXML, formulas map[string]string) map[string]string {
	if !strings.EqualFold(raw.Start.TriggerType, "RecordAfterSave") || !(strings.EqualFold(raw.Start.RecordTriggerType, "Create") || strings.EqualFold(raw.Start.RecordTriggerType, "Update")) {
		return nil
	}
	var selected map[string]string
	for name, expression := range formulas {
		if !capturedTemplateAPIFormula(expression) {
			continue
		}
		for _, template := range raw.TextTemplates {
			if strings.Contains(strings.ToLower(template.Text), "{!"+name+"}") {
				if selected == nil {
					selected = make(map[string]string)
				}
				selected[name] = expression
				break
			}
		}
	}
	return selected
}
