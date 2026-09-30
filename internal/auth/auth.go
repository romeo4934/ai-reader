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
