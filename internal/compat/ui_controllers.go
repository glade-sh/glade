package compat

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/typesys"
	"github.com/open-aer/oaer/internal/uicontroller"
)

type UIControllerReport struct {
	Target      string                   `json:"target"`
	Ready       bool                     `json:"ready"`
	Project     string                   `json:"project"`
	Summary     UIControllerSummary      `json:"summary"`
	ApexMethods []UIControllerApexMethod `json:"apexMethods,omitempty"`
	AuraActions []UIControllerAuraAction `json:"auraActions,omitempty"`
	LWCImports  []UIControllerLWCImport  `json:"lwcImports,omitempty"`
	Wires       []UIControllerWire       `json:"wires,omitempty"`
	Failures    []string                 `json:"failures,omitempty"`
	Index       *uicontroller.Index      `json:"index,omitempty"`
}

type UIControllerSummary struct {
	AuraBundles    int `json:"auraBundles"`
	LWCBundles     int `json:"lwcBundles"`
	ApexMethods    int `json:"apexMethods"`
	ResolvedApex   int `json:"resolvedApex"`
	UnresolvedApex int `json:"unresolvedApex"`
	LWCImports     int `json:"lwcImports"`
	Wires          int `json:"wires"`
}

type UIControllerApexMethod struct {
	Framework  string `json:"framework"`
	ClassName  string `json:"className"`
	MethodName string `json:"methodName"`
	Resolved   bool   `json:"resolved"`
	ReturnType string `json:"returnType,omitempty"`
}

type UIControllerAuraAction struct {
	Bundle     string `json:"bundle"`
	Name       string `json:"name"`
	ClassName  string `json:"className,omitempty"`
	Resolved   bool   `json:"resolved"`
	ReturnType string `json:"returnType,omitempty"`
}

