package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"ricrac/config"
)

const SessionTTL = 30 * 24 * 3600

var b64 = base64.RawURLEncoding

// Keys signs values (session cookies, the sign-in round trip).
type Keys struct {
	secret        [32]byte
	SessionCookie string
}

func NewKeys(cfg config.Config) Keys {
	return Keys{secret: sha256.Sum256([]byte(cfg.SecretKey)), SessionCookie: cfg.CookieName("rr_session")}
}

func (k Keys) mac(data []byte) []byte {
	mac := hmac.New(sha256.New, k.secret[:])
	mac.Write(data)
	return mac.Sum(nil)
}

func (k Keys) Sign(payload string) string {
	return b64.EncodeToString([]byte(payload)) + "." + b64.EncodeToString(k.mac([]byte(payload)))
}

func (k Keys) Verify(token string) (string, bool) {
	payloadB64, sigB64, found := strings.Cut(token, ".")
	if !found {
		return "", false
	}
	payload, err := b64.DecodeString(payloadB64)
	if err != nil {
		return "", false
	}
	sig, err := b64.DecodeString(sigB64)
	if err != nil || !hmac.Equal(k.mac(payload), sig) {
		return "", false
	}
	return string(payload), true
}

func (k Keys) SessionToken(user uuid.UUID) string {
	return k.Sign(fmt.Sprintf("user:%d:%s", time.Now().Unix()+SessionTTL, user))
}

func (k Keys) SessionUser(r *http.Request) (uuid.UUID, bool) {
	cookie, err := r.Cookie(k.SessionCookie)
	if err != nil {
		return uuid.Nil, false
	}
	payload, ok := k.Verify(cookie.Value)
	if !ok {
		return uuid.Nil, false
	}
	rest, ok := strings.CutPrefix(payload, "user:")
	if !ok {
		return uuid.Nil, false
	}
	expires, user, ok := strings.Cut(rest, ":")
	if !ok {
		return uuid.Nil, false
	}
	exp, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || exp <= time.Now().Unix() {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(user)
	return id, err == nil
}

func IsSecureRequest(cfg config.Config, r *http.Request) bool {
	if cfg.SecureCookies != nil {
		return *cfg.SecureCookies
	}
	proto, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
	return strings.EqualFold(strings.TrimSpace(proto), "https")
}

func BuildCookie(name, value string, maxAge int64, secure bool) string {
	cookie := fmt.Sprintf("%s=%s; Path=/; Max-Age=%d; HttpOnly; SameSite=Lax", name, value, max(maxAge, 0))
	if secure {
		cookie += "; Secure"
	}
	return cookie
}

func ClientIP(r *http.Request) string {
	for _, name := range []string{"Cf-Connecting-Ip", "X-Real-Ip", "X-Forwarded-For"} {
		value, _, _ := strings.Cut(r.Header.Get(name), ",")
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return "unknown"
}

// RandomSecret is a random URL-safe secret with a readable prefix, e.g. rr_at_...
func RandomSecret(prefix string) string {
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	return prefix + b64.EncodeToString(bytes)
}

// SecretHash is how secrets (tokens, codes, keys, client secrets) are stored.
func SecretHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// APIKeyPrefix starts every API key.
const APIKeyPrefix = "rr_key_"
