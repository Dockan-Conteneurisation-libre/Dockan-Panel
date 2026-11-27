package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const base32Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

// NewTOTPSecret returns a 32-character base32 secret (160 bits).
func NewTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("totp secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}

// TOTPCode returns the RFC 6238 code (SHA-1, 6 digits, 30 s) for a counter.
func TOTPCode(secret string, counter uint64) (string, error) {
	clean := strings.Map(func(r rune) rune {
		r = toUpper(r)
		if strings.ContainsRune(base32Alphabet, r) {
			return r
		}
		return -1
	}, secret)
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(clean[:len(clean)/8*8])
	if err != nil {
		return "", fmt.Errorf("totp secret: %w", err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[19] & 0x0f
	v := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", v%1000000), nil
}

func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

// VerifyTOTP accepts the code of the current period and its neighbours.
func VerifyTOTP(secret, code string, now time.Time) bool {
	_, ok := MatchTOTP(secret, code, now)
	return ok
}

// MatchTOTP returns the time step a valid code belongs to, so callers can
// refuse a code that was already used.
func MatchTOTP(secret, code string, now time.Time) (uint64, bool) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, code)
	if len(digits) != 6 {
		return 0, false
	}
	step := uint64(now.Unix() / 30)
	var match uint64
	ok := false
	for _, c := range []uint64{step - 1, step, step + 1} {
		want, err := TOTPCode(secret, c)
		if err == nil && subtle.ConstantTimeCompare([]byte(want), []byte(digits)) == 1 {
			match, ok = c, true
		}
	}
	return match, ok
}

// TOTPURI is the otpauth:// URI for authenticator apps.
func TOTPURI(issuer, user, secret string) string {
	return "otpauth://totp/" + url.PathEscape(issuer+":"+user) +
		"?secret=" + url.QueryEscape(secret) + "&issuer=" + url.QueryEscape(issuer) +
		"&algorithm=SHA1&digits=6&period=30"
}
