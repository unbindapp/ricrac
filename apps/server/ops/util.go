package ops

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode"

	"ricrac/app"
)

// slugID is a random 12-character id for URLs (projects, documents, views, ...).
func slugID() string {
	const alphabet = "0123456789abcdef"
	id := make([]byte, 12)
	for i := range id {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		id[i] = alphabet[n.Int64()]
	}
	return string(id)
}

// parseDate takes YYYY-MM-DD, or a time whose day it keeps.
func parseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if date, err := time.Parse(time.DateOnly, value); err == nil {
		return date, nil
	}
	if at, err := parseDatetime(value); err == nil {
		return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Time{}, app.BadRequest("%q isn't a date; use YYYY-MM-DD", value)
}

// parseDatetime takes an ISO 8601 date or time, or a duration relative to now such as -P1D, P2W
// or -PT3H.
func parseDatetime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if at, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return at.UTC(), nil
	}
	if date, err := time.Parse(time.DateOnly, value); err == nil {
		return date, nil
	}
	if duration, ok := parseDuration(value); ok {
		return time.Now().UTC().Add(duration), nil
	}
	return time.Time{}, app.BadRequest(
		"%q isn't a time; use ISO 8601 (2026-01-31 or 2026-01-31T12:00:00Z) or a duration like -P1D", value)
}

func parseDuration(value string) (time.Duration, bool) {
	negative := false
	rest := value
	if r, ok := strings.CutPrefix(rest, "-"); ok {
		negative, rest = true, r
	} else {
		rest = strings.TrimPrefix(rest, "+")
	}
	if !strings.HasPrefix(rest, "P") && !strings.HasPrefix(rest, "p") {
		return 0, false
	}
	rest = rest[1:]
	datePart, timePart, hasTime := strings.Cut(strings.ReplaceAll(rest, "t", "T"), "T")
	var total time.Duration
	parse := func(part string, units map[rune]float64) bool {
		number := ""
		for _, c := range part {
			if unicode.IsDigit(c) || c == '.' {
				number += string(c)
				continue
			}
			seconds, ok := units[unicode.ToUpper(c)]
			if !ok {
				return false
			}
			n, err := strconv.ParseFloat(number, 64)
			if err != nil {
				return false
			}
			total += time.Duration(n * seconds * float64(time.Second))
			number = ""
		}
		return number == ""
	}
	if !parse(datePart, map[rune]float64{'Y': 365 * 86400, 'M': 30 * 86400, 'W': 7 * 86400, 'D': 86400}) {
		return 0, false
	}
	if hasTime && !parse(timePart, map[rune]float64{'H': 3600, 'M': 60, 'S': 1}) {
		return 0, false
	}
	if total == 0 {
		return 0, false
	}
	if negative {
		total = -total
	}
	return total, true
}

// Page is paging, built from a list operation's limit and cursor arguments.
type Page struct {
	Limit  *int64  `json:"limit" doc:"How many to return, 50 by default, at most 250."`
	Cursor *string `json:"cursor" doc:"The endCursor of the previous page, to continue after it."`
}

func (p Page) limit() int64 {
	if p.Limit == nil {
		return 50
	}
	return min(max(*p.Limit, 1), 250)
}

func (p Page) offset() (int64, error) {
	if p.Cursor == nil {
		return 0, nil
	}
	invalid := app.BadRequest("Invalid cursor; pass the endCursor of the previous page")
	bytes, err := base64.RawURLEncoding.DecodeString(*p.Cursor)
	if err != nil {
		return 0, invalid
	}
	n, ok := strings.CutPrefix(string(bytes), "o:")
	if !ok {
		return 0, invalid
	}
	offset, err := strconv.ParseInt(n, 10, 64)
	if err != nil {
		return 0, invalid
	}
	return offset, nil
}

// wrap takes the limit()+1 rows fetched from offset(), trims the extra row and says whether
// there are more.
func (p Page) wrap(nodes []any) (M, error) {
	if nodes == nil {
		nodes = []any{}
	}
	limit := int(p.limit())
	hasNext := len(nodes) > limit
	if hasNext {
		nodes = nodes[:limit]
	}
	offset, err := p.offset()
	if err != nil {
		return nil, err
	}
	var endCursor any
	if hasNext {
		endCursor = base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, "o:%d", offset+int64(len(nodes))))
	}
	return M{"nodes": nodes, "pageInfo": M{"hasNextPage": hasNext, "endCursor": endCursor}}, nil
}

type PageInfo struct {
	HasNextPage bool    `json:"hasNextPage"`
	EndCursor   *string `json:"endCursor" doc:"Pass it as cursor to get the next page."`
}

// PageOf is the shape of a list operation's result.
type PageOf[T any] struct {
	Nodes    []T      `json:"nodes"`
	PageInfo PageInfo `json:"pageInfo"`
}

// TextEdit is one edit of a Markdown text, applied in order with the others of a patch.
type TextEdit struct {
	Op         string `json:"op"`
	OldString  string `json:"old_string,omitempty"`
	NewString  string `json:"new_string,omitempty"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
	Anchor     string `json:"anchor,omitempty"`
	Text       string `json:"text,omitempty"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
}

