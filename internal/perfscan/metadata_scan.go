package perfscan

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/uicontroller"
	"github.com/glade-sh/glade/internal/visualforce"
)

func scanMetadata(report *Report, p project.Project, index typesys.Index) {
	vf := visualforce.LoadProjectBestEffort(p)
	for _, page := range vf.Pages {
		report.AddEntryPoint(EntryPoint{Kind: EntryVisualforce, Name: page.Name, File: page.File})
		if page.Action != "" {
			report.AddFinding(Finding{
				ID:         "perf.ui.visualforce.action",
				Category:   CategoryUI,
				Severity:   SeverityMedium,
				Confidence: ConfidenceStatic,
				Score:      58,
				EntryPoint: EntryPoint{Kind: EntryVisualforce, Name: page.Name, File: page.File},
				Message:    "Visualforce page action runs controller code during page load and can hide SOQL, DML, and view-state work behind navigation.",
				Location:   Location{File: page.File},
				Evidence:   []Evidence{{Kind: "visualforce", Message: "page action", Value: page.Action}},
				Fix:        "Keep page-load actions read-light, avoid DML during initial render, and move expensive work behind explicit user actions or async jobs.",
			})
		}
	}

	ui, err := uicontroller.Build(p, index)
	if err == nil {
		for _, ref := range ui.ApexMethods {
			kind := EntryAura
			id := "perf.ui.aura.server-action"
			if ref.Framework == "lwc" {
				kind = EntryLWC
				id = "perf.ui.lwc.wire-apex"
			}
			report.AddEntryPoint(EntryPoint{Kind: kind, Name: ref.ClassName + "." + ref.MethodName, File: ref.File, Line: ref.Line, Method: ref.MethodName})
			report.AddFinding(Finding{
				ID:         id,
				Category:   CategoryUI,
				Severity:   SeverityLow,
				Confidence: ConfidenceStatic,
				Score:      42,
				EntryPoint: EntryPoint{Kind: kind, Name: ref.ClassName + "." + ref.MethodName, File: ref.File, Line: ref.Line, Method: ref.MethodName},
				Message:    "Lightning UI entry points can repeat Apex work across components, wires, and user interactions.",
				Location:   Location{File: ref.File, Line: ref.Line},
				Evidence:   []Evidence{{Kind: ref.Framework, Message: "Apex controller method", Value: ref.ClassName + "." + ref.MethodName}},
				Fix:        "Use cacheable read methods, Lightning Data Service where possible, and consolidate duplicate server calls from the same view.",
			})
		}
	}

	for _, path := range p.FlowFiles {
		scanFlowFile(report, path)
	}
	for _, path := range p.WorkflowFiles {
		scanWorkflowFile(report, path)
	}
}

func scanFlowFile(report *Report, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	source := string(data)
	name := strings.TrimSuffix(filepath.Base(path), ".flow-meta.xml")
	report.AddEntryPoint(EntryPoint{Kind: EntryFlow, Name: name, File: path})
	lookups := strings.Count(source, "<recordLookups>")
	updates := strings.Count(source, "<recordUpdates>") + strings.Count(source, "<recordCreates>") + strings.Count(source, "<recordDeletes>")
	if lookups+updates > 0 {
		report.AddFinding(Finding{
			ID:         "perf.automation.flow.data-fanout",
			Category:   CategoryAutomation,
			Severity:   SeverityMedium,
			Confidence: ConfidenceStatic,
			Score:      62 + (lookups+updates)*4,
			EntryPoint: EntryPoint{Kind: EntryFlow, Name: name, File: path},
			Message:    "Flow data elements add SOQL or DML work inside the same transaction as Apex, triggers, and validation.",
			Location:   Location{File: path},
			Evidence: []Evidence{
				{Kind: "flow", Message: "record lookup count", Value: stringInt(lookups)},
				{Kind: "flow", Message: "record mutation count", Value: stringInt(updates)},
			},
			Fix: "Reduce data elements in loops, prefer before-save field updates for simple same-record changes, and avoid duplicate lookup/update paths with Apex triggers.",
		})
	}
}

func scanWorkflowFile(report *Report, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	source := string(data)
	name := strings.TrimSuffix(filepath.Base(path), ".workflow-meta.xml")
	report.AddEntryPoint(EntryPoint{Kind: EntryWorkflow, Name: name, File: path})
	if strings.Contains(strings.ToLower(source), "<active>true</active>") {
		report.AddFinding(Finding{
			ID:         "perf.automation.workflow.active-rule",
			Category:   CategoryAutomation,
			Severity:   SeverityLow,
			Confidence: ConfidenceStatic,
			Score:      36,
			EntryPoint: EntryPoint{Kind: EntryWorkflow, Name: name, File: path},
			Message:    "Active Workflow rules can add field updates and email work after DML and can cause additional save-order passes.",
			Location:   Location{File: path},
			Evidence:   []Evidence{{Kind: "workflow", Message: "active workflow metadata"}},
			Fix:        "Check whether Workflow field updates duplicate trigger or Flow work, and consolidate save-order side effects where safe.",
		})
	}
}

func stringInt(value int) string {
	return strconv.Itoa(value)
}
