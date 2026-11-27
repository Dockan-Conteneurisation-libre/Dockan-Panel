package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"dockan-panel/internal/auth"
	"dockan-panel/internal/config"
	"dockan-panel/internal/dockan"
	"dockan-panel/internal/runner"
	"dockan-panel/internal/stacks"
	"dockan-panel/internal/store"
	"dockan-panel/internal/terminal"
)

// phpHash is a bcrypt hash made by PHP password_hash() for "correct horse battery".
const phpHash = "$2y$12$C.X7OtegiaM3Cs64fdA63.AUN4inx5oYg0aSx7HNmQYG2em2AR2GO"

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{Dockan: "/bin/false", Storage: dir, HomeDir: dir}
	if err := cfg.EnsureStorage(); err != nil {
		t.Fatal(err)
	}
	r := &runner.Runner{Bin: "/bin/false"}
	s := &Server{
		Cfg: cfg, R: r, D: dockan.Client{R: r},
		Users: auth.NewUsers(cfg.AuthFile()), Sessions: auth.NewSessions(), Rate: auth.NewRateLimiter(),
		Stacks: stacks.Store{Dir: cfg.StacksDir()},
		Store:  &store.Store{R: r, Dir: cfg.StoreDir(), RootDir: cfg.StoreRoot(), StateFile: cfg.ImageStateFile(), Home: dir},
		Terms:  terminal.NewManager(),
		UI:     fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>panel</title>")}},
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts
}

type client struct {
	t    *testing.T
	base string
	http *http.Client
	csrf string
}

func newClient(t *testing.T, base string) *client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &client{t: t, base: base, http: &http.Client{Jar: jar}}
	var s struct{ CSRF string }
	c.do("GET", "/api/session", "", nil, &s)
	c.csrf = s.CSRF
	return c
}

func (c *client) do(method, path, body string, header map[string]string, out any) int {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if method != "GET" && c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			c.t.Fatalf("%s %s: decode: %v", method, path, err)
		}
	}
	return res.StatusCode
}