type UIControllerLWCImport struct {
	Bundle       string `json:"bundle"`
	LocalName    string `json:"localName,omitempty"`
	Kind         string `json:"kind"`
	ClassName    string `json:"className,omitempty"`
	MethodName   string `json:"methodName,omitempty"`
	LabelName    string `json:"labelName,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
	SchemaName   string `json:"schemaName,omitempty"`
	Module       string `json:"module,omitempty"`
}

type UIControllerWire struct {
	Bundle        string   `json:"bundle"`
	Adapter       string   `json:"adapter"`
	AdapterKind   string   `json:"adapterKind,omitempty"`
	ApexClassName string   `json:"apexClassName,omitempty"`
	ApexMethod    string   `json:"apexMethodName,omitempty"`
	Reactive      []string `json:"reactiveParameters,omitempty"`
}

func RunUIControllerDiscovery(root string, includeIndex bool) (UIControllerReport, error) {
	if root == "" {
		root = "."
	}
	absRoot, _ := filepath.Abs(root)
	report := UIControllerReport{Target: "UI controller discovery", Project: absRoot}
	p, err := project.Load(root)
	if err != nil {
		return report, err
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		return report, err
	}
	idx, err := uicontroller.Build(p, typesys.Build(p, s))
	if err != nil {
		return report, err
	}
	report.Summary.AuraBundles = len(idx.AuraBundles)
	report.Summary.LWCBundles = len(idx.LWCBundles)
	report.Summary.ApexMethods = len(idx.ApexMethods)
	for _, method := range idx.ApexMethods {
		if method.Resolved {
			report.Summary.ResolvedApex++
		} else {
			report.Summary.UnresolvedApex++
		}
		report.ApexMethods = append(report.ApexMethods, UIControllerApexMethod{
			Framework:  method.Framework,
			ClassName:  method.ClassName,
			MethodName: method.MethodName,
			Resolved:   method.Resolved,
			ReturnType: method.ReturnType,
		})
	}
	for _, bundle := range idx.AuraBundles {
		for _, action := range bundle.ActionReferences {
			report.AuraActions = append(report.AuraActions, UIControllerAuraAction{
				Bundle:     bundle.Name,
				Name:       action.Name,
				ClassName:  action.ClassName,
				Resolved:   action.Resolved,
				ReturnType: action.ReturnType,
			})
		}
	}
	for _, bundle := range idx.LWCBundles {
		report.Summary.LWCImports += len(bundle.Imports)
		report.Summary.Wires += len(bundle.Wires)
		for _, imp := range bundle.Imports {
			report.LWCImports = append(report.LWCImports, UIControllerLWCImport{
				Bundle:       bundle.Name,
				LocalName:    imp.LocalName,
				Kind:         imp.Kind,
				ClassName:    imp.ClassName,
				MethodName:   imp.MethodName,
				LabelName:    imp.LabelName,
				ResourceName: imp.ResourceName,
				SchemaName:   imp.SchemaName,
				Module:       imp.Module,
			})
		}
		for _, wire := range bundle.Wires {
			report.Wires = append(report.Wires, UIControllerWire{
				Bundle:        bundle.Name,
				Adapter:       wire.Adapter,
				AdapterKind:   wire.AdapterKind,
				ApexClassName: wire.ApexClassName,
				ApexMethod:    wire.ApexMethodName,
				Reactive:      append([]string(nil), wire.ReactiveParameters...),
			})
		}
	}
	sortUIControllerReport(&report)
	report.Ready = report.Summary.UnresolvedApex == 0
	if includeIndex {
		report.Index = &idx
	}
	return report, nil
}

func CheckUIControllerDiscovery(path string) (UIControllerReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return UIControllerReport{}, err
	}
	var expected UIControllerReport
	if err := json.Unmarshal(data, &expected); err != nil {
		return UIControllerReport{}, err
	}
	projectRoot := expected.Project
	if !filepath.IsAbs(projectRoot) {
		absPath, _ := filepath.Abs(path)
		projectRoot = filepath.Clean(filepath.Join(filepath.Dir(absPath), projectRoot))
	}
	actual, err := RunUIControllerDiscovery(projectRoot, false)
	if err != nil {
		return actual, err
	}
	actual.Project = expected.Project
	compareUIController("ready", actual.Ready, expected.Ready, &actual.Failures)
	compareUIController("summary", actual.Summary, expected.Summary, &actual.Failures)
	compareUIController("apexMethods", actual.ApexMethods, expected.ApexMethods, &actual.Failures)
	compareUIController("auraActions", actual.AuraActions, expected.AuraActions, &actual.Failures)
	compareUIController("lwcImports", actual.LWCImports, expected.LWCImports, &actual.Failures)
	compareUIController("wires", actual.Wires, expected.Wires, &actual.Failures)
	actual.Ready = len(actual.Failures) == 0
	if !actual.Ready {
		return actual, fmt.Errorf("UI controller discovery baseline mismatch: %v", actual.Failures)
	}
	return actual, nil
}

func WriteUIControllerJSON(w io.Writer, report UIControllerReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteUIControllerText(w io.Writer, report UIControllerReport) {
	state := "ready"
	if !report.Ready {
		state = "not ready"
	}
	fmt.Fprintf(w, "UI controller discovery: %s\n", state)
	fmt.Fprintf(w, "project: %s\n", report.Project)
	fmt.Fprintf(w, "summary: aura=%d lwc=%d apex=%d resolved=%d unresolved=%d imports=%d wires=%d\n",
		report.Summary.AuraBundles,
		report.Summary.LWCBundles,
		report.Summary.ApexMethods,
		report.Summary.ResolvedApex,
		report.Summary.UnresolvedApex,
		report.Summary.LWCImports,
		report.Summary.Wires,
	)
	for _, failure := range report.Failures {
		fmt.Fprintf(w, "! %s\n", failure)
	}
}

func compareUIController[T any](name string, actual, expected T, failures *[]string) {
	if !reflect.DeepEqual(actual, expected) {
		*failures = append(*failures, fmt.Sprintf("%s = %#v, want %#v", name, actual, expected))
	}
}

func sortUIControllerReport(report *UIControllerReport) {
	sort.Slice(report.ApexMethods, func(i, j int) bool {
		if report.ApexMethods[i].Framework == report.ApexMethods[j].Framework {
			if report.ApexMethods[i].ClassName == report.ApexMethods[j].ClassName {
				return report.ApexMethods[i].MethodName < report.ApexMethods[j].MethodName
			}
			return report.ApexMethods[i].ClassName < report.ApexMethods[j].ClassName
		}
		return report.ApexMethods[i].Framework < report.ApexMethods[j].Framework
	})
	sort.Slice(report.AuraActions, func(i, j int) bool {
		if report.AuraActions[i].Bundle == report.AuraActions[j].Bundle {
			return report.AuraActions[i].Name < report.AuraActions[j].Name
		}
		return report.AuraActions[i].Bundle < report.AuraActions[j].Bundle
	})
	sort.Slice(report.LWCImports, func(i, j int) bool {
		if report.LWCImports[i].Bundle == report.LWCImports[j].Bundle {
			if report.LWCImports[i].Kind == report.LWCImports[j].Kind {
				return report.LWCImports[i].LocalName < report.LWCImports[j].LocalName
			}
			return report.LWCImports[i].Kind < report.LWCImports[j].Kind
		}
		return report.LWCImports[i].Bundle < report.LWCImports[j].Bundle
	})
	sort.Slice(report.Wires, func(i, j int) bool {
		if report.Wires[i].Bundle == report.Wires[j].Bundle {
			return report.Wires[i].Adapter < report.Wires[j].Adapter
		}
		return report.Wires[i].Bundle < report.Wires[j].Bundle
	})
}
