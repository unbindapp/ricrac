package auth

import (
	"crypto/aes"
	"crypto/cipher"
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

// Seal encrypts a secret Ricrac has to read back, like a GitHub user token.
func (k Keys) Seal(secret string) string {
	aead := k.aead()
	nonce := make([]byte, aead.NonceSize())
	_, _ = rand.Read(nonce)
	return b64.EncodeToString(aead.Seal(nonce, nonce, []byte(secret), nil))
}

// Open decrypts what Seal sealed; false when it wasn't sealed with this SECRET_KEY.
func (k Keys) Open(sealed string) (string, bool) {
	aead := k.aead()
	data, err := b64.DecodeString(sealed)
	if err != nil || len(data) < aead.NonceSize() {
		return "", false
	}
	secret, err := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], nil)
	return string(secret), err == nil
}

func (k Keys) aead() cipher.AEAD {
	block, _ := aes.NewCipher(k.mac([]byte("sealing key")))
	aead, _ := cipher.NewGCM(block)
	return aead
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

// AssistantTokenPrefix starts the tokens the assistant's CLI calls MCP with.
const AssistantTokenPrefix = "rr_as_"

// AssistantToken lets the assistant act as user in one workspace, with read and write, for ttl.
func (k Keys) AssistantToken(user, workspace uuid.UUID, ttl time.Duration) string {
	return AssistantTokenPrefix + k.Sign(fmt.Sprintf("assistant:%d:%s:%s", time.Now().Add(ttl).Unix(), user, workspace))
}

// AssistantCaller is the caller of an assistant token, false when it isn't a valid one.
func (k Keys) AssistantCaller(token string) (*Caller, bool) {
	signed, ok := strings.CutPrefix(token, AssistantTokenPrefix)
	if !ok {
		return nil, false
	}
	payload, ok := k.Verify(signed)
	if !ok {
		return nil, false
	}
	parts := strings.Split(payload, ":")
	if len(parts) != 4 || parts[0] != "assistant" {
		return nil, false
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || expires <= time.Now().Unix() {
		return nil, false
	}
	user, userErr := uuid.Parse(parts[2])
	workspace, workspaceErr := uuid.Parse(parts[3])
	if userErr != nil || workspaceErr != nil {
		return nil, false
	}
	return &Caller{User: user, Scopes: []Access{Read, Write}, Workspaces: []uuid.UUID{workspace}}, true
}
