package lightningout

import "regexp"

type UseCall struct {
	App string
}

type CreateCall struct {
	Component string
	Locator   string // element id or selector
}

type Calls struct {
	Use    []UseCall
	Create []CreateCall
}

var (
	lightningUseRE    = regexp.MustCompile(`\$Lightning\.use\s*\(\s*["']([^"']+)["']`)
	createComponentRE = regexp.MustCompile(`\$Lightning\.createComponent\s*\(\s*["']([^"']+)["']\s*,\s*(\{[^}]*\}|[^,]+)\s*,\s*["']([^"']+)["']`)
)

func ParseLightningCalls(script string) (Calls, error) {
	var calls Calls
	for _, m := range lightningUseRE.FindAllStringSubmatch(script, -1) {
		calls.Use = append(calls.Use, UseCall{App: m[1]})
	}
	for _, m := range createComponentRE.FindAllStringSubmatch(script, -1) {
		calls.Create = append(calls.Create, CreateCall{
			Component: m[1],
			Locator:   m[3],
		})
	}
	return calls, nil
}
