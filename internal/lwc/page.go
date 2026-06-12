package lwc

import (
	"fmt"
	"os"

	"github.com/glade-sh/glade/internal/project"
)

type RenderRequest struct {
	Index         Index
	ComponentName string
	Overrides     map[string]string
	WireMocks     map[string]Value
	Namespace     string
}

type RenderResult struct {
	HTML string
}

func RenderComponent(req RenderRequest) (RenderResult, error) {
	bundle, ok := req.Index.Bundle(req.ComponentName)
	if !ok {
		return RenderResult{}, fmt.Errorf("unknown lwc component %q", req.ComponentName)
	}
	htmlSource, err := bundle.ReadHTML()
	if err != nil {
		return RenderResult{}, err
	}
	tree, err := ParseTemplate(htmlSource)
	if err != nil {
		return RenderResult{}, err
	}
	props := PropertyBag{}
	if bundle.JSFile != "" {
		js, err := os.ReadFile(bundle.JSFile)
		if err != nil {
			return RenderResult{}, err
		}
		defaults, err := ParseAPIProperties(string(js))
		if err != nil {
			return RenderResult{}, err
		}
		for name, literal := range defaults {
			props[name] = literalToValue(literal)
		}
	}
	for k, v := range req.Overrides {
		props[k] = StringValue(v)
	}
	for k, v := range req.WireMocks {
		props[k] = v
	}
	ctx := &RenderContext{
		Index:      req.Index,
		Properties: props,
		WireData:   req.WireMocks,
		Namespace:  req.Namespace,
	}
	html, err := RenderTemplate(tree, ctx)
	if err != nil {
		return RenderResult{}, err
	}
	return RenderResult{HTML: html}, nil
}

func RenderComponentForTest(projectRoot, componentName string, overrides map[string]string) (string, error) {
	p, err := project.Load(projectRoot)
	if err != nil {
		return "", err
	}
	idx, err := BuildIndex(p)
	if err != nil {
		return "", err
	}
	result, err := RenderComponent(RenderRequest{
		Index:         idx,
		ComponentName: componentName,
		Overrides:     overrides,
	})
	if err != nil {
		return "", err
	}
	return result.HTML, nil
}
