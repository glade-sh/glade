package playground

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const publicSessionCookie = "glade_playground_session"
const publicSessionLimit = 64
const publicSessionsPerClient = 8
const publicSessionIdle = 30 * time.Minute

type publicSession struct {
	server   *Server
	root     string
	lastUsed time.Time
	active   int
	clientIP string
}

type publicSessions struct {
	mu              sync.Mutex
	opts            ServerOptions
	items           map[string]*publicSession
	closed          bool
	creationLimiter *fixedWindowRateLimiter
}

func (p *publicSessions) serve(w http.ResponseWriter, r *http.Request, limiter *fixedWindowRateLimiter) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		writeError(w, http.StatusServiceUnavailable, "public playground is closed")
		return
	}
	now := time.Now()
	for id, session := range p.items {
		if session.active == 0 && now.Sub(session.lastUsed) > publicSessionIdle {
			delete(p.items, id)
			_ = os.RemoveAll(session.root)
		}
	}
	var id string
	if cookie, err := r.Cookie(publicSessionCookie); err == nil {
		id = cookie.Value
	}
	session := p.items[id]
	if session == nil {
		client := publicIngressClientIP(r, p.opts.TrustedProxy, p.opts.DigitalOceanIngress)
		if !p.creationLimiter.allow(client, now) {
			p.mu.Unlock()
			writeError(w, http.StatusTooManyRequests, "session creation rate limit exceeded")
			return
		}
		clientSessions := 0
		for _, existing := range p.items {
			if existing.clientIP == client {
				clientSessions++
			}
		}
		if clientSessions >= publicSessionsPerClient {
			p.mu.Unlock()
			writeError(w, http.StatusTooManyRequests, "public playground client session capacity reached")
			return
		}
		if len(p.items) >= publicSessionLimit {
			p.mu.Unlock()
			writeError(w, http.StatusServiceUnavailable, "public playground session capacity reached")
			return
		}
		var token [32]byte
		if _, err := rand.Read(token[:]); err != nil {
			p.mu.Unlock()
			writeError(w, http.StatusInternalServerError, "cannot create session")
			return
		}
		root, err := os.MkdirTemp("", "glade-playground-session-")
		if err != nil {
			p.mu.Unlock()
			writeError(w, http.StatusInternalServerError, "cannot create session workspace")
			return
		}
		ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: root})
		if err != nil {
			_ = os.RemoveAll(root)
			p.mu.Unlock()
			writeError(w, http.StatusInternalServerError, "cannot create session workspace")
			return
		}
		if err := installPublicConfig(ws); err != nil {
			_ = os.RemoveAll(root)
			p.mu.Unlock()
			writeError(w, http.StatusInternalServerError, "cannot initialize session configuration")
			return
		}
		opts := p.opts
		opts.DBPath = ""
		child := newServer(ws, opts)
		child.rateLimiter = limiter
		id = hex.EncodeToString(token[:])
		session = &publicSession{server: child, root: root, clientIP: client}
		p.items[id] = session
		cookie := &http.Cookie{Name: publicSessionCookie, Value: id, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
		if p.opts.InsecureLocalCookie && p.opts.TrustedProxy == "" && !p.opts.DigitalOceanIngress && r.TLS == nil && isLocalCookieHost(r.Host) {
			cookie.Secure = false
		}
		http.SetCookie(w, cookie)
	}
	session.active++
	session.lastUsed = now
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		session.active--
		session.lastUsed = time.Now()
		if p.closed && session.active == 0 {
			delete(p.items, id)
			_ = os.RemoveAll(session.root)
		}
		p.mu.Unlock()
	}()
	session.server.serveHTTP(w, r)
}

// Close prevents new sessions and removes idle session data. Active sessions
// are removed when their requests finish.
func (s *Server) Close() {
	if s == nil || s.sessions == nil {
		return
	}
	p := s.sessions
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	for id, session := range p.items {
		if session.active == 0 {
			delete(p.items, id)
			_ = os.RemoveAll(session.root)
		}
	}
}

// installPublicConfig prevents the project loader from inheriting host config.
func installPublicConfig(workspace *Workspace) error {
	root, err := workspace.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	if err := rejectSymlinkComponents(root, "glade.yml"); err != nil {
		return err
	}
	return root.WriteFile("glade.yml", []byte("# Managed by the public playground.\n"), 0600)
}

// isLocalCookieHost restricts the explicit local-HTTP exception to loopback
// URL hosts. It does not establish that the listener itself is private.
func isLocalCookieHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
