package auth

import (
	"strings"
	"unicode"
)

// DisplayName is a short handle for @mentions: the first name, lowercased, or the email's local
// part.
func DisplayName(name, email string) string {
	var first string
	if words := strings.Fields(name); len(words) > 0 {
		first = strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return r
			}
			return -1
		}, words[0])
	}
	if first != "" {
		return strings.ToLower(first)
	}
	local, _, _ := strings.Cut(email, "@")
	if local == "" {
		return "user"
	}
	return strings.ToLower(local)
}
