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

// IssueNumber is a GitHub issue mentioned as #123, owner/repo#123 or its link; Repo is empty
// for #123, the repository the mention is in.
type IssueNumber struct {
	Repo   string
	Number int
	Closes bool
}

const (
	repoName    = `[a-z0-9][a-z0-9-]{0,38}/[a-z0-9._-]{1,100}`
	issueNumber = `(?:https?://[^\s/]+/(` + repoName + `)/issues/([0-9]{1,9})|(` + repoName + `)?#([0-9]{1,9}))`
)

var (
	textIssueNumber   = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_&/])` + issueNumber + `\b`)
	closingIssueTexts = regexp.MustCompile(`(?i)\b(?:fix(?:es|ed)?|close[sd]?|resolve[sd]?)\b:?((?:(?:[ \t]*,[ \t]*|[ \t]+and[ \t]+|[ \t]+)` + issueNumber + `)+)`)
)

var exactIssue = regexp.MustCompile(`(?i)^(?:https?://[^\s/]+/(` + repoName + `)/issues/([0-9]{1,9})(?:[/?#]\S*)?|(` + repoName + `)#([0-9]{1,9}))$`)

// ParseIssue reads a GitHub issue given as owner/repo#123 or its link.
func ParseIssue(value string) (string, int, bool) {
	match := exactIssue.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return "", 0, false
	}
	repo, digits := match[1], match[2]
	if digits == "" {
		repo, digits = match[3], match[4]
	}
	number, err := strconv.Atoi(digits)
	return strings.ToLower(repo), number, err == nil && number > 0
}

// IssueNumbers is the GitHub issues texts mention, in the order they first appear.
func IssueNumbers(texts ...string) []IssueNumber {
	var found []IssueNumber
	add := func(match []string, closes bool) {
		repo, digits := match[1], match[2]
		if digits == "" {
			repo, digits = match[3], match[4]
		}
		number, err := strconv.Atoi(digits)
		if err != nil || number <= 0 {
			return
		}
		ref := IssueNumber{Repo: strings.ToLower(repo), Number: number, Closes: closes}
		for i := range found {
			if found[i].Repo == ref.Repo && found[i].Number == ref.Number {
				found[i].Closes = found[i].Closes || closes
				return
			}
		}
		if len(found) < MaxReferences {
			found = append(found, ref)
		}
	}
	for _, text := range texts {
		for _, phrase := range closingIssueTexts.FindAllStringSubmatch(text, -1) {
			for _, match := range textIssueNumber.FindAllStringSubmatch(phrase[1], -1) {
				add(match, true)
			}
		}
		for _, match := range textIssueNumber.FindAllStringSubmatch(text, -1) {
			add(match, false)
		}
	}
	return found
}