// writePHPAuthFile writes an auth file in the shape the PHP panel left.
func writePHPAuthFile(t *testing.T, s *Server) {
	t.Helper()
	data := `{"users":[{"username":"admin","password_hash":"` + phpHash + `","role":"admin","passkeys":[]}]}`
	if err := os.WriteFile(s.Cfg.AuthFile(), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func signIn(t *testing.T, s *Server, base string) *client {
	t.Helper()
	writePHPAuthFile(t, s)
	c := newClient(t, base)
	var sess struct {
		CSRF string
		User *struct{ Username string }
	}
	if code := c.do("POST", "/api/login", `{"username":"admin","password":"correct horse battery"}`, nil, &sess); code != 200 {
		t.Fatalf("login with the PHP hash: status %d", code)
	}
	if sess.User == nil || sess.User.Username != "admin" {
		t.Fatalf("login did not sign in: %+v", sess)
	}
	c.csrf = sess.CSRF
	return c
}

func TestSignedRoutesNeedASession(t *testing.T) {
	_, ts := newTestServer(t)
	c := newClient(t, ts.URL)
	for _, p := range []string{"/api/containers", "/api/images", "/api/volumes", "/api/system", "/api/users", "/api/store"} {
		if code := c.do("GET", p, "", nil, nil); code != http.StatusUnauthorized {
			t.Errorf("GET %s without sign-in: status %d, want 401", p, code)
		}
	}
	if code := c.do("POST", "/api/containers/run", `{"name":"x","image":"alpine"}`, nil, nil); code != http.StatusUnauthorized {
		t.Errorf("POST run without sign-in: status %d, want 401", code)
	}
}

func TestWritesNeedTheCSRFToken(t *testing.T) {
	s, ts := newTestServer(t)
	c := signIn(t, s, ts.URL)
	good := c.csrf
	c.csrf = ""
	if code := c.do("POST", "/api/volumes/create", `{"name":"data"}`, nil, nil); code != http.StatusForbidden {
		t.Errorf("without token: status %d, want 403", code)
	}
	c.csrf = "wrong"
	if code := c.do("POST", "/api/volumes/create", `{"name":"data"}`, nil, nil); code != http.StatusForbidden {
		t.Errorf("with a wrong token: status %d, want 403", code)
	}
	c.csrf = good
	if code := c.do("POST", "/api/logout", "", map[string]string{"Origin": "http://evil.example"}, nil); code != http.StatusForbidden {
		t.Errorf("cross-origin: status %d, want 403", code)
	}
}

func TestLoginRotatesTheSessionAndCountsFailures(t *testing.T) {
	s, ts := newTestServer(t)
	writePHPAuthFile(t, s)
	c := newClient(t, ts.URL)
	before := c.csrf
	if code := c.do("POST", "/api/login", `{"username":"admin","password":"wrong password"}`, nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("bad password: status %d, want 401", code)
	}
	var sess struct{ CSRF string }
	if code := c.do("POST", "/api/login", `{"username":"admin","password":"correct horse battery"}`, nil, &sess); code != 200 {
		t.Fatalf("good password: status %d", code)
	}
	if sess.CSRF == "" || sess.CSRF == before {
		t.Error("the session was not rotated at sign-in")
	}
	for i := 0; i < 8; i++ {
		c.do("POST", "/api/login", `{"username":"admin","password":"wrong password"}`, map[string]string{"X-CSRF-Token": sess.CSRF}, nil)
	}
	c.csrf = sess.CSRF
	if code := c.do("POST", "/api/login", `{"username":"admin","password":"correct horse battery"}`, nil, nil); code != http.StatusTooManyRequests {
		t.Errorf("after 8 failures: status %d, want 429", code)
	}
}

func TestSetupOnlyWhileNoAccountExists(t *testing.T) {
	s, ts := newTestServer(t)
	writePHPAuthFile(t, s)
	c := newClient(t, ts.URL)
	body := `{"username":"intruder","password":"0123456789ab","confirm":"0123456789ab"}`
	if code := c.do("POST", "/api/setup", body, nil, nil); code == 200 {
		t.Fatal("setup accepted although an account exists")
	}
}

func TestBackupFileStaysInTheBackupFolder(t *testing.T) {
	s, _ := newTestServer(t)
	outside := filepath.Join(t.TempDir(), "secret.tar.gz")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(s.Cfg.BackupDir(), "link.tar.gz")); err != nil {
		t.Fatal(err)
	}
	ok := filepath.Join(s.Cfg.BackupDir(), "data-20260101-000000.tar.gz")
	if err := os.WriteFile(ok, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../../etc/passwd", "link.tar.gz", outside, "", "missing.tar.gz"} {
		if _, err := s.backupFile(name); err == nil {
			t.Errorf("backupFile(%q) accepted", name)
		}
	}
	if got, err := s.backupFile(filepath.Base(ok)); err != nil || filepath.Base(got) != filepath.Base(ok) {
		t.Errorf("backupFile(valid) = %q, %v", got, err)
	}
}

func TestInputValidation(t *testing.T) {
	s, ts := newTestServer(t)
	c := signIn(t, s, ts.URL)
	cases := []struct{ path, body string }{
		{"/api/containers/run", `{"name":"x","image":"alpine","isolation":"weird"}`},
		{"/api/containers/run", `{"name":"Bad Name!","image":"alpine"}`},
		{"/api/containers/run", `{"name":"x","image":"alpine","network":"--privileged"}`},
		{"/api/containers/run", `{"name":"x","image":"alpine","ports":"8080:80\n--privileged"}`},
		{"/api/containers/run", `{"name":"x","image":"alpine","volumes":"-v=/:/host"}`},
		{"/api/stacks/save", `{"name":"web","yaml":"a: b","requiredImages":"--all","registryDir":""}`},
		{"/api/stacks/save", `{"name":"web","yaml":"a: b","requiredImages":"alpine","registryDir":"relative/dir"}`},
		{"/api/containers/run", `{"name":"x","image":"alpine","command":"\"unclosed"}`},
		{"/api/compose", `{"file":"relative/dockan.yml","action":"up"}`},
		{"/api/compose", `{"file":"/etc/passwd","action":"rm"}`},
		{"/api/images/pull", `{"ref":"--help"}`},
		{"/api/stacks/save", `{"name":"../x","yaml":"a: b"}`},
		{"/api/system/deps", `{"kind":"profile","value":"core; rm -rf /","mode":"preview"}`},
		{"/api/system/deps", `{"kind":"packages","value":"ok","mode":"root-now"}`},
	}
	for _, tc := range cases {
		if code := c.do("POST", tc.path, tc.body, nil, nil); code < 400 || code >= 500 {
			t.Errorf("POST %s %s: status %d, want 4xx", tc.path, tc.body, code)
		}
	}
}

func TestSPAFallbackAndMethods(t *testing.T) {
	_, ts := newTestServer(t)
	res, err := http.Get(ts.URL + "/some/client/route")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Security-Policy"), "script-src 'self'") {
		t.Errorf("fallback: status %d, CSP %q", res.StatusCode, res.Header.Get("Content-Security-Policy"))
	}
	res, err = http.Post(ts.URL+"/index.html", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST on the interface: status %d, want 405", res.StatusCode)
	}
}

