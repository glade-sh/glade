package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"strings"
)

// These test-context 56.0 endpoints are the paths captured in flow_api_merge.
func (e *Engine) flowAPIReference(reference string) (storage.Value, bool) {
	if e.Org == nil {
		return storage.Value{}, false
	}
	var kind string
	switch strings.ToLower(strings.TrimSpace(reference)) {
	case "$api.partner_server_url_560":
		kind = "u"
	case "$api.enterprise_server_url_560":
		kind = "c"
	default:
		return storage.Value{}, false
	}
	origin, err := storage.NormalizeOrgDomainURL(e.Org.DomainURL)
	if err != nil {
		return storage.Value{}, false
	}
	if origin == "" {
		origin = "https://local.glade.example"
	}
	orgID := strings.TrimSpace(e.Org.OrgID)
	if orgID == "" {
		orgID = storage.DefaultOrgID
	}
	return storage.StringValue(origin + "/services/Soap/" + kind + "/56.0/" + orgID), true
}
