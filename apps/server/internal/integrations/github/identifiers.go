// Package github talks to GitHub as a GitHub App (app JWT, installation tokens, the REST API,
// webhook signatures and payloads) and finds Ricrac identifiers like ENG-123 in branches, pull
// requests and commit messages.
package github

import (
	"regexp"
	"strconv"
	"strings"
)

// MaxReferences caps how many issues one branch, pull request or commit links.
const MaxReferences = 50

// Reference is an identifier mentioned on GitHub, and whether a closing phrase (fixes, closes,
// resolves) named it.
type Reference struct {
	Key    string
	Number int
	Closes bool
}

func (r Reference) Identifier() string {
	return r.Key + "-" + strconv.Itoa(r.Number)
}

const identifier = `([a-z][a-z0-9]{0,6})-([0-9]{1,9})`

var (
	branchIdentifier  = regexp.MustCompile(`(?i)(?:^|[/_-])` + identifier + `(?:$|[^a-z0-9])`)
	textIdentifier    = regexp.MustCompile(`(?i)\b` + identifier + `\b`)
	closingPhrase     = regexp.MustCompile(`(?i)\b(?:fix(?:es|ed)?|close[sd]?|resolve[sd]?)\b:?((?:(?:[ \t]*,[ \t]*|[ \t]+and[ \t]+|[ \t]+)\[?[a-z][a-z0-9]{0,6}-[0-9]{1,9}\]?)+)`)
	closingIdentifier = regexp.MustCompile(`(?i)` + identifier)
)

// References is the identifiers in a branch name and texts (titles, bodies, commit messages),
// in the order they first appear, the branch's first.
func References(branch string, texts ...string) []Reference {
	var found []Reference
	add := func(match []string, closes bool) {
		number, err := strconv.Atoi(match[2])
		if err != nil || number <= 0 {
			return
		}
		ref := Reference{Key: strings.ToUpper(match[1]), Number: number, Closes: closes}
		for i := range found {
			if found[i].Key == ref.Key && found[i].Number == ref.Number {
				found[i].Closes = found[i].Closes || closes
				return
			}
		}
		if len(found) < MaxReferences {
			found = append(found, ref)
		}
	}
	for _, match := range findOverlapping(branchIdentifier, branch) {
		add(match, false)
	}
	for _, text := range texts {
		for _, phrase := range closingPhrase.FindAllStringSubmatch(text, -1) {
			for _, match := range closingIdentifier.FindAllStringSubmatch(phrase[1], -1) {
				add(match, true)
			}
		}
		for _, match := range textIdentifier.FindAllStringSubmatch(text, -1) {
			add(match, false)
		}
	}
	return found
}

// findOverlapping finds every match, letting one start at the separator the previous ended on.
func findOverlapping(pattern *regexp.Regexp, text string) [][]string {
	var matches [][]string
	for start := 0; start < len(text); {
		loc := pattern.FindStringSubmatchIndex(text[start:])
		if loc == nil {
			break
		}
		matches = append(matches, []string{text[start+loc[0] : start+loc[1]], text[start+loc[2] : start+loc[3]], text[start+loc[4] : start+loc[5]]})
		start += loc[5]
	}
	return matches
}
