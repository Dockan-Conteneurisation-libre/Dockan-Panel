// Package api is the panel's JSON API and the server of the embedded Vue
// interface.
package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dockan-panel/internal/auth"
	"dockan-panel/internal/config"
	"dockan-panel/internal/dockan"
	"dockan-panel/internal/runner"
	"dockan-panel/internal/stacks"
	"dockan-panel/internal/store"
	"dockan-panel/internal/terminal"
)

const cookieName = "dockan_panel"

// Server holds the panel's dependencies.
type Server struct {
	Cfg      config.Config
	R        *runner.Runner
	D        dockan.Client
	Users    *auth.Users
	Sessions *auth.Sessions
	Rate     *auth.RateLimiter
	Stacks   stacks.Store
	Store    *store.Store
	Terms    *terminal.Manager
	UI       fs.FS

	guard guardState
}

type ctxKey int

// apiError is an error whose message is safe to show.
type apiError struct {
	status int
	msg    string
}

func (e apiError) Error() string { return e.msg }

func badRequest(msg string) error { return apiError{http.StatusBadRequest, msg} }

var errAuth = apiError{http.StatusUnauthorized, "sign-in required"}

// Handler returns the HTTP handler of the panel.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.routes(mux)
	mux.Handle("/", s.ui())
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; "+
			"script-src 'self'; connect-src 'self'; manifest-src 'self'; worker-src 'self'; base-uri 'none'; "+
			"frame-ancestors 'none'; form-action 'self'; object-src 'none'")
		if isHTTPS(r) {
			h.Set("Strict-Transport-Security", "max-age=15552000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// ui serves the single-page application; unknown paths fall back to
// index.html so client-side routes survive a reload.
func (s *Server) ui() http.Handler {
	files := http.FileServer(http.FS(s.UI))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" {
			if st, err := fs.Stat(s.UI, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		files.ServeHTTP(w, r2)
	})
}

// handler is an API endpoint: it returns a JSON-encodable value or an error.
type handler func(w http.ResponseWriter, r *http.Request, sess *auth.Session) (any, error)

// access levels of an endpoint.
const (
	public = iota // no session needed (session is created if missing)
	signed        // signed-in user
)

func (s *Server) api(level int, h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, err := s.session(w, r, level == public && r.Method == http.MethodGet)
		if err != nil {
			writeError(w, err)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if err := checkCSRF(r, sess); err != nil {
				writeError(w, err)
				return
			}
		}
		if level == signed {
			if sess.User == "" {
				writeError(w, errAuth)
				return
			}
			if _, err := s.Users.Find(sess.User); err != nil {
				s.Sessions.Delete(sess.Token)
				writeError(w, errAuth)
				return
			}
		}
		out, err := h(w, r, sess)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// session returns the browser's session. Only GET /api/session creates
// one: other requests without a live session are refused, so clients that
// never load the interface cannot fill the session table.
func (s *Server) session(w http.ResponseWriter, r *http.Request, create bool) (*auth.Session, error) {
	if c, err := r.Cookie(cookieName); err == nil {
		if sess := s.Sessions.Get(c.Value); sess != nil {
			return sess, nil
		}
	}
	if !create {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			return nil, errAuth
		}
		return nil, apiError{http.StatusForbidden, "invalid session token: reload the page"}
	}
	sess, err := s.Sessions.New()
	if errors.Is(err, auth.ErrTooManySessions) {
		return nil, apiError{http.StatusServiceUnavailable, err.Error()}
	}
	if err != nil {
		return nil, err
	}
	s.setCookie(w, r, sess.Token)
	return sess, nil
}

func (s *Server) setCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", HttpOnly: true,
		Secure: isHTTPS(r), SameSite: http.SameSiteStrictMode,
	})
}

// checkCSRF requires the session token in a header and, when the browser
// sends one, a same-origin Origin header.
func checkCSRF(r *http.Request, sess *auth.Session) error {
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host {
			return apiError{http.StatusForbidden, "cross-origin request refused"}
		}
	}
	tok := r.Header.Get("X-CSRF-Token")
	if tok == "" || !auth.SafeEqual(tok, sess.CSRF) {
		return apiError{http.StatusForbidden, "invalid session token: reload the page"}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	var ae apiError
	if errors.As(err, &ae) {
		writeJSON(w, ae.status, map[string]string{"error": ae.msg})
		return
	}
	// Command failures and validation errors carry messages meant for the
	// person; other errors are logged and summarised.
	var ce *runner.ErrCommand
	if errors.As(err, &ce) || isUserError(err) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("request error: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error: see the panel log"})
}

// isUserError recognises plain validation errors created with errors.New.
func isUserError(err error) bool {
	return errors.Unwrap(err) == nil
}

// decode reads a JSON body (at most 1 MiB).
func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		return badRequest("invalid request body")
	}
	return nil
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func rpID(r *http.Request) string {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return "localhost"
	}
	return host
}

// output is the result of an action.
type output struct {
	Output string `json:"output"`
}

func done(text string, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(text) == "" {
		text = "Done."
	}
	return output{Output: text}, nil
}

// NewHTTPServer builds the http.Server. Long actions (installs, updates)
// can last minutes, so there is no write timeout.
func NewHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr: addr, Handler: h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
	}
}