func (TextEdit) JSONSchema() M {
	variant := func(op, description string, properties M, required ...string) M {
		properties["op"] = M{"type": "string", "const": op}
		list := []any{"op"}
		for _, r := range required {
			list = append(list, r)
		}
		return M{"type": "object", "description": description, "properties": properties, "required": list}
	}
	text := M{"type": "string"}
	return M{"oneOf": []any{
		variant("replace", "Replace text that occurs exactly once (or every occurrence with replace_all).",
			M{"old_string": text, "new_string": text, "replace_all": M{"type": "boolean", "default": false}}, "old_string", "new_string"),
		variant("insert_before", "Insert text directly before an anchor that occurs exactly once.",
			M{"anchor": text, "text": text}, "anchor", "text"),
		variant("insert_after", "Insert text directly after an anchor that occurs exactly once.",
			M{"anchor": text, "text": text}, "anchor", "text"),
		variant("prepend", "Insert text at the very start.", M{"text": text}, "text"),
		variant("append", "Insert text at the very end.", M{"text": text}, "text"),
		variant("replace_range", "Replace from `from` (inclusive) up to `to` (exclusive, kept), each occurring once.",
			M{"from": text, "to": text, "new_string": text}, "from", "to", "new_string"),
	}}
}

func findOnce(content, needle string) (int, error) {
	if needle == "" {
		return 0, app.BadRequest("Patch anchors can't be empty")
	}
	at := strings.Index(content, needle)
	if at < 0 {
		return 0, app.BadRequest("Patch text not found: %q", needle)
	}
	if strings.Contains(content[at+len(needle):], needle) {
		return 0, app.BadRequest("Patch text occurs more than once, add context: %q", needle)
	}
	return at, nil
}

// applyPatch applies every edit or none.
func applyPatch(content string, edits []TextEdit) (string, error) {
	text := content
	for _, edit := range edits {
		switch edit.Op {
		case "replace":
			if edit.ReplaceAll {
				if edit.OldString == "" || !strings.Contains(text, edit.OldString) {
					return "", app.BadRequest("Patch text not found: %q", edit.OldString)
				}
				text = strings.ReplaceAll(text, edit.OldString, edit.NewString)
				continue
			}
			at, err := findOnce(text, edit.OldString)
			if err != nil {
				return "", err
			}
			text = text[:at] + edit.NewString + text[at+len(edit.OldString):]
		case "insert_before":
			at, err := findOnce(text, edit.Anchor)
			if err != nil {
				return "", err
			}
			text = text[:at] + edit.Text + text[at:]
		case "insert_after":
			at, err := findOnce(text, edit.Anchor)
			if err != nil {
				return "", err
			}
			at += len(edit.Anchor)
			text = text[:at] + edit.Text + text[at:]
		case "prepend":
			text = edit.Text + text
		case "append":
			text += edit.Text
		case "replace_range":
			start, err := findOnce(text, edit.From)
			if err != nil {
				return "", err
			}
			end, err := findOnce(text[start:], edit.To)
			if err != nil {
				return "", err
			}
			text = text[:start] + edit.NewString + text[start+end:]
		default:
			return "", app.BadRequest("Unknown patch op %q; use replace, insert_before, insert_after, prepend, append or replace_range", edit.Op)
		}
	}
	return text, nil
}

// newContent is the new content from either a full replacement or a patch of the current one;
// nil when neither was given.
func newContent(current string, replacement *string, patch []TextEdit) (*string, error) {
	switch {
	case replacement != nil && patch != nil:
		return nil, app.BadRequest("Pass either the full content or a patch, not both")
	case replacement != nil:
		return replacement, nil
	case patch != nil:
		text, err := applyPatch(current, patch)
		if err != nil {
			return nil, err
		}
		return &text, nil
	}
	return nil, nil
}

func checkColor(color string) (string, error) {
	hex := strings.TrimPrefix(strings.TrimSpace(color), "#")
	valid := len(hex) == 6 || len(hex) == 3
	for _, c := range hex {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			valid = false
		}
	}
	if !valid {
		return "", app.BadRequest("%q isn't a hex color like #5e6ad2", color)
	}
	return "#" + strings.ToLower(hex), nil
}

// pickColor is a color for things created without one.
func pickColor(seed string) string {
	colors := []string{"#5e6ad2", "#26b5ce", "#0f783c", "#4cb782", "#f2c94c", "#f2994a", "#eb5757", "#bb87fc",
		"#95a2b3", "#ef4b9f", "#6d8a9d", "#c4a46b"}
	var index uint
	for _, b := range []byte(seed) {
		index = index*31 + uint(b)
	}
	return colors[index%uint(len(colors))]
}

func nonEmpty(value, what string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", app.BadRequest("%s can't be empty", what)
	}
	return value, nil
}

var priorityNames = []string{"No priority", "Urgent", "High", "Medium", "Low"}

func checkPriority(priority int) (int, error) {
	if priority < 0 || priority > 4 {
		return 0, app.BadRequest("priority is 0 (none), 1 (urgent), 2 (high), 3 (medium) or 4 (low)")
	}
	return priority, nil
}

// orEmpty is the string, or "" for nil.
func orEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
