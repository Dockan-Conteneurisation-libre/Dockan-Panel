package api

import (
	"errors"
	"net/http"
	"time"

	"dockan-panel/internal/auth"
	"dockan-panel/internal/config"
)

type sessionInfo struct {
	Setup   bool      `json:"setup"` // true while no account exists
	CSRF    string    `json:"csrf"`
	Version string    `json:"version"`
	User    *userInfo `json:"user"`
}

type userInfo struct {
	Username string        `json:"username"`
	Role     string        `json:"role"`
	TOTP     bool          `json:"totp"`
	Passkeys []passkeyInfo `json:"passkeys"`
}

type passkeyInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

func toUserInfo(u auth.User) *userInfo {
	keys := make([]passkeyInfo, 0, len(u.Passkeys))
	for _, k := range u.Passkeys {
		keys = append(keys, passkeyInfo{ID: k.ID, Name: k.Name, CreatedAt: k.CreatedAt})
	}
	return &userInfo{Username: u.Username, Role: u.Role, TOTP: u.TOTPSecret != "", Passkeys: keys}
}

func (s *Server) getSession(_ http.ResponseWriter, _ *http.Request, sess *auth.Session) (any, error) {
	any, err := s.Users.Any()
	if err != nil {
		return nil, err
	}
	info := sessionInfo{Setup: !any, CSRF: sess.CSRF, Version: config.Version}
	if !any {
		if err := s.ensureSetupToken(); err != nil {
			return nil, err
		}
	}
	var user string
	s.Sessions.With(sess, func(x *auth.Session) { user = x.User })
	if user != "" {
		if u, err := s.Users.Find(user); err == nil {
			info.User = toUserInfo(u)
		}
	}
	return info, nil
}

// signIn rotates the session (fixation protection) and attaches the user.
func (s *Server) signIn(w http.ResponseWriter, r *http.Request, old *auth.Session, user string) (any, error) {
	s.Sessions.Delete(old.Token)
	sess, err := s.Sessions.New()
	if err != nil {
		return nil, err
	}
	s.Sessions.With(sess, func(x *auth.Session) { x.User = user })
	s.setCookie(w, r, sess.Token)
	return s.getSession(w, r, sess)
}

func (s *Server) postSetup(w http.ResponseWriter, r *http.Request, sess *auth.Session) (any, error) {
	var body struct{ Username, Password, Confirm, Token string }
	if err := decode(r, &body); err != nil {
		return nil, err
	}
	if err := s.Rate.Attempt(clientIP(r), "setup"); err != nil {
		return nil, apiError{http.StatusTooManyRequests, err.Error()}
	}
	if !s.checkSetupToken(body.Token) {
		return nil, apiError{http.StatusForbidden, "invalid setup code: read it in the panel log or in storage/setup-token"}
	}
	if body.Password != body.Confirm {
		return nil, badRequest("the passwords do not match")
	}
	if err := s.Users.Create(body.Username, body.Password, true); err != nil {
		return nil, err
	}
	s.clearSetupToken()
	name, _ := auth.CleanUsername(body.Username)
	return s.signIn(w, r, sess, name)
}

func (s *Server) postLogin(w http.ResponseWriter, r *http.Request, sess *auth.Session) (any, error) {
	var body struct{ Username, Password, TOTP string }
	if err := decode(r, &body); err != nil {
		return nil, err
	}
	name, err := auth.CleanUsername(body.Username)
	if err != nil {
		return nil, auth.ErrBadCredentials
	}
	ip := clientIP(r)
	if err := s.Rate.Attempt(ip, name); err != nil {
		return nil, apiError{http.StatusTooManyRequests, err.Error()}
	}
	u, err := s.Users.Find(name)
	hash := u.PasswordHash
	if err != nil {
		hash = dummyHash() // same bcrypt cost whether the account exists or not
	}
	passOK := auth.CheckPassword(hash, body.Password) && err == nil
	totpOK := u.TOTPSecret == "" || (passOK && s.useTOTP(name, u.TOTPSecret, body.TOTP))
	if !passOK || !totpOK {
		return nil, apiError{http.StatusUnauthorized, errSignIn}
	}
	s.Rate.Clear(ip, name)
	return s.signIn(w, r, sess, name)
}

func (s *Server) postLogout(w http.ResponseWriter, r *http.Request, sess *auth.Session) (any, error) {
	s.Sessions.Delete(sess.Token)
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	return output{Output: "Signed out."}, nil
}

// Passkeys.

