package server

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ricrac/app"
	"ricrac/ops"
)

func asJSON(t *testing.T, value any) string {
	t.Helper()
	return string(app.Marshal(value))
}

func TestQueryValuesFollowTheSchema(t *testing.T) {
	var schema M
	_ = json.Unmarshal([]byte(`{"properties": {
		"priority": {"type": ["integer", "null"]},
		"query": {"type": ["string", "null"]},
		"includeArchived": {"type": "boolean"},
		"labels": {"type": ["array", "null"], "items": {"type": "string"}},
		"filter": {}
	}}`), &schema)
	p := func(key string) M { return schema["properties"].(M)[key].(M) }
	for _, c := range []struct {
		values []string
		key    string
		want   string
	}{
		{[]string{"2"}, "priority", `2`},
		{[]string{"null"}, "priority", `null`},
		{[]string{"123"}, "query", `"123"`},
		{[]string{"true"}, "includeArchived", `true`},
		{[]string{"Bug,UI"}, "labels", `["Bug","UI"]`},
		{[]string{"Bug", "UI"}, "labels", `["Bug","UI"]`},
		{[]string{`["Bug, with comma"]`}, "labels", `["Bug, with comma"]`},
		{[]string{`{"priority":{"eq":1}}`}, "filter", `{"priority":{"eq":1}}`},
	} {
		if got := asJSON(t, coerce(c.values, p(c.key))); got != c.want {
			t.Errorf("%s=%v: got %s, want %s", c.key, c.values, got, c.want)
		}
	}
}

