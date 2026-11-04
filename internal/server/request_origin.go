package server

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// localRequestOriginAllowed prevents browser requests from a different origin
// from reaching the local API. Clients without browser origin headers remain
// supported.
func localRequestOriginAllowed(r *http.Request) bool {
	site := strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))
	if strings.EqualFold(site, "cross-site") || strings.EqualFold(site, "same-site") {
		return false
	}
	origins := r.Header.Values("Origin")
	if len(origins) == 0 {
		return true
	}
	if len(origins) != 1 {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	origin, err := url.Parse(origins[0])
	if err != nil || origin.User != nil || origin.Hostname() == "" || origin.Path != "" || origin.RawQuery != "" || origin.ForceQuery || origin.Fragment != "" || origin.Opaque != "" {
		return false
	}
	if proxyOrigin := parseProxyOrigin(origins[0]); proxyOrigin != nil && proxyOriginAllowed(r.Host, proxyOrigin) {
		return true
	}
	if configured, _ := r.Context().Value(proxyAuthorityContextKey{}).(bool); configured {
		return false
	}
	if !strings.EqualFold(origin.Scheme, scheme) {
		return false
	}
	target, err := url.Parse(scheme + "://" + r.Host)
	if err != nil || target.User != nil || target.Hostname() == "" || target.Path != "" || target.RawQuery != "" || target.Fragment != "" {
		return false
	}
	originHost, targetHost := origin.Hostname(), target.Hostname()
	if !strings.EqualFold(originHost, targetHost) {
		// IPv4-mapped IPv6 and IPv4 literals are distinct browser origins.
		if strings.Contains(originHost, ":") != strings.Contains(targetHost, ":") {
			return false
		}
		first, second := net.ParseIP(originHost), net.ParseIP(targetHost)
		if first == nil || second == nil || !first.Equal(second) {
			return false
		}
	}
	originPort, targetPort := origin.Port(), target.Port()
	defaultPort := "80"
	if scheme == "https" {
		defaultPort = "443"
	}
	if originPort == "" {
		originPort = defaultPort
	}
	if targetPort == "" {
		targetPort = defaultPort
	}
	return originPort == targetPort
}