func (s *Server) passkeyLoginOptions(_ http.ResponseWriter, r *http.Request, sess *auth.Session) (any, error) {
	var body struct{ Username string }
	if err := decode(r, &body); err != nil {
		return nil, err
	}
	name, err := auth.CleanUsername(body.Username)
	if err != nil {
		return nil, err
	}
	if err := s.Rate.Attempt(clientIP(r), name); err != nil {
		return nil, apiError{http.StatusTooManyRequests, err.Error()}
	}
	u, err := s.Users.Find(name)
	if err != nil || len(u.Passkeys) == 0 {
		return nil, badRequest("no passkey is available for this sign-in")
	}
	ch, err := auth.NewChallenge()
	if err != nil {
		return nil, err
	}
	s.Sessions.With(sess, func(x *auth.Session) { x.LogChallenge, x.LogUser = ch, name })
	allow := make([]map[string]string, 0, len(u.Passkeys))
	for _, k := range u.Passkeys {
		allow = append(allow, map[string]string{"type": "public-key", "id": k.ID})
	}
	return map[string]any{"publicKey": map[string]any{
		"challenge": ch, "allowCredentials": allow, "userVerification": "required", "timeout": 60000, "rpId": rpID(r),
	}}, nil
}

type assertion struct {
	ID                string `json:"id"`
	ClientDataJSON    string `json:"clientDataJSON"`
	AuthenticatorData string `json:"authenticatorData"`
	Signature         string `json:"signature"`
	PublicKey         string `json:"publicKey"`
}

func (s *Server) passkeyLoginVerify(w http.ResponseWriter, r *http.Request, sess *auth.Session) (any, error) {
	var body assertion
	if err := decode(r, &body); err != nil {
		return nil, err
	}
	var challenge, name string
	s.Sessions.With(sess, func(x *auth.Session) {
		challenge, name = x.LogChallenge, x.LogUser
		x.LogChallenge, x.LogUser = "", ""
	})
	ip := clientIP(r)
	if name == "" {
		return nil, badRequest("missing passkey challenge: start again")
	}
	if err := s.Rate.Attempt(ip, name); err != nil {
		return nil, apiError{http.StatusTooManyRequests, err.Error()}
	}
	if err := s.verifyAssertion(r, body, challenge, name); err != nil {
		return nil, apiError{http.StatusUnauthorized, err.Error()}
	}
	s.Rate.Clear(ip, name)
	return s.signIn(w, r, sess, name)
}

func (s *Server) verifyAssertion(r *http.Request, body assertion, challenge, name string) error {
	u, err := s.Users.Find(name)
	if err != nil {
		return errors.New("passkey user not found")
	}
	cd, _, err := auth.CheckClientData(body.ClientDataJSON, "webauthn.get", challenge, rpID(r))
	if err != nil {
		return err
	}
	var key *auth.Passkey
	for i := range u.Passkeys {
		if u.Passkeys[i].ID == body.ID {
			key = &u.Passkeys[i]
		}
	}
	if key == nil {
		return errors.New("unknown passkey")
	}
	authData, err1 := auth.FromB64URL(body.AuthenticatorData)
	sig, err2 := auth.FromB64URL(body.Signature)
	if err1 != nil || err2 != nil || len(sig) == 0 {
		return errors.New("incomplete passkey response")
	}
	if err := auth.CheckAuthenticatorData(authData, rpID(r)); err != nil {
		return err
	}
	return auth.VerifyAssertion(key.PublicKey, authData, cd, sig)
}

func (s *Server) passkeyRegisterOptions(_ http.ResponseWriter, r *http.Request, sess *auth.Session) (any, error) {
	ch, err := auth.NewChallenge()
	if err != nil {
		return nil, err
	}
	s.Sessions.With(sess, func(x *auth.Session) { x.RegChallenge = ch })
	u, err := s.Users.Find(sess.User)
	if err != nil {
		return nil, err
	}
	return map[string]any{"publicKey": map[string]any{
		"challenge": ch,
		"rp":        map[string]string{"name": config.AppName, "id": rpID(r)},
		"user":      map[string]string{"id": auth.B64URL([]byte(u.Username)), "name": u.Username, "displayName": u.DisplayName},
		"pubKeyCredParams": []map[string]any{
			{"type": "public-key", "alg": -7}, {"type": "public-key", "alg": -257},
		},
		"authenticatorSelection": map[string]string{"residentKey": "preferred", "userVerification": "required"},
		"timeout":                60000,
		"attestation":            "none",
	}}, nil
}