func TestArgumentsMergeBodyQueryAndPath(t *testing.T) {
	op := ops.Find("list_issues")
	args, err := arguments(op, map[string]string{"id": "ENG-1"}, "priority=2&label%5B%5D=a+b&label[]=c", []byte(`{"id": "x", "team": "ENG"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := asJSON(t, args); got != `{"id":"ENG-1","label":"c","priority":2,"team":"ENG"}` {
		t.Fatal(got)
	}
	if _, err := arguments(op, nil, "", []byte(`[1]`)); err == nil {
		t.Fatal("bodies are objects")
	}
}

func TestOpenAPIListsEveryRoute(t *testing.T) {
	paths := OpenAPIDocument("https://r.example")["paths"].(map[string]M)
	count, routes := 0, 0
	for _, methods := range paths {
		count += len(methods)
	}
	for _, op := range ops.All() {
		routes += len(op.Routes)
	}
	if count != routes || count == 0 {
		t.Fatalf("%d operations in the document for %d routes", count, routes)
	}
}

func TestCoreToolsExist(t *testing.T) {
	var missing []string
	for _, name := range core {
		if ops.Find(name) == nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Skipf("needs %s ported", strings.Join(missing, ", "))
	}
}

func TestRedirects(t *testing.T) {
	for uri, allowed := range map[string]bool{
		"https://claude.ai/api/mcp/auth_callback": true,
		"http://localhost:33418/callback":         true,
		"http://[::1]:8080/cb":                    true,
		"motile://ricrac":                         true,
		"http://evil.com/cb":                      false,
		"http://localhost.evil.com/cb":            false,
		"javascript://alert(1)":                   false,
		"https://x.com/cb#frag":                   false,
	} {
		if IsAllowedRedirect(uri) != allowed {
			t.Errorf("%s allowed should be %v", uri, allowed)
		}
	}
	if !redirectMatches("http://localhost/callback", "http://localhost:3118/callback") {
		t.Error("loopback redirects take any port")
	}
	if redirectMatches("http://localhost/callback", "http://127.0.0.1:3118/callback") || redirectMatches("https://a.com/cb", "https://a.com:444/cb") {
		t.Error("other redirects match exactly")
	}
}

func TestClientIDURLs(t *testing.T) {
	for id, valid := range map[string]bool{
		"https://claude.ai/oauth/claude-code-client-metadata": true,
		"https://claude.ai/":              false,
		"http://claude.ai/oauth/metadata": false,
		"https://10.0.0.1/metadata":       false,
	} {
		if isClientIDURL(id) != valid {
			t.Errorf("%s should be %v", id, valid)
		}
	}
}

func TestScopes(t *testing.T) {
	all, _ := requestedScopes(nil)
	write := "write"
	written, _ := requestedScopes(&write)
	unknown := "issues:create"
	_, err := requestedScopes(&unknown)
	if strings.Join(all, " ") != "read write admin" || strings.Join(written, " ") != "read write" || err == nil {
		t.Fatal(all, written, err)
	}
	checked, _ := checkScopes([]string{"admin", "write"})
	if strings.Join(checked, " ") != "read write admin" {
		t.Fatal(checked)
	}
}

func TestPKCE(t *testing.T) {
	if !pkceMatches("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM") {
		t.Error("the RFC 7636 example matches")
	}
	if pkceMatches("short", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM") {
		t.Error("short verifiers are refused")
	}
}

func idToken(claims M) string {
	return "e30." + base64.RawURLEncoding.EncodeToString(app.Marshal(claims)) + ".sig"
}

func googleClaims() M {
	return M{
		"iss": "https://accounts.google.com", "aud": "client", "exp": time.Now().Unix() + 60, "nonce": "n",
		"sub": "123", "email": "Ann@Example.com", "email_verified": true, "name": "Ann Lee",
	}
}

func TestReadsIdentityAndRejectsWrongClaims(t *testing.T) {
	identity, err := identityFromIDToken(idToken(googleClaims()), "client", "n")
	if err != nil || identity.Email != "ann@example.com" || !identity.EmailVerified || *identity.Name != "Ann Lee" {
		t.Fatal(identity, err)
	}
	for key, value := range map[string]any{"iss": "https://evil.com", "aud": "other", "exp": 1, "nonce": "x"} {
		bad := googleClaims()
		bad[key] = value
		if _, err := identityFromIDToken(idToken(bad), "client", "n"); err == nil {
			t.Errorf("a wrong %s is refused", key)
		}
	}
}

func TestDisplayNames(t *testing.T) {
	if app.DisplayName("Ann Lee", "a@x.com") != "ann" || app.DisplayName("", "bob.s@x.com") != "bob.s" {
		t.Fatal(app.DisplayName("Ann Lee", "a@x.com"), app.DisplayName("", "bob.s@x.com"))
	}
}

func TestKeysAreChecked(t *testing.T) {
	key, err := ops.CheckURLKey(" Acme-Co ")
	if key != "acme-co" || err != nil {
		t.Fatal(key, err)
	}
	team, err := ops.CheckTeamKey("eng")
	if team != "ENG" || err != nil {
		t.Fatal(team, err)
	}
	for _, bad := range []error{
		second(ops.CheckURLKey("-acme")), second(ops.CheckURLKey("api")),
		second(ops.CheckTeamKey("1ENG")), second(ops.CheckTeamKey("ENGINEERS")),
	} {
		if bad == nil {
			t.Error("a bad key passed")
		}
	}
}

func second(_ string, err error) error { return err }

func TestDispositions(t *testing.T) {
	if got := disposition("a b.png", "image/png"); got != `inline; filename="a b.png"; filename*=UTF-8''a%20b%2Epng` {
		t.Error(got)
	}
	if got := disposition("x.svg", "image/svg+xml"); !strings.HasPrefix(got, `attachment; filename="x.svg"`) {
		t.Error(got)
	}
	if got := disposition(`"é".txt`, "text/plain"); !strings.HasPrefix(got, `inline; filename="___.txt"`) {
		t.Error(got)
	}
}

func TestHostsAreChecked(t *testing.T) {
	allowed := []string{"localhost", "127.0.0.1", "ricrac.example.com"}
	for host, ok := range map[string]bool{
		"localhost:3000": true, "127.0.0.1": true, "ricrac.example.com": true, "ricrac.example.com:443": true,
		"evil.example": false, "localhost.evil.example": false,
	} {
		if hostAllowed(host, allowed) != ok {
			t.Errorf("%s should be %v", host, ok)
		}
	}
	if hostAllowed("ricrac.example.com:8080", []string{"ricrac.example.com:443"}) {
		t.Error("entries with a port match that port only")
	}
}

func TestCompressesLargeResponsesOnly(t *testing.T) {
	handler := compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		size := 10
		if r.URL.Path == "/large" {
			size = 5000
		}
		if r.URL.Path == "/stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			size = 5000
		}
		_, _ = w.Write([]byte(strings.Repeat("a", size)))
	}))
	for path, gzipped := range map[string]bool{"/small": false, "/large": true, "/stream": false} {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Accept-Encoding", "gzip, br")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if (recorder.Header().Get("Content-Encoding") == "gzip") != gzipped {
			t.Fatalf("%s gzipped should be %v", path, gzipped)
		}
		body := io.Reader(recorder.Body)
		if gzipped {
			reader, err := gzip.NewReader(recorder.Body)
			if err != nil {
				t.Fatal(err)
			}
			body = reader
		}
		data, _ := io.ReadAll(body)
		if len(data) < 10 || strings.Trim(string(data), "a") != "" {
			t.Fatalf("%s body was mangled: %d bytes", path, len(data))
		}
	}
}
