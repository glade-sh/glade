package server

import (
	"container/list"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/storage"
)

// GuardHTTPAuthority checks the HTTP authority before routing to a local server.
// Public wildcard listeners may specify explicit hostnames in
// GLADE_SERVER_ALLOWED_HOSTS, separated by commas.
func GuardHTTPAuthority(next http.Handler, addr string, configured ...string) http.Handler {
	configuredHost, port, err := net.SplitHostPort(addr)
	allowed := map[string]bool{}
	if err == nil {
		configuredHost = strings.ToLower(strings.TrimSpace(configuredHost))
		ip := net.ParseIP(configuredHost)
		if configuredHost == "localhost" || (ip != nil && ip.IsLoopback()) {
			allowed[configuredHost] = true
			allowed["localhost"] = true
			allowed["127.0.0.1"] = true
			allowed["::1"] = true
		} else if configuredHost != "" && configuredHost != "0.0.0.0" && configuredHost != "::" {
			allowed[configuredHost] = true
		}
		for _, configuredAddr := range configured {
			host, _, parseErr := net.SplitHostPort(configuredAddr)
			host = strings.ToLower(strings.TrimSpace(host))
			if parseErr == nil && host != "" && host != "0.0.0.0" && host != "::" {
				allowed[host] = true
			}
		}
		for _, host := range strings.Split(os.Getenv("GLADE_SERVER_ALLOWED_HOSTS"), ",") {
			host = strings.ToLower(strings.TrimSpace(host))
			if host != "" {
				allowed[host] = true
			}
		}
	}
	capability := os.Getenv("GLADE_SERVER_CAPABILITY")
	// A server is exposed when it can be reached beyond loopback: a capability
	// is configured, extra hosts are allowed, or the bind address is not loopback.
	// In that case an origin-less Bearer header is not enough; it must be a
	// token this server issued.
	exposed := capability != "" || strings.TrimSpace(os.Getenv("GLADE_SERVER_ALLOWED_HOSTS")) != ""
	for _, configuredAddr := range configured {
		host, _, parseErr := net.SplitHostPort(configuredAddr)
		ip := net.ParseIP(host)
		if parseErr != nil || !(strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())) {
			exposed = true
		}
	}
	if err == nil {
		ip := net.ParseIP(configuredHost)
		if !(configuredHost == "localhost" || (ip != nil && ip.IsLoopback())) {
			exposed = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, requestPort, splitErr := net.SplitHostPort(r.Host)
		listenerPort := port
		if listenerPort == "0" {
			if local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
				_, listenerPort, _ = net.SplitHostPort(local.String())
			}
		}
		if splitErr != nil && listenerPort == "80" {
			if !strings.Contains(r.Host, ":") {
				host, requestPort, splitErr = r.Host, "80", nil
			} else if strings.HasPrefix(r.Host, "[") && strings.HasSuffix(r.Host, "]") {
				candidate := strings.TrimSuffix(strings.TrimPrefix(r.Host, "["), "]")
				if ip := net.ParseIP(candidate); ip != nil && strings.Contains(candidate, ":") {
					host, requestPort, splitErr = candidate, "80", nil
				}
			}
		}
		proxyAuthority := proxyAuthorityAllowed(r.Host)
		if (splitErr != nil || listenerPort == "" || requestPort != listenerPort || !allowed[strings.ToLower(host)]) && !proxyAuthority {
			http.Error(w, "unrecognized HTTP authority", http.StatusForbidden)
			return
		}
		if proxyAuthority {
			r = r.WithContext(context.WithValue(r.Context(), proxyAuthorityContextKey{}, true))
		}
		if !localRequestOriginAllowed(r) {
			http.Error(w, "unrecognized request origin", http.StatusForbidden)
			return
		}
		provided := r.Header.Get("X-Glade-Capability")
		hasCapability := capability != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(capability)) == 1
		clientHeaders := r.Header.Values("X-Glade-Client")
		explicitClient := len(clientHeaders) == 1 && clientHeaders[0] == "1"
		site := strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))
		// Salesforce CLI sends a bearer header but cannot send Glade-specific
		// headers. Browsers cannot add Authorization to a cross-origin request
		// without a preflight, and browser requests carry origin metadata.
		authorizations := r.Header.Values("Authorization")
		bearerClient := false
		issuedBearer := false
		if len(authorizations) > 1 {
			http.Error(w, "multiple authorization headers", http.StatusUnauthorized)
			return
		}
		if len(authorizations) == 1 {
			fields := strings.Fields(authorizations[0])
			if len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") {
				userID, ok := issuedTokenUser(fields[1])
				if !ok && strings.HasPrefix(fields[1], localIssuedTokenPrefix) {
					http.Error(w, "invalid or expired local token", http.StatusUnauthorized)
					return
				}
				if ok {
					issuedBearer = true
					r = r.WithContext(context.WithValue(r.Context(), issuedUserContextKey{}, userID))
					r = r.WithContext(context.WithValue(r.Context(), issuedTokenContextKey{}, fields[1]))
				}
				if site == "" && len(r.Header.Values("Origin")) == 0 {
					bearerClient = ok || !(exposed || proxyAuthority)
				}
			}
		}
		if strings.EqualFold(site, "none") && !hasCapability && !explicitClient && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; frame-ancestors 'none'")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write([]byte(`<!doctype html><title>Open local preview</title><p>Continue to the requested local page?</p><a href="">Continue</a>`))
			return
		}
		if (strings.EqualFold(site, "none") || (len(r.Header.Values("Origin")) == 0 && !strings.EqualFold(site, "same-origin"))) && !hasCapability && !explicitClient && !bearerClient {
			http.Error(w, "capability required for requests without origin metadata", http.StatusForbidden)
			return
		}
		parts := splitPath(r.URL.EscapedPath())
		// Token issuance on an exposed server needs the capability. The
		// X-Glade-Client marker is not enough there, or any remote client
		// could mint a token and bypass the capability on executeAnonymous.
		if (exposed || proxyAuthority) && len(parts) == 3 && parts[0] == "services" && parts[1] == "oauth2" && parts[2] == "token" && !hasCapability {
			http.Error(w, "capability required", http.StatusForbidden)
			return
		}
		anonymousREST := len(parts) == 5 && parts[0] == "services" && parts[1] == "data" && parts[3] == "tooling" && parts[4] == "executeAnonymous"
		anonymousSOAP := len(parts) == 5 && parts[0] == "services" && parts[1] == "Soap" && parts[2] == "s"
		if (anonymousREST || anonymousSOAP) && !hasCapability && !issuedBearer {
			http.Error(w, "capability required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// issuedTokens holds the access tokens this process handed out through the
// local token endpoint. executeAnonymous accepts a bearer only from this set.
// Entries are keyed by the SHA-256 of the token, so lookups are map reads that
// do not compare token bytes. The store is a bounded LRU: at maxIssuedTokens
// entries the least recently used token is evicted, never the whole set.
var issuedTokens = struct {
	sync.Mutex
	byHash map[[sha256.Size]byte]*list.Element
	order  *list.List // front = most recently used
}{byHash: map[[sha256.Size]byte]*list.Element{}, order: list.New()}

const maxIssuedTokens = 4096
const localIssuedTokenPrefix = "glade_"

type issuedUserContextKey struct{}
type issuedTokenContextKey struct{}

type issuedTokenEntry struct {
	hash   [sha256.Size]byte
	userID string
	owner  *Server
}

func hasIssuedTokens() bool {
	issuedTokens.Lock()
	defer issuedTokens.Unlock()
	return len(issuedTokens.byHash) != 0
}

func clearIssuedTokens() {
	issuedTokens.Lock()
	defer issuedTokens.Unlock()
	issuedTokens.byHash = make(map[[sha256.Size]byte]*list.Element)
	issuedTokens.order.Init()
}

// issueToken returns a new random URL-safe access token (32 bytes of
// crypto/rand) bound server-side to userID.
func issueToken(userID string) (string, error) {
	return issueTokenFor(nil, userID)
}

func issueTokenFor(owner *Server, userID string) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	token := localIssuedTokenPrefix + base64.RawURLEncoding.EncodeToString(raw[:])
	hash := sha256.Sum256([]byte(token))
	issuedTokens.Lock()
	defer issuedTokens.Unlock()
	for issuedTokens.order.Len() >= maxIssuedTokens {
		oldest := issuedTokens.order.Back()
		delete(issuedTokens.byHash, oldest.Value.(*issuedTokenEntry).hash)
		issuedTokens.order.Remove(oldest)
	}
	issuedTokens.byHash[hash] = issuedTokens.order.PushFront(&issuedTokenEntry{hash: hash, userID: userID, owner: owner})
	return token, nil
}

func issuedTokenUserFor(token string, owner *Server) (string, bool) {
	hash := sha256.Sum256([]byte(token))
	issuedTokens.Lock()
	defer issuedTokens.Unlock()
	el, ok := issuedTokens.byHash[hash]
	if !ok {
		return "", false
	}
	entry := el.Value.(*issuedTokenEntry)
	if entry.owner != nil && entry.owner != owner {
		return "", false
	}
	issuedTokens.order.MoveToFront(el)
	return entry.userID, true
}

func snapshotIssuedUsers(owner *Server, org *storage.OrgState) map[storage.ID]storage.Record {
	if org == nil {
		return nil
	}
	users := org.Objects["User"].Records
	before := make(map[storage.ID]storage.Record, len(users))
	for id, record := range users {
		before[id] = record.Clone()
	}
	return before
}

func revokeChangedUsers(owner *Server, before, after map[storage.ID]storage.Record) {
	issuedTokens.Lock()
	defer issuedTokens.Unlock()
	for hash, el := range issuedTokens.byHash {
		entry := el.Value.(*issuedTokenEntry)
		if entry.owner != nil && entry.owner != owner {
			continue
		}
		id := storage.ID(entry.userID)
		old, oldOK := before[id]
		current, currentOK := after[id]
		if oldOK != currentOK || !reflect.DeepEqual(old.Clone(), current.Clone()) {
			delete(issuedTokens.byHash, hash)
			issuedTokens.order.Remove(el)
		}
	}
}

func revokeFixtureUserTokens(owner *Server, fixture storage.Fixture) {
	ids := make(map[storage.ID]struct{})
	for _, object := range fixture.Objects {
		name, ok := storage.ResolveObjectName(*owner.Org, object.Name)
		if !ok || name != "User" {
			continue
		}
		for _, record := range object.Records {
			if record.ID == "" {
				revokeServerTokens(owner)
				return
			}
			ids[record.ID] = struct{}{}
		}
	}
	issuedTokens.Lock()
	defer issuedTokens.Unlock()
	for hash, el := range issuedTokens.byHash {
		entry := el.Value.(*issuedTokenEntry)
		if entry.owner != nil && entry.owner != owner {
			continue
		}
		if _, ok := ids[storage.ID(entry.userID)]; ok {
			delete(issuedTokens.byHash, hash)
			issuedTokens.order.Remove(el)
		}
	}
}

func revokeServerTokens(owner *Server) {
	issuedTokens.Lock()
	defer issuedTokens.Unlock()
	for hash, el := range issuedTokens.byHash {
		entry := el.Value.(*issuedTokenEntry)
		if entry.owner == nil || entry.owner == owner {
			delete(issuedTokens.byHash, hash)
			issuedTokens.order.Remove(el)
		}
	}
}

// issuedTokenUser reports the user bound to an issued token.
func issuedTokenUser(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	hash := sha256.Sum256([]byte(token))
	issuedTokens.Lock()
	defer issuedTokens.Unlock()
	el, ok := issuedTokens.byHash[hash]
	if !ok {
		return "", false
	}
	entry := el.Value.(*issuedTokenEntry)
	if subtle.ConstantTimeCompare(entry.hash[:], hash[:]) != 1 {
		return "", false
	}
	issuedTokens.order.MoveToFront(el)
	return entry.userID, true
}