func TestSetupNeedsTheSetupCode(t *testing.T) {
	s, ts := newTestServer(t)
	c := newClient(t, ts.URL) // GET /api/session creates the code
	raw, err := os.ReadFile(s.setupTokenFile())
	if err != nil {
		t.Fatalf("setup code file: %v", err)
	}
	code := strings.TrimSpace(string(raw))
	body := `{"username":"admin","password":"0123456789ab","confirm":"0123456789ab","token":"%s"}`
	if got := c.do("POST", "/api/setup", strings.Replace(body, "%s", "wrong", 1), nil, nil); got != http.StatusForbidden {
		t.Fatalf("setup with a wrong code: status %d, want 403", got)
	}
	if got := c.do("POST", "/api/setup", strings.Replace(body, "%s", code, 1), nil, nil); got != 200 {
		t.Fatalf("setup with the code: status %d", got)
	}
	if _, err := os.Stat(s.setupTokenFile()); !os.IsNotExist(err) {
		t.Error("the setup code file was not removed")
	}
	if s.checkSetupToken(code) {
		t.Error("the setup code is still valid after setup")
	}
}

func TestNoSessionWithoutTheInterface(t *testing.T) {
	s, ts := newTestServer(t)
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"POST", "/api/login", http.StatusForbidden},
		{"GET", "/api/containers", http.StatusUnauthorized},
	} {
		req, _ := http.NewRequest(tc.method, ts.URL+tc.path, strings.NewReader("{}"))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != tc.want || res.Header.Get("Set-Cookie") != "" {
			t.Errorf("%s %s: status %d, cookie %q", tc.method, tc.path, res.StatusCode, res.Header.Get("Set-Cookie"))
		}
	}
	s.Sessions.MaxAnon = 0
	if code := newClientRaw(ts.URL); code != http.StatusServiceUnavailable {
		t.Errorf("session cap: status %d, want 503", code)
	}
}

func newClientRaw(base string) int {
	res, err := http.Get(base + "/api/session")
	if err != nil {
		return 0
	}
	res.Body.Close()
	return res.StatusCode
}

func TestPasswordChangeClosesOtherSessions(t *testing.T) {
	s, ts := newTestServer(t)
	a := signIn(t, s, ts.URL)
	b := signIn(t, s, ts.URL)
	if code := a.do("POST", "/api/account/password", `{"username":"admin","password":"a new password 1"}`, nil, nil); code != 200 {
		t.Fatalf("password change: status %d", code)
	}
	if code := b.do("GET", "/api/users", "", nil, nil); code != http.StatusUnauthorized {
		t.Errorf("other session after a password change: status %d, want 401", code)
	}
	if code := a.do("GET", "/api/users", "", nil, nil); code != 200 {
		t.Errorf("current session after a password change: status %d", code)
	}
}

func TestWeakeningSecurityNeedsThePassword(t *testing.T) {
	s, ts := newTestServer(t)
	c := signIn(t, s, ts.URL)
	if code := c.do("POST", "/api/account/totp-disable", `{}`, nil, nil); code != http.StatusForbidden {
		t.Errorf("totp-disable without password: status %d, want 403", code)
	}
	if code := c.do("POST", "/api/account/totp-disable", `{"password":"correct horse battery"}`, nil, nil); code != 200 {
		t.Errorf("totp-disable with password: status %d", code)
	}
}

func TestTOTPCodeIsSingleUse(t *testing.T) {
	s, _ := newTestServer(t)
	secret, _ := auth.NewTOTPSecret()
	code, _ := auth.TOTPCode(secret, uint64(time.Now().Unix()/30))
	if !s.useTOTP("admin", secret, code) {
		t.Fatal("valid code refused")
	}
	if s.useTOTP("admin", secret, code) {
		t.Fatal("the same code was accepted twice")
	}
}

func TestInternalErrorsAreNotShown(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, fmt.Errorf("read users: %w", os.ErrPermission))
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "read users") {
		t.Errorf("500 body leaks the cause: %s", rec.Body.String())
	}
}
