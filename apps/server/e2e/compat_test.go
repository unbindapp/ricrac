package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
)

// Every test records what crosses the server's edges: the requests it answers, the webhooks
// and GitHub calls it makes and the emails it sends. With RICRAC_RECORD=1 the recording is
// written to testdata/compat; otherwise it must match the one there. Ids, secrets and times
// are numbered or masked first, so two runs of the same server record the same thing.

const compatDir = "testdata/compat"

// exchange is one thing that crossed the server's edges.
type exchange struct {
	Kind    string            `json:"kind"`
	Method  string            `json:"method,omitempty"`
	Path    string            `json:"path,omitempty"`
	Auth    string            `json:"auth,omitempty"`
	Request any               `json:"request,omitempty"`
	Status  int               `json:"status,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
}

type tape struct {
	t       *testing.T
	mu      sync.Mutex
	entries []exchange
}

func newTape(t *testing.T) *tape {
	tp := &tape{t: t}
	t.Cleanup(tp.finish)
	return tp
}

var recordedHeaders = []string{
	"Content-Type", "Content-Disposition", "Content-Security-Policy", "Location", "Www-Authenticate",
	"Cache-Control", "Set-Cookie", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy",
	"Access-Control-Allow-Origin", "Access-Control-Allow-Methods", "Access-Control-Allow-Headers",
	"Access-Control-Expose-Headers", "Mcp-Session-Id", "X-Ricrac-Event", "X-Ricrac-Delivery",
}

// wrap serves the backend's router, recording each request and answer.
func (tp *tape) wrap(b *backend) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		recorder := httptest.NewRecorder()
		b.Router().ServeHTTP(recorder, r)
		for name, values := range recorder.Header() {
			w.Header()[name] = values
		}
		w.WriteHeader(recorder.Code)
		_, _ = w.Write(recorder.Body.Bytes())
		auth := ""
		switch {
		case r.Header.Get("Authorization") != "":
			auth = "bearer"
		case r.Header.Get("Cookie") != "":
			auth = "session"
		}
		tp.add(exchange{
			Kind: "http", Method: r.Method, Path: r.URL.RequestURI(), Auth: auth,
			Request: decodeBody(body, r.Header.Get("Content-Type")),
			Status:  recorder.Code, Headers: pickHeaders(recorder.Header()),
			Body: decodeBody(recorder.Body.Bytes(), recorder.Header().Get("Content-Type")),
		})
	})
}

// outbound records a request the server made, like a webhook delivery or a GitHub API call.
func (tp *tape) outbound(kind string, r *http.Request, body []byte) {
	tp.add(exchange{
		Kind: kind, Method: r.Method, Path: r.URL.RequestURI(), Headers: pickHeaders(r.Header),
		Request: decodeBody(body, r.Header.Get("Content-Type")),
	})
}

func (tp *tape) email(to, subject, text, html string) {
	tp.add(exchange{Kind: "email", Path: to, Request: M{"subject": subject, "text": text, "html": html}})
}

func (tp *tape) add(e exchange) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.entries = append(tp.entries, e)
}

func pickHeaders(header http.Header) map[string]string {
	picked := map[string]string{}
	for _, name := range recordedHeaders {
		if values := header.Values(name); len(values) > 0 {
			picked[name] = strings.Join(values, "\n")
		}
	}
	if len(picked) == 0 {
		return nil
	}
	return picked
}

func decodeBody(body []byte, contentType string) any {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	var value any
	if json.Unmarshal(body, &value) == nil {
		return value
	}
	if strings.HasPrefix(contentType, "text/event-stream") {
		var events []any
		for line := range strings.SplitSeq(string(body), "\n") {
			data, ok := strings.CutPrefix(line, "data: ")
			if !ok {
				continue
			}
			var event any
			if json.Unmarshal([]byte(data), &event) == nil {
				events = append(events, event)
			}
		}
		return M{"events": events}
	}
	return string(body)
}

func compatFile(t *testing.T) string {
	return filepath.Join(compatDir, strings.ReplaceAll(t.Name(), "/", "__")+".json")
}

func (tp *tape) finish() {
	tp.mu.Lock()
	entries := slices.Clone(tp.entries)
	tp.mu.Unlock()
	got := normalizeEntries(entries)
	file := compatFile(tp.t)
	if os.Getenv("RICRAC_RECORD") == "1" {
		if tp.t.Failed() {
			return
		}
		if err := os.MkdirAll(compatDir, 0o755); err != nil {
			tp.t.Fatal(err)
		}
		data, _ := json.MarshalIndent(got, "", "  ")
		if err := os.WriteFile(file, append(data, '\n'), 0o644); err != nil {
			tp.t.Fatal(err)
		}
		return
	}
	if os.Getenv("RICRAC_COMPAT") == "off" {
		return
	}
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		if os.Getenv("RICRAC_COMPAT") == "require" {
			tp.t.Errorf("no recording in %s", file)
		}
		return
	}
	if err != nil {
		tp.t.Fatal(err)
	}
	var want []any
	if err := json.Unmarshal(data, &want); err != nil {
		tp.t.Fatal(err)
	}
	var have []any
	_ = json.Unmarshal(marshal(got), &have)
	if !strictDates() {
		want = maskDates(want).([]any)
		have = maskDates(have).([]any)
	}
	compareEntries(tp.t, want, have)
}

func compareEntries(t *testing.T, want, have []any) {
	reported := 0
	for i := range max(len(want), len(have)) {
		if reported == 5 {
			t.Errorf("... and more differences from the recording")
			return
		}
		var w, h any
		if i < len(want) {
			w = want[i]
		}
		if i < len(have) {
			h = have[i]
		}
		if reflect.DeepEqual(w, h) {
			continue
		}
		reported++
		var differences []string
		diff("", w, h, &differences)
		t.Errorf("exchange %d differs from the recording (%s):\n  %s", i, describe(w, h), strings.Join(differences[:min(len(differences), 12)], "\n  "))
	}
}

func describe(w, h any) string {
	pick := func(v any) string {
		e, _ := v.(map[string]any)
		if e == nil {
			return "nothing"
		}
		return fmt.Sprintf("%v %v %v", e["kind"], e["method"], e["path"])
	}
	return "recorded " + pick(w) + ", got " + pick(h)
}

func diff(path string, w, h any, out *[]string) {
	wm, wok := w.(map[string]any)
	hm, hok := h.(map[string]any)
	if wok && hok {
		keys := slices.Sorted(maps.Keys(wm))
		for key := range hm {
			if _, ok := wm[key]; !ok {
				keys = append(keys, key)
			}
		}
		for _, key := range keys {
			if !reflect.DeepEqual(wm[key], hm[key]) {
				diff(path+"."+key, wm[key], hm[key], out)
			}
		}
		return
	}
	wl, wok := w.([]any)
	hl, hok := h.([]any)
	if wok && hok && len(wl) == len(hl) {
		for i := range wl {
			if !reflect.DeepEqual(wl[i], hl[i]) {
				diff(fmt.Sprintf("%s[%d]", path, i), wl[i], hl[i], out)
			}
		}
		return
	}
	ws, wok := w.(string)
	hs, hok := h.(string)
	if wok && hok && (len(ws) > 200 || len(hs) > 200) {
		at := 0
		for at < min(len(ws), len(hs)) && ws[at] == hs[at] {
			at++
		}
		from := max(0, at-80)
		*out = append(*out, fmt.Sprintf("%s: from byte %d, recorded %q, got %q", path, at, ws[from:min(len(ws), at+120)], hs[from:min(len(hs), at+120)]))
		return
	}
	*out = append(*out, fmt.Sprintf("%s: recorded %s, got %s", path, short(w), short(h)))
}

func short(v any) string {
	text := string(marshal(v))
	if len(text) > 300 {
		return text[:300] + "..."
	}
	return text
}

func strictDates() bool {
	return os.Getenv("RICRAC_STRICT_DATES") == "1"
}

var (
	uuidPattern   = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	timePattern   = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?`)
	httpDate      = regexp.MustCompile(`[A-Z][a-z]{2}, \d{2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2} GMT`)
	secretPattern = regexp.MustCompile(`\b(rr|gh[osu]|dcr)_([a-z]+_)?[A-Za-z0-9_-]{16,}`)
	hintPattern   = regexp.MustCompile(`\b(rr_[a-z]+_)[A-Za-z0-9_-]{4}…[A-Za-z0-9_-]{4}`)
	tokenPattern  = regexp.MustCompile(`[A-Za-z0-9_-]{32,}`)
	slugPattern   = regexp.MustCompile(`\b[0-9a-f]{12}\b`)
	datePattern   = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)
	unixPattern   = regexp.MustCompile(`\b1[7-9]\d{8}\b`)
	localServer   = regexp.MustCompile(`http://127\.0\.0\.1:\d+`)
	avatarColor   = regexp.MustCompile(`bgcolor="#[0-9a-f]{6}" style="width:20px;height:20px;border-radius:10px;background-color:#[0-9a-f]{6}`)
)

