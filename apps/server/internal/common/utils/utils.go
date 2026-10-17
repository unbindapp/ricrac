// Package utils holds small helpers for values agents pass: dates, durations, colors, names,
// slugs and identifiers.
package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"ricrac/internal/common/errdefs"
)

// SlugID is a random 12-character id for URLs (projects, documents, views, ...).
func SlugID() string {
	const alphabet = "0123456789abcdef"
	id := make([]byte, 12)
	for i := range id {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		id[i] = alphabet[n.Int64()]
	}
	return string(id)
}

// ParseDate takes YYYY-MM-DD, or a time whose day it keeps.
func ParseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if date, err := time.Parse(time.DateOnly, value); err == nil {
		return date, nil
	}
	if at, err := ParseDatetime(value); err == nil {
		return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Time{}, errdefs.BadRequest("%q isn't a date; use YYYY-MM-DD", value)
}

// ParseDatetime takes an ISO 8601 date or time, or a duration relative to now such as -P1D, P2W
// or -PT3H.
func ParseDatetime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if at, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return at.UTC(), nil
	}
	if date, err := time.Parse(time.DateOnly, value); err == nil {
		return date, nil
	}
	if duration, ok := ParseDuration(value); ok {
		return time.Now().UTC().Add(duration), nil
	}
	return time.Time{}, errdefs.BadRequest(
		"%q isn't a time; use ISO 8601 (2026-01-31 or 2026-01-31T12:00:00Z) or a duration like -P1D", value)
}

func ParseDuration(value string) (time.Duration, bool) {
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

func CheckColor(color string) (string, error) {
	hex := strings.TrimPrefix(strings.TrimSpace(color), "#")
	valid := len(hex) == 6 || len(hex) == 3
	for _, c := range hex {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			valid = false
		}
	}
	if !valid {
		return "", errdefs.BadRequest("%q isn't a hex color like #5e6ad2", color)
	}
	return "#" + strings.ToLower(hex), nil
}

// PickColor is a color for things created without one.
func PickColor(seed string) string {
	colors := []string{"#5e6ad2", "#26b5ce", "#0f783c", "#4cb782", "#f2c94c", "#f2994a", "#eb5757", "#bb87fc",
		"#95a2b3", "#ef4b9f", "#6d8a9d", "#c4a46b"}
	var index uint
	for _, b := range []byte(seed) {
		index = index*31 + uint(b)
	}
	return colors[index%uint(len(colors))]
}

func NonEmpty(value, what string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errdefs.BadRequest("%s can't be empty", what)
	}
	return value, nil
}

var PriorityNames = []string{"No priority", "Urgent", "High", "Medium", "Low"}

func CheckPriority(priority int) (int, error) {
	if priority < 0 || priority > 4 {
		return 0, errdefs.BadRequest("priority is 0 (none), 1 (urgent), 2 (high), 3 (medium) or 4 (low)")
	}
	return priority, nil
}

// OrEmpty is the string, or "" for nil.
func OrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func AsUUID(value string) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	return id, err == nil
}

// LastSegment is the last path segment of a link, or the value itself.
func LastSegment(value string) string {
	value = strings.TrimSpace(value)
	if !strings.Contains(value, "://") {
		return value
	}
	path := value
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimRight(path, "/")
	return path[strings.LastIndex(path, "/")+1:]
}

// SlugTail is the id slugs end in: my-project-0a1b2c3d4e5f.
func SlugTail(value string) string {
	segment := LastSegment(value)
	i := strings.LastIndex(segment, "-")
	if i < 0 {
		return segment
	}
	tail := segment[i+1:]
	if len(tail) == 12 && IsHex(tail) {
		return tail
	}
	return segment
}

// NameSlug is the words part of a link slug (launch-v2 for "Launch v2"), as the links are made.
func NameSlug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
			continue
		}
		dash = true
	}
	slug := b.String()
	if len(slug) > 60 {
		slug = slug[:60]
	}
	return strings.Trim(slug, "-")
}

// MatchesName is whether value names the thing: its name in any case, or a link slug of it
// without the slug id (/acme/project/launch-v2).
func MatchesName(name, value string) bool {
	if strings.EqualFold(name, value) {
		return true
	}
	slug := NameSlug(name)
	return slug != "" && slug == strings.ToLower(LastSegment(value))
}

func IsHex(value string) bool {
	for _, c := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

func Options(names []string) string {
	if len(names) == 0 {
		return "There are none."
	}
	shown := names[:min(len(names), 30)]
	more := ""
	if len(names) > len(shown) {
		more = fmt.Sprintf(" and %d more", len(names)-len(shown))
	}
	return fmt.Sprintf("Options: %s%s.", strings.Join(shown, ", "), more)
}

func AppendUnique(ids []uuid.UUID, id uuid.UUID) []uuid.UUID {
	if slices.Contains(ids, id) {
		return ids
	}
	return append(ids, id)
}

// ParseIdentifier splits ENG-123 into the team key and number.
func ParseIdentifier(value string) (string, int, bool) {
	value = strings.TrimSpace(value)
	i := strings.LastIndex(value, "-")
	if i < 0 {
		return "", 0, false
	}
	key, digits := value[:i], value[i+1:]
	number, err := strconv.Atoi(digits)
	if err != nil || digits == "" || strings.ContainsAny(digits, "+-") {
		return "", 0, false
	}
	if key == "" || len(key) > 10 {
		return "", 0, false
	}
	for _, c := range key {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			return "", 0, false
		}
	}
	return strings.ToUpper(key), number, true
}

func SplitGroup(value string) (group *string, name string) {
	g, n, found := strings.Cut(value, "/")
	if !found {
		return nil, value
	}
	g = strings.TrimSpace(g)
	return &g, strings.TrimSpace(n)
}

// MentionHandles is the @handles in Markdown, lowercased, without repeats.
func MentionHandles(text string) []string {
	var handles []string
	chars := []rune(text)
	for i, c := range chars {
		startsWord := i == 0 || !(unicode.IsLetter(chars[i-1]) || unicode.IsDigit(chars[i-1]) || chars[i-1] == '@')
		if c != '@' || !startsWord {
			continue
		}
		var handle []rune
		for _, h := range chars[i+1:] {
			if !(unicode.IsLetter(h) || unicode.IsDigit(h) || strings.ContainsRune("._-@+", h)) {
				break
			}
			handle = append(handle, h)
		}
		name := strings.ToLower(strings.TrimRight(string(handle), ".-"))
		if name != "" && !slices.Contains(handles, name) {
			handles = append(handles, name)
		}
	}
	return handles
}
