package github

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func TestConfigNeedsEveryAppSetting(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	if config, err := ParseConfig("", "", "", "", "", "", "", ""); config != nil || err != nil {
		t.Fatalf("no settings is no app, got %v %v", config, err)
	}
	if _, err := ParseConfig("12", "ricrac", encoded, "", "", "", "", ""); err == nil || !strings.Contains(err.Error(), "GITHUB_WEBHOOK_SECRET") {
		t.Fatalf("missing settings are named, got %v", err)
	}
	oneLine := strings.ReplaceAll(encoded, "\n", `\n`)
	config, err := ParseConfig("12", "ricrac", oneLine, "secret", "Iv1.x", "shh", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if config.APIURL != "https://api.github.com" || config.WebURL != "https://github.com" || !config.PrivateKey.Equal(key) {
		t.Fatalf("got %+v", config)
	}
	if _, err := ParseConfig("12", "ricrac", "not a key", "secret", "Iv1.x", "shh", "", ""); err == nil {
		t.Fatal("a broken key is refused")
	}
}

func TestJWTIsSignedByTheAppKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp(Config{AppID: 12, PrivateKey: key}, nil)
	now := time.Unix(1_800_000_000, 0)
	token, err := app.JWT(now)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	signature, _ := base64.RawURLEncoding.DecodeString(parts[2])
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature); err != nil {
		t.Fatalf("signature: %v", err)
	}
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims struct {
		Iat, Exp int64
		Iss      string
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Iss != "12" || claims.Iat != now.Unix()-60 || claims.Exp != now.Unix()+540 {
		t.Fatalf("got %+v", claims)
	}
}

func TestNextPage(t *testing.T) {
	link := `<https://api.github.com/repositories/1/issues?page=3>; rel="next", <https://api.github.com/repositories/1/issues?page=9>; rel="last"`
	if got := nextPage(link); got != 3 {
		t.Fatalf("got %d", got)
	}
	if got := nextPage(`<https://api.github.com/x?page=1>; rel="prev"`); got != 0 {
		t.Fatalf("got %d", got)
	}
}