func (s *Server) passkeyRegisterVerify(_ http.ResponseWriter, r *http.Request, sess *auth.Session) (any, error) {
	var body assertion
	if err := decode(r, &body); err != nil {
		return nil, err
	}
	var challenge string
	s.Sessions.With(sess, func(x *auth.Session) { challenge, x.RegChallenge = x.RegChallenge, "" })
	_, origin, err := auth.CheckClientData(body.ClientDataJSON, "webauthn.create", challenge, rpID(r))
	if err != nil {
		return nil, err
	}
	der, err := auth.FromB64URL(body.PublicKey)
	if err != nil || body.ID == "" || len(body.ID) > 1024 {
		return nil, badRequest("this browser did not return a usable passkey")
	}
	pemKey, err := auth.PublicKeyPEM(der)
	if err != nil {
		return nil, err
	}
	err = s.Users.Update(sess.User, func(u *auth.User) error {
		for _, k := range u.Passkeys {
			if k.ID == body.ID {
				return errors.New("this passkey is already registered")
			}
		}
		u.Passkeys = append(u.Passkeys, auth.Passkey{
			ID: body.ID, Name: "Passkey " + time.Now().Format("2006-01-02 15:04"),
			PublicKey: pemKey, Origin: origin, CreatedAt: time.Now().Format(time.RFC3339),
		})
		return nil
	})
	return done("Passkey registered.", err)
}

// Account management.

type accountBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Code     string `json:"code"`
	ID       string `json:"id"`
}

func (s *Server) getUsers(_ http.ResponseWriter, _ *http.Request, _ *auth.Session) (any, error) {
	users, err := s.Users.All()
	if err != nil {
		return nil, err
	}
	out := make([]*userInfo, 0, len(users))
	for _, u := range users {
		out = append(out, toUserInfo(u))
	}
	return out, nil
}

func (s *Server) postAccount(_ http.ResponseWriter, r *http.Request, sess *auth.Session) (any, error) {
	var body accountBody
	if err := decode(r, &body); err != nil {
		return nil, err
	}
	switch r.PathValue("action") {
	case "add":
		return done("User created.", s.Users.Create(body.Username, body.Password, false))
	case "delete":
		name, err := auth.CleanUsername(body.Username)
		if err != nil {
			return nil, err
		}
		if err := s.Users.Delete(name, sess.User); err != nil {
			return nil, err
		}
		s.Sessions.DeleteUser(name, "")
		s.Terms.StopUser(name)
		return done("User deleted.", nil)
	case "password":
		name, err := auth.CleanUsername(body.Username)
		if err != nil {
			return nil, err
		}
		if err := s.Users.SetPassword(name, body.Password); err != nil {
			return nil, err
		}
		s.Sessions.DeleteUser(name, sess.Token) // stolen cookies stop working
		return done("Password updated. Other sessions of this account were closed.", nil)
	case "totp-begin":
		secret, err := auth.NewTOTPSecret()
		if err != nil {
			return nil, err
		}
		s.Sessions.With(sess, func(x *auth.Session) { x.PendingTOTP = secret })
		return map[string]string{"secret": secret, "uri": auth.TOTPURI(config.AppName, sess.User, secret)}, nil
	case "totp-confirm":
		var secret string
		s.Sessions.With(sess, func(x *auth.Session) { secret = x.PendingTOTP })
		if secret == "" {
			return nil, badRequest("generate a secret first")
		}
		if !s.useTOTP(sess.User, secret, body.Code) {
			return nil, badRequest(auth.ErrBadTOTP.Error())
		}
		s.Sessions.With(sess, func(x *auth.Session) { x.PendingTOTP = "" })
		return done("Two-factor authentication enabled.", s.Users.Update(sess.User, func(u *auth.User) error { u.TOTPSecret = secret; return nil }))
	case "totp-disable":
		if err := s.reauth(r, sess.User, body.Password); err != nil {
			return nil, err
		}
		return done("Two-factor authentication disabled.", s.Users.Update(sess.User, func(u *auth.User) error { u.TOTPSecret = ""; return nil }))
	case "passkey-delete":
		if err := s.reauth(r, sess.User, body.Password); err != nil {
			return nil, err
		}
		return done("Passkey deleted.", s.Users.Update(sess.User, func(u *auth.User) error {
			kept := u.Passkeys[:0]
			for _, k := range u.Passkeys {
				if k.ID != body.ID {
					kept = append(kept, k)
				}
			}
			u.Passkeys = kept
			return nil
		}))
	}
	return nil, apiError{http.StatusNotFound, "unknown action"}
}
