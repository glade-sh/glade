package lwcshell

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const ContextPresetFilename = "glade.lwc.json"

type ContextPresetFile struct {
	DefaultContext string                   `json:"defaultContext,omitempty"`
	Contexts       map[string]ContextPreset `json:"contexts,omitempty"`
	Path           string                   `json:"-"`
}

type ContextPreset struct {
	Target        string            `json:"target,omitempty"`
	Component     string            `json:"component,omitempty"`
	ObjectAPIName string            `json:"objectApiName,omitempty"`
	RecordID      string            `json:"recordId,omitempty"`
	Page          string            `json:"page,omitempty"`
	Tab           string            `json:"tab,omitempty"`
	Action        string            `json:"action,omitempty"`
	App           string            `json:"app,omitempty"`
	FormFactor    string            `json:"formFactor,omitempty"`
	State         map[string]string `json:"state,omitempty"`
}

type ContextPresetError struct {
	Diagnostic Diagnostic
	Err        error
}

func (e *ContextPresetError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s %s: %v", e.Diagnostic.Code, e.Diagnostic.Message, e.Err)
	}
	return fmt.Sprintf("%s %s", e.Diagnostic.Code, e.Diagnostic.Message)
}

func (e *ContextPresetError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func LoadContextPresets(root string) (ContextPresetFile, error) {
	path := filepath.Join(root, ContextPresetFilename)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ContextPresetFile{Contexts: map[string]ContextPreset{}}, nil
	}
	if err != nil {
		return ContextPresetFile{}, contextPresetError("GLADELWC020", "context preset file invalid", err)
	}
	file, err := parseContextPresetFile(data)
	if err != nil {
		return ContextPresetFile{}, err
	}
	file.Path = path
	return file, nil
}

func LoadContextPresetFile(path string) (ContextPresetFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ContextPresetFile{}, contextPresetError("GLADELWC020", "context preset file invalid", err)
	}
	file, err := parseContextPresetFile(data)
	if err != nil {
		return ContextPresetFile{}, err
	}
	file.Path = path
	return file, nil
}

func parseContextPresetFile(data []byte) (ContextPresetFile, error) {
	var file ContextPresetFile
	if err := json.Unmarshal(data, &file); err != nil {
		return ContextPresetFile{}, contextPresetError("GLADELWC020", "context preset file invalid", err)
	}
	if file.Contexts == nil {
		file.Contexts = map[string]ContextPreset{}
	}
	return file, nil
}

func (f ContextPresetFile) Preset(name string) (ContextPreset, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = strings.TrimSpace(f.DefaultContext)
	}
	if name == "" {
		return ContextPreset{}, contextPresetError("GLADELWC021", "context preset not found", nil)
	}
	if preset, ok := f.Contexts[name]; ok {
		return preset, nil
	}
	for key, preset := range f.Contexts {
		if strings.EqualFold(key, name) {
			return preset, nil
		}
	}
	return ContextPreset{}, contextPresetError("GLADELWC021", "context preset not found", nil)
}

func (p ContextPreset) ToPageContext() (PageContext, error) {
	ctx := PageContext{
		ComponentName: strings.TrimSpace(p.Component),
		PageName:      strings.TrimSpace(p.Page),
		RecordID:      strings.TrimSpace(p.RecordID),
		ObjectAPIName: strings.TrimSpace(p.ObjectAPIName),
		AppName:       strings.TrimSpace(p.App),
		TabName:       strings.TrimSpace(p.Tab),
		ActionName:    strings.TrimSpace(p.Action),
		FormFactor:    strings.TrimSpace(p.FormFactor),
		State:         copyContextState(p.State),
	}
	switch normalizePresetTarget(p.Target) {
	case "recordpage":
		ctx.Kind = RenderTargetRecordPage
		if ctx.RecordID == "" {
			return PageContext{}, contextPresetError("GLADELWC023", "context record required", nil)
		}
		if ctx.PageName == "" {
			return PageContext{}, contextPresetError("GLADELWC024", "context page required", nil)
		}
	case "apppage":
		ctx.Kind = RenderTargetAppPage
		if ctx.PageName == "" {
			return PageContext{}, contextPresetError("GLADELWC024", "context page required", nil)
		}
	case "homepage":
		ctx.Kind = RenderTargetHomePage
		if ctx.PageName == "" {
			return PageContext{}, contextPresetError("GLADELWC024", "context page required", nil)
		}
	case "component":
		ctx.Kind = RenderTargetComponent
	case "urladdressable":
		ctx.Kind = RenderTargetURLAddressable
	case "tab":
		ctx.Kind = RenderTargetTab
	case "quickaction", "recordaction", "globalaction":
		ctx.Kind = RenderTargetQuickAction
		if ctx.ActionName == "" {
			return PageContext{}, contextPresetError("GLADELWC070", "quick action required", nil)
		}
	default:
		return PageContext{}, contextPresetError("GLADELWC022", "context target unsupported", nil)
	}
	return ctx, nil
}

func normalizePresetTarget(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "_", "")
	return value
}

func copyContextState(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		if strings.TrimSpace(key) == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func contextPresetError(code, message string, err error) *ContextPresetError {
	return &ContextPresetError{Diagnostic: Diagnostic{Code: code, Message: message}, Err: err}
}
