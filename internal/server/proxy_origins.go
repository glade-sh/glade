package server

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type proxyAuthorityContextKey struct{}

// proxyOrigins are explicit external origins. Forwarded request headers never
// establish trust; the reverse proxy must preserve the external Host.
func proxyOrigins() []*url.URL {
	var origins []*url.URL
	for _, value := range strings.Split(os.Getenv("GLADE_SERVER_PROXY_ORIGINS"), ",") {
		if origin := parseProxyOrigin(strings.TrimSpace(value)); origin != nil {
			origins = append(origins, origin)
		}
	}
	return origins
}

func parseProxyOrigin(value string) *url.URL {
	if strings.ContainsAny(value, "?#") {
		return nil
	}
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Hostname() == "" || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || strings.HasSuffix(u.Host, ":") {
		return nil
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return nil
		}
	}
	return u
}

func sameProxyAuthority(a, b *url.URL) bool {
	if a.Scheme != b.Scheme {
		return false
	}
	ah, bh := a.Hostname(), b.Hostname()
	if !strings.EqualFold(ah, bh) {
		if strings.Contains(ah, ":") != strings.Contains(bh, ":") {
			return false
		}
		ai, bi := net.ParseIP(ah), net.ParseIP(bh)
		if ai == nil || bi == nil || !ai.Equal(bi) {
			return false
		}
	}
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		if u.Scheme == "https" {
			return "443"
		}
		return "80"
	}
	return port(a) == port(b)
}

func proxyAuthorityAllowed(host string) bool {
	for _, configured := range proxyOrigins() {
		target := parseProxyOrigin(configured.Scheme + "://" + host)
		if target != nil && sameProxyAuthority(configured, target) {
			return true
		}
	}
	return false
}

func proxyOriginAllowed(host string, origin *url.URL) bool {
	for _, configured := range proxyOrigins() {
		target := parseProxyOrigin(configured.Scheme + "://" + host)
		if target != nil && sameProxyAuthority(configured, target) && sameProxyAuthority(configured, origin) {
			return true
		}
	}
	return false
}
