package auth

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
)

// B64URL encodes without padding, as WebAuthn uses.
func B64URL(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// FromB64URL decodes base64url with or without padding.
func FromB64URL(s string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(trimPad(s))
	if err != nil {
		return nil, errors.New("invalid base64url value")
	}
	return b, nil
}

func trimPad(s string) string {
	for len(s) > 0 && s[len(s)-1] == '=' {
		s = s[:len(s)-1]
	}
	return s
}

type clientData struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
}

// CheckClientData verifies the type, challenge and origin host of a
// WebAuthn clientDataJSON. It returns the decoded JSON bytes and the origin.
func CheckClientData(encoded, wantType, wantChallenge, host string) ([]byte, string, error) {
	if wantChallenge == "" {
		return nil, "", errors.New("missing passkey challenge: start again")
	}
	raw, err := FromB64URL(encoded)
	if err != nil {
		return nil, "", err
	}
	var cd clientData
	if err := json.Unmarshal(raw, &cd); err != nil {
		return nil, "", errors.New("invalid passkey client data")
	}
	if cd.Type != wantType {
		return nil, "", errors.New("invalid passkey response type")
	}
	if subtle.ConstantTimeCompare([]byte(cd.Challenge), []byte(wantChallenge)) != 1 {
		return nil, "", errors.New("invalid passkey challenge")
	}
	u, err := url.Parse(cd.Origin)
	if err != nil || u.Hostname() == "" || u.Hostname() != host {
		return nil, "", errors.New("invalid passkey origin")
	}
	return raw, cd.Origin, nil
}

// CheckAuthenticatorData verifies the relying party hash and the user
// presence and verification flags.
func CheckAuthenticatorData(data []byte, rpID string) error {
	if len(data) < 37 {
		return errors.New("invalid passkey authenticator data")
	}
	want := sha256.Sum256([]byte(rpID))
	if !bytes.Equal(data[:32], want[:]) {
		return errors.New("invalid passkey relying party")
	}
	flags := data[32]
	if flags&0x01 == 0 {
		return errors.New("passkey user presence was not confirmed")
	}
	if flags&0x04 == 0 {
		return errors.New("passkey user verification is required")
	}
	return nil
}

// PublicKeyPEM checks a DER SubjectPublicKeyInfo (from getPublicKey()) and
// returns it as PEM.
func PublicKeyPEM(der []byte) (string, error) {
	key, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return "", errors.New("invalid passkey public key")
	}
	switch key.(type) {
	case *ecdsa.PublicKey, *rsa.PublicKey:
	default:
		return "", errors.New("unsupported passkey key type")
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}

// VerifyAssertion checks the signature over authenticatorData ||
// SHA-256(clientDataJSON) with a stored PEM key (ES256 or RS256).
func VerifyAssertion(pemKey string, authData, clientDataJSON, sig []byte) error {
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return errors.New("stored passkey key is unreadable")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("stored passkey key: %w", err)
	}
	cdHash := sha256.Sum256(clientDataJSON)
	signed := append(append([]byte{}, authData...), cdHash[:]...)
	digest := sha256.Sum256(signed)
	switch k := key.(type) {
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(k, digest[:], sig) {
			return errors.New("invalid passkey signature")
		}
	case *rsa.PublicKey:
		if rsa.VerifyPKCS1v15(k, crypto.SHA256, digest[:], sig) != nil {
			return errors.New("invalid passkey signature")
		}
	default:
		return errors.New("unsupported passkey key type")
	}
	return nil
}

// NewChallenge returns a base64url WebAuthn challenge of 32 random bytes,
// the encoding the browser echoes back in clientDataJSON.
func NewChallenge() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return B64URL(b), nil
}
