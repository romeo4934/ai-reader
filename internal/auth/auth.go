// Package auth handles password hashing and signed session cookies for the
// multi-user deployment — same HMAC-signed-cookie shape calgoal uses, but
// carrying a user id instead of a single shared password's yes/no.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const SessionCookie = "ai_reader_session"
const SessionTTL = 30 * 24 * time.Hour

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Sign produces a cookie value encoding userID and an expiry, MAC'd with
// secret so it can't be forged or extended by the client.
func Sign(secret []byte, userID int64) string {
	exp := time.Now().Add(SessionTTL).Unix()
	payload := strconv.FormatInt(userID, 10) + "." + strconv.FormatInt(exp, 10)
	return payload + "." + mac(secret, payload)
}

// Verify checks the cookie's MAC and expiry, returning the userID if valid.
func Verify(secret []byte, value string) (userID int64, ok bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return 0, false
	}
	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(mac(secret, payload))) {
		return 0, false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return 0, false
	}
	userID, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, false
	}
	return userID, true
}

func mac(secret []byte, payload string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// Link tokens (email verification, password reset) are signed the same way
// as session cookies, plus a purpose so one kind can't be replayed as the
// other, and a binding: a value the token is only good for as long as it
// stays unchanged (the email being verified; the current password hash for a
// reset, which makes a reset link single-use — using it changes the hash).
const (
	PurposeVerify = "verify"
	PurposeReset  = "reset"
	PurposeInvite = "invite"
)

func SignLink(secret []byte, purpose string, userID int64, binding string, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	payload := strconv.FormatInt(userID, 10) + "." + strconv.FormatInt(exp, 10)
	return payload + "." + mac(secret, purpose+"|"+binding+"|"+payload)
}

// LinkUserID reads the user id out of a link token without trusting it —
// the caller looks the user up to get the binding, then calls VerifyLink.
func LinkUserID(token string) (int64, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, false
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	return id, err == nil
}

func VerifyLink(secret []byte, purpose, token, binding string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(mac(secret, purpose+"|"+binding+"|"+payload))) {
		return false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	return err == nil && time.Now().Unix() <= exp
}
