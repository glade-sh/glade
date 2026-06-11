package pluginhost

import (
	"fmt"
	"strings"
)

type PluginRef struct {
	Name    string
	Version string
}

var firstPartyAliases = map[string]string{
	"compat":      "@glade/compat",
	"performance": "@glade/performance",
}

func ParsePluginRef(input string) (PluginRef, error) {
	raw := strings.TrimSpace(input)
	if raw == "" || strings.Contains(raw, "://") || strings.ContainsAny(raw, `/\`) && !strings.HasPrefix(raw, "@") {
		return PluginRef{}, fmt.Errorf("invalid plugin coordinate %q", input)
	}
	if canonical, ok := firstPartyAliases[raw]; ok {
		raw = canonical
	}
	name, version := raw, ""
	if strings.HasPrefix(raw, "@") {
		if at := strings.LastIndex(raw[1:], "@"); at >= 0 {
			split := at + 1
			name = raw[:split]
			version = raw[split+1:]
		}
	} else if at := strings.LastIndex(raw, "@"); at > 0 {
		name = raw[:at]
		version = raw[at+1:]
	}
	ref := PluginRef{Name: name, Version: version}
	if err := ref.Validate(); err != nil {
		return PluginRef{}, err
	}
	return ref, nil
}

func (r PluginRef) Validate() error {
	if strings.HasPrefix(r.Name, "@") {
		parts := strings.Split(r.Name[1:], "/")
		if len(parts) != 2 {
			return fmt.Errorf("plugin coordinate %q must be @scope/name", r.Name)
		}
		if err := validatePluginPathToken("plugin scope", parts[0]); err != nil {
			return err
		}
		if err := validatePluginPathToken("plugin package", parts[1]); err != nil {
			return err
		}
	} else if err := validatePluginPathToken("plugin name", r.Name); err != nil {
		return err
	}
	if r.Version != "" {
		if err := validatePluginPathToken("plugin version", r.Version); err != nil {
			return err
		}
	}
	return nil
}

func (r PluginRef) ManifestName() string {
	if strings.HasPrefix(r.Name, "@") {
		return strings.SplitN(r.Name[1:], "/", 2)[1]
	}
	return r.Name
}

func (r PluginRef) StorageName() string {
	if strings.HasPrefix(r.Name, "@") {
		parts := strings.SplitN(r.Name[1:], "/", 2)
		return parts[0] + "__" + parts[1]
	}
	return r.Name
}