// normalizer numbers ids, slugs and secrets in the order they first appear, and masks times.
type normalizer struct {
	seen map[string]string
	next map[string]int
}

func normalizeEntries(entries []exchange) []any {
	n := &normalizer{seen: map[string]string{}, next: map[string]int{}}
	var plain []any
	_ = json.Unmarshal(marshal(entries), &plain)
	return n.value(plain).([]any)
}

func (n *normalizer) name(kind, value string) string {
	key := kind + ":" + strings.ToLower(value)
	if name, ok := n.seen[key]; ok {
		return name
	}
	n.next[kind]++
	name := fmt.Sprintf("<%s%d>", kind, n.next[kind])
	n.seen[key] = name
	return name
}

func (n *normalizer) value(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for _, key := range slices.Sorted(maps.Keys(v)) {
			out[n.text(key)] = n.value(v[key])
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = n.value(item)
		}
		return out
	case string:
		return n.text(v)
	case float64:
		switch {
		case v >= 1.7e9 && v < 2.1e9:
			return "<unix>"
		case v >= 1.7e12 && v < 2.1e12:
			return "<unix-ms>"
		}
	}
	return v
}

func (n *normalizer) text(s string) string {
	s = uuidPattern.ReplaceAllStringFunc(s, func(m string) string { return n.name("id", m) })
	s = timePattern.ReplaceAllStringFunc(s, func(m string) string {
		zone := timePattern.FindStringSubmatch(m)[2]
		return "<time" + zone + ">"
	})
	s = httpDate.ReplaceAllString(s, "<http-date>")
	s = localServer.ReplaceAllString(s, "<local-server>")
	s = avatarColor.ReplaceAllString(s, "<avatar-color>")
	s = secretPattern.ReplaceAllStringFunc(s, func(m string) string {
		parts := secretPattern.FindStringSubmatch(m)
		return parts[1] + "_" + parts[2] + n.name("secret", m)
	})
	s = hintPattern.ReplaceAllString(s, "${1}<hint>")
	s = tokenPattern.ReplaceAllStringFunc(s, func(m string) string { return n.name("token", m) })
	s = slugPattern.ReplaceAllStringFunc(s, func(m string) string { return n.name("slug", m) })
	s = unixPattern.ReplaceAllString(s, "<unix>")
	return s
}

// maskDates hides calendar days, which move with the day the tests run.
func maskDates(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			out[datePattern.ReplaceAllString(key, "<date>")] = maskDates(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = maskDates(item)
		}
		return out
	case string:
		return datePattern.ReplaceAllString(v, "<date>")
	}
	return v
}
