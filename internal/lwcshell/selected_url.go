package lwcshell

import (
	"net/url"
	"strings"
)

// SelectedURL returns the local LWC shell route for a resolved render context.
func SelectedURL(baseURL string, ctx PageContext) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" || ctx.Kind == "" {
		return ""
	}
	values := url.Values{}
	addQuery := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			values.Set(key, value)
		}
	}
	addQuery("app", ctx.AppName)
	addQuery("formFactor", ctx.FormFactor)
	for key, value := range ctx.State {
		if strings.TrimSpace(key) != "" {
			values.Set("state."+key, value)
		}
	}
	var selectedPath string
	switch ctx.Kind {
	case RenderTargetComponent, RenderTargetURLAddressable:
		namespace, component := splitSelectedURLComponentName(ctx.ComponentName)
		if component == "" {
			return ""
		}
		addQuery("recordId", ctx.RecordID)
		addQuery("objectApiName", ctx.ObjectAPIName)
		if ctx.Kind == RenderTargetURLAddressable {
			selectedPath = "/lwc/preview/cmp/" + selectedURLPathEscape(namespace) + "/" + selectedURLPathEscape(component)
		} else {
			selectedPath = "/lwc/preview/component/" + selectedURLPathEscape(namespace) + "/" + selectedURLPathEscape(component)
		}
	case RenderTargetRecordPage:
		if ctx.ObjectAPIName == "" || ctx.RecordID == "" {
			return ""
		}
		selectedPath = "/lwc/preview/record/" + selectedURLPathEscape(ctx.ObjectAPIName) + "/" + selectedURLPathEscape(ctx.RecordID)
		addQuery("page", ctx.PageName)
	case RenderTargetAppPage:
		if ctx.PageName == "" {
			return ""
		}
		selectedPath = "/lwc/preview/app/" + selectedURLPathEscape(ctx.PageName)
	case RenderTargetHomePage:
		if ctx.PageName == "" {
			return ""
		}
		selectedPath = "/lwc/preview/home/" + selectedURLPathEscape(ctx.PageName)
	case RenderTargetTab:
		if ctx.TabName == "" {
			return ""
		}
		selectedPath = "/lwc/preview/tab/" + selectedURLPathEscape(ctx.TabName)
	case RenderTargetQuickAction:
		if ctx.ActionName == "" {
			return ""
		}
		actionName := ctx.ActionName
		if ctx.ObjectAPIName != "" && strings.Contains(actionName, ".") {
			if before, after, ok := strings.Cut(actionName, "."); ok && strings.EqualFold(before, ctx.ObjectAPIName) {
				actionName = after
			}
		}
		if ctx.ObjectAPIName != "" && ctx.RecordID != "" {
			selectedPath = "/lwc/preview/action/" + selectedURLPathEscape(ctx.ObjectAPIName) + "/" + selectedURLPathEscape(ctx.RecordID) + "/" + selectedURLPathEscape(actionName)
		} else {
			selectedPath = "/lwc/preview/action/global/" + selectedURLPathEscape(actionName)
		}
	default:
		return ""
	}
	if encoded := values.Encode(); encoded != "" {
		return baseURL + selectedPath + "?" + encoded
	}
	return baseURL + selectedPath
}

func splitSelectedURLComponentName(name string) (string, string) {
	name = strings.TrimSpace(name)
	namespace, component, ok := strings.Cut(name, ":")
	if !ok {
		return "c", name
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = "c"
	}
	return namespace, strings.TrimSpace(component)
}

func selectedURLPathEscape(value string) string {
	return strings.ReplaceAll(url.PathEscape(value), "%2F", "/")
}
