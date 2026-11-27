package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// A hash produced by PHP password_hash(..., PASSWORD_DEFAULT) for
// "correct horse battery".
const phpHash = "$2y$12$C.X7OtegiaM3Cs64fdA63.AUN4inx5oYg0aSx7HNmQYG2em2AR2GO"

func TestCheckPasswordAcceptsPHPHashes(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	php := "$2y$" + hash[4:]
	if !CheckPassword(php, "correct horse battery") {
		t.Fatal("PHP $2y$ hash rejected")
	}
	if CheckPassword(php, "wrong password here") {
		t.Fatal("wrong password accepted")
	}
}

func TestUsersLifecycle(t *testing.T) {
	u := NewUsers(filepath.Join(t.TempDir(), "auth-users.json"))
	if err := u.Create("admin", "short", true); err == nil {
		t.Fatal("short password accepted")
	}
	if err := u.Create("admin", "a-long-password", true); err != nil {
		t.Fatal(err)
	}
	if err := u.Create("other", "a-long-password", true); err == nil {
		t.Fatal("second first-admin accepted")
	}
	if err := u.Create("other", "a-long-password", false); err != nil {
		t.Fatal(err)
	}
	if err := u.Delete("admin", "admin"); err == nil {
		t.Fatal("self deletion accepted")
	}
	if err := u.Delete("other", "admin"); err != nil {
		t.Fatal(err)
	}
	usr, err := u.Find("admin")
	if err != nil || !CheckPassword(usr.PasswordHash, "a-long-password") {
		t.Fatalf("find admin: %+v %v", usr, err)
	}
}

func TestReadsPHPUsersFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth-users.json")
	raw := `{"version":1,"users":[{"username":"admin","display_name":"admin","role":"admin","password_hash":"` + phpHash + `","totp_secret":"","passkeys":[],"created_at":"2026-08-01T10:00:00+00:00"}]}`
	if err := writeFile(path, raw); err != nil {
		t.Fatal(err)
	}
	usr, err := NewUsers(path).Find("admin")
	if err != nil || usr.Role != "admin" {
		t.Fatalf("find = %+v %v", usr, err)
	}
	if !CheckPassword(usr.PasswordHash, "correct horse battery") {
		t.Fatal("password hashed by PHP password_hash rejected")
	}
}

func TestTOTPKnownVector(t *testing.T) {
	// RFC 6238 SHA-1 test key "12345678901234567890" in base32.
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	code, err := TOTPCode(secret, uint64(59/30))
	if err != nil || code != "287082" {
		t.Fatalf("code = %s %v", code, err)
	}
	if !VerifyTOTP(secret, "287 082", time.Unix(59, 0)) {
		t.Fatal("valid code rejected")
	}
	if VerifyTOTP(secret, "287082", time.Unix(59+300, 0)) {
		t.Fatal("expired code accepted")
	}
}

func TestPasskeyAssertion(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pemKey, err := PublicKeyPEM(der)
	if err != nil {
		t.Fatal(err)
	}
	rp := sha256.Sum256([]byte("localhost"))
	authData := append(rp[:], 0x05, 0, 0, 0, 1)
	cd, _ := json.Marshal(map[string]string{"type": "webauthn.get", "challenge": "abc", "origin": "http://localhost:9090"})
	if err := CheckAuthenticatorData(authData, "localhost"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := CheckClientData(B64URL(cd), "webauthn.get", "abc", "localhost"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := CheckClientData(B64URL(cd), "webauthn.get", "abc", "evil.example"); err == nil {
		t.Fatal("foreign origin accepted")
	}
	cdHash := sha256.Sum256(cd)
	digest := sha256.Sum256(append(append([]byte{}, authData...), cdHash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyAssertion(pemKey, authData, cd, sig); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	sig[len(sig)-1] ^= 1
	if VerifyAssertion(pemKey, authData, cd, sig) == nil {
		t.Fatal("tampered signature accepted")
	}
}

func TestRateLimiter(t *testing.T) {
	r := NewRateLimiter()
	for i := 0; i < 8; i++ {
		if err := r.Attempt("1.2.3.4", "admin"); err != nil {
			t.Fatalf("locked out after %d attempts", i)
		}
	}
	if r.Attempt("1.2.3.4", "admin") != ErrTooMany {
		t.Fatal("not locked out after 8 attempts")
	}
	if r.Attempt("5.6.7.8", "admin") != nil {
		t.Fatal("other client locked out")
	}
	// Per-account cap across clients (password spraying).
	for i := 0; i < 40; i++ {
		_ = r.Attempt(fmt.Sprintf("10.0.0.%d", i), "victim")
	}
	if r.Attempt("10.9.9.9", "victim") != ErrTooMany {
		t.Fatal("no per-account cap")
	}
}

func TestRateLimiterIsAtomic(t *testing.T) {
	r := NewRateLimiter()
	var wg sync.WaitGroup
	var mu sync.Mutex
	passed := 0
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r.Attempt("1.2.3.4", "admin") == nil {
				mu.Lock()
				passed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if passed != 8 {
		t.Fatalf("%d parallel attempts passed, want 8", passed)
	}
}

func TestAnonymousSessionsAreCapped(t *testing.T) {
	s := NewSessions()
	s.MaxAnon = 3
	for i := 0; i < 3; i++ {
		if _, err := s.New(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.New(); err != ErrTooManySessions {
		t.Fatalf("4th anonymous session: %v", err)
	}
}

func TestTOTPMatchStep(t *testing.T) {
	secret, _ := NewTOTPSecret()
	now := time.Now()
	code, _ := TOTPCode(secret, uint64(now.Unix()/30))
	step, ok := MatchTOTP(secret, code, now)
	if !ok || step != uint64(now.Unix()/30) {
		t.Fatalf("MatchTOTP = %d, %v", step, ok)
	}
}

func writeFile(path, content string) error { return os.WriteFile(path, []byte(content), 0o600) }
