package lwcbrowser

import (
	"encoding/json"
	"strings"

	"github.com/glade-sh/glade/internal/aura"
	lwcembed "github.com/glade-sh/glade/internal/lwcruntime/embed"
)

type PageConfig struct {
	Namespace string
	OutApps   []string
	Manifest  Manifest
}

type pageConfigJSON struct {
	Namespace string       `json:"namespace"`
	OutApps   []string     `json:"outApps,omitempty"`
	Manifest  manifestJSON `json:"manifest"`
}

type manifestJSON struct {
	Modules map[string]moduleJSON `json:"modules"`
}

type moduleJSON struct {
	URL string `json:"url"`
	Tag string `json:"tag"`
}

func BootstrapHTML(cfg PageConfig) string {
	payload := pageConfigJSON{
		Namespace: cfg.Namespace,
		OutApps:   append([]string(nil), cfg.OutApps...),
		Manifest: manifestJSON{
			Modules: make(map[string]moduleJSON, len(cfg.Manifest.Modules)),
		},
	}
	for qualified, entry := range cfg.Manifest.Modules {
		payload.Manifest.Modules[strings.ToLower(qualified)] = moduleJSON{
			URL: entry.URL,
			Tag: entry.Tag,
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = []byte(`{"manifest":{"modules":{}}}`)
	}
	var b strings.Builder
	b.WriteString(`<script>window.process={env:{NODE_ENV:"production"}};</script>`)
	b.WriteString(`<script type="importmap">`)
	b.WriteString(importMapJSON(cfg))
	b.WriteString(`</script>`)
	b.WriteString(`<script type="application/json" id="glade-lightning-config">`)
	b.WriteString(string(raw))
	b.WriteString(`</script>`)
	b.WriteString(`<script>`)
	b.WriteString(lightningStubJS())
	b.WriteString(`</script>`)
	b.WriteString(`<script type="module" src="`)
	b.WriteString(lwcembed.ScriptURL())
	b.WriteString(`"></script>`)
	return b.String()
}

func importMapJSON(cfg PageConfig) string {
	imports := map[string]string{
		"lwc":                   "/lightning/vendor/lwc.js",
		"@lwc/synthetic-shadow": "/lightning/vendor/synthetic-shadow.js",
	}
	for key, value := range SalesforceImportMap() {
		imports[key] = value
	}
	for key, value := range LocalLWCImportMap(cfg.Namespace, cfg.Manifest) {
		imports[key] = value
	}
	raw, err := json.Marshal(map[string]any{"imports": imports})
	if err != nil {
		return `{"imports":{"lwc":"/lightning/vendor/lwc.js","@lwc/synthetic-shadow":"/lightning/vendor/synthetic-shadow.js"}}`
	}
	return string(raw)
}

func lightningStubJS() string {
	return `window.__gladeLightningPending=window.__gladeLightningPending||[];` +
		`window.$Lightning=window.$Lightning||{` +
		`use:function(a,c){window.__gladeLightningPending.push(["use",a,c]);},` +
		`createComponent:function(q,p,l,c){window.__gladeLightningPending.push(["create",q,p,l,c]);}` +
		`};`
}

func OutAppQualifiedNames(apps []aura.OutApp, namespace string) []string {
	if namespace == "" {
		namespace = "c"
	}
	names := make([]string, 0, len(apps))
	for _, app := range apps {
		names = append(names, namespace+":"+app.Name)
	}
	return names
}
