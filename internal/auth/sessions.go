package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Session is one signed-in (or signing-in) browser.
type Session struct {
	Token        string
	User         string
	CSRF         string
	PendingTOTP  string
	RegChallenge string
	LogChallenge string
	LogUser      string
	LastSeen     time.Time
}

// Sessions keeps sessions in memory: a restart signs everyone out.
// Anonymous sessions (sign-in pages) live briefly and are capped, so
// unauthenticated clients cannot fill the memory.
type Sessions struct {
	mu      sync.Mutex
	byID    map[string]*Session
	TTL     time.Duration // signed-in idle timeout
	AnonTTL time.Duration
	MaxAnon int
	lastGC  time.Time
}

// ErrTooManySessions is returned when the anonymous session cap is reached.
var ErrTooManySessions = errors.New("too many open sign-in pages: try again in a few minutes")

// NewSessions returns an empty session store.
func NewSessions() *Sessions {
	return &Sessions{byID: map[string]*Session{}, TTL: 12 * time.Hour, AnonTTL: 15 * time.Minute, MaxAnon: 2000}
}

// RandomToken returns n random bytes, hex-encoded.
func RandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// New creates an anonymous session.
func (s *Sessions) New() (*Session, error) {
	tok, err := RandomToken(32)
	if err != nil {
		return nil, err
	}
	csrf, err := RandomToken(24)
	if err != nil {
		return nil, err
	}
	sess := &Session{Token: tok, CSRF: csrf, LastSeen: time.Now()}
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.lastGC) > time.Minute {
		s.gcLocked()
	}
	if s.anonLocked() >= s.MaxAnon {
		s.gcLocked()
		if s.anonLocked() >= s.MaxAnon {
			return nil, ErrTooManySessions
		}
	}
	s.byID[tok] = sess
	return sess, nil
}

func (s *Sessions) anonLocked() int {
	n := 0
	for _, v := range s.byID {
		if v.User == "" {
			n++
		}
	}
	return n
}

func (s *Sessions) expiredLocked(v *Session) bool {
	ttl := s.TTL
	if v.User == "" {
		ttl = s.AnonTTL
	}
	return time.Since(v.LastSeen) > ttl
}

// Get returns a live session and refreshes its idle timer.
func (s *Sessions) Get(tok string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.byID[tok]
	if !ok {
		return nil
	}
	if s.expiredLocked(sess) {
		delete(s.byID, tok)
		return nil
	}
	sess.LastSeen = time.Now()
	return sess
}

// With runs fn on a session under the store lock.
func (s *Sessions) With(sess *Session, fn func(*Session)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(sess)
}

// Delete ends a session.
func (s *Sessions) Delete(tok string) {
	s.mu.Lock()
	delete(s.byID, tok)
	s.mu.Unlock()
}

// DeleteUser ends every session of a user except the one whose token is
// keep (empty: all of them).
func (s *Sessions) DeleteUser(user, keep string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.byID {
		if v.User == user && k != keep {
			delete(s.byID, k)
		}
	}
}

func (s *Sessions) gcLocked() {
	s.lastGC = time.Now()
	for k, v := range s.byID {
		if s.expiredLocked(v) {
			delete(s.byID, k)
		}
	}
}

// RateLimiter limits sign-in attempts in memory: 8 per client and username
// and 30 per username from any client, in 15 minutes. An attempt is counted
// before the password is checked, so parallel requests cannot pass the limit.
type RateLimiter struct {
	mu                        sync.Mutex
	pairs                     map[string]*rateRow
	users                     map[string]*rateRow
	Window                    time.Duration
	PerPair, PerUser, MaxRows int
}

type rateRow struct {
	first time.Time
	count int
}

// NewRateLimiter returns an empty limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{pairs: map[string]*rateRow{}, users: map[string]*rateRow{},
		Window: 15 * time.Minute, PerPair: 8, PerUser: 30, MaxRows: 20000}
}

// ErrTooMany is returned while a client is locked out.
var ErrTooMany = errors.New("too many sign-in attempts: try again in a few minutes")

func rateKey(ip, user string) string {
	sum := sha256.Sum256([]byte(ip + "|" + user))
	return hex.EncodeToString(sum[:])
}

func (r *RateLimiter) row(m map[string]*rateRow, k string, now time.Time) *rateRow {
	row := m[k]
	if row == nil || now.Sub(row.first) > r.Window {
		row = &rateRow{first: now}
		m[k] = row
	}
	return row
}

func (r *RateLimiter) pruneLocked(now time.Time) {
	for _, m := range []map[string]*rateRow{r.pairs, r.users} {
		for k, v := range m {
			if now.Sub(v.first) > r.Window {
				delete(m, k)
			}
		}
	}
}

// Attempt reserves one sign-in attempt, or fails while locked out.
func (r *RateLimiter) Attempt(ip, user string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if len(r.pairs)+len(r.users) > r.MaxRows {
		r.pruneLocked(now)
		if len(r.pairs)+len(r.users) > r.MaxRows {
			return ErrTooMany
		}
	}
	pair, per := r.row(r.pairs, rateKey(ip, user), now), r.row(r.users, user, now)
	if pair.count >= r.PerPair || per.count >= r.PerUser {
		return ErrTooMany
	}
	pair.count++
	per.count++
	return nil
}

// Clear forgets the attempts of a client after a successful sign-in.
func (r *RateLimiter) Clear(ip, user string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.pairs, rateKey(ip, user))
	if row := r.users[user]; row != nil && row.count > 0 {
		row.count--
	}
}

// SafeEqual compares two secrets in constant time.
func SafeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
