package enterprise

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
)

func WriteJSON(w io.Writer, report Report) error {
	report.RefreshSummary()
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteMarkdown(w io.Writer, report Report) error {
	report.RefreshSummary()
	if _, err := fmt.Fprintln(w, "# Glade Enterprise Report"); err != nil {
		return err
	}
	fmt.Fprintf(w, "\nCommand: `%s`\n\n", report.Command)
	fmt.Fprintf(w, "Status: `%s`\n\n", report.Status)
	fmt.Fprintf(w, "Project: `%s`\n\n", report.Project.Root)
	fmt.Fprintf(w, "Summary: critical=%d high=%d medium=%d low=%d info=%d\n\n", report.Summary.Critical, report.Summary.High, report.Summary.Medium, report.Summary.Low, report.Summary.Info)
	if report.Trace != nil {
		fmt.Fprintf(w, "Trace: events=%d soql=%d dml=%d async=%d callouts=%d\n\n", report.Trace.Events, report.Trace.SOQLStatements, report.Trace.DMLOperations, report.Trace.AsyncEvents, report.Trace.Callouts)
	}
	for _, section := range report.Sections {
		fmt.Fprintf(w, "## %s\n\n%s\n\n", section.Title, section.Summary)
		for _, item := range section.Items {
			fmt.Fprintf(w, "- %s", item.Label)
			if item.Value != "" {
				fmt.Fprintf(w, ": %s", item.Value)
			}
			fmt.Fprintln(w)
		}
		if len(section.Items) > 0 {
			fmt.Fprintln(w)
		}
	}
	if len(report.Findings) > 0 {
		fmt.Fprintln(w, "## Findings")
		fmt.Fprintln(w)
	}
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "### %s\n\n", finding.Title)
		fmt.Fprintf(w, "- ID: `%s`\n", finding.ID)
		fmt.Fprintf(w, "- Severity: `%s`\n", finding.Severity)
		fmt.Fprintf(w, "- Confidence: `%s`\n", finding.Confidence)
		fmt.Fprintf(w, "- Summary: %s\n", finding.Summary)
		for _, evidence := range finding.Evidence {
			fmt.Fprintf(w, "- Evidence: %s\n", evidence.Message)
		}
		fmt.Fprintf(w, "- Recommendation: %s\n\n", finding.Recommendation)
	}
	if len(report.Limitations) > 0 {
		fmt.Fprintln(w, "## Limitations")
		fmt.Fprintln(w)
		for _, limitation := range report.Limitations {
			fmt.Fprintf(w, "- %s\n", limitation)
		}
	}
	return nil
}

func WriteHTML(w io.Writer, report Report) error {
	report.RefreshSummary()
	const tpl = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Glade Enterprise Report</title>
<style>body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:32px;line-height:1.45;color:#172026}code{background:#eef2f5;padding:2px 4px;border-radius:4px}.finding{border:1px solid #d8dee4;border-radius:6px;padding:16px;margin:12px 0}.meta{color:#57606a}.counts{display:flex;gap:12px;flex-wrap:wrap}.counts span{border:1px solid #d8dee4;border-radius:4px;padding:6px 8px}</style></head>
<body>
<h1>Glade Enterprise Report</h1>
<p class="meta"><code>{{.Command}}</code></p>
	<p>Status: <strong>{{.Status}}</strong></p>
	<div class="counts">
	<span>Critical {{.Summary.Critical}}</span><span>High {{.Summary.High}}</span><span>Medium {{.Summary.Medium}}</span><span>Low {{.Summary.Low}}</span><span>Info {{.Summary.Info}}</span>
	</div>
	{{if .Trace}}<section><h2>Trace Summary</h2><ul><li>Events: {{.Trace.Events}}</li><li>SOQL: {{.Trace.SOQLStatements}}</li><li>DML: {{.Trace.DMLOperations}}</li><li>Async: {{.Trace.AsyncEvents}}</li><li>Callouts: {{.Trace.Callouts}}</li></ul></section>{{end}}
	{{range .Sections}}<section><h2>{{.Title}}</h2><p>{{.Summary}}</p>{{if .Items}}<ul>{{range .Items}}<li>{{.Label}}{{if .Value}}: {{.Value}}{{end}}</li>{{end}}</ul>{{end}}</section>{{end}}
{{if .Findings}}<h2>Findings</h2>{{end}}
{{range .Findings}}<article class="finding"><h3>{{.Title}}</h3><p class="meta">{{.ID}} · {{.Severity}} · {{.Confidence}}</p><p>{{.Summary}}</p><ul>{{range .Evidence}}<li>{{.Message}}</li>{{end}}</ul><p><strong>Recommendation:</strong> {{.Recommendation}}</p></article>{{end}}
{{if .Limitations}}<h2>Limitations</h2><ul>{{range .Limitations}}<li>{{.}}</li>{{end}}</ul>{{end}}
</body></html>`
	return template.Must(template.New("report").Parse(tpl)).Execute(w, report)
}
