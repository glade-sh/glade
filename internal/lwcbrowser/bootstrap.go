package lwcbrowser

import (
	"encoding/json"
	"strings"

	"github.com/glade-sh/glade/internal/aura"
	lwcembed "github.com/glade-sh/glade/internal/lwcruntime/embed"
)

type PageConfig struct {
	Namespace          string
	OutApps            []string
	OutAppDependencies map[string][]string
	Manifest           Manifest
}

type pageConfigJSON struct {
	Namespace          string              `json:"namespace"`
	OutApps            []string            `json:"outApps,omitempty"`
	OutAppDependencies map[string][]string `json:"outAppDependencies,omitempty"`
	Manifest           manifestJSON        `json:"manifest"`
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
		Namespace:          cfg.Namespace,
		OutApps:            append([]string(nil), cfg.OutApps...),
		OutAppDependencies: copyStringSliceMap(cfg.OutAppDependencies),
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
	return `(function(){` +
		`function n(v){return String(v||"").trim().toLowerCase();}` +
		`function c(){var el=document.getElementById("glade-lightning-config");if(!el){return {outApps:[],manifest:{modules:{}}};}try{return JSON.parse(el.textContent||"{}");}catch(_e){return {outApps:[],manifest:{modules:{}}};}}` +
		`function m(cfg){return cfg&&cfg.manifest&&cfg.manifest.modules||{};}` +
		`function b(q){return n(q).indexOf(":")===-1;}` +
		`function u(q){return n(q).indexOf("lightning:")===0;}` +
		`function e(cb,msg){if(typeof cb==="function"){cb(null,"ERROR",msg);}}` +
		`window.__gladeLightningPending=window.__gladeLightningPending||[];` +
		`window.$Lightning=window.$Lightning||{` +
		`use:function(a,cb){var cfg=c();var apps=(cfg.outApps||[]).map(n);if(apps.indexOf(n(a))===-1){console.error("[glade] Lightning Out app not found",a);e(cb,"Lightning Out app not found: "+a);return;}window.__gladeLightningPending.push(["use",a,cb]);},` +
		`createComponent:function(q,p,l,cb){var cfg=c();if(b(q)){e(cb,"Bad Lightning component name: "+q);return;}if(u(q)){e(cb,"Unsupported Lightning service: "+q);return;}if(!m(cfg)[n(q)]){console.error("[glade] Lightning component not found",q);e(cb,"Lightning component not found: "+q);return;}window.__gladeLightningPending.push(["create",q,p,l,cb]);}` +
		`};` +
		`})();`
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

func OutAppDependencyMap(apps []aura.OutApp, namespace string) map[string][]string {
	if namespace == "" {
		namespace = "c"
	}
	out := make(map[string][]string, len(apps))
	for _, app := range apps {
		appName := normalizeQualified(namespace + ":" + app.Name)
		deps := make([]string, 0, len(app.Dependencies))
		for _, dep := range app.Dependencies {
			if qualified := normalizeAuraDependency(dep, namespace); qualified != "" {
				deps = append(deps, qualified)
			}
		}
		out[appName] = deps
	}
	return out
}

func normalizeAuraDependency(dep, namespace string) string {
	dep = strings.TrimSpace(dep)
	dep = strings.TrimPrefix(dep, "markup://")
	if dep == "" {
		return ""
	}
	parts := strings.SplitN(dep, ":", 2)
	if len(parts) != 2 {
		return normalizeQualified(namespace + ":" + dep)
	}
	if strings.EqualFold(parts[0], "c") || strings.EqualFold(parts[0], namespace) {
		return normalizeQualified(namespace + ":" + parts[1])
	}
	return normalizeQualified(parts[0] + ":" + parts[1])
}

func copyStringSliceMap(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for key, values := range in {
		out[normalizeQualified(key)] = append([]string(nil), values...)
	}
	return out
}
