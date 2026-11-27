// Package auth manages panel accounts (stored in auth-users.json, the same
// file and format as the previous PHP panel), sessions, login rate limits,
// TOTP second factors and passkeys.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Passkey is a registered WebAuthn credential. PublicKey is a PEM-encoded
// SubjectPublicKeyInfo, as the PHP panel stored it.
type Passkey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
	Origin    string `json:"origin,omitempty"`
	CreatedAt string `json:"created_at"`
}

// User is one panel account.
type User struct {
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	Role         string    `json:"role"`
	PasswordHash string    `json:"password_hash"`
	TOTPSecret   string    `json:"totp_secret"`
	Passkeys     []Passkey `json:"passkeys"`
	CreatedAt    string    `json:"created_at"`
}

type usersFile struct {
	Version int    `json:"version"`
	Users   []User `json:"users"`
}

// Users is the account store.
type Users struct {
	path string
	mu   sync.Mutex
}

// NewUsers opens the account store at path.
func NewUsers(path string) *Users { return &Users{path: path} }

var usernameRe = regexp.MustCompile(`^[a-z0-9_.-]{2,32}$`)

// Errors shown to the person signing in or managing accounts.
var (
	ErrBadCredentials = errors.New("invalid username or password")
	ErrBadTOTP        = errors.New("invalid two-factor code")
	ErrNotFound       = errors.New("user not found")
)

// CleanUsername normalises and validates a username.
func CleanUsername(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if !usernameRe.MatchString(s) {
		return "", errors.New("invalid username: use 2 to 32 lowercase letters, digits, dot, dash or underscore")
	}
	return s, nil
}

// ValidatePassword enforces the minimum length.
func ValidatePassword(p string) error {
	if len(p) < 10 {
		return errors.New("the password must contain at least 10 characters")
	}
	if len(p) > 72 {
		return errors.New("the password must contain at most 72 bytes")
	}
	return nil
}

// HashPassword returns a bcrypt hash, readable by the PHP panel too.
func HashPassword(p string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(p), 12)
	if err != nil {
		return "", fmt.Errorf("password hash: %w", err)
	}
	return string(h), nil
}

// CheckPassword compares a password with a stored bcrypt hash, including
// PHP's "$2y$" variant.
func CheckPassword(hash, password string) bool {
	if strings.HasPrefix(hash, "$2y$") {
		hash = "$2a$" + hash[4:]
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (u *Users) load() ([]User, error) {
	data, err := os.ReadFile(u.path)
	if errors.Is(err, os.ErrNotExist) {
		return []User{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read users: %w", err)
	}
	var f usersFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse users: %w", err)
	}
	for i := range f.Users {
		if f.Users[i].Passkeys == nil {
			f.Users[i].Passkeys = []Passkey{}
		}
	}
	return f.Users, nil
}

func (u *Users) save(users []User) error {
	data, err := json.MarshalIndent(usersFile{Version: 1, Users: users}, "", "    ")
	if err != nil {
		return err
	}
	tmp := u.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("save users: %w", err)
	}
	return os.Rename(tmp, u.path)
}

// All returns every account.
func (u *Users) All() ([]User, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.load()
}

// Any reports whether at least one account exists.
func (u *Users) Any() (bool, error) {
	users, err := u.All()
	return len(users) > 0, err
}

// Find returns one account.
func (u *Users) Find(name string) (User, error) {
	users, err := u.All()
	if err != nil {
		return User{}, err
	}
	for _, x := range users {
		if x.Username == name {
			return x, nil
		}
	}
	return User{}, ErrNotFound
}

// Update applies fn to one account and saves.
func (u *Users) Update(name string, fn func(*User) error) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	users, err := u.load()
	if err != nil {
		return err
	}
	for i := range users {
		if users[i].Username == name {
			if err := fn(&users[i]); err != nil {
				return err
			}
			return u.save(users)
		}
	}
	return ErrNotFound
}

// Create adds an admin account. firstOnly refuses when accounts exist.
func (u *Users) Create(name, password string, firstOnly bool) error {
	name, err := CleanUsername(name)
	if err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	users, err := u.load()
	if err != nil {
		return err
	}
	if firstOnly && len(users) > 0 {
		return errors.New("setup is already complete")
	}
	for _, x := range users {
		if x.Username == name {
			return errors.New("this user already exists")
		}
	}
	users = append(users, User{
		Username: name, DisplayName: name, Role: "admin", PasswordHash: hash,
		Passkeys: []Passkey{}, CreatedAt: time.Now().Format(time.RFC3339),
	})
	return u.save(users)
}

// Delete removes an account, keeping at least one.
func (u *Users) Delete(name, self string) error {
	if name == self {
		return errors.New("you cannot delete your own account")
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	users, err := u.load()
	if err != nil {
		return err
	}
	kept := users[:0]
	for _, x := range users {
		if x.Username != name {
			kept = append(kept, x)
		}
	}
	if len(kept) == len(users) {
		return ErrNotFound
	}
	if len(kept) == 0 {
		return errors.New("at least one account is required")
	}
	return u.save(kept)
}

// SetPassword replaces an account's password.
func (u *Users) SetPassword(name, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return u.Update(name, func(x *User) error { x.PasswordHash = hash; return nil })
}
