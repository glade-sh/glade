package testreport

import (
	"fmt"
	"html"
	"io"
	"strings"
)

type HTMLReportOptions struct {
	Title string
}

func WriteGitHubAnnotations(w io.Writer, run Run) error {
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			status := normalizeStatus(testCase.Status)
			if status == StatusPass || status == StatusSkipped {
				continue
			}
			file, line, col := firstProblemLocation(testCase)
			title := string(status)
			message := testCase.displayName(suite.Name)
			if testCase.Problem != nil {
				if testCase.Problem.Type != "" {
					title = testCase.Problem.Type
				}
				if testCase.Problem.Message != "" {
					message = testCase.Problem.Message
				}
			}
			props := []string{}
			if file != "" {
				props = append(props, "file="+escapeGitHubAnnotationProperty(file))
			}
			if line > 0 {
				props = append(props, fmt.Sprintf("line=%d", line))
				if col > 0 {
					props = append(props, fmt.Sprintf("col=%d", col))
				}
			}
			if title != "" {
				props = append(props, "title="+escapeGitHubAnnotationProperty(title))
			}
			command := "::error"
			if len(props) > 0 {
				command += " " + strings.Join(props, ",")
			}
			if _, err := fmt.Fprintf(w, "%s::%s\n", command, escapeGitHubAnnotationData(message)); err != nil {
				return err
			}
		}
	}
	return nil
}

func WriteHTML(w io.Writer, run Run, opts HTMLReportOptions) error {
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = "Glade Test Report"
	}
	summary := run.Summary()
	if _, err := fmt.Fprintf(w, "<!doctype html>\n<html><head><meta charset=\"utf-8\"><title>%s</title><style>%s</style></head><body>\n", html.EscapeString(title), reportCSS()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "<main><h1>%s</h1><section class=\"summary\"><span>%d total</span><span>%d passed</span><span>%d failed</span><span>%d errors</span><span>%d unsupported</span></section>\n", html.EscapeString(title), summary.Total, summary.Passed, summary.Failed, summary.Errors, summary.Unsupported); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "<table><thead><tr><th>Test</th><th>Status</th><th>Duration</th><th>Problem</th><th>Location</th></tr></thead><tbody>\n"); err != nil {
		return err
	}
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			file, line, _ := firstProblemLocation(testCase)
			location := file
			if line > 0 {
				location = fmt.Sprintf("%s:%d", file, line)
			}
			problem := ""
			if testCase.Problem != nil {
				problem = strings.TrimSpace(strings.TrimSpace(testCase.Problem.Type) + " " + strings.TrimSpace(testCase.Problem.Message))
			}
			if _, err := fmt.Fprintf(w, "<tr class=\"status-%s\"><td>%s</td><td>%s</td><td>%dms</td><td>%s</td><td>%s</td></tr>\n",
				html.EscapeString(string(normalizeStatus(testCase.Status))),
				html.EscapeString(testCase.displayName(suite.Name)),
				html.EscapeString(string(normalizeStatus(testCase.Status))),
				testCase.DurationMS,
				html.EscapeString(problem),
				html.EscapeString(location),
			); err != nil {
				return err
			}
		}
	}
	_, err := io.WriteString(w, "</tbody></table></main></body></html>\n")
	return err
}

func firstProblemLocation(testCase Case) (file string, line int, col int) {
	if testCase.Problem == nil || len(testCase.Problem.Stack) == 0 {
		return "", 0, 0
	}
	frame := testCase.Problem.Stack[0]
	return frame.File, frame.Line, frame.Column
}

func reportCSS() string {
	return `body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:0;color:#1f2937;background:#f8fafc}main{max-width:1100px;margin:0 auto;padding:32px}h1{font-size:28px;margin:0 0 20px}.summary{display:flex;gap:8px;flex-wrap:wrap;margin-bottom:24px}.summary span{background:#fff;border:1px solid #d7dee8;border-radius:6px;padding:8px 10px}table{width:100%;border-collapse:collapse;background:#fff;border:1px solid #d7dee8}th,td{text-align:left;border-bottom:1px solid #e5e7eb;padding:9px 10px;vertical-align:top}th{background:#eef2f7}.status-pass td:first-child{border-left:4px solid #16a34a}.status-fail td:first-child,.status-runtime_error td:first-child,.status-compile_error td:first-child{border-left:4px solid #dc2626}.status-unsupported td:first-child{border-left:4px solid #d97706}`
}

func escapeGitHubAnnotationData(value string) string {
	value = strings.ReplaceAll(value, "%", "%25")
	value = strings.ReplaceAll(value, "\r", "%0D")
	value = strings.ReplaceAll(value, "\n", "%0A")
	return value
}

func escapeGitHubAnnotationProperty(value string) string {
	value = escapeGitHubAnnotationData(value)
	value = strings.ReplaceAll(value, ":", "%3A")
	value = strings.ReplaceAll(value, ",", "%2C")
	return value
}
