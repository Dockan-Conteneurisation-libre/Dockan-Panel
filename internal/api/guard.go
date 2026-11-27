package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"dockan-panel/internal/auth"
)

// errSignIn is the single sign-in failure message: it does not tell whether
// the account, the password or the two-factor code was wrong.
const errSignIn = "invalid username, password or two-factor code"

// guardState holds the in-memory secrets of the sign-in flow.
type guardState struct {
	mu         sync.Mutex
	setupToken string
	totpSteps  map[string]uint64 // last accepted TOTP step per user (replay protection)
}

var (
	dummyOnce sync.Once
	dummy     string
)

// dummyHash is a bcrypt hash of a random password, checked when the account
// does not exist so that both cases take the same time.
func dummyHash() string {
	dummyOnce.Do(func() {
		pw, err := auth.RandomToken(16)
		if err == nil {
			dummy, err = auth.HashPassword(pw)
		}
		if err != nil {
			log.Printf("dummy hash: %v", err)
		}
	})
	return dummy
}

func (s *Server) setupTokenFile() string { return filepath.Join(s.Cfg.Storage, "setup-token") }

// ensureSetupToken creates the one-time setup code while no account exists.
// The first account can only be created with it, so a fresh panel reachable
// from the network (or through DNS rebinding) cannot be taken over.
func (s *Server) ensureSetupToken() error {
	s.guard.mu.Lock()
	defer s.guard.mu.Unlock()
	if s.guard.setupToken != "" {
		return nil
	}
	tok, err := auth.RandomToken(8)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.setupTokenFile(), []byte(tok+"\n"), 0o600); err != nil {
		return fmt.Errorf("write the setup code: %w", err)
	}
	s.guard.setupToken = tok
	log.Printf("no account yet: create the first one with the setup code %s (also in %s)", tok, s.setupTokenFile())
	return nil
}

func (s *Server) checkSetupToken(tok string) bool {
	s.guard.mu.Lock()
	defer s.guard.mu.Unlock()
	return s.guard.setupToken != "" && auth.SafeEqual(tok, s.guard.setupToken)
}

func (s *Server) clearSetupToken() {
	s.guard.mu.Lock()
	s.guard.setupToken = ""
	s.guard.mu.Unlock()
	if err := os.Remove(s.setupTokenFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("remove the setup code: %v", err)
	}
}

// useTOTP accepts a valid code once: the same or an older step is refused.
func (s *Server) useTOTP(user, secret, code string) bool {
	step, ok := auth.MatchTOTP(secret, code, time.Now())
	if !ok {
		return false
	}
	s.guard.mu.Lock()
	defer s.guard.mu.Unlock()
	if s.guard.totpSteps == nil {
		s.guard.totpSteps = map[string]uint64{}
	}
	if last, seen := s.guard.totpSteps[user]; seen && step <= last {
		return false
	}
	s.guard.totpSteps[user] = step
	return true
}

// reauth asks for the password again before a security setting is weakened.
func (s *Server) reauth(r *http.Request, user, password string) error {
	if err := s.Rate.Attempt(clientIP(r), user); err != nil {
		return apiError{http.StatusTooManyRequests, err.Error()}
	}
	u, err := s.Users.Find(user)
	if err != nil || !auth.CheckPassword(u.PasswordHash, password) {
		return apiError{http.StatusForbidden, "enter your current password to confirm"}
	}
	s.Rate.Clear(clientIP(r), user)
	return nil
}
